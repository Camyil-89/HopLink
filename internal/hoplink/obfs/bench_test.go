package obfs

import (
	"net"
	"testing"
	"time"
)

func benchSalamanderPacket(size int) []byte {
	p := make([]byte, size)
	for i := range p {
		p[i] = byte(i)
	}
	return p
}

func BenchmarkSalamanderMask_1200B(b *testing.B) {
	pc, err := NewPacketConn(&net.UDPConn{}, "salamander", "test-password")
	if err != nil {
		b.Fatal(err)
	}
	c := pc.(*PacketConn)
	packet := benchSalamanderPacket(1200)
	b.SetBytes(int64(1200))
	b.ReportAllocs()
	for b.Loop() {
		wirep, wire, err := c.mask(packet)
		if err != nil {
			b.Fatal(err)
		}
		plainp, _, ok := c.unmask(wire)
		putWire(wirep)
		putWire(plainp)
		if !ok {
			b.Fatal("unmask failed")
		}
	}
}

func BenchmarkSalamanderMask_256B(b *testing.B) {
	pc, err := NewPacketConn(&net.UDPConn{}, "salamander", "test-password")
	if err != nil {
		b.Fatal(err)
	}
	c := pc.(*PacketConn)
	packet := benchSalamanderPacket(256)
	b.SetBytes(int64(256))
	b.ReportAllocs()
	for b.Loop() {
		wirep, wire, err := c.mask(packet)
		if err != nil {
			b.Fatal(err)
		}
		plainp, _, ok := c.unmask(wire)
		putWire(wirep)
		putWire(plainp)
		if !ok {
			b.Fatal("unmask failed")
		}
	}
}

func salamanderPair(b *testing.B) (client, server net.PacketConn, serverAddr net.Addr) {
	b.Helper()
	rawServer, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		b.Fatal(err)
	}
	rawClient, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		b.Fatal(err)
	}
	server, err = NewPacketConn(rawServer, "salamander", "bench-password")
	if err != nil {
		b.Fatal(err)
	}
	client, err = NewPacketConn(rawClient, "salamander", "bench-password")
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		_ = client.Close()
		_ = server.Close()
	})
	return client, server, rawServer.LocalAddr()
}

// One packet in flight: ns/op is the loopback round-trip of a QUIC-sized datagram.
func BenchmarkSalamanderRTT_1200B(b *testing.B) {
	client, server, addr := salamanderPair(b)
	packet := benchSalamanderPacket(1200)
	buf := make([]byte, 2048)
	b.SetBytes(1200)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := client.WriteTo(packet, addr); err != nil {
			b.Fatal(err)
		}
		_ = server.SetReadDeadline(time.Now().Add(2 * time.Second))
		n, _, err := server.ReadFrom(buf)
		if err != nil || n != 1200 {
			b.Fatalf("read n=%d err=%v", n, err)
		}
	}
}

// Writer blocks on the socket when the reader falls behind, so bytes/sec is the path throughput.
func BenchmarkSalamanderThroughput_1200B(b *testing.B) {
	client, server, addr := salamanderPair(b)
	packet := benchSalamanderPacket(1200)
	done := make(chan struct{})
	go func() {
		buf := make([]byte, 2048)
		for {
			_ = server.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
			_, _, err := server.ReadFrom(buf)
			if err != nil {
				select {
				case <-done:
					return
				default:
				}
			}
		}
	}()
	b.SetBytes(1200)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := client.WriteTo(packet, addr); err != nil {
			b.Fatal(err)
		}
	}
	close(done)
}