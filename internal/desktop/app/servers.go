package desktop

import (
	"fmt"
)

func (a *App) SaveServers(servers []Server) (AppConfig, error) {
	seen := map[string]bool{}
	for _, s := range servers {
		if s.ID == "" || seen[s.ID] || s.Host == "" || s.SSHUser == "" || s.SSHPort < 1 || s.SSHPort > 65535 || (s.SSHPassword == "" && s.SSHPrivateKey == "") {
			return a.GetState(), fmt.Errorf("проверьте id, адрес, SSH-пользователя и порт сервера")
		}
		seen[s.ID] = true
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	requested := map[string]bool{}
	for _, s := range servers {
		requested[s.ID] = true
	}
	for _, s := range a.config.Servers {
		if requested[s.ID] {
			continue
		}
		for _, mapped := range a.config.MapServers {
			if mapped.ID == s.ID {
				return a.config, fmt.Errorf("сначала уберите %s с карты и примените изменения", s.Name)
			}
		}
		for _, deployed := range a.config.DeployedServers {
			if deployed.ID == s.ID {
				return a.config, fmt.Errorf("сначала удалите HopLink с %s через карту или вкладку удаления", s.Name)
			}
		}
	}
	a.config.Servers = servers
	return a.config, a.saveLocked()
}

// SaveRouteDraft persists the route map and all route-specific connection settings as one operation.
