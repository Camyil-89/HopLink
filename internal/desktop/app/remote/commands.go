package remote

import "strings"

const (
	disablePingMarker = "hoplink-disable-icmp"
	disablePingDropIn = "/etc/sysctl.d/99-hoplink-no-ping.conf"
)

func ShellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }

// DisablePingCommand turns off ICMP echo replies idempotently and never rolls back on route removal.
func DisablePingCommand() string {
	dropIn := ShellQuote(disablePingDropIn)
	marker := ShellQuote(disablePingMarker)
	return "grep -qs " + marker + " " + dropIn + " 2>/dev/null && exit 0; " +
		"sysctl -n net.ipv4.icmp_echo_ignore_all 2>/dev/null | grep -qx 1 && exit 0; " +
		"printf '%s\\n' '# " + disablePingMarker + "' 'net.ipv4.icmp_echo_ignore_all=1' 'net.ipv6.icmp.echo_ignore_all=1' > " + dropIn + " && " +
		"(sysctl --system >/dev/null 2>&1 || sysctl -p " + dropIn + " >/dev/null 2>&1 || true)"
}

func CleanupCommandAt(_ int, installDir, unitName string) string {
	unitBase := strings.TrimSuffix(unitName, ".service")
	units := ShellQuote("/etc/systemd/system/" + unitName)
	if unitBase != "hoplink" {
		units += " /etc/systemd/system/" + unitBase + "-*.service"
	}
	unitCleanup := "for unit in " + units + "; do [ -e \"$unit\" ] || continue; unit_file=${unit##*/}; systemctl disable --now \"$unit_file\" 2>/dev/null || true; rm -f -- \"$unit\"; done; systemctl daemon-reload; "
	// Route lifecycle owns its files and service only, never host networking.
	return unitCleanup + "rm -rf -- " + ShellQuote(installDir)
}
