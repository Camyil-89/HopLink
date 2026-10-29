package desktop

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/crypto/ssh"
	"hoplink/internal/desktop/app/remote"
	"io"
	"net"
	"os"
	"regexp"
	"strings"
	"time"
)

func (a *App) log(s string) { runtime.EventsEmit(a.ctx, "deploy-log", s) }
func (a *App) Apply() error {
	a.mu.Lock()
	if a.applyCancel != nil {
		a.mu.Unlock()
		return fmt.Errorf("synchronization already running")
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.applyCancel = cancel
	a.mu.Unlock()
	defer func() {
		cancel()
		a.mu.Lock()
		a.applyCancel = nil
		a.mu.Unlock()
	}()
	err := a.apply(ctx)
	if ctx.Err() != nil {
		a.log("Synchronization cancelled by user.")
		return fmt.Errorf("synchronization cancelled")
	}
	return err
}

func (a *App) CancelApply() bool {
	a.mu.Lock()
	cancel := a.applyCancel
	a.mu.Unlock()
	if cancel == nil {
		return false
	}
	cancel()
	return true
}

func (a *App) apply(ctx context.Context) error {
	a.mu.Lock()
	servers := append([]Server(nil), a.config.MapServers...)
	edges := append([]Edge(nil), a.config.Edges...)
	users := append([]User(nil), a.config.Users...)
	cfg := a.config
	a.mu.Unlock()
	installDir, unitName := routeRemoteLayout(cfg.ActiveRouteID, cfg.Routes)
	oldLayouts := previousRouteLayouts(cfg.ActiveRouteID, cfg.Routes)
	if e := validateMap(servers, edges); e != nil {
		return e
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	order, e := orderedChain(servers, edges)
	if e != nil {
		return e
	}
	for _, other := range cfg.Routes {
		if other.ID == cfg.ActiveRouteID || other.Port != cfg.Port || len(other.DeployedServers) == 0 {
			continue
		}
		for _, activeServer := range order {
			for _, otherServer := range other.DeployedServers {
				if strings.EqualFold(activeServer.Host, otherServer.Host) && activeServer.SSHPort == otherServer.SSHPort {
					return fmt.Errorf("сервер %s уже использует UDP-порт %d в маршруте «%s»; задайте другому маршруту отдельный порт", activeServer.Host, cfg.Port, other.Name)
				}
			}
		}
	}
	if len(order) < 2 {
		for _, old := range cfg.DeployedServers {
			a.log("Удаляю HopLink: на карте осталось меньше двух серверов")
			if removeErr := a.removeOneAtContext(ctx, old, cfg.Port, installDir, unitName); removeErr != nil {
				return removeErr
			}
			if removeErr := a.removePreviousLayoutsContext(ctx, old, cfg.Port, oldLayouts); removeErr != nil {
				return removeErr
			}
		}
		a.mu.Lock()
		a.config.DeployedServers = []Server{}
		a.clearUserLinksLocked()
		a.clearRouteMigrationLayoutsLocked(cfg.ActiveRouteID)
		e = a.saveLocked()
		a.mu.Unlock()
		if e != nil {
			return e
		}
		return fmt.Errorf("на карте должно быть минимум два сервера. Установленная цепочка удалена")
	}
	activeUsers := make([]User, 0, len(users))
	for _, user := range users {
		if user.isActive() {
			activeUsers = append(activeUsers, user)
		}
	}
	startedAt := time.Now().Unix()
	if cfg.ChainKey == "" || cfg.Certificate == "" {
		if e = a.ensureSecrets(); e != nil {
			return e
		}
		a.mu.Lock()
		cfg = a.config
		a.mu.Unlock()
	}
	a.log("Проверяю SSH-доступ ко всем серверам…")
	clients := make([]*ssh.Client, len(order))
	archs := make([]string, len(order))
	defer func() {
		for _, c := range clients {
			if c != nil {
				_ = c.Close()
			}
		}
	}()
	for i, s := range order {
		if err := ctx.Err(); err != nil {
			return err
		}
		c, e := a.sshConnectContext(ctx, s)
		if e != nil {
			return e
		}
		clients[i] = c
		identity, e := runRemoteContext(ctx, c, "id -un && uname -m", nil)
		if e != nil {
			return fmt.Errorf("%s: SSH preflight: %w", s.Host, e)
		}
		identityParts := strings.Fields(identity)
		if len(identityParts) < 2 {
			return fmt.Errorf("%s: could not determine remote user and architecture", s.Host)
		}
		remoteUser, machine := identityParts[0], identityParts[1]
		switch machine {
		case "x86_64":
			archs[i] = "amd64"
		case "aarch64", "arm64":
			archs[i] = "arm64"
		default:
			return fmt.Errorf("%s: unsupported architecture %s", s.Host, machine)
		}
		if remoteUser != "root" {
			if _, sudoErr := runRemoteContext(ctx, c, "sudo -n true", nil); sudoErr != nil {
				if _, e = runServerContext(ctx, c, s, "true", nil); e != nil {
					return fmt.Errorf("%s: sudo preflight failed: %w", s.Host, e)
				}
			}
		}
		if _, e = runRemoteContext(ctx, c, "command -v systemctl >/dev/null && command -v base64 >/dev/null", nil); e != nil {
			return fmt.Errorf("%s: systemctl/base64 preflight failed: %w", s.Host, e)
		}
		a.log(fmt.Sprintf("SSH ready: %s (%s, %s)", s.Host, remoteUser, archs[i]))
	}
	bins := map[string]string{}
	for _, arch := range archs {
		if bins[arch] == "" {
			if cfg.InstallSource == "github" {
				a.log("Скачиваю последний HopLink для Linux/" + arch + " с GitHub…")
			} else {
				a.log("Собираю HopLink для Linux/" + arch + " из исходников…")
			}
			p, e := a.serverBinary(ctx, arch, cfg.InstallSource)
			if e != nil {
				return e
			}
			bins[arch] = p
		}
	}
	if cfg.InstallSource == "github" {
		defer func() {
			for _, path := range bins {
				_ = os.Remove(path)
			}
		}()
	}
	for _, old := range cfg.DeployedServers {
		if len(oldLayouts) == 0 {
			break
		}
		a.log("Переношу старый сервис маршрута на порядковый номер: " + old.Host)
		if cleanupErr := a.removePreviousLayoutsContext(ctx, old, cfg.Port, oldLayouts); cleanupErr != nil {
			return fmt.Errorf("миграция сервиса маршрута на %s: %w", old.Host, cleanupErr)
		}
	}
	userLines := ""
	for _, u := range activeUsers {
		userLines += u.Name + " " + u.Password + "\n"
	}
	a.log("\u041e\u0441\u0442\u0430\u043d\u0430\u0432\u043b\u0438\u0432\u0430\u044e \u0442\u0435\u043a\u0443\u0449\u0438\u0435 \u0441\u043b\u0443\u0436\u0431\u044b \u043d\u0430 \u0441\u0435\u0440\u0432\u0435\u0440\u0430\u0445...")
	// Stop all units first so no node forwards into a half-updated chain.
	for i, c := range clients {
		if err := ctx.Err(); err != nil {
			return err
		}
		_, e = runServerContext(ctx, c, order[i], "systemctl stop "+unitName+" 2>/dev/null || true", nil)
		ignoredStopStatus := false
		if e != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			var missingStatus *ssh.ExitMissingError
			var exitStatus *ssh.ExitError
			if errors.As(e, &exitStatus) {
				ignoredStopStatus = true
				a.log("Stop returned a remote exit status; continuing like deploy_chain.py check=False: " + order[i].Host)
			} else if errors.Is(e, io.EOF) || errors.As(e, &missingStatus) {
				a.log("\u041f\u043e\u0441\u043b\u0435 \u043e\u0441\u0442\u0430\u043d\u043e\u0432\u043a\u0438 HopLink SSH \u043e\u0442\u043a\u043b\u044e\u0447\u0438\u043b\u0441\u044f; \u0432\u043e\u0441\u0441\u0442\u0430\u043d\u0430\u0432\u043b\u0438\u0432\u0430\u044e \u0441\u043e\u0435\u0434\u0438\u043d\u0435\u043d\u0438\u0435: " + order[i].Host)
				_ = c.Close()
				next, reconnectErr := a.reconnectSSHContext(ctx, order[i])
				if reconnectErr != nil {
					return fmt.Errorf("\u041d\u0435 \u0443\u0434\u0430\u043b\u043e\u0441\u044c \u0432\u043e\u0441\u0441\u0442\u0430\u043d\u043e\u0432\u0438\u0442\u044c SSH \u043f\u043e\u0441\u043b\u0435 \u043e\u0441\u0442\u0430\u043d\u043e\u0432\u043a\u0438 HopLink \u043d\u0430 %s: %w", order[i].Host, reconnectErr)
				}
				clients[i] = next
			} else {
				return fmt.Errorf("stop %s: %w", order[i].Host, e)
			}
		}
		if ignoredStopStatus {
			a.log("\u041f\u0440\u043e\u0434\u043e\u043b\u0436\u0430\u044e \u043f\u043e\u0434\u0433\u043e\u0442\u043e\u0432\u043a\u0443 \u0441\u0435\u0440\u0432\u0435\u0440\u0430: " + order[i].Host)
		} else {
			a.log("\u0421\u043b\u0443\u0436\u0431\u0430 \u043e\u0441\u0442\u0430\u043d\u043e\u0432\u043b\u0435\u043d\u0430: " + order[i].Host)
		}
	}
	for i, s := range order {
		if err := ctx.Err(); err != nil {
			return err
		}
		a.log("\u0417\u0430\u0433\u0440\u0443\u0436\u0430\u044e \u0444\u0430\u0439\u043b\u044b \u043d\u0430 " + s.Host + "...")
		stage := "/tmp/hoplink-" + newID()
		if _, e = runServerContext(ctx, clients[i], s, "mkdir -m 700 "+remote.ShellQuote(stage), nil); e != nil {
			return e
		}
		binary, e := os.ReadFile(bins[archs[i]])
		if e != nil {
			return e
		}
		if _, e = runServerContext(ctx, clients[i], s, "base64 -d > "+remote.ShellQuote(stage+"/HopLink"), []byte(base64.StdEncoding.EncodeToString(binary))); e != nil {
			return fmt.Errorf("загрузка бинарника на %s: %w", s.Host, e)
		}
		if _, e = runServerContext(ctx, clients[i], s, "printf %s "+remote.ShellQuote(base64.StdEncoding.EncodeToString([]byte(cfg.ChainKey+"\n")))+" | base64 -d > "+remote.ShellQuote(stage+"/shared.key"), nil); e != nil {
			return e
		}
		role := "hy2-enter"
		if i == len(order)-1 {
			role = "hy2-forward"
		}
		args := []string{installDir + "/HopLink", "-mode", role, "-hy2-listen", ":" + fmt.Sprint(cfg.Port), "-key-file", installDir + "/shared.key", "-log-level", "warn"}
		if i < len(order)-1 {
			args = append(args, "-f", net.JoinHostPort(order[i+1].Host, fmt.Sprint(cfg.Port)))
		}
		if i == 0 {
			publicHost := cfg.EntryDomain
			if publicHost == "" {
				publicHost = s.Host
			}
			args = append(args, "-hy2-public-host", publicHost, "-hy2-users-file", installDir+"/users.txt", "-hy2-sni", cfg.SNI, "-hy2-cert", installDir+"/hy2-server.crt", "-hy2-cert-key", installDir+"/hy2-server.key")
		}
		q := make([]string, len(args))
		for j, x := range args {
			q[j] = remote.ShellQuote(x)
		}
		unit := "[Unit]\nDescription=HopLink Hysteria2 relay\nAfter=network-online.target\nWants=network-online.target\n[Service]\nType=simple\nUser=root\nWorkingDirectory=" + installDir + "\nExecStart=" + strings.Join(q, " ") + "\nRestart=on-failure\nRestartSec=3\nLimitNOFILE=65535\nUMask=0077\n[Install]\nWantedBy=multi-user.target\n"
		if i == 0 {
			if _, e = runServerContext(ctx, clients[i], s, "base64 -d > "+remote.ShellQuote(stage+"/users.txt"), []byte(base64.StdEncoding.EncodeToString([]byte(userLines)))); e != nil {
				return e
			}
			if _, e = runServerContext(ctx, clients[i], s, "base64 -d > "+remote.ShellQuote(stage+"/hy2-server.crt"), []byte(base64.StdEncoding.EncodeToString([]byte(cfg.Certificate)))); e != nil {
				return e
			}
			if _, e = runServerContext(ctx, clients[i], s, "base64 -d > "+remote.ShellQuote(stage+"/hy2-server.key"), []byte(base64.StdEncoding.EncodeToString([]byte(cfg.CertificateKey)))); e != nil {
				return e
			}
		}
		if _, e = runServerContext(ctx, clients[i], s, "base64 -d > "+remote.ShellQuote(stage+"/"+unitName), []byte(base64.StdEncoding.EncodeToString([]byte(unit)))); e != nil {
			return e
		}
		install := "set -eu; systemctl stop " + unitName + " 2>/dev/null || true; install -d -m 700 " + remote.ShellQuote(installDir) + "; install -m 755 " + remote.ShellQuote(stage+"/HopLink") + " " + remote.ShellQuote(installDir+"/HopLink") + "; install -m 600 " + remote.ShellQuote(stage+"/shared.key") + " " + remote.ShellQuote(installDir+"/shared.key") + "; install -m 644 " + remote.ShellQuote(stage+"/"+unitName) + " /etc/systemd/system/" + unitName + "; "
		if i == 0 {
			install += "install -m 600 " + remote.ShellQuote(stage+"/users.txt") + " " + remote.ShellQuote(installDir+"/users.txt") + "; install -m 644 " + remote.ShellQuote(stage+"/hy2-server.crt") + " " + remote.ShellQuote(installDir+"/hy2-server.crt") + "; install -m 600 " + remote.ShellQuote(stage+"/hy2-server.key") + " " + remote.ShellQuote(installDir+"/hy2-server.key") + "; "
		}
		install += "rm -rf -- " + remote.ShellQuote(stage) + "; systemctl daemon-reload; systemctl enable " + unitName
		install += fmt.Sprintf("; if command -v ufw >/dev/null 2>&1 && ufw status 2>/dev/null | grep -q '^Status: active'; then if ! ufw status | grep -F '%d/udp' | grep -q ALLOW; then ufw allow %d/udp comment 'HopLink'; printf 'ufw\\n' > %s; fi; elif command -v firewall-cmd >/dev/null 2>&1 && firewall-cmd --state >/dev/null 2>&1; then if ! firewall-cmd --query-port=%d/udp >/dev/null 2>&1; then firewall-cmd --permanent --add-port=%d/udp; firewall-cmd --reload; printf 'firewalld\\n' > %s; fi; fi", cfg.Port, cfg.Port, remote.ShellQuote(installDir+"/.firewall-managed"), cfg.Port, cfg.Port, remote.ShellQuote(installDir+"/.firewall-managed"))
		a.log("\u0423\u0441\u0442\u0430\u043d\u0430\u0432\u043b\u0438\u0432\u0430\u044e HopLink: " + s.Host + "...")
		if _, e = runServerContext(ctx, clients[i], s, install, nil); e != nil {
			return fmt.Errorf("установка %s: %w", s.Host, e)
		}
	}
	a.log("\u0417\u0430\u043f\u0443\u0441\u043a\u0430\u044e \u0446\u0435\u043f\u043e\u0447\u043a\u0443 \u0432 \u043e\u0431\u0440\u0430\u0442\u043d\u043e\u043c \u043f\u043e\u0440\u044f\u0434\u043a\u0435...")
	for i := len(order) - 1; i >= 0; i-- {
		if err := ctx.Err(); err != nil {
			return err
		}
		s := order[i]
		c := clients[i]
		if _, e = runServerContext(ctx, c, s, "systemctl restart "+unitName+" && sleep 2 && systemctl is-active --quiet "+unitName, nil); e != nil {
			out, _ := runServerContext(ctx, c, s, "systemctl status "+unitName+" --no-pager -l; journalctl -u "+unitName+" -n 40 --no-pager", nil)
			return fmt.Errorf("%s: сервис не запустился: %w\n%s", s.Host, e, out)
		}
		a.log("HopLink запущен: " + s.Host)
	}
	a.log("\u041f\u0440\u043e\u0432\u0435\u0440\u044f\u044e \u0437\u0430\u043f\u0443\u0441\u043a \u0446\u0435\u043f\u043e\u0447\u043a\u0438...")
	journal, journalErr := runServerContext(ctx, clients[0], order[0], fmt.Sprintf("journalctl -u %s --since '@%d' -o cat --no-pager", unitName, startedAt), nil)
	if journalErr == nil {
		links := map[string]string{}
		linkLine := regexp.MustCompile(`^([A-Za-z0-9_.-]+):\s+(hysteria2://\S+)\s*$`)
		for _, line := range strings.Split(journal, "\n") {
			if match := linkLine.FindStringSubmatch(strings.TrimSpace(line)); len(match) == 3 {
				links[match[1]] = match[2]
			}
		}
		a.mu.Lock()
		a.links = links
		a.mu.Unlock()
	}
	active := map[string]bool{}
	for _, s := range order {
		active[s.ID] = true
	}
	for _, old := range cfg.DeployedServers {
		if active[old.ID] {
			continue
		}
		a.log("Удаляю HopLink с исключённого из карты сервера " + old.Host + "…")
		if cleanupErr := a.removeOneAtContext(ctx, old, cfg.Port, installDir, unitName); cleanupErr != nil {
			return cleanupErr
		}
		a.log("HopLink удалён: " + old.Host)
	}
	a.mu.Lock()
	a.config.ChainKey = cfg.ChainKey
	a.config.Certificate = cfg.Certificate
	a.config.CertificateKey = cfg.CertificateKey
	a.config.DeployedServers = order
	a.clearRouteMigrationLayoutsLocked(cfg.ActiveRouteID)
	e = a.saveLocked()
	a.mu.Unlock()
	if e != nil {
		return e
	}
	a.log("Синхронизация цепочки завершена.")
	return nil
}
