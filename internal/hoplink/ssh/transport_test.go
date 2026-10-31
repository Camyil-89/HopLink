package ssh

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"hoplink/internal/hoplink/relay"

	"golang.org/x/crypto/ssh"
)

func TestUDPCodecRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	payload := bytes.Repeat([]byte{0xab}, 1200)
	if err := WriteUDPFrame(&buf, "127.0.0.1:54321", payload); err != nil {
		t.Fatal(err)
	}
	src, out, err := ReadUDPFrame(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if src != "127.0.0.1:54321" || !bytes.Equal(out, payload) {
		t.Fatalf("codec round trip failed")
	}
}

func TestUDPRelayOpaqueEcho(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping SSH relay integration in -short mode")
	}
	_, clientPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	_, hostPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	clientKeyPath := writeTestKey(t, dir, "client.key", clientPriv)
	hostKeyPath := writeTestKey(t, dir, "host.key", hostPriv)
	authPath := filepath.Join(dir, "auth")
	writeTestAuthorized(t, authPath, clientPriv.Public().(ed25519.PublicKey))
	hostPubPath := filepath.Join(dir, "host.pub")
	writeTestAuthorized(t, hostPubPath, hostPriv.Public().(ed25519.PublicKey))

	echo := udpEcho(t)
	downstream := startTestRelay(t, echo.LocalAddr().String(), relay.DefaultIdleTimeout)
	sshListen, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_ = StartServer(ctx, ServerConfig{
			Listen:             sshListen.Addr().String(),
			HostKeyFile:        hostKeyPath,
			AuthorizedKeysFile: authPath,
			LocalUDPPort:       downstream.LocalAddr().(*net.UDPAddr).Port,
			IdleTimeout:        time.Second,
		})
	}()
	upstream, err := NewUpstream(ctx, ClientConfig{
		NextAddr:       sshListen.Addr().String(),
		ClientKeyFile:  clientKeyPath,
		HostPubKeyFile: hostPubPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	entry, err := relay.NewWithTransport("127.0.0.1:0", "", relay.DefaultIdleTimeout, "", true)
	if err != nil {
		t.Fatal(err)
	}
	entry.SetUpstream(upstream)
	done := make(chan error, 1)
	go func() { done <- entry.Serve(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(800 * time.Millisecond):
			t.Error("entry relay failed to shut down")
		}
	})

	peer, err := net.DialUDP("udp", nil, entry.LocalAddr().(*net.UDPAddr))
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	initial := make([]byte, 1200)
	initial[0], initial[4] = 0xc0, 1
	_ = peer.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := peer.Write(initial); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, maxUDPPacketSize)
	n, err := peer.Read(buf)
	if err != nil || !bytes.Equal(buf[:n], initial) {
		t.Fatalf("ssh relay opaque echo failed: n=%d err=%v", n, err)
	}
}

func udpEcho(t *testing.T) *net.UDPConn {
	t.Helper()
	c, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	go func() {
		buf := make([]byte, maxUDPPacketSize)
		for {
			n, addr, err := c.ReadFromUDPAddrPort(buf)
			if err != nil {
				return
			}
			_, _ = c.WriteToUDPAddrPort(buf[:n], addr)
		}
	}()
	return c
}

func startTestRelay(t *testing.T, target string, idle time.Duration) *relay.Relay {
	t.Helper()
	r, err := relay.New("127.0.0.1:0", target, idle)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- r.Serve(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(800 * time.Millisecond):
			t.Error("relay failed to shut down")
		}
	})
	return r
}

func writeTestKey(t *testing.T, dir, name string, key ed25519.PrivateKey) string {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeTestAuthorized(t *testing.T, path string, pub ed25519.PublicKey) {
	t.Helper()
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, ssh.MarshalAuthorizedKey(sshPub), 0600); err != nil {
		t.Fatal(err)
	}
}
