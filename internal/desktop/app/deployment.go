package desktop

import (
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
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
	cert, err := tls.X509KeyPair([]byte(cfg.Certificate), []byte(cfg.CertificateKey))
	if err != nil {
		return nil, fmt.Errorf("сертификат маршрута: %w", err)
	}
	if len(cert.Certificate) == 0 {
		return nil, fmt.Errorf("сертификат маршрута пустой")
	}
	pin := sha256.Sum256(cert.Certificate[0])
	host := cfg.EntryDomain
	if host == "" {
		host = entry.Host
	}
	host = strings.Trim(strings.TrimSpace(host), "[]")
	links := make(map[string]string, len(users))
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
		u := &url.URL{Scheme: "hysteria2", User: url.User(user.Password), Host: net.JoinHostPort(host, strconv.Itoa(cfg.Port)), Path: "/", RawQuery: query.Encode(), Fragment: "HopLink-" + user.Name}
		links[user.Name] = u.String()
	}
	return links, nil
}
