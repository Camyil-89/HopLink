package hysteria

import "net"

const (
	// HY2 and cascade entry listeners: high burst tolerance.
	udpBufferBytes = 8 << 20
	// Relay listen socket: many peers, moderate kernel footprint.
	udpRelayListenBytes = 4 << 20
	// Per-relay upstream session socket: one QUIC flow, default-ish kernel buffers.
	udpRelaySessionBytes = 512 << 10
)

func GrowUDPBuffers(pc net.PacketConn) {
	setUDPBuffers(pc, udpBufferBytes)
}

func GrowRelayListenBuffers(pc net.PacketConn) {
	setUDPBuffers(pc, udpRelayListenBytes)
}

func GrowRelaySessionBuffers(pc net.PacketConn) {
	setUDPBuffers(pc, udpRelaySessionBytes)
}

func setUDPBuffers(pc net.PacketConn, nbytes int) {
	type bufferSetter interface {
		SetReadBuffer(int) error
		SetWriteBuffer(int) error
	}
	c, ok := pc.(bufferSetter)
	if !ok {
		return
	}
	_ = c.SetReadBuffer(nbytes)
	_ = c.SetWriteBuffer(nbytes)
}