package desktop

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"golang.org/x/crypto/ssh"
	"hoplink/internal/desktop/remote"
	"net"
	"strings"
	"time"
)

func fingerprint(key ssh.PublicKey) string {
	sum := sha256.Sum256(key.Marshal())
	return "SHA256:" + base64.RawStdEncoding.EncodeToString(sum[:])
}
func (a *App) hostCallback(host string) ssh.HostKeyCallback {
	return func(_ string, _ net.Addr, key ssh.PublicKey) error {
		fp := fingerprint(key)
		a.mu.Lock()
		trusted := a.config.KnownHosts[host]
		a.mu.Unlock()
		if trusted == "" {
			return fmt.Errorf("SSH host key не подтверждён для %s (fingerprint %s). Подтвердите fingerprint в приложении", host, fp)
		}
		if trusted != fp {
			return fmt.Errorf("SSH host key fingerprint %s для %s не совпадает с сохранённым %s", fp, host, trusted)
		}
		return nil
	}
}
func (a *App) TrustHost(host, fp string) error {
	encoded, ok := strings.CutPrefix(fp, "SHA256:")
	decoded, err := base64.RawStdEncoding.DecodeString(encoded)
	if !ok || host == "" || err != nil || len(decoded) != sha256.Size {
		return fmt.Errorf("неверный fingerprint")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.config.KnownHosts[host] = fp
	return a.saveLocked()
}
func (a *App) sshConnect(s Server) (*ssh.Client, error) {
	return a.sshConnectContext(context.Background(), s)
}
func (a *App) sshConnectContext(ctx context.Context, s Server) (*ssh.Client, error) {
	auth := []ssh.AuthMethod{}
	if s.SSHPassword != "" {
		auth = append(auth, ssh.Password(s.SSHPassword))
	}
	if s.SSHPrivateKey != "" {
		var signer ssh.Signer
		var err error
		if s.SSHKeyPassphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase([]byte(s.SSHPrivateKey), []byte(s.SSHKeyPassphrase))
		} else {
			signer, err = ssh.ParsePrivateKey([]byte(s.SSHPrivateKey))
		}
		if err != nil {
			return nil, fmt.Errorf("SSH private key для %s: %w", s.Host, err)
		}
		auth = append(auth, ssh.PublicKeys(signer))
	}
	if len(auth) == 0 {
		return nil, fmt.Errorf("для %s задайте SSH пароль или приватный ключ", s.Host)
	}
	cfg := &ssh.ClientConfig{User: s.SSHUser, Auth: auth, HostKeyCallback: a.hostCallback(s.Host), Timeout: 20 * time.Second}
	address := net.JoinHostPort(s.Host, fmt.Sprint(s.SSHPort))
	conn, err := (&net.Dialer{Timeout: cfg.Timeout}).DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, err
	}
	stopClose := context.AfterFunc(ctx, func() { _ = conn.Close() })
	clientConn, channels, requests, err := ssh.NewClientConn(conn, address, cfg)
	stopClose()
	if err != nil {
		_ = conn.Close()
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, err
	}
	_ = conn.SetDeadline(time.Time{})
	client := ssh.NewClient(clientConn, channels, requests)
	if err := ctx.Err(); err != nil {
		_ = client.Close()
		return nil, err
	}
	return client, nil
}
func (a *App) reconnectSSHContext(ctx context.Context, s Server) (*ssh.Client, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if attempt > 0 {
			timer := time.NewTimer(time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
			}
		}
		client, err := a.sshConnectContext(ctx, s)
		if err == nil {
			return client, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("SSH reconnect after HopLink stop failed: %w", lastErr)
}
func runRemoteContext(ctx context.Context, c *ssh.Client, command string, input []byte) (string, error) {
	return remote.RunRemoteContext(ctx, c, command, input)
}
func runServerContext(ctx context.Context, c *ssh.Client, s Server, command string, input []byte) (string, error) {
	return remote.RunServerContext(ctx, c, s.SSHUser, s.SSHPassword, command, input)
}
