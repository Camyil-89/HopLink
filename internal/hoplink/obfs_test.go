package hoplink

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"net"
	"testing"
	"time"

	hyclient "github.com/apernet/hysteria/core/v2/client"
	hyserver "github.com/apernet/hysteria/core/v2/server"
)

type obfsClientConnFactory struct{ mode, password string }

func (f obfsClientConnFactory) New(addr net.Addr) (net.PacketConn, error) {
	pc, err := net.ListenPacket("udp", "")
	if err != nil {
		return nil, err
	}
	wrapped, err := newObfsPacketConn(pc, f.mode, f.password)
	if err != nil {
		_ = pc.Close()
		return nil, err
	}
	return wrapped, nil
}

func TestObfsPacketConnRoundTrip(t *testing.T) {
	for _, mode := range []string{"salamander", "gecko"} {
		t.Run(mode, func(t *testing.T) {
			serverRaw, err := net.ListenPacket("udp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer serverRaw.Close()
			server, err := newObfsPacketConn(serverRaw, mode, "same-test-mask-password")
			if err != nil {
				t.Fatal(err)
			}
			clientRaw, err := net.ListenPacket("udp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer clientRaw.Close()
			client, err := newObfsPacketConn(clientRaw, mode, "same-test-mask-password")
			if err != nil {
				t.Fatal(err)
			}
			for _, size := range []int{1200, 24} {
				want := bytes.Repeat([]byte{0x5a}, size)
				want[0] = 0xc0 // QUIC long-header packet, fragmented in Gecko mode.
				if _, err = client.WriteTo(want, serverRaw.LocalAddr()); err != nil {
					t.Fatal(err)
				}
				_ = server.SetReadDeadline(time.Now().Add(2 * time.Second))
				got := make([]byte, 1500)
				n, from, err := server.ReadFrom(got)
				if err != nil {
					t.Fatal(err)
				}
				if from == nil || !bytes.Equal(got[:n], want) {
					t.Fatalf("round trip mismatch: len=%d from=%v", n, from)
				}
			}
		})
	}
}

func TestObfsPacketConnRequiresValidModeAndPassword(t *testing.T) {
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer pc.Close()
	if _, err := newObfsPacketConn(pc, "unknown", "long-enough"); err == nil {
		t.Fatal("expected unsupported mode error")
	}
	if _, err := newObfsPacketConn(pc, "salamander", "abc"); err == nil {
		t.Fatal("expected short password error")
	}
}

func TestHysteriaObfsThroughUDPRelays(t *testing.T) {
	for _, mode := range []string{"salamander", "gecko"} {
		t.Run(mode, func(t *testing.T) {
			const password = "same-obfs-password"
			certPEM, keyPEM, err := generateHy2Certificate("localhost")
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
			serverPC, err := newObfsPacketConn(raw, mode, password)
			if err != nil {
				t.Fatal(err)
			}
			cfg := exitServerConfig(serverPC, []tls.Certificate{cert}, "chain-key", nil)
			srv, err := hyserver.NewServer(cfg)
			if err != nil {
				_ = raw.Close()
				t.Fatal(err)
			}
			serverDone := make(chan error, 1)
			go func() { serverDone <- srv.Serve() }()
			t.Cleanup(func() { _ = srv.Close(); <-serverDone })
			middle := startObfsTestRelay(t, raw.LocalAddr().String(), mode)
			entry := startObfsTestRelay(t, middle.conn.LocalAddr().String(), mode)
			client, _, err := hyclient.NewClient(&hyclient.Config{
				ServerAddr: entry.conn.LocalAddr(), Auth: "chain-key",
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

func startObfsTestRelay(t *testing.T, target, mode string) *udpRelay {
	t.Helper()
	r, err := newUDPRelayWithObfs("127.0.0.1:0", target, defaultRelayIdleTimeout, mode)
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
		case <-time.After(5 * time.Second):
			t.Error("obfuscated relay failed to shut down")
		}
	})
	return r
}
