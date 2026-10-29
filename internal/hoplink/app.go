package hoplink

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

// Run starts an opaque UDP relay or the end-to-end Hysteria2 exit server.
func Run() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	mode := flag.String("mode", "hy2-forward", "режим узла: udp-relay или hy2-forward (выход)")
	capabilities := flag.Bool("capabilities", false, "вывести возможности бинарника и выйти")
	forward := flag.String("f", "", "адрес следующего UDP узла для udp-relay")
	relayMaxSessions := flag.Int("relay-max-sessions", defaultRelayMaxSessions, "максимум UDP-сессий реле")
	relayIdleTimeout := flag.Duration("relay-idle-timeout", defaultRelayIdleTimeout, "время простоя UDP-сессии реле")
	listen := flag.String("hy2-listen", ":22033", "адрес UDP прослушивания Hysteria2")
	sni := flag.String("hy2-sni", defaultHy2SNI, "SNI Hysteria2")
	key := flag.String("key", "", "общий ключ узлов цепочки")
	keyFile := flag.String("key-file", "", "файл общего ключа узлов")
	cert := flag.String("hy2-cert", "", "TLS сертификат PEM")
	certKey := flag.String("hy2-cert-key", "", "TLS ключ PEM")
	certDir := flag.String("hy2-cert-dir", ".HopLink", "каталог постоянного автоматически созданного TLS сертификата")
	publicHost := flag.String("hy2-public-host", "", "публичный адрес входного узла для ссылки Hiddify")
	usersFile := flag.String("hy2-users-file", "", "файл клиентов: по одной строке <имя> <пароль>")
	logLevel := flag.String("log-level", "info", "уровень журнала: trace, debug, info, warn, error")
	flag.Parse()
	if *capabilities {
		fmt.Println("udp-relay-v1")
		return
	}
	if err := setLogLevel(*logLevel); err != nil {
		log.Fatal(err)
	}
	// hy2-enter is retained as an alias for manually configured entry nodes.
	if *mode == "udp-relay" || *mode == "hy2-enter" {
		if *forward == "" {
			log.Fatal("для udp-relay укажите -f host:port следующего узла")
		}
		relay, err := newUDPRelay(*listen, parseHy2Forward(*forward), *relayMaxSessions, *relayIdleTimeout)
		if err != nil {
			log.Fatal(err)
		}
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		if err := relay.Serve(ctx); err != nil {
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
	logInfof("HopLink: запуск Hysteria2 mode=%s", *mode)
	if err := runHy2(*listen, *key, *sni, *cert, *certKey, *certDir, *publicHost, *usersFile); err != nil {
		log.Fatalf("Hysteria2: %v", err)
	}
}
