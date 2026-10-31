package hoplink

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"hoplink/internal/hoplink/hysteria"
	hlog "hoplink/internal/hoplink/log"
	"hoplink/internal/hoplink/relay"
	"hoplink/internal/hoplink/ssh"
	"hoplink/internal/hoplink/ws"
)

// Run starts an opaque UDP relay or the end-to-end Hysteria2 exit server.
func Run() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	mode := flag.String("mode", "hy2-forward", "режим узла: udp-relay или hy2-forward (выход)")
	capabilities := flag.Bool("capabilities", false, "вывести возможности бинарника и выйти")
	forward := flag.String("f", "", "адрес следующего UDP узла для udp-relay")
	relayIdleTimeout := flag.Duration("relay-idle-timeout", relay.DefaultIdleTimeout, "время простоя UDP-сессии реле")
	relayObfs := flag.String("relay-obfs", "", "маскировка на клиентском участке: salamander или gecko")
	relayTransport := flag.String("relay-transport", "udp", "транспорт между реле: udp, ssh или ws")
	relaySSHListen := flag.String("relay-ssh-listen", "", "адрес встроенного SSH-сервера реле, например :22034")
	relaySSHHostKeyFile := flag.String("relay-ssh-host-key-file", "", "файл host key PEM для SSH-сервера реле")
	relaySSHAuthorizedKeys := flag.String("relay-ssh-authorized-keys-file", "", "authorized_keys для SSH-сервера реле")
	relaySSHNext := flag.String("relay-ssh-next", "", "адрес SSH-транспорта следующего узла host:port")
	relaySSHUser := flag.String("relay-ssh-user", "hoplink", "имя пользователя SSH-транспорта")
	relaySSHClientKeyFile := flag.String("relay-ssh-client-key-file", "", "файл client key PEM для SSH-транспорта")
	relaySSHHostPubFile := flag.String("relay-ssh-host-pub-file", "", "файл public host key следующего узла для проверки SSH-транспорта")
	relayWSListen := flag.String("relay-ws-listen", "", "адрес WSS-сервера реле, например :22034")
	relayWSCert := flag.String("relay-ws-cert", "", "TLS сертификат PEM для WSS-сервера реле")
	relayWSCertKey := flag.String("relay-ws-cert-key", "", "TLS ключ PEM для WSS-сервера реле")
	relayWSNext := flag.String("relay-ws-next", "", "адрес WSS-транспорта следующего узла host:port")
	relayWSTrustCert := flag.String("relay-ws-trust-cert", "", "PEM маршрутного сертификата для проверки следующего WSS-узла")
	listen := flag.String("hy2-listen", ":22033", "адрес UDP прослушивания Hysteria2")
	sni := flag.String("hy2-sni", "", "необязательный SNI Hysteria2")
	key := flag.String("key", "", "общий ключ узлов цепочки")
	keyFile := flag.String("key-file", "", "файл общего ключа узлов")
	cert := flag.String("hy2-cert", "", "TLS сертификат PEM")
	certKey := flag.String("hy2-cert-key", "", "TLS ключ PEM")
	certDir := flag.String("hy2-cert-dir", ".HopLink", "каталог постоянного автоматически созданного TLS сертификата")
	publicHost := flag.String("hy2-public-host", "", "публичный адрес входного узла для ссылки Hiddify")
	usersFile := flag.String("hy2-users-file", "", "файл клиентов: по одной строке <имя> <пароль>")
	hy2Obfs := flag.String("hy2-obfs", "", "маскировка Hysteria2: salamander или gecko")
	hy2ObfsKeyFile := flag.String("hy2-obfs-key-file", "", "файл пароля UDP-маскировки")
	logLevel := flag.String("log-level", "info", "уровень журнала: trace, debug, info, warn, error")
	flag.Parse()
	if *capabilities {
		fmt.Println("udp-relay-v1")
		fmt.Println("ssh-udp-relay-v1")
		fmt.Println("ws-udp-relay-v1")
		return
	}
	if err := hlog.SetLevel(*logLevel); err != nil {
		log.Fatal(err)
	}
	if *mode == "udp-relay" || *mode == "hy2-enter" {
		transport := strings.ToLower(strings.TrimSpace(*relayTransport))
		if transport == "" {
			transport = "udp"
		}
		if transport != "udp" && transport != "ssh" && transport != "ws" {
			log.Fatalf("неподдерживаемый -relay-transport %q; доступны udp, ssh и ws", *relayTransport)
		}
		if transport == "udp" && *forward == "" {
			log.Fatal("для udp-relay укажите -f host:port следующего узла")
		}
		if transport == "ssh" && *relaySSHNext == "" {
			log.Fatal("для relay-transport=ssh укажите -relay-ssh-next host:port")
		}
		if transport == "ws" && *relayWSNext == "" {
			log.Fatal("для relay-transport=ws укажите -relay-ws-next host:port")
		}
		if transport == "ws" && *relayWSTrustCert == "" {
			log.Fatal("для relay-transport=ws укажите -relay-ws-trust-cert")
		}
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		startRelaySSHServerBackground(ctx, *relaySSHListen, *relaySSHHostKeyFile, *relaySSHAuthorizedKeys, *listen, *relayIdleTimeout)
		startRelayWSServerBackground(ctx, *relayWSListen, *relayWSCert, *relayWSCertKey, *listen, *relayIdleTimeout)
		tunnel := transport == "ssh" || transport == "ws"
		r, err := relay.NewWithTransport(*listen, relay.ParseForwardTarget(*forward), *relayIdleTimeout, *relayObfs, tunnel)
		if err != nil {
			log.Fatal(err)
		}
		if transport == "ssh" {
			upstream, err := ssh.NewUpstream(ctx, ssh.ClientConfig{
				NextAddr:       *relaySSHNext,
				User:           *relaySSHUser,
				ClientKeyFile:  *relaySSHClientKeyFile,
				HostPubKeyFile: *relaySSHHostPubFile,
			})
			if err != nil {
				log.Fatal(err)
			}
			r.SetUpstream(upstream)
		}
		if transport == "ws" {
			upstream, err := ws.NewUpstream(ctx, ws.ClientConfig{
				NextAddr:      *relayWSNext,
				TrustCertFile: *relayWSTrustCert,
			})
			if err != nil {
				log.Fatal(err)
			}
			r.SetUpstream(upstream)
		}
		if err := r.Serve(ctx); err != nil {
			log.Fatalf("UDP relay: %v", err)
		}
		return
	}
	if *keyFile != "" {
		data, err := os.ReadFile(*keyFile)
		if err != nil {
			log.Fatalf("прочитать -key-file %s: %v", *keyFile, err)
		}
		fileKey := strings.TrimSpace(string(data))
		if *key != "" && *key != fileKey {
			log.Fatal("-key и -key-file содержат разные ключи")
		}
		*key = fileKey
	}
	if *mode != "hy2-forward" {
		log.Fatalf("неподдерживаемый режим %q; доступны udp-relay и hy2-forward", *mode)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	startRelaySSHServerBackground(ctx, *relaySSHListen, *relaySSHHostKeyFile, *relaySSHAuthorizedKeys, *listen, *relayIdleTimeout)
	startRelayWSServerBackground(ctx, *relayWSListen, *relayWSCert, *relayWSCertKey, *listen, *relayIdleTimeout)
	hlog.Infof("HopLink: запуск Hysteria2 mode=%s", *mode)
	if err := hysteria.Run(*listen, *key, *sni, *cert, *certKey, *certDir, *publicHost, *usersFile, *hy2Obfs, *hy2ObfsKeyFile); err != nil {
		log.Fatalf("Hysteria2: %v", err)
	}
}

func startRelaySSHServerBackground(ctx context.Context, listen, hostKeyFile, authorizedKeysFile, hy2Listen string, idle time.Duration) {
	if listen == "" {
		return
	}
	listenPort := 0
	if addr, err := net.ResolveUDPAddr("udp", hy2Listen); err == nil {
		listenPort = addr.Port
	}
	go func() {
		if err := ssh.StartServer(ctx, ssh.ServerConfig{
			Listen:             listen,
			HostKeyFile:        hostKeyFile,
			AuthorizedKeysFile: authorizedKeysFile,
			LocalUDPPort:       listenPort,
			IdleTimeout:        idle,
		}); err != nil && ctx.Err() == nil {
			log.Fatalf("relay ssh server: %v", err)
		}
	}()
}

func startRelayWSServerBackground(ctx context.Context, listen, certFile, keyFile, hy2Listen string, idle time.Duration) {
	if listen == "" {
		return
	}
	listenPort := 0
	if addr, err := net.ResolveUDPAddr("udp", hy2Listen); err == nil {
		listenPort = addr.Port
	}
	go func() {
		if err := ws.StartServer(ctx, ws.ServerConfig{
			Listen:       listen,
			CertFile:     certFile,
			KeyFile:      keyFile,
			LocalUDPPort: listenPort,
			IdleTimeout:  idle,
		}); err != nil && ctx.Err() == nil {
			log.Fatalf("relay ws server: %v", err)
		}
	}()
}
