package hoplink

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	hyclient "github.com/apernet/hysteria/core/v2/client"
	hyserver "github.com/apernet/hysteria/core/v2/server"
)

func startTestRelay(t *testing.T, target string, idle time.Duration) *udpRelay {
	t.Helper()
	r, err := newUDPRelay("127.0.0.1:0", target, idle)
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
			if r.queuedBytes.Load() != 0 {
				t.Errorf("relay leaked queued bytes: %d", r.queuedBytes.Load())
			}
		case <-time.After(5 * time.Second):
			t.Error("relay failed to shut down")
		}
	})
	return r
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

func TestUDPRelayOpaquePacketsIndependentSessionsAndIdle(t *testing.T) {
	echo := udpEcho(t)
	r := startTestRelay(t, echo.LocalAddr().String(), time.Second)
	peer, err := net.DialUDP("udp", nil, r.conn.LocalAddr().(*net.UDPAddr))
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	buf := make([]byte, maxUDPPacketSize)
	exchange := func(packet []byte) {
		t.Helper()
		_ = peer.SetDeadline(time.Now().Add(3 * time.Second))
		if _, err := peer.Write(packet); err != nil {
			t.Fatal(err)
		}
		n, err := peer.Read(buf)
		if err != nil || !bytes.Equal(buf[:n], packet) {
			t.Fatalf("opaque packet changed or lost: n=%d err=%v", n, err)
		}
	}
	// The relay must not parse, decrypt or rewrite the payload.
	initial := make([]byte, 1200)
	initial[0], initial[4] = 0xc0, 1
	exchange(initial)
	exchange([]byte{0x40, 0, 1, 2, 0xff})
	other, err := net.DialUDP("udp", nil, r.conn.LocalAddr().(*net.UDPAddr))
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	_ = other.SetDeadline(time.Now().Add(3 * time.Second))
	_, _ = other.Write(initial)
	if n, err := other.Read(buf); err != nil || !bytes.Equal(buf[:n], initial) {
		t.Fatalf("new session was not admitted: n=%d err=%v", n, err)
	}
	exchange([]byte{0x40, 9, 8, 7}) // A new source must not disrupt the existing one.
	deadline := time.Now().Add(4 * time.Second)
	for {
		r.mu.Lock()
		count := len(r.sessions)
		r.mu.Unlock()
		if count == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("idle session was not reclaimed")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestObfuscatedRelayAdmissionUsesPacketSize(t *testing.T) {
	if !isRelayInitial(bytes.Repeat([]byte{0x9a}, 1200+salamanderSaltSize), "salamander") {
		t.Fatal("Salamander initial datagram was rejected")
	}
	if isRelayInitial(bytes.Repeat([]byte{0x9a}, 1200), "salamander") {
		t.Fatal("undersized Salamander datagram was admitted")
	}
	if !isRelayInitial(bytes.Repeat([]byte{0x9a}, geckoMinPacketSize), "gecko") {
		t.Fatal("Gecko initial fragment was rejected")
	}
	if isRelayInitial(bytes.Repeat([]byte{0x9a}, geckoMinPacketSize-1), "gecko") {
		t.Fatal("undersized Gecko datagram was admitted")
	}
}

func TestUDPRelayQUICAdmission(t *testing.T) {
	packet := make([]byte, 1200)
	packet[0], packet[4] = 0xc0, 1
	if !isQUICInitial(packet) {
		t.Fatal("v1 Initial rejected")
	}
	packet[0] = 0xe0
	if isQUICInitial(packet) || isQUICInitial([]byte("not QUIC")) {
		t.Fatal("non-Initial admitted")
	}
	packet[0] = 0xd0
	copy(packet[1:5], []byte{0x6b, 0x33, 0x43, 0xcf})
	if !isQUICInitial(packet) {
		t.Fatal("v2 Initial rejected")
	}
}

func TestEndToEndHysteriaThroughTwoUDPRelays(t *testing.T) {
	certPEM, keyPEM, err := generateHy2Certificate("localhost")
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
	cfg := exitServerConfig(pc, []tls.Certificate{cert}, "chain-test", []hy2User{{name: "alice", password: "alice-test"}, {name: "bob", password: "bob-test"}})
	srv, err := hyserver.NewServer(cfg)
	if err != nil {
		pc.Close()
		t.Fatal(err)
	}
	serverDone := make(chan error, 1)
	go func() { serverDone <- srv.Serve() }()
	t.Cleanup(func() { _ = srv.Close(); <-serverDone })
	middle := startTestRelay(t, pc.LocalAddr().String(), defaultRelayIdleTimeout)
	entry := startTestRelay(t, middle.conn.LocalAddr().String(), defaultRelayIdleTimeout)
	pin := sha256.Sum256(cert.Certificate[0])
	connect := func(password string) hyclient.Client {
		t.Helper()
		c, info, err := hyclient.NewClient(&hyclient.Config{
			ServerAddr: entry.conn.LocalAddr(), Auth: password,
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
	echo := udpEcho(t)
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
	udpExchange(bob) // Closing one user must not reset another user's tunnel.
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
