package remote

import (
	"fmt"
	"strings"
)

func ShellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }

func CleanupCommandAt(port int, installDir, unitName string) string {
	unitBase := strings.TrimSuffix(unitName, ".service")
	units := ShellQuote("/etc/systemd/system/" + unitName)
	if unitBase != "hoplink" {
		units += " /etc/systemd/system/" + unitBase + "-*.service"
	}
	unitCleanup := "for unit in " + units + "; do [ -e \"$unit\" ] || continue; unit_file=${unit##*/}; systemctl disable --now \"$unit_file\" 2>/dev/null || true; rm -f -- \"$unit\"; done; systemctl daemon-reload; "
	return fmt.Sprintf("if [ -f %s ]; then kind=$(cat %s); if [ \"$kind\" = ufw ] && command -v ufw >/dev/null 2>&1; then ufw --force delete allow %d/udp comment 'HopLink' || true; elif [ \"$kind\" = firewalld ] && command -v firewall-cmd >/dev/null 2>&1; then firewall-cmd --permanent --remove-port=%d/udp || true; firewall-cmd --reload || true; fi; fi; "+unitCleanup+"rm -rf -- %s", ShellQuote(installDir+"/.firewall-managed"), ShellQuote(installDir+"/.firewall-managed"), port, port, ShellQuote(installDir))
}
