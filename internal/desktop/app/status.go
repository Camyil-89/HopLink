package desktop

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"hoplink/internal/desktop/app/remote"
)

type ServerServiceStatus struct {
	ServerID       string   `json:"serverId"`
	Running        bool     `json:"running"`
	Version        string   `json:"version,omitempty"`
	LatestVersion  string   `json:"latestVersion,omitempty"`
	VersionCurrent *bool    `json:"versionCurrent,omitempty"`
	Error          string   `json:"error,omitempty"`
	NextServerID   string   `json:"nextServerId,omitempty"`
	NextLatencyMs  *float64 `json:"nextLatencyMs,omitempty"`
	NextPingError  string   `json:"nextPingError,omitempty"`
}

type RouteServiceStatus struct {
	RouteID        string                `json:"routeId"`
	Running        int                   `json:"running"`
	Total          int                   `json:"total"`
	Servers        []ServerServiceStatus `json:"servers"`
	EntryLatencyMs *float64              `json:"entryLatencyMs,omitempty"`
	EntryPingError string                `json:"entryPingError,omitempty"`
}

// CheckRouteServices is called only by the UI's explicit Check button. It does
// not save the result or change the route's deployment metadata.
func (a *App) CheckRouteServices() ([]RouteServiceStatus, error) {
	a.mu.Lock()
	if a.vaultLocked {
		a.mu.Unlock()
		return nil, fmt.Errorf("разблокируйте настройки перед проверкой сервисов")
	}
	routes := append([]Route(nil), a.config.Routes...)
	knownServers := make(map[string]Server, len(a.config.Servers))
	for _, server := range a.config.Servers {
		knownServers[server.ID] = server
	}
	a.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	releases, releaseErr := fetchReleases(ctx)
	cancel()
	latestVersion := ""
	if releaseErr == nil {
		_, latestVersion, releaseErr = latestComponent(releases, "hoplink")
	}
	if releaseErr != nil {
		a.log("Не удалось получить последнюю версию HopLink: " + releaseErr.Error())
	}

	statuses := make([]RouteServiceStatus, len(routes))
	var workers sync.WaitGroup
	limit := make(chan struct{}, 4)
	for routeIndex, route := range routes {
		statuses[routeIndex] = RouteServiceStatus{
			RouteID: route.ID,
			Total:   len(route.MapServers),
			Servers: make([]ServerServiceStatus, len(route.MapServers)),
		}
		_, unit := indexedRouteLayout(route.Index)
		installDir, _ := routeRemoteLayout(route.ID, routes)
		entryHost := ""
		if chain, err := orderedChain(route.MapServers, route.Edges); err == nil && len(chain) > 0 {
			entryHost = chain[0].Host
		}
		if entryHost != "" {
			latency, pingErr := pingHost(context.Background(), entryHost)
			if pingErr != nil {
				statuses[routeIndex].EntryPingError = pingErr.Error()
			} else {
				statuses[routeIndex].EntryLatencyMs = &latency
			}
		}
		nextHosts := make(map[string]Server, len(route.Edges))
		for _, edge := range route.Edges {
			if target, ok := knownServers[edge.Target]; ok {
				nextHosts[edge.Source] = target
			}
		}
		units := []string{unit}
		for _, previous := range previousRouteLayouts(route.ID, routes) {
			units = append(units, previous.unit)
		}
		for serverIndex, mapped := range route.MapServers {
			workers.Add(1)
			limit <- struct{}{}
			go func(routeIndex, serverIndex int, mapped Server, units []string, binaryPath string, next Server) {
				defer workers.Done()
				defer func() { <-limit }()
				server, ok := knownServers[mapped.ID]
				if !ok {
					statuses[routeIndex].Servers[serverIndex] = ServerServiceStatus{ServerID: mapped.ID, Error: "Сервер удалён из библиотеки"}
					return
				}
				statuses[routeIndex].Servers[serverIndex] = a.checkServerService(server, units, binaryPath, latestVersion, next)
			}(routeIndex, serverIndex, mapped, units, installDir+"/HopLink", nextHosts[mapped.ID])
		}
	}
	workers.Wait()
	for i := range statuses {
		for _, server := range statuses[i].Servers {
			if server.Running {
				statuses[i].Running++
			}
		}
	}
	return statuses, nil
}

func (a *App) checkServerService(server Server, units []string, binaryPath, latestVersion string, next Server) ServerServiceStatus {
	status := ServerServiceStatus{ServerID: server.ID, LatestVersion: latestVersion}
	if next.ID != "" {
		status.NextServerID = next.ID
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, err := a.sshConnectContext(ctx, server)
	if err != nil {
		status.Error = fmt.Sprintf("SSH: %v", err)
		return status
	}
	defer client.Close()
	if next.ID != "" {
		output, pingErr := runServerContext(ctx, client, server, "ping -n -c 1 -W 2 -- "+remote.ShellQuote(next.Host), nil)
		if pingErr != nil {
			status.NextPingError = "ICMP недоступен"
		} else if latency, ok := parsePingLatency(output); ok {
			status.NextLatencyMs = &latency
		} else {
			status.NextPingError = "нет ответа на ICMP"
		}
	}
	commands := make([]string, 0, len(units))
	for _, unit := range units {
		commands = append(commands, "systemctl is-active --quiet "+remote.ShellQuote(unit))
	}
	command := strings.Join(commands, " || ")
	_, serviceErr := runServerContext(ctx, client, server, command, nil)
	if serviceErr != nil {
		if ctx.Err() != nil {
			status.Error = "Время проверки истекло"
		} else {
			status.Error = "Сервис не запущен"
		}
	} else {
		status.Running = true
	}
	output, versionErr := runServerContext(ctx, client, server, remote.ShellQuote(binaryPath)+" -version 2>/dev/null", nil)
	if versionErr == nil {
		fields := strings.Fields(output)
		if len(fields) >= 2 && fields[0] == "hoplink" {
			status.Version = fields[1]
			if latestVersion != "" {
				_, installedOK := parseVersion(status.Version)
				_, latestOK := parseVersion(latestVersion)
				if !installedOK || !latestOK {
					return status
				}
				current := !newerVersion(latestVersion, status.Version)
				status.VersionCurrent = &current
			}
		}
	}
	return status
}

var pingTimePattern = regexp.MustCompile(`(?i)time[=<]\s*([0-9]+(?:\.[0-9]+)?)\s*ms`)

func parsePingLatency(output string) (float64, bool) {
	match := pingTimePattern.FindStringSubmatch(output)
	if len(match) != 2 {
		return 0, false
	}
	var latency float64
	if _, err := fmt.Sscanf(match[1], "%f", &latency); err != nil {
		return 0, false
	}
	return latency, true
}

func pingHost(ctx context.Context, host string) (float64, error) {
	pingCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	var command *exec.Cmd
	if runtime.GOOS == "windows" {
		command = exec.CommandContext(pingCtx, "ping.exe", "-n", "1", "-w", "2500", host)
	} else {
		command = exec.CommandContext(pingCtx, "ping", "-n", "-c", "1", "-W", "2", host)
	}
	hideCommandWindow(command)
	output, err := command.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("ICMP недоступен")
	}
	if latency, ok := parsePingLatency(string(output)); ok {
		return latency, nil
	}
	return 0, fmt.Errorf("нет ответа на ICMP")
}
