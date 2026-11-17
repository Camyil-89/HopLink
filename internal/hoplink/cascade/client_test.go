package cascade

import (
	"net"
	"testing"
)

func TestObfsConnFactoryBindsLocally(t *testing.T) {
	f := obfsConnFactory{}
	remote, err := net.ResolveUDPAddr("udp", "203.0.113.5:22111")
	if err != nil {
		t.Fatal(err)
	}
	pc, err := f.New(remote)
	if err != nil {
		t.Fatal(err)
	}
	defer pc.Close()
	la, ok := pc.LocalAddr().(*net.UDPAddr)
	if !ok || la.Port == 0 {
		t.Fatalf("local addr %v", pc.LocalAddr())
	}
}