package relay

import (
	"bytes"
	"context"
	"net"
	"testing"
	"time"

	"hoplink/internal/hoplink/obfs"
)

func benchEchoUDP(b *testing.B) *net.UDPConn {
	b.Helper()
	c, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = c.Close() })
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

func benchRelay(b *testing.B, target string, idle time.Duration) *Relay {
	b.Helper()
	r, err := New("127.0.0.1:0", target, idle)
	if err != nil {
		b.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- r.Serve(ctx) }()
	b.Cleanup(func() {
		cancel()
		<-done
	})
	return r
}

func quicLikePacket(size int) []byte {
	p := make([]byte, size)
	p[0] = 0xc0
	copy(p[1:5], []byte{0, 0, 0, 1})
	return p
}

func BenchmarkIsQUICInitial(b *testing.B) {
	p := quicLikePacket(1200)
	b.SetBytes(int64(len(p)))
	b.ReportAllocs()
	for b.Loop() {
		_ = IsQUICInitial(p)
	}
}

func BenchmarkIsRelayInitial_Plain(b *testing.B) {
	p := quicLikePacket(1200)
	b.SetBytes(int64(len(p)))
	b.ReportAllocs()
	for b.Loop() {
		_ = IsRelayInitial(p, "")
	}
}

func BenchmarkIsRelayInitial_Salamander(b *testing.B) {
	p := bytes.Repeat([]byte{0x9a}, 1200+obfs.SalamanderSaltSize)
	b.SetBytes(int64(len(p)))
	b.ReportAllocs()
	for b.Loop() {
		_ = IsRelayInitial(p, "salamander")
	}
}

func BenchmarkRelayRoundTrip_256B(b *testing.B) {
	benchmarkRelayRoundTrip(b, 256)
}

func BenchmarkRelayRoundTrip_1200B(b *testing.B) {
	benchmarkRelayRoundTrip(b, 1200)
}

func BenchmarkRelayRoundTrip_4096B(b *testing.B) {
	benchmarkRelayRoundTrip(b, 4096)
}

func benchmarkRelayRoundTrip(b *testing.B, payloadSize int) {
	echo := benchEchoUDP(b)
	r := benchRelay(b, echo.LocalAddr().String(), 30*time.Second)
	peer, err := net.DialUDP("udp", nil, r.LocalAddr().(*net.UDPAddr))
	if err != nil {
		b.Fatal(err)
	}
	defer peer.Close()

	initial := quicLikePacket(1200)
	if _, err := peer.Write(initial); err != nil {
		b.Fatal(err)
	}
	buf := make([]byte, maxUDPPacketSize)
	if _, err := peer.Read(buf); err != nil {
		b.Fatal(err)
	}

	packet := quicLikePacket(payloadSize)
	packet[0] = 0x40 // non-initial after session exists

	b.SetBytes(int64(payloadSize))
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		if _, err := peer.Write(packet); err != nil {
			b.Fatal(err)
		}
		if _, err := peer.Read(buf); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRelayRoundTrip_10Sessions_1200B(b *testing.B) {
	echo := benchEchoUDP(b)
	r := benchRelay(b, echo.LocalAddr().String(), 30*time.Second)

	const sessions = 10
	peers := make([]*net.UDPConn, sessions)
	bufs := make([][]byte, sessions)
	for i := range peers {
		p, err := net.DialUDP("udp", nil, r.LocalAddr().(*net.UDPAddr))
		if err != nil {
			b.Fatal(err)
		}
		peers[i] = p
		defer p.Close()
		initial := quicLikePacket(1200)
		if _, err := p.Write(initial); err != nil {
			b.Fatal(err)
		}
		bufs[i] = make([]byte, maxUDPPacketSize)
		if _, err := p.Read(bufs[i]); err != nil {
			b.Fatal(err)
		}
	}

	packet := quicLikePacket(1200)
	packet[0] = 0x40

	b.SetBytes(int64(1200))
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		i := b.N % sessions
		if _, err := peers[i].Write(packet); err != nil {
			b.Fatal(err)
		}
		if _, err := peers[i].Read(bufs[i]); err != nil {
			b.Fatal(err)
		}
	}
}