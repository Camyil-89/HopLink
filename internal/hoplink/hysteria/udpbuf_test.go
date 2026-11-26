package hysteria

import (
	"net"
	"testing"
)

func TestRelayBufferTiers(t *testing.T) {
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer pc.Close()

	GrowUDPBuffers(pc)
	GrowRelayListenBuffers(pc)
	GrowRelaySessionBuffers(pc)
	if udpBufferBytes <= udpRelayListenBytes || udpRelayListenBytes <= udpRelaySessionBytes {
		t.Fatalf("buffer tiers: hy2=%d listen=%d session=%d", udpBufferBytes, udpRelayListenBytes, udpRelaySessionBytes)
	}
}