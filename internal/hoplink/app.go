package hoplink

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

var ipifyHTTP = &http.Client{Timeout: 8 * time.Second}

// Run starts the Hysteria2 relay node. Other transport implementations can be
// added behind the Transport interface without changing the command package.
func Run() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	mode := flag.String("mode", "hy2-forward", "режим узла: hy2-enter или hy2-forward")
	forward := flag.String("f", "", "адрес следующего Hysteria2 узла для hy2-enter")
	listen := flag.String("hy2-listen", ":22033", "адрес UDP прослушивания Hysteria2")
	sni := flag.String("hy2-sni", defaultHy2SNI, "SNI Hysteria2")
	upstreamSNI := flag.String("hy2-upstream-sni", "", "SNI следующего узла; по умолчанию совпадает с -hy2-sni")
	key := flag.String("key", "", "общий ключ узлов цепочки")
	keyFile := flag.String("key-file", "", "файл общего ключа узлов")
	cert := flag.String("hy2-cert", "", "TLS сертификат PEM")
	certKey := flag.String("hy2-cert-key", "", "TLS ключ PEM")
	certDir := flag.String("hy2-cert-dir", ".HopLink", "каталог постоянного автоматически созданного TLS сертификата")
	ca := flag.String("hy2-ca", "", "PEM CA сертификат upstream")
	publicHost := flag.String("hy2-public-host", "", "публичный адрес входного узла для ссылки Hiddify")
	usersFile := flag.String("hy2-users-file", "", "файл клиентов: по одной строке <имя> <пароль>")
	insecure := flag.Bool("hy2-insecure", true, "не проверять сертификат upstream")
	logLevel := flag.String("log-level", "info", "уровень журнала: trace, debug, info, warn, error")
	flag.Parse()
	if err := setLogLevel(*logLevel); err != nil {
		log.Fatal(err)
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
	if *mode != "hy2-enter" && *mode != "hy2-forward" {
		log.Fatalf("неподдерживаемый режим %q; доступны hy2-enter и hy2-forward", *mode)
	}
	if *mode == "hy2-enter" {
		*forward = parseHy2Forward(*forward)
	}
	logInfof("HopLink: запуск Hysteria2 mode=%s", *mode)
	if err := runHy2(*mode, *listen, *forward, *key, *sni, *upstreamSNI, *cert, *certKey, *certDir, *ca, *publicHost, *usersFile, *insecure); err != nil {
		log.Fatalf("Hysteria2: %v", err)
	}
}

func fetchPublicIP(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.ipify.org", nil)
	if err != nil {
		return "", err
	}
	resp, err := ipifyHTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("api.ipify.org: HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64))
	if err != nil {
		return "", err
	}
	ip := strings.TrimSpace(string(body))
	if net.ParseIP(ip) == nil {
		return "", fmt.Errorf("неожиданный ответ: %q", ip)
	}
	return ip, nil
}
