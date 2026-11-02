package desktop

import (
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"hoplink/internal/desktop/app/route"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// Every hop except the last is an opaque UDP relay. Route credentials and the
// existing pinned certificate belong only on the exit, preserving client URLs.
func routeNodeArgs(cfg AppConfig, order []Server, index int, installDir string) []string {
	level := cfg.LogLevel
	if level == "" {
		level = "warn"
	}
	transport := cfg.RelayTransport
	if transport == "" {
		transport = "udp"
	}
	args := []string{installDir + "/HopLink", "-hy2-listen", ":" + strconv.Itoa(cfg.Port), "-log-level", level}
	if index < len(order)-1 {
		args = append(args, "-mode", "udp-relay")
		switch transport {
		case "ssh":
			next := order[index+1]
			args = append(args, "-relay-transport", "ssh",
				"-relay-ssh-next", relayTransportHostPort(next.Host, cfg.Port),
				"-relay-ssh-client-key-file", installDir+"/relay_transport_client.key",
				"-relay-ssh-host-pub-file", installDir+"/relay_transport_next_host.pub",
			)
		case "ws":
			next := order[index+1]
			args = append(args, "-relay-transport", "ws",
				"-relay-ws-next", relayTransportHostPort(next.Host, cfg.Port),
				"-relay-ws-trust-cert", installDir+"/hy2-server.crt",
			)
		default:
			args = append(args, "-f", net.JoinHostPort(order[index+1].Host, strconv.Itoa(cfg.Port)))
		}
		if index > 0 && transport == "ssh" {
			args = append(args,
				"-relay-ssh-listen", ":"+strconv.Itoa(relayTransportPort(cfg.Port)),
				"-relay-ssh-host-key-file", installDir+"/relay_ssh_host.key",
				"-relay-ssh-authorized-keys-file", installDir+"/relay_transport_auth",
			)
		}
		if index > 0 && transport == "ws" {
			args = append(args,
				"-relay-ws-listen", ":"+strconv.Itoa(relayTransportPort(cfg.Port)),
				"-relay-ws-cert", installDir+"/hy2-server.crt",
				"-relay-ws-cert-key", installDir+"/hy2-server.key",
			)
		}
		if cfg.ObfsType != "" {
			args = append(args, "-relay-obfs", cfg.ObfsType)
		}
		return args
	}
	args = append(args, "-mode", "hy2-forward", "-key-file", installDir+"/shared.key", "-hy2-users-file", installDir+"/users.txt", "-hy2-sni", cfg.SNI, "-hy2-cert", installDir+"/hy2-server.crt", "-hy2-cert-key", installDir+"/hy2-server.key")
	if index > 0 && transport == "ssh" {
		args = append(args,
			"-relay-ssh-listen", ":"+strconv.Itoa(relayTransportPort(cfg.Port)),
			"-relay-ssh-host-key-file", installDir+"/relay_ssh_host.key",
			"-relay-ssh-authorized-keys-file", installDir+"/relay_transport_auth",
		)
	}
	if index > 0 && transport == "ws" {
		args = append(args,
			"-relay-ws-listen", ":"+strconv.Itoa(relayTransportPort(cfg.Port)),
			"-relay-ws-cert", installDir+"/hy2-server.crt",
			"-relay-ws-cert-key", installDir+"/hy2-server.key",
		)
	}
	if cfg.ObfsType != "" {
		args = append(args, "-hy2-obfs", cfg.ObfsType, "-hy2-obfs-key-file", installDir+"/obfs.key")
	}
	return args
}

func routeUserLinks(cfg AppConfig, entry Server, users []User) (map[string]string, error) {
	links, _, err := routeUserLinkVariants(cfg, entry, users)
	return links, err
}

func routeUserLinkVariants(cfg AppConfig, entry Server, users []User) (map[string]string, map[string]string, error) {
	cert, err := tls.X509KeyPair([]byte(cfg.Certificate), []byte(cfg.CertificateKey))
	if err != nil {
		return nil, nil, fmt.Errorf("сертификат маршрута: %w", err)
	}
	if len(cert.Certificate) == 0 {
		return nil, nil, fmt.Errorf("сертификат маршрута пустой")
	}
	pin := sha256.Sum256(cert.Certificate[0])
	dnsHost := strings.Trim(strings.TrimSpace(cfg.EntryDomain), "[]")
	ipHost := strings.Trim(strings.TrimSpace(entry.Host), "[]")
	links := make(map[string]string, len(users))
	ipLinks := make(map[string]string, len(users))
	for _, user := range users {
		if !user.isActive() {
			continue
		}
		// Trust the route certificate by its pin, including self-signed certificates.
		query := url.Values{"insecure": {"1"}, "pinSHA256": {hex.EncodeToString(pin[:])}}
		if cfg.SNI != "" {
			query.Set("sni", cfg.SNI)
		}
		if cfg.ObfsType != "" {
			query.Set("obfs", cfg.ObfsType)
			query.Set("obfs-password", cfg.ObfsPassword)
		}
		makeLink := func(host string) string {
			u := &url.URL{Scheme: "hysteria2", User: url.User(user.Password), Host: net.JoinHostPort(host, strconv.Itoa(cfg.Port)), Path: "/", RawQuery: query.Encode(), Fragment: "HopLink-" + user.Name}
			return u.String()
		}
		ipLinks[user.Name] = makeLink(ipHost)
		if dnsHost != "" {
			links[user.Name] = makeLink(dnsHost)
		}
	}
	return links, ipLinks, nil
}

func (a *App) GetUserLinkVariants(userID string) (map[string]string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	var user *User
	for i := range a.config.Users {
		if a.config.Users[i].ID == userID {
			user = &a.config.Users[i]
			break
		}
	}
	if user == nil || !user.isActive() {
		return nil, fmt.Errorf("пользователь не найден или отключён")
	}
	order, err := route.OrderedChain(mapRouteServers(a.config.MapServers), mapRouteEdges(a.config.Edges))
	if err != nil || len(order) == 0 {
		return nil, fmt.Errorf("сначала сохраните карту с входным сервером")
	}
	cfg := a.config
	if cfg.Certificate == "" || cfg.CertificateKey == "" {
		return nil, fmt.Errorf("сначала задайте сертификат в настройках маршрута")
	}
	entry := Server{}
	for _, server := range a.config.MapServers {
		if server.ID == order[0].ID {
			entry = server
			break
		}
	}
	if entry.ID == "" {
		return nil, fmt.Errorf("не найден входной сервер маршрута")
	}
	dnsLinks, ipLinks, err := routeUserLinkVariants(cfg, entry, []User{*user})
	if err != nil {
		return nil, err
	}
	return map[string]string{"dns": dnsLinks[user.Name], "ip": ipLinks[user.Name]}, nil
}
