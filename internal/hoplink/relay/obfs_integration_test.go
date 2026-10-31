//go:build integration

package relay

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"testing"
	"time"

	"hoplink/internal/hoplink/hysteria"
	"hoplink/internal/hoplink/obfs"

	hyclient "github.com/apernet/hysteria/core/v2/client"
	hyserver "github.com/apernet/hysteria/core/v2/server"
)

type obfsClientConnFactory struct{ mode, password string }

func (f obfsClientConnFactory) New(addr net.Addr) (net.PacketConn, error) {
	pc, err := net.ListenPacket("udp", addr.String())
	if err != nil {
		return nil, err
	}
	return obfs.NewPacketConn(pc, f.mode, f.password)
}

func TestHysteriaObfsThroughUDPRelays(t *testing.T) {
	for _, mode := range []string{"salamander", "gecko"} {
		t.Run(mode, func(t *testing.T) {
			const password = "same-obfs-password"
			certPEM, keyPEM, err := hysteria.GenerateCertificate("localhost")
			if err != nil {
				t.Fatal(err)
			}
			cert, err := tls.X509KeyPair(certPEM, keyPEM)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := net.ListenPacket("udp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			serverPC, err := obfs.NewPacketConn(raw, mode, password)
			if err != nil {
				t.Fatal(err)
			}
			cfg := hysteria.ExitServerConfig(serverPC, []tls.Certificate{cert}, "chain-key", nil)
			srv, err := hyserver.NewServer(cfg)
			if err != nil {
				_ = raw.Close()
				t.Fatal(err)
			}
			serverDone := make(chan error, 1)
			go func() { serverDone <- srv.Serve() }()
			t.Cleanup(func() { _ = srv.Close(); <-serverDone })
			middle := startObfsTestRelay(t, raw.LocalAddr().String(), mode)
			entry := startObfsTestRelay(t, middle.LocalAddr().String(), mode)
			client, _, err := hyclient.NewClient(&hyclient.Config{
				ServerAddr: entry.LocalAddr(), Auth: "chain-key",
				ConnFactory:     obfsClientConnFactory{mode: mode, password: password},
				TLSConfig:       hyclient.TLSConfig{ServerName: "localhost", InsecureSkipVerify: true},
				BandwidthConfig: hyclient.BandwidthConfig{MaxRx: 1_000_000_000},
			})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = client.Close() })
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			go func() {
				conn, err := listener.Accept()
				if err == nil {
					defer conn.Close()
					_, _ = io.Copy(conn, conn)
				}
			}()
			stream, err := client.TCP(listener.Addr().String())
			if err != nil {
				t.Fatal(err)
			}
			defer stream.Close()
			_ = stream.SetDeadline(time.Now().Add(5 * time.Second))
			if _, err = stream.Write([]byte("mask round trip")); err != nil {
				t.Fatal(err)
			}
			got := make([]byte, len("mask round trip"))
			if _, err = io.ReadFull(stream, got); err != nil || string(got) != "mask round trip" {
				t.Fatalf("round trip = %q, %v", got, err)
			}
		})
	}
}

func startObfsTestRelay(t *testing.T, target, mode string) *Relay {
	t.Helper()
	r, err := NewWithObfs("127.0.0.1:0", target, DefaultIdleTimeout, mode)
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
		case <-time.After(2 * time.Second):
			t.Error("obfuscated relay failed to shut down")
		}
	})
	return r
}
