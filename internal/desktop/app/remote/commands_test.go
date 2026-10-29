package remote

import (
	"strings"
	"testing"
)

func TestCleanupDoesNotChangeHostNetworking(t *testing.T) {
	command := CleanupCommandAt(22033, "/opt/HopLink-1", "hoplink-1.service")
	for _, forbidden := range []string{"ufw", "firewall-cmd", "iptables", "nft ", "sysctl", "tc "} {
		if strings.Contains(command, forbidden) {
			t.Fatalf("cleanup changes host networking: %s", forbidden)
		}
	}
	if !strings.Contains(command, "hoplink-1.service") || strings.Contains(command, "hoplink-2.service") {
		t.Fatal("cleanup lost route scope")
	}
}
