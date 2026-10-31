package hysteria

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"hoplink/internal/hoplink/log"
	"hoplink/internal/hoplink/obfs"

	hyserver "github.com/apernet/hysteria/core/v2/server"
)

type User struct{ Name, Password string }

type PasswordAuth struct {
	ChainPassword string
	Users         []User
}

func (a PasswordAuth) Authenticate(addr net.Addr, auth string, tx uint64) (bool, string) {
	digest := sha256.Sum256([]byte(auth))
	relayDigest := sha256.Sum256([]byte(a.ChainPassword))
	selected := subtle.ConstantTimeCompare(digest[:], relayDigest[:])
	index := 0
	for i, user := range a.Users {
		userDigest := sha256.Sum256([]byte(user.Password))
		match := subtle.ConstantTimeCompare(digest[:], userDigest[:])
		selectUser := match & (1 - selected)
		index = subtle.ConstantTimeSelect(selectUser, i+1, index)
		selected |= match
	}
	if selected == 0 {
		log.Warnf("[HY2_AUTH_REJECT] peer=%s reason=invalid_credential tx=%d", addr, tx)
		return false, ""
	}
	if index == 0 {
		log.Debugf("[HY2_AUTH_ACCEPT] peer=%s id=relay tx=%d", addr, tx)
		return true, "relay"
	}
	log.Debugf("[HY2_AUTH_ACCEPT] peer=%s id=%s tx=%d", addr, a.Users[index-1].Name, tx)
	return true, a.Users[index-1].Name
}

func loadUsers(path string) ([]User, error) {
	if path == "" {
		return nil, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var users []User
	names, passwords := map[string]bool{}, map[string]bool{}
	scanner := bufio.NewScanner(f)
	for lineNo := 1; scanner.Scan(); lineNo++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return nil, fmt.Errorf("%s:%d: ожидается формат <имя> <пароль>", path, lineNo)
		}
		if names[fields[0]] {
			return nil, fmt.Errorf("%s:%d: повтор имени пользователя %q", path, lineNo, fields[0])
		}
		if passwords[fields[1]] {
			return nil, fmt.Errorf("%s:%d: пароль пользователя должен быть уникальным", path, lineNo)
		}
		names[fields[0]], passwords[fields[1]] = true, true
		users = append(users, User{Name: fields[0], Password: fields[1]})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return users, nil
}

type events struct{}

func (events) Connect(addr net.Addr, id string, tx uint64) {
	log.Infof("[HY2_CLIENT_CONNECT] peer=%s id=%s tx=%d", addr, id, tx)
}
func (events) Disconnect(addr net.Addr, id string, err error) {
	log.Warnf("[HY2_CLIENT_DISCONNECT] peer=%s id=%s err=%v", addr, id, err)
}
func (events) TCPRequest(addr net.Addr, id, target string) {
	log.Debugf("[HY2_TCP_REQUEST] peer=%s id=%s target=%q", addr, id, target)
}
func (events) TCPError(addr net.Addr, id, target string, err error) {
	if err != nil {
		log.Warnf("[HY2_TCP_END] peer=%s id=%s target=%q err=%v", addr, id, target, err)
	}
}
func (events) UDPRequest(addr net.Addr, id string, session uint32, target string) {
	log.Debugf("[HY2_UDP_REQUEST] peer=%s id=%s session=%d target=%q", addr, id, session, target)
}
func (events) UDPError(addr net.Addr, id string, session uint32, err error) {
	if err != nil {
		log.Debugf("[HY2_UDP_END] peer=%s id=%s session=%d err=%v", addr, id, session, err)
	}
}

func loadCertificate(certPath, keyPath, certDir, sni string) ([]tls.Certificate, string, string, error) {
	if (certPath == "") != (keyPath == "") {
		return nil, "", "", fmt.Errorf("укажите вместе -hy2-cert и -hy2-cert-key")
	}
	if certPath == "" {
		if certDir == "" {
			certDir = ".HopLink"
		}
		if err := os.MkdirAll(certDir, 0700); err != nil {
			return nil, "", "", err
		}
		certPath = filepath.Join(certDir, "hy2-server.crt")
		keyPath = filepath.Join(certDir, "hy2-server.key")
		_, certErr := os.Stat(certPath)
		_, keyErr := os.Stat(keyPath)
		certExists, keyExists := certErr == nil, keyErr == nil
		if certExists != keyExists {
			return nil, "", "", fmt.Errorf("неполная пара TLS-файлов: %s и %s; удалите оба или восстановите оба", certPath, keyPath)
		}
		if !certExists {
			certPEM, keyPEM, err := GenerateCertificate(sni)
			if err != nil {
				return nil, "", "", err
			}
			if err = os.WriteFile(keyPath, keyPEM, 0600); err != nil {
				return nil, "", "", err
			}
			if err = os.WriteFile(certPath, certPEM, 0644); err != nil {
				return nil, "", "", err
			}
			log.Infof("[HY2_CERT_CREATED] certificate=%s private_key=%s", certPath, keyPath)
		}
	}
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, "", "", err
	}
	if len(cert.Certificate) == 0 {
		return nil, "", "", fmt.Errorf("сертификат %s пустой", certPath)
	}
	fingerprint := sha256.Sum256(cert.Certificate[0])
	return []tls.Certificate{cert}, hex.EncodeToString(fingerprint[:]), certPath, nil
}

func GenerateCertificate(sni string) ([]byte, []byte, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 120))
	if err != nil {
		return nil, nil, err
	}
	now := time.Now()
	tmpl := x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: sni}, NotBefore: now.Add(-time.Minute), NotAfter: now.AddDate(100, 0, 0), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, BasicConstraintsValid: true}
	if ip := net.ParseIP(sni); ip != nil {
		tmpl.IPAddresses = []net.IP{ip}
	} else if sni != "" {
		tmpl.DNSNames = []string{sni}
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, nil, err
	}
	priv, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: priv}), nil
}

func Run(listen, password, sni, certPath, keyPath, certDir, publicHost, usersFile, obfsMode, obfsKeyFile string) error {
	if password == "" {
		return fmt.Errorf("укажите общий непустой -key для Hysteria2")
	}
	certs, certPin, activeCertPath, err := loadCertificate(certPath, keyPath, certDir, sni)
	if err != nil {
		return fmt.Errorf("Hysteria2 TLS certificate: %w", err)
	}
	users, err := loadUsers(usersFile)
	if err != nil {
		return fmt.Errorf("Hysteria2 users: %w", err)
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
		obfsPassword = string(data)
		if len([]byte(obfsPassword)) < 4 {
			return fmt.Errorf("пароль UDP-маскировки должен содержать не менее 4 байт")
		}
	} else if obfsKeyFile != "" {
		return fmt.Errorf("-hy2-obfs-key-file требует -hy2-obfs")
	}
	for _, user := range users {
		if len(user.Password) == len(password) && subtle.ConstantTimeCompare([]byte(user.Password), []byte(password)) == 1 {
			return fmt.Errorf("пароль клиента %q должен отличаться от ключа межсерверной цепочки", user.Name)
		}
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
	config := ExitServerConfig(pc, certs, password, users)
	srv, err := hyserver.NewServer(config)
	if err != nil {
		_ = pc.Close()
		return fmt.Errorf("create Hysteria2 server: %w", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	serveErr := make(chan error, 1)
	sort.Slice(users, func(i, j int) bool { return users[i].Name < users[j].Name })
	go func() { serveErr <- srv.Serve() }()
	log.Infof("[HY2_SERVER_START] mode=hy2-forward listen=%s sni=%s end_to_end=true obfs=%s", listen, sni, obfsMode)
	log.Infof("[HY2_DIAGNOSTICS] level=trace logs UDP packet metadata; authentication rejection logs peer and reason; credentials and packet contents are never logged")
	if publicHost != "" {
		_, port, splitErr := net.SplitHostPort(pc.LocalAddr().String())
		if splitErr != nil {
			log.Warnf("[HY2_HIDDIFY_LINK] не удалось определить порт из %q: %v", pc.LocalAddr(), splitErr)
		} else {
			fmt.Println("\nHiddify — строки подключения:")
			if len(users) == 0 {
				fmt.Printf("%s\n\n", MakeShareLink(password, publicHost, port, sni, certPin, "HopLink"))
			} else {
				for _, user := range users {
					fmt.Printf("%s: %s\n", user.Name, MakeShareLink(user.Password, publicHost, port, sni, certPin, "HopLink-"+user.Name))
				}
				fmt.Println()
			}
			log.Infof("[HY2_HIDDIFY_LINK_READY] host=%s port=%s sni=%s pin_sha256=%s certificate=%s", publicHost, port, sni, certPin, activeCertPath)
			log.Infof("[HY2_USERS_READY] count=%d", len(users))
		}
	}
	select {
	case <-ctx.Done():
		_ = srv.Close()
	case err = <-serveErr:
		_ = srv.Close()
		if err != nil {
			return fmt.Errorf("Hysteria2 Serve: %w", err)
		}
	}
	log.Infof("[HY2_SERVER_STOP] listen=%s", listen)
	return nil
}

func ExitServerConfig(pc net.PacketConn, certs []tls.Certificate, password string, users []User) *hyserver.Config {
	hyUsers := make([]User, len(users))
	copy(hyUsers, users)
	return &hyserver.Config{
		TLSConfig:             hyserver.TLSConfig{Certificates: certs},
		Conn:                  pc,
		Authenticator:         PasswordAuth{ChainPassword: password, Users: hyUsers},
		EventLogger:           events{},
		IgnoreClientBandwidth: true,
		CongestionConfig:      hyserver.CongestionConfig{Type: "bbr", BBRProfile: "conservative"},
	}
}

func MakeShareLink(password, host, port, sni, fingerprint, name string) string {
	host = strings.Trim(strings.TrimSpace(host), "[]")
	query := url.Values{"pinSHA256": {fingerprint}}
	if sni != "" {
		query.Set("sni", sni)
	}
	u := &url.URL{Scheme: "hysteria2", User: url.User(password), Host: net.JoinHostPort(host, port), Path: "/", RawQuery: query.Encode(), Fragment: name}
	return u.String()
}
