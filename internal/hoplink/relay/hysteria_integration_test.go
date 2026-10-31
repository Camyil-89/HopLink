//go:build integration

package relay

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"hoplink/internal/hoplink/hysteria"

	hyclient "github.com/apernet/hysteria/core/v2/client"
	hyserver "github.com/apernet/hysteria/core/v2/server"
)

func TestEndToEndHysteriaThroughTwoUDPRelays(t *testing.T) {
	certPEM, keyPEM, err := hysteria.GenerateCertificate("localhost")
	if err != nil {
		t.Fatal(err)
	}
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	cfg := hysteria.ExitServerConfig(pc, []tls.Certificate{cert}, "chain-test", []hysteria.User{{Name: "alice", Password: "alice-test"}, {Name: "bob", Password: "bob-test"}})
	srv, err := hyserver.NewServer(cfg)
	if err != nil {
		pc.Close()
		t.Fatal(err)
	}
	serverDone := make(chan error, 1)
	go func() { serverDone <- srv.Serve() }()
	t.Cleanup(func() { _ = srv.Close(); <-serverDone })
	middle := startIntegrationRelay(t, pc.LocalAddr().String(), DefaultIdleTimeout)
	entry := startIntegrationRelay(t, middle.LocalAddr().String(), DefaultIdleTimeout)
	pin := sha256.Sum256(cert.Certificate[0])
	connect := func(password string) hyclient.Client {
		t.Helper()
		c, info, err := hyclient.NewClient(&hyclient.Config{
			ServerAddr: entry.LocalAddr(), Auth: password,
			TLSConfig: hyclient.TLSConfig{ServerName: "localhost", InsecureSkipVerify: true, VerifyPeerCertificate: func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
				if len(rawCerts) == 0 || sha256.Sum256(rawCerts[0]) != pin {
					return fmt.Errorf("exit certificate pin mismatch")
				}
				return nil
			}},
			BandwidthConfig: hyclient.BandwidthConfig{MaxRx: 1_000_000_000},
		})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = c.Close() })
		if !info.UDPEnabled || info.Tx != 0 {
			t.Fatal("exit did not enforce automatic congestion control with UDP enabled")
		}
		return c
	}
	alice, bob := connect("alice-test"), connect("bob-test")
	echo := integrationUDPEcho(t)
	udpExchange := func(c hyclient.Client) {
		t.Helper()
		u, err := c.UDP()
		if err != nil {
			t.Fatal(err)
		}
		defer u.Close()
		done := make(chan error, 1)
		go func() {
			if err := u.Send([]byte("end-to-end UDP"), echo.LocalAddr().String()); err != nil {
				done <- err
				return
			}
			packet, _, err := u.Receive()
			if err == nil && string(packet) != "end-to-end UDP" {
				err = fmt.Errorf("UDP payload changed")
			}
			done <- err
		}()
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("UDP exchange timed out")
		}
	}
	udpExchange(alice)
	udpExchange(bob)
	_ = alice.Close()
	udpExchange(bob)
	tcp, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer tcp.Close()
	go func() {
		c, err := tcp.Accept()
		if err == nil {
			defer c.Close()
			_, _ = io.Copy(c, c)
		}
	}()
	stream, err := bob.TCP(tcp.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	_ = stream.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := stream.Write([]byte("end-to-end TCP")); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, len("end-to-end TCP"))
	if _, err := io.ReadFull(stream, buf); err != nil || string(buf) != "end-to-end TCP" {
		t.Fatalf("TCP exchange: %q, %v", buf, err)
	}
}

func startIntegrationRelay(t *testing.T, target string, idle time.Duration) *Relay {
	t.Helper()
	r, err := New("127.0.0.1:0", target, idle)
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
			t.Error("relay failed to shut down")
		}
	})
	return r
}

func integrationUDPEcho(t *testing.T) *net.UDPConn {
	t.Helper()
	c, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	go func() {
		buf := make([]byte, 65535)
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
