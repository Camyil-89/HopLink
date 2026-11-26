package hysteria

import "net"

const udpBufferBytes = 8 << 20

func GrowUDPBuffers(pc net.PacketConn) {
	type bufferSetter interface {
		SetReadBuffer(int) error
		SetWriteBuffer(int) error
	}
	c, ok := pc.(bufferSetter)
	if !ok {
		return
	}
	_ = c.SetReadBuffer(udpBufferBytes)
	_ = c.SetWriteBuffer(udpBufferBytes)
}
