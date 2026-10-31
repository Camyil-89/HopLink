package relay

import (
	"bytes"
	"context"
	"net"
	"testing"
	"time"

	"hoplink/internal/hoplink/obfs"
)

func startTestRelay(t *testing.T, target string, idle time.Duration) *Relay {
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
			if r.QueuedBytes() != 0 {
				t.Errorf("relay leaked queued bytes: %d", r.QueuedBytes())
			}
		case <-time.After(2 * time.Second):
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

func TestOpaquePacketsIndependentSessionsAndIdle(t *testing.T) {
	echo := udpEcho(t)
	r := startTestRelay(t, echo.LocalAddr().String(), time.Second)
	peer, err := net.DialUDP("udp", nil, r.LocalAddr().(*net.UDPAddr))
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
	initial := make([]byte, 1200)
	initial[0], initial[4] = 0xc0, 1
	exchange(initial)
	exchange([]byte{0x40, 0, 1, 2, 0xff})
	other, err := net.DialUDP("udp", nil, r.LocalAddr().(*net.UDPAddr))
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	_ = other.SetDeadline(time.Now().Add(3 * time.Second))
	_, _ = other.Write(initial)
	if n, err := other.Read(buf); err != nil || !bytes.Equal(buf[:n], initial) {
		t.Fatalf("new session was not admitted: n=%d err=%v", n, err)
	}
	exchange([]byte{0x40, 9, 8, 7})
	deadline := time.Now().Add(3 * time.Second)
	for {
		if r.SessionCount() == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("idle session was not reclaimed")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestObfuscatedRelayAdmissionUsesPacketSize(t *testing.T) {
	if !IsRelayInitial(bytes.Repeat([]byte{0x9a}, 1200+obfs.SalamanderSaltSize), "salamander") {
		t.Fatal("Salamander initial datagram was rejected")
	}
	if IsRelayInitial(bytes.Repeat([]byte{0x9a}, 1200), "salamander") {
		t.Fatal("undersized Salamander datagram was admitted")
	}
	if !IsRelayInitial(bytes.Repeat([]byte{0x9a}, obfs.GeckoMinPacketSize), "gecko") {
		t.Fatal("Gecko initial fragment was rejected")
	}
	if IsRelayInitial(bytes.Repeat([]byte{0x9a}, obfs.GeckoMinPacketSize-1), "gecko") {
		t.Fatal("undersized Gecko datagram was admitted")
	}
}

func TestQUICAdmission(t *testing.T) {
	packet := make([]byte, 1200)
	packet[0], packet[4] = 0xc0, 1
	if !IsQUICInitial(packet) {
		t.Fatal("v1 Initial rejected")
	}
	packet[0] = 0xe0
	if IsQUICInitial(packet) || IsQUICInitial([]byte("not QUIC")) {
		t.Fatal("non-Initial admitted")
	}
	packet[0] = 0xd0
	copy(packet[1:5], []byte{0x6b, 0x33, 0x43, 0xcf})
	if !IsQUICInitial(packet) {
		t.Fatal("v2 Initial rejected")
	}
}
