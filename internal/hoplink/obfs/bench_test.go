package obfs

import (
	"net"
	"testing"
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
		wire, err := c.mask(packet)
		if err != nil {
			b.Fatal(err)
		}
		_, ok := c.unmask(wire)
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
		wire, err := c.mask(packet)
		if err != nil {
			b.Fatal(err)
		}
		_, ok := c.unmask(wire)
		if !ok {
			b.Fatal("unmask failed")
		}
	}
}