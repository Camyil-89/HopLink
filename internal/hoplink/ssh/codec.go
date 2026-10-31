package ssh

import (
	"io"

	"hoplink/internal/hoplink/wire"
)

const (
	udpChannel       = "hoplink-udp-relay"
	maxUDPPacketSize = wire.MaxPayload
	peerQueueDepth   = 8
)

func writeUDP(w io.Writer, src string, payload []byte) error {
	return wire.Write(w, src, payload)
}

func readUDP(r io.Reader) (string, []byte, error) {
	return wire.Read(r)
}

// WriteUDPFrame encodes a relay datagram for tests.
func WriteUDPFrame(w io.Writer, src string, payload []byte) error {
	return wire.Write(w, src, payload)
}

// ReadUDPFrame decodes a relay datagram for tests.
func ReadUDPFrame(r io.Reader) (string, []byte, error) {
	return wire.Read(r)
}
