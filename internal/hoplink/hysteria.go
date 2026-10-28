package hoplink

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
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	hyclient "github.com/apernet/hysteria/core/v2/client"
	hyerrors "github.com/apernet/hysteria/core/v2/errors"
	hyserver "github.com/apernet/hysteria/core/v2/server"
)

const defaultHy2SNI = "www.cloudflare.com"

type hy2User struct{ name, password string }
type hy2PasswordAuth struct {
	password string
	users    []hy2User
}

func (a hy2PasswordAuth) Authenticate(_ net.Addr, auth string, _ uint64) (bool, string) {
	if len(auth) == len(a.password) && subtle.ConstantTimeCompare([]byte(auth), []byte(a.password)) == 1 {
		return true, "relay"
	}
	for _, user := range a.users {
		if len(auth) == len(user.password) && subtle.ConstantTimeCompare([]byte(auth), []byte(user.password)) == 1 {
			return true, user.name
		}
	}
	return false, ""
}

func loadHy2Users(path string) ([]hy2User, error) {
	if path == "" {
		return nil, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var users []hy2User
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
		users = append(users, hy2User{name: fields[0], password: fields[1]})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(users) == 0 {
		return nil, fmt.Errorf("файл пользователей %s не содержит пользователей", path)
	}
	return users, nil
}

type hy2Events struct{}

func (hy2Events) Connect(addr net.Addr, id string, tx uint64) {
	logInfof("[HY2_CLIENT_CONNECT] peer=%s id=%s tx=%d", addr, id, tx)
}
func (hy2Events) Disconnect(addr net.Addr, id string, err error) {
	logWarnf("[HY2_CLIENT_DISCONNECT] peer=%s id=%s err=%v", addr, id, err)
}
func (hy2Events) TCPRequest(addr net.Addr, id, target string) {
	logDebugf("[HY2_TCP_REQUEST] peer=%s id=%s target=%q", addr, id, target)
}
func (hy2Events) TCPError(addr net.Addr, id, target string, err error) {
	if err != nil {
		logWarnf("[HY2_TCP_END] peer=%s id=%s target=%q err=%v", addr, id, target, err)
	}
}
func (hy2Events) UDPRequest(addr net.Addr, id string, session uint32, target string) {
	logDebugf("[HY2_UDP_REQUEST] peer=%s id=%s session=%d target=%q", addr, id, session, target)
}
func (hy2Events) UDPError(addr net.Addr, id string, session uint32, err error) {
	if err != nil {
		logDebugf("[HY2_UDP_END] peer=%s id=%s session=%d err=%v", addr, id, session, err)
	}
}

type hy2ClientManager struct {
	mu       sync.Mutex
	current  hyclient.Client
	addr     string
	password string
	sni      string
	insecure bool
	rootCAs  *x509.CertPool
}

func newHy2ClientManager(addr, password, sni string, insecure bool, rootCAs *x509.CertPool) *hy2ClientManager {
	return &hy2ClientManager{addr: addr, password: password, sni: sni, insecure: insecure, rootCAs: rootCAs}
}
func (m *hy2ClientManager) get() (hyclient.Client, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.current != nil {
		return m.current, nil
	}
	addr, err := net.ResolveUDPAddr("udp", m.addr)
	if err != nil {
		return nil, fmt.Errorf("resolve Hysteria2 forward %q: %w", m.addr, err)
	}
	started := time.Now()
	c, info, err := hyclient.NewClient(&hyclient.Config{ServerAddr: addr, Auth: m.password, TLSConfig: hyclient.TLSConfig{ServerName: m.sni, InsecureSkipVerify: m.insecure, RootCAs: m.rootCAs}, QUICConfig: hyclient.QUICConfig{KeepAlivePeriod: 10 * time.Second, MaxIdleTimeout: 60 * time.Second}})
	if err != nil {
		return nil, err
	}
	m.current = c
	logInfof("[HY2_UPSTREAM_READY] server=%s remote=%s udp=%t elapsed=%s", m.addr, info.ServerAddr, info.UDPEnabled, time.Since(started).Round(time.Millisecond))
	if !info.UDPEnabled {
		_ = c.Close()
		m.current = nil
		return nil, fmt.Errorf("upstream Hysteria2 server did not enable UDP")
	}
	return c, nil
}
func (m *hy2ClientManager) invalidate(c hyclient.Client) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.current == c {
		m.current = nil
		_ = c.Close()
		logWarnf("[HY2_UPSTREAM_RESET] server=%s reason=connection_closed", m.addr)
	}
}
func isHy2Closed(err error) bool { var closed hyerrors.ClosedError; return errors.As(err, &closed) }
func (m *hy2ClientManager) TCP(addr string) (net.Conn, error) {
	c, err := m.get()
	if err != nil {
		return nil, err
	}
	stream, err := c.TCP(addr)
	if err == nil {
		return hy2TCPConn{Conn: stream, manager: m, owner: c}, nil
	}
	if !isHy2Closed(err) {
		return nil, err
	}
	m.invalidate(c)
	c, err = m.get()
	if err != nil {
		return nil, err
	}
	stream, err = c.TCP(addr)
	if err != nil && isHy2Closed(err) {
		m.invalidate(c)
	}
	if err != nil {
		return nil, err
	}
	return hy2TCPConn{Conn: stream, manager: m, owner: c}, nil
}
func (m *hy2ClientManager) UDP(addr string) (hyserver.UDPConn, error) {
	c, err := m.get()
	if err != nil {
		return nil, err
	}
	u, err := c.UDP()
	if err == nil {
		return hy2UDPConn{conn: u, manager: m, owner: c}, nil
	}
	if !isHy2Closed(err) {
		return nil, err
	}
	m.invalidate(c)
	c, err = m.get()
	if err != nil {
		return nil, err
	}
	u, err = c.UDP()
	if err != nil && isHy2Closed(err) {
		m.invalidate(c)
	}
	if err != nil {
		return nil, err
	}
	return hy2UDPConn{conn: u, manager: m, owner: c}, nil
}
func (*hy2ClientManager) CheckUDP(string) error { return nil }

func (m *hy2ClientManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.current != nil {
		_ = m.current.Close()
		m.current = nil
	}
}

type hy2TCPConn struct {
	net.Conn
	manager *hy2ClientManager
	owner   hyclient.Client
}

func (c hy2TCPConn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	if isHy2Closed(err) {
		c.manager.invalidate(c.owner)
	}
	return n, err
}
func (c hy2TCPConn) Write(p []byte) (int, error) {
	n, err := c.Conn.Write(p)
	if isHy2Closed(err) {
		c.manager.invalidate(c.owner)
	}
	return n, err
}

type hy2UDPConn struct {
	conn    hyclient.HyUDPConn
	manager *hy2ClientManager
	owner   hyclient.Client
}

func (c hy2UDPConn) ReadFrom(b []byte) (int, string, error) {
	p, addr, err := c.conn.Receive()
	if isHy2Closed(err) {
		c.manager.invalidate(c.owner)
	}
	if err != nil {
		return 0, "", err
	}
	return copy(b, p), addr, nil
}
func (c hy2UDPConn) WriteTo(p []byte, addr string) (int, error) {
	if err := c.conn.Send(p, addr); err != nil {
		if isHy2Closed(err) {
			c.manager.invalidate(c.owner)
		}
		return 0, err
	}
	return len(p), nil
}
func (c hy2UDPConn) Close() error { return c.conn.Close() }

func loadHy2Certificate(certPath, keyPath, certDir, sni string) ([]tls.Certificate, string, string, error) {
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
			certPEM, keyPEM, err := generateHy2Certificate(sni)
			if err != nil {
				return nil, "", "", err
			}
			if err = os.WriteFile(keyPath, keyPEM, 0600); err != nil {
				return nil, "", "", err
			}
			if err = os.WriteFile(certPath, certPEM, 0644); err != nil {
				return nil, "", "", err
			}
			logInfof("[HY2_CERT_CREATED] certificate=%s private_key=%s", certPath, keyPath)
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

func generateHy2Certificate(sni string) ([]byte, []byte, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 120))
	if err != nil {
		return nil, nil, err
	}
	now := time.Now()
	tmpl := x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: sni}, NotBefore: now.Add(-time.Minute), NotAfter: now.AddDate(5, 0, 0), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, BasicConstraintsValid: true}
	if ip := net.ParseIP(sni); ip != nil {
		tmpl.IPAddresses = []net.IP{ip}
	} else {
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

func loadHy2RootCAs(caPath string) (*x509.CertPool, error) {
	if caPath == "" {
		return nil, nil
	}
	pemData, err := os.ReadFile(caPath)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pemData) {
		return nil, fmt.Errorf("в -hy2-ca нет PEM сертификатов")
	}
	return pool, nil
}

func runHy2(mode, listen, forward, password, sni, upstreamSNI, certPath, keyPath, certDir, caPath, publicHost, usersFile string, insecure bool) error {
	if password == "" {
		return fmt.Errorf("укажите общий непустой -key для Hysteria2")
	}
	if sni == "" {
		sni = defaultHy2SNI
	}
	if upstreamSNI == "" {
		upstreamSNI = sni
	}
	certs, certPin, activeCertPath, err := loadHy2Certificate(certPath, keyPath, certDir, sni)
	if err != nil {
		return fmt.Errorf("Hysteria2 TLS certificate: %w", err)
	}
	users, err := loadHy2Users(usersFile)
	if err != nil {
		return fmt.Errorf("Hysteria2 users: %w", err)
	}
	for _, user := range users {
		if len(user.password) == len(password) && subtle.ConstantTimeCompare([]byte(user.password), []byte(password)) == 1 {
			return fmt.Errorf("пароль клиента %q должен отличаться от ключа межсерверной цепочки", user.name)
		}
	}
	var outbound Transport
	var manager *hy2ClientManager
	if mode == "hy2-enter" {
		if forward == "" {
			return fmt.Errorf("в режиме hy2-enter укажите -f host:port второго Hysteria2 сервера")
		}
		rootCAs, caErr := loadHy2RootCAs(caPath)
		if caErr != nil {
			return fmt.Errorf("load Hysteria2 upstream CA: %w", caErr)
		}
		manager = newHy2ClientManager(forward, password, upstreamSNI, insecure, rootCAs)
		defer manager.Close()
		if _, err = manager.get(); err != nil {
			return fmt.Errorf("connect to Hysteria2 forward %s: %w", forward, err)
		}
		outbound = manager
	} else if mode != "hy2-forward" {
		return fmt.Errorf("unknown Hysteria2 mode %q", mode)
	}
	pc, err := net.ListenPacket("udp", listen)
	if err != nil {
		return fmt.Errorf("listen Hysteria2 UDP %s: %w", listen, err)
	}
	config := &hyserver.Config{TLSConfig: hyserver.TLSConfig{Certificates: certs}, Conn: pc, Outbound: outbound, Authenticator: hy2PasswordAuth{password: password, users: users}, EventLogger: hy2Events{}}
	srv, err := hyserver.NewServer(config)
	if err != nil {
		_ = pc.Close()
		return fmt.Errorf("create Hysteria2 server: %w", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve() }()
	logInfof("[HY2_SERVER_START] mode=%s listen=%s sni=%s forward=%s udp=true tun_interface=false", mode, listen, sni, forward)
	if mode == "hy2-enter" {
		if publicHost == "" {
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			publicHost, err = fetchPublicIP(ctx)
			cancel()
		}
		if err != nil {
			logWarnf("[HY2_HIDDIFY_LINK] не удалось определить публичный адрес: %v; задайте -hy2-public-host", err)
		} else if publicHost != "" {
			_, port, splitErr := net.SplitHostPort(pc.LocalAddr().String())
			if splitErr != nil {
				logWarnf("[HY2_HIDDIFY_LINK] не удалось определить порт из %q: %v", pc.LocalAddr(), splitErr)
			} else {
				fmt.Println("\nHiddify — строки подключения:")
				if len(users) == 0 {
					fmt.Printf("%s\n\n", makeHy2ShareLink(password, publicHost, port, sni, certPin, "HopLink"))
				} else {
					sort.Slice(users, func(i, j int) bool { return users[i].name < users[j].name })
					for _, user := range users {
						fmt.Printf("%s: %s\n", user.name, makeHy2ShareLink(user.password, publicHost, port, sni, certPin, "HopLink-"+user.name))
					}
					fmt.Println()
				}
				logInfof("[HY2_HIDDIFY_LINK_READY] host=%s port=%s sni=%s pin_sha256=%s certificate=%s", publicHost, port, sni, certPin, activeCertPath)
				logInfof("[HY2_USERS_READY] count=%d", len(users))
			}
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
	logInfof("[HY2_SERVER_STOP] mode=%s listen=%s", mode, listen)
	return nil
}

func makeHy2ShareLink(password, host, port, sni, fingerprint, name string) string {
	host = strings.Trim(strings.TrimSpace(host), "[]")
	u := &url.URL{Scheme: "hysteria2", User: url.User(password), Host: net.JoinHostPort(host, port), Path: "/", RawQuery: url.Values{"pinSHA256": {fingerprint}, "sni": {sni}}.Encode(), Fragment: name}
	return u.String()
}

func parseHy2Forward(s string) string {
	if s == "" {
		return ""
	}
	if _, _, err := net.SplitHostPort(s); err == nil {
		return s
	}
	return net.JoinHostPort(s, strconv.Itoa(22033))
}
