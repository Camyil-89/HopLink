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

func TestDisablePingCommandIsIdempotent(t *testing.T) {
	command := DisablePingCommand()
	if !strings.Contains(command, disablePingMarker) {
		t.Fatal("missing hoplink marker")
	}
	if !strings.Contains(command, disablePingDropIn) {
		t.Fatal("missing sysctl drop-in path")
	}
	if !strings.Contains(command, "grep -qs") || !strings.Contains(command, "icmp_echo_ignore_all") {
		t.Fatal("missing idempotent checks")
	}
}
