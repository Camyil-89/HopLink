package remote

import "strings"

func ShellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }

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
