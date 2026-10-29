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
	args := []string{installDir + "/HopLink", "-hy2-listen", ":" + strconv.Itoa(cfg.Port), "-log-level", "warn"}
	if index < len(order)-1 {
		return append(args, "-mode", "udp-relay", "-f", net.JoinHostPort(order[index+1].Host, strconv.Itoa(cfg.Port)))
	}
	return append(args, "-mode", "hy2-forward", "-key-file", installDir+"/shared.key", "-hy2-users-file", installDir+"/users.txt", "-hy2-sni", cfg.SNI, "-hy2-cert", installDir+"/hy2-server.crt", "-hy2-cert-key", installDir+"/hy2-server.key")
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
	sni := cfg.SNI
	if sni == "" {
		sni = "www.cloudflare.com"
	}
	links := make(map[string]string, len(users))
	for _, user := range users {
		if !user.isActive() {
			continue
		}
		u := &url.URL{Scheme: "hysteria2", User: url.User(user.Password), Host: net.JoinHostPort(host, strconv.Itoa(cfg.Port)), Path: "/", RawQuery: url.Values{"pinSHA256": {hex.EncodeToString(pin[:])}, "sni": {sni}}.Encode(), Fragment: "HopLink-" + user.Name}
		links[user.Name] = u.String()
	}
	return links, nil
}
