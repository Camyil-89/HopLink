package desktop

import (
	"fmt"
	"strings"
)

func (a *App) SaveMap(servers []Server, edges []Edge) (AppConfig, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := validateMap(servers, edges); err != nil {
		return a.config, err
	}
	a.config.MapServers = append([]Server{}, servers...)
	byID := make(map[string]int, len(a.config.Servers))
	for i, s := range a.config.Servers {
		byID[s.ID] = i
	}
	for _, s := range servers {
		if i, ok := byID[s.ID]; ok {
			a.config.Servers[i] = s
		} else {
			a.config.Servers = append(a.config.Servers, s)
		}
	}
	a.config.Edges = append([]Edge{}, edges...)
	if err := a.saveMapLocked(); err != nil {
		return a.config, err
	}
	if err := a.saveLocked(); err != nil {
		return a.config, err
	}
	return a.config, nil
}
func (a *App) SaveRouteDraft(servers []Server, edges []Edge, sni string, port int, entryDomain, chainKey, cert, certKey string) (AppConfig, error) {
	if err := validateMap(servers, edges); err != nil {
		return a.GetState(), err
	}
	if port < 1 || port > 65535 {
		return a.GetState(), fmt.Errorf("порт должен быть от 1 до 65535")
	}
	entryDomain = strings.TrimSpace(entryDomain)
	if err := validateEntryDomain(entryDomain); err != nil {
		return a.GetState(), err
	}
	if (cert == "") != (certKey == "") {
		return a.GetState(), fmt.Errorf("сертификат и приватный ключ нужно задать вместе")
	}
	if sni == "" {
		sni = "www.cloudflare.com"
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.config.MapServers = append([]Server{}, servers...)
	byID := make(map[string]int, len(a.config.Servers))
	for i, server := range a.config.Servers {
		byID[server.ID] = i
	}
	for _, server := range servers {
		if i, ok := byID[server.ID]; ok {
			a.config.Servers[i] = server
		} else {
			a.config.Servers = append(a.config.Servers, server)
		}
	}
	a.config.Edges = append([]Edge{}, edges...)
	a.config.SNI, a.config.Port, a.config.EntryDomain = sni, port, entryDomain
	a.config.ChainKey, a.config.Certificate, a.config.CertificateKey = chainKey, cert, certKey
	return a.config, a.saveLocked()
}
func (a *App) clearRouteMigrationLayoutsLocked(routeID string) {
	for i := range a.config.Routes {
		if a.config.Routes[i].ID == routeID {
			a.config.Routes[i].LegacyLayout = false
			a.config.Routes[i].LegacyIDLayout = false
			return
		}
	}
}
