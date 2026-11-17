package cascade

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"hoplink/internal/hoplink/hysteria"
	"hoplink/internal/hoplink/log"
	"hoplink/internal/hoplink/obfs"

	hyserver "github.com/apernet/hysteria/core/v2/server"
)

// Run starts a cascade Hysteria2 hop (entry, middle, or exit).
func Run(listen, role, password, sni, certPath, keyPath, certDir, usersFile, nextAddr, domainPatternsFile, sniPatternsFile, trustCert, obfsMode, obfsKeyFile string) error {
	role = strings.ToLower(strings.TrimSpace(role))
	switch role {
	case "entry", "hop", "exit":
	default:
		return fmt.Errorf("неподдерживаемая роль каскада %q; доступны entry, hop и exit", role)
	}
	if password == "" {
		return fmt.Errorf("укажите общий непустой -key для Hysteria2")
	}
	certs, _, _, err := hysteria.LoadCertificatePair(certPath, keyPath, certDir, sni)
	if err != nil {
		return fmt.Errorf("TLS certificate: %w", err)
	}
	var users []hysteria.User
	if role == "entry" {
		users, err = hysteria.LoadUsersFile(usersFile)
		if err != nil {
			return fmt.Errorf("users: %w", err)
		}
		for _, user := range users {
			if len(user.Password) == len(password) && subtle.ConstantTimeCompare([]byte(user.Password), []byte(password)) == 1 {
				return fmt.Errorf("пароль клиента %q должен отличаться от ключа межсерверной цепочки", user.Name)
			}
		}
	}
	var obfsPassword string
	if obfsMode != "" {
		if obfsMode != "salamander" && obfsMode != "gecko" {
			return fmt.Errorf("неподдерживаемый тип UDP-маскировки %q", obfsMode)
		}
		if obfsKeyFile == "" {
			return fmt.Errorf("для UDP-маскировки укажите -hy2-obfs-key-file")
		}
		data, readErr := os.ReadFile(obfsKeyFile)
		if readErr != nil {
			return fmt.Errorf("прочитать пароль UDP-маскировки: %w", readErr)
		}
		obfsPassword = strings.TrimSpace(string(data))
	}
	pc, err := net.ListenPacket("udp", listen)
	if err != nil {
		return fmt.Errorf("listen Hysteria2 UDP %s: %w", listen, err)
	}
	if obfsMode != "" {
		pc, err = obfs.NewPacketConn(pc, obfsMode, obfsPassword)
		if err != nil {
			return fmt.Errorf("Hysteria2 UDP obfuscation: %w", err)
		}
	}
	domainPatterns, err := LoadPatternsFile(domainPatternsFile)
	if err != nil {
		_ = pc.Close()
		return fmt.Errorf("domain patterns: %w", err)
	}
	sniPatterns, err := LoadPatternsFile(sniPatternsFile)
	if err != nil {
		_ = pc.Close()
		return fmt.Errorf("SNI patterns: %w", err)
	}
	trust := trustCert
	if trust == "" {
		trust = certPath
	}
	upstream, err := newUpstreamClient(nextAddr, password, sni, trust, obfsMode, obfsPassword)
	if err != nil {
		_ = pc.Close()
		return err
	}
	config := hysteria.ExitServerConfig(pc, certs, password, users)
	var requestHook hyserver.RequestHook
	switch role {
	case "entry":
		chain := newChainOutbound(upstream)
		router := newEntryRouter(nextAddr, domainPatterns, sniPatterns, chain)
		requestHook = router
		config.Outbound = &entryOutbound{router: router}
	case "hop":
		if strings.TrimSpace(nextAddr) != "" && upstream != nil {
			config.Outbound = newChainOutbound(upstream)
		} else {
			config.Outbound = newDirectOutbound()
		}
	case "exit":
		config.Outbound = newDirectOutbound()
	}
	config.RequestHook = requestHook
	srv, err := hyserver.NewServer(config)
	if err != nil {
		_ = pc.Close()
		if upstream != nil {
			_ = upstream.Close()
		}
		return fmt.Errorf("create Hysteria2 server: %w", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve() }()
	log.Infof("[HY2_CASCADE_START] role=%s listen=%s next=%s domain_rules=%d sni_rules=%d obfs=%s", role, listen, nextAddr, domainPatterns.RuleCount(), sniPatterns.RuleCount(), obfsMode)
	select {
	case <-ctx.Done():
		_ = srv.Close()
	case err = <-serveErr:
		_ = srv.Close()
		if err != nil {
			return fmt.Errorf("Hysteria2 Serve: %w", err)
		}
	}
	if upstream != nil {
		_ = upstream.Close()
	}
	log.Infof("[HY2_CASCADE_STOP] role=%s listen=%s", role, listen)
	return nil
}