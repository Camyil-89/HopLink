package desktop

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"hoplink/internal/desktop/app/remote"

	"golang.org/x/crypto/ssh"
)

// TrafficVolume is total traffic in kilobytes.
type TrafficVolume struct {
	In  uint64 `json:"in"`
	Out uint64 `json:"out"`
}

// TrafficDay is per-day traffic in kilobytes.
type TrafficDay struct {
	Date string `json:"date"`
	In   uint64 `json:"in"`
	Out  uint64 `json:"out"`
}

// TrafficUser is per-user traffic in kilobytes.
type TrafficUser struct {
	ID   string       `json:"id"`
	In   uint64       `json:"in"`
	Out  uint64       `json:"out"`
	Days []TrafficDay `json:"days"`
}

// ProcessRssDay is per-day RSS aggregates (kilobytes).
type ProcessRssDay struct {
	Last uint64 `json:"last"`
	Max  uint64 `json:"max"`
}

// ProcessCpuDay is per-day CPU aggregates (percent of one core).
type ProcessCpuDay struct {
	Avg float64 `json:"avg"`
	Max float64 `json:"max"`
}

// ProcessDay is calendar-day process resource usage.
type ProcessDay struct {
	Date   string          `json:"date"`
	RssKB  ProcessRssDay   `json:"rssKB"`
	CpuPct ProcessCpuDay   `json:"cpuPct"`
}

// ProcessCurrent is the latest resource sample.
type ProcessCurrent struct {
	RssKB  uint64  `json:"rssKB"`
	CpuPct float64 `json:"cpuPct"`
}

// ProcessStats is hoplink process RAM/CPU from stats.json.
type ProcessStats struct {
	SampledAt         string         `json:"sampledAt,omitempty"`
	SampleIntervalSec int            `json:"sampleIntervalSec,omitempty"`
	Current           ProcessCurrent `json:"current"`
	Days              []ProcessDay   `json:"days"`
}

// HopTrafficStats is the remote stats.json payload from one hop.
type HopTrafficStats struct {
	Total   TrafficVolume  `json:"total"`
	Days    []TrafficDay   `json:"days"`
	Users   []TrafficUser  `json:"users"`
	Process *ProcessStats  `json:"process,omitempty"`
}

// RouteHopStats is traffic stats for one deployed server in a route.
type RouteHopStats struct {
	ServerID     string          `json:"serverId"`
	ServerHost   string          `json:"serverHost"`
	ServerName   string          `json:"serverName"`
	ServiceDir   string          `json:"serviceDir"`
	StatsPath    string          `json:"statsPath"`
	StatsEnabled bool            `json:"statsEnabled"`
	Stats        HopTrafficStats `json:"stats"`
	Missing      bool            `json:"missing"`
	Error        string          `json:"error,omitempty"`
}

// GetRouteTrafficStats returns stats.json from every hop where the route is installed.
func (a *App) GetRouteTrafficStats(routeID string) ([]RouteHopStats, error) {
	a.mu.Lock()
	if a.vaultLocked {
		a.mu.Unlock()
		return nil, fmt.Errorf("разблокируйте настройки перед чтением статистики")
	}
	var selected *Route
	for i := range a.config.Routes {
		if a.config.Routes[i].ID == routeID {
			copy := a.config.Routes[i]
			selected = &copy
			break
		}
	}
	servers := append([]Server(nil), a.config.Servers...)
	a.mu.Unlock()
	if selected == nil {
		return nil, fmt.Errorf("маршрут не найден")
	}
	if len(selected.DeployedServers) == 0 {
		return nil, fmt.Errorf("маршрут ещё не установлен ни на одном сервере")
	}
	units := routeServiceUnits(*selected)
	results := make([]RouteHopStats, 0, len(selected.DeployedServers))
	for _, deployed := range selected.DeployedServers {
		host := deployed.Host
		name := deployed.Host
		for _, s := range servers {
			if s.ID == deployed.ID {
				host = s.Host
				if s.Name != "" {
					name = s.Name
				}
				break
			}
		}
		item := RouteHopStats{
			ServerID:   deployed.ID,
			ServerHost: host,
			ServerName: name,
		}
		stats, diag, err := a.readHopStats(deployed.ID, *selected, units, servers)
		item.ServiceDir = diag.workDir
		item.StatsPath = diag.statsPath
		item.StatsEnabled = diag.statsInUnit
		if err != nil {
			item.Error = err.Error()
		} else if stats.missing {
			item.Missing = true
			if diag.statsPath != "" {
				item.Error = fmt.Sprintf("файл не найден: %s", diag.statsPath)
			} else if len(diag.tried) > 0 {
				item.Error = "stats.json не найден; проверены: " + strings.Join(diag.tried, ", ")
			}
		} else {
			item.Stats = stats.data
		}
		results = append(results, item)
	}
	return results, nil
}

type hopStatsDiag struct {
	workDir     string
	statsPath   string
	statsInUnit bool
	tried       []string
}

type hopStatsResult struct {
	data    HopTrafficStats
	missing bool
}

func routeServiceUnits(route Route) []string {
	seen := map[string]bool{}
	var units []string
	add := func(u string) {
		if u == "" || seen[u] {
			return
		}
		seen[u] = true
		units = append(units, u)
	}
	if route.LegacyLayout {
		add(serviceName)
	}
	if route.LegacyIDLayout {
		id := route.ID
		if len(id) > 32 {
			id = id[:32]
		}
		add("hoplink-" + id + ".service")
	}
	_, unit := routeRemoteLayout(route.ID, []Route{route})
	add(unit)
	for _, prev := range previousRouteLayouts(route.ID, []Route{route}) {
		add(prev.unit)
	}
	return units
}

func statsFileCandidates(route Route) []string {
	seen := map[string]bool{}
	var paths []string
	add := func(dir string) {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			return
		}
		p := dir + "/stats.json"
		if seen[p] {
			return
		}
		seen[p] = true
		paths = append(paths, p)
	}
	install, _ := routeRemoteLayout(route.ID, []Route{route})
	add(install)
	add(routeInstallDir(route))
	for _, prev := range previousRouteLayouts(route.ID, []Route{route}) {
		add(prev.dir)
	}
	add(remoteDir)
	return paths
}

func (a *App) readHopStats(serverID string, route Route, units []string, servers []Server) (hopStatsResult, hopStatsDiag, error) {
	var server Server
	for _, s := range servers {
		if s.ID == serverID {
			server = s
			break
		}
	}
	if server.ID == "" {
		return hopStatsResult{}, hopStatsDiag{}, fmt.Errorf("сервер %s не найден в настройках", serverID)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, err := a.sshConnectContext(ctx, server)
	if err != nil {
		return hopStatsResult{}, hopStatsDiag{}, fmt.Errorf("SSH %s: %w", server.Host, err)
	}
	defer client.Close()

	candidates := statsFileCandidates(route)
	body, diag, err := remoteLoadHoplinkStats(ctx, client, server, units, candidates)
	if err != nil {
		return hopStatsResult{}, diag, err
	}
	diag.tried = candidates
	if strings.TrimSpace(body) == "" {
		if diag.statsPath == "" && len(candidates) > 0 {
			diag.statsPath = candidates[0]
		}
		return hopStatsResult{missing: true}, diag, nil
	}
	var data HopTrafficStats
	if err := json.Unmarshal([]byte(body), &data); err != nil {
		return hopStatsResult{}, diag, fmt.Errorf("неверный stats.json на %s: %w", server.Host, err)
	}
	return hopStatsResult{data: data}, diag, nil
}

func remoteLoadHoplinkStats(ctx context.Context, client *ssh.Client, server Server, units []string, candidates []string) (string, hopStatsDiag, error) {
	var unitQuoted []string
	for _, u := range units {
		unitQuoted = append(unitQuoted, remote.ShellQuote(u))
	}
	var pathQuoted []string
	for _, p := range candidates {
		pathQuoted = append(pathQuoted, remote.ShellQuote(p))
	}
	script := `stats_flag=0
workdir=""
for u in ` + strings.Join(unitQuoted, " ") + `; do
  wd=$(systemctl show "$u" -p WorkingDirectory --value 2>/dev/null || true)
  if [ -z "$wd" ] || [ "$wd" = "n/a" ]; then
    continue
  fi
  exec=$(systemctl show "$u" -p ExecStart --value 2>/dev/null || true)
  case "$exec" in
    *stats-file*) stats_flag=1 ;;
  esac
  if systemctl is-active --quiet "$u" 2>/dev/null; then
    workdir="$wd"
    break
  fi
  if [ -z "$workdir" ]; then
    workdir="$wd"
  fi
done
found=""
if [ -n "$workdir" ] && [ -f "$workdir/stats.json" ]; then
  found="$workdir/stats.json"
fi
if [ -z "$found" ]; then
  for p in ` + strings.Join(pathQuoted, " ") + `; do
    if [ -f "$p" ]; then
      found="$p"
      break
    fi
  done
fi
printf 'WORKDIR=%s\n' "$workdir"
printf 'STATS_FLAG=%s\n' "$stats_flag"
printf 'STATS_PATH=%s\n' "$found"
if [ -n "$found" ]; then
  cat "$found"
fi`
	out, err := runServerContext(ctx, client, server, script, nil)
	if err != nil {
		return "", hopStatsDiag{}, err
	}
	diag := hopStatsDiag{}
	var bodyLines []string
	pastHeader := false
	for _, line := range strings.Split(out, "\n") {
		if !pastHeader {
			trim := strings.TrimSpace(line)
			if after, ok := strings.CutPrefix(trim, "WORKDIR="); ok {
				diag.workDir = after
				continue
			}
			if after, ok := strings.CutPrefix(trim, "STATS_FLAG="); ok {
				diag.statsInUnit = after == "1"
				continue
			}
			if after, ok := strings.CutPrefix(trim, "STATS_PATH="); ok {
				diag.statsPath = after
				if after != "" {
					if i := strings.LastIndex(after, "/"); i > 0 {
						diag.workDir = after[:i]
					}
					pastHeader = true
				}
				continue
			}
			if trim == "" && diag.statsPath != "" {
				pastHeader = true
				continue
			}
		}
		if pastHeader || strings.HasPrefix(strings.TrimSpace(line), "{") {
			pastHeader = true
			bodyLines = append(bodyLines, line)
		}
	}
	if diag.statsPath != "" {
		diag.statsInUnit = true
	}
	body := strings.TrimSpace(strings.Join(bodyLines, "\n"))
	if body == "" {
		if i := strings.Index(out, "{\n"); i >= 0 {
			body = strings.TrimSpace(out[i:])
		} else if i := strings.Index(out, "{"); i >= 0 {
			body = strings.TrimSpace(out[i:])
		}
	}
	return body, diag, nil
}