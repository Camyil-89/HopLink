package obfs

import (
	"net"
	"testing"
)

func TestPacketConnRoundTrip(t *testing.T) {
	for _, mode := range []string{"salamander", "gecko"} {
		t.Run(mode, func(t *testing.T) {
			serverRaw, err := net.ListenPacket("udp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer serverRaw.Close()
			server, err := NewPacketConn(serverRaw, mode, "same-test-mask-password")
			if err != nil {
				t.Fatal(err)
			}
			clientRaw, err := net.ListenPacket("udp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer clientRaw.Close()
			client, err := NewPacketConn(clientRaw, mode, "same-test-mask-password")
			if err != nil {
				t.Fatal(err)
			}
			payload := []byte("hello obfs")
			if _, err := client.WriteTo(payload, server.LocalAddr()); err != nil {
				t.Fatal(err)
			}
			buf := make([]byte, 2048)
			n, _, err := server.ReadFrom(buf)
			if err != nil || string(buf[:n]) != string(payload) {
				t.Fatalf("server read = %q, %v", buf[:n], err)
			}
			if _, err := server.WriteTo(buf[:n], client.LocalAddr()); err != nil {
				t.Fatal(err)
			}
			n, _, err = client.ReadFrom(buf)
			if err != nil || string(buf[:n]) != string(payload) {
				t.Fatalf("client read = %q, %v", buf[:n], err)
			}
		})
	}
}

func TestPacketConnRequiresValidModeAndPassword(t *testing.T) {
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer pc.Close()
	if _, err := NewPacketConn(pc, "unknown", "long-enough"); err == nil {
		t.Fatal("expected mode error")
	}
	if _, err := NewPacketConn(pc, "salamander", "abc"); err == nil {
		t.Fatal("expected short password error")
	}
}
