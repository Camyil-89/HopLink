package desktop

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"hoplink/internal/desktop/app/security"
)

const vaultFormat = security.Format

type vaultEnvelope = security.Envelope

type VaultStatus struct {
	Locked     bool   `json:"locked"`
	NeedsSetup bool   `json:"needsSetup"`
	HasData    bool   `json:"hasData"`
	Error      string `json:"error"`
}

type remoteLayout struct {
	dir  string
	unit string
}

func defaultConfig() AppConfig {
	return AppConfig{InstallSource: "github", Servers: []Server{}, MapServers: []Server{}, DeployedServers: []Server{}, Users: []User{}, Edges: []Edge{}, Routes: []Route{}, SNI: "", Port: defaultPort, KnownHosts: map[string]string{}}
}

func deriveVaultKey(password string, salt []byte) []byte {
	return security.DeriveKey(password, salt)
}

func encryptVault(plaintext, key, salt []byte) ([]byte, error) {
	return security.Encrypt(plaintext, key, salt)
}

func decryptVault(data []byte, password string) (AppConfig, []byte, []byte, error) {
	plaintext, key, salt, err := security.Decrypt(data, password)
	if err != nil {
		return AppConfig{}, nil, nil, err
	}
	config := defaultConfig()
	if err := json.Unmarshal(plaintext, &config); err != nil {
		return AppConfig{}, nil, nil, fmt.Errorf("расшифрованный файл настроек повреждён")
	}
	return config, key, salt, nil
}

func (a *App) UnlockVault(password string) error {
	if password == "" {
		return fmt.Errorf("введите пароль")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.vaultLocked || a.vaultNeedsSetup {
		return fmt.Errorf("файл настроек не ожидает разблокировки")
	}
	sourcePath := a.vaultSourcePath
	if sourcePath == "" {
		sourcePath = a.configPath
	}
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("не удалось прочитать файл настроек: %w", err)
	}
	config, key, salt, err := decryptVault(data, password)
	if err != nil {
		return err
	}
	a.config = config
	a.vaultKey, a.vaultSalt = key, salt
	a.vaultLocked = false
	a.vaultError = ""
	if err := a.prepareRoutesLocked(); err != nil {
		a.vaultLocked = true
		return err
	}
	a.links = linksForUsers(a.config.Users)
	if sourcePath != a.configPath || a.legacyMapPath != "" {
		if err := a.saveLocked(); err != nil {
			a.vaultLocked = true
			return fmt.Errorf("настройки расшифрованы, но не удалось завершить перенос рядом с приложением: %w", err)
		}
	}
	return nil
}

func (a *App) SetVaultPassword(password string) error {
	if password == "" {
		return fmt.Errorf("пароль шифрования не может быть пустым")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.vaultNeedsSetup {
		return fmt.Errorf("пароль шифрования уже настроен")
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return err
	}
	a.vaultKey = deriveVaultKey(password, salt)
	a.vaultSalt = salt
	a.vaultLocked = false
	a.storeActiveRouteLocked()
	if err := a.prepareRoutesLocked(); err != nil {
		return err
	}
	if err := a.saveLocked(); err != nil {
		a.vaultLocked = true
		return err
	}
	a.links = linksForUsers(a.config.Users)
	return nil
}

func linksForUsers(users []User) map[string]string {
	links := make(map[string]string, len(users))
	for _, user := range users {
		if user.isActive() && user.Link != "" {
			links[user.Name] = user.Link
		}
	}
	return links
}

func (a *App) prepareRoutesLocked() error {
	if a.config.Servers == nil {
		a.config.Servers = []Server{}
	}
	if a.config.KnownHosts == nil {
		a.config.KnownHosts = map[string]string{}
	}
	if a.config.Port == 0 {
		a.config.Port = defaultPort
	}
	for i := range a.config.Users {
		if a.config.Users[i].Active == nil {
			a.config.Users[i].Active = boolPointer(true)
		}
	}
	hasLegacyRouteData := len(a.config.MapServers) > 0 || len(a.config.DeployedServers) > 0 || len(a.config.Users) > 0 || len(a.config.Edges) > 0 || a.config.ChainKey != "" || a.config.Certificate != "" || a.config.CertificateKey != ""
	if len(a.config.Routes) == 0 && hasLegacyRouteData {
		a.config.Routes = []Route{{
			ID: newID(), Name: "Маршрут 1", Index: 1, LegacyLayout: true, MapServers: append([]Server{}, a.config.MapServers...),
			DeployedServers: append([]Server{}, a.config.DeployedServers...), Users: append([]User{}, a.config.Users...),
			Edges: append([]Edge{}, a.config.Edges...), SNI: a.config.SNI, Port: a.config.Port, EntryDomain: a.config.EntryDomain,
			ChainKey: a.config.ChainKey, Certificate: a.config.Certificate, CertificateKey: a.config.CertificateKey,
			ObfsType: a.config.ObfsType, ObfsPassword: a.config.ObfsPassword,
		}}
		a.config.ActiveRouteID = a.config.Routes[0].ID
	}
	found := false
	maxIndex := 0
	for _, route := range a.config.Routes {
		if route.Index > maxIndex {
			maxIndex = route.Index
		}
	}
	for i := range a.config.Routes {
		route := &a.config.Routes[i]
		if route.ID == "" {
			route.ID = newID()
		}
		if route.Name == "" {
			route.Name = fmt.Sprintf("Маршрут %d", i+1)
		}
		if route.Index < 1 {
			maxIndex++
			route.Index = maxIndex
			if !route.LegacyLayout && len(route.DeployedServers) > 0 {
				route.LegacyIDLayout = true
			}
		}
		if route.MapServers == nil {
			route.MapServers = []Server{}
		}
		if route.DeployedServers == nil {
			route.DeployedServers = []Server{}
		}
		if route.Users == nil {
			route.Users = []User{}
		}
		for userIndex := range route.Users {
			if route.Users[userIndex].Active == nil {
				route.Users[userIndex].Active = boolPointer(true)
			}
		}
		if route.Edges == nil {
			route.Edges = []Edge{}
		}
		if route.Port == 0 {
			route.Port = defaultPort
		}
		if route.LogLevel == "" {
			route.LogLevel = "warn"
		}
		if !validObfsType(route.ObfsType) {
			route.ObfsType = ""
			route.ObfsPassword = ""
		}
		if route.ObfsType == "" {
			route.ObfsPassword = ""
		}
		if !validLogLevel(route.LogLevel) {
			route.LogLevel = "warn"
		}
		if !validRelayTransport(route.RelayTransport) {
			route.RelayTransport = "udp"
		}
		if route.RelayTransport == "" {
			route.RelayTransport = "udp"
		}
		if route.ID == a.config.ActiveRouteID {
			found = true
		}
	}
	if !found && len(a.config.Routes) > 0 {
		a.config.ActiveRouteID = a.config.Routes[0].ID
	} else if len(a.config.Routes) == 0 {
		a.config.ActiveRouteID = ""
	}
	a.loadActiveRouteLocked()
	return nil
}

func routeRemoteLayout(routeID string, routes []Route) (string, string) {
	for _, route := range routes {
		if route.ID != routeID {
			continue
		}
		return indexedRouteLayout(route.Index)
	}
	return remoteDir, serviceName
}

func indexedRouteLayout(index int) (string, string) {
	if index < 1 {
		index = 1
	}
	name := fmt.Sprintf("hoplink-%d", index)
	return "/opt/" + name, name + ".service"
}

func previousRouteLayouts(routeID string, routes []Route) []remoteLayout {
	for _, route := range routes {
		if route.ID != routeID {
			continue
		}
		layouts := []remoteLayout{}
		if route.LegacyLayout {
			layouts = append(layouts, remoteLayout{dir: remoteDir, unit: serviceName})
		}
		if route.LegacyIDLayout {
			id := route.ID
			if len(id) > 32 {
				id = id[:32]
			}
			layouts = append(layouts, remoteLayout{dir: "/opt/hoplink-routes/" + id, unit: "hoplink-" + id + ".service"})
		}
		return layouts
	}
	return nil
}

func (a *App) storeActiveRouteLocked() {
	for i := range a.config.Routes {
		if a.config.Routes[i].ID != a.config.ActiveRouteID {
			continue
		}
		route := &a.config.Routes[i]
		route.MapServers = append([]Server{}, a.config.MapServers...)
		route.DeployedServers = append([]Server{}, a.config.DeployedServers...)
		route.Users = append([]User{}, a.config.Users...)
		route.Edges = append([]Edge{}, a.config.Edges...)
		route.SNI, route.Port = a.config.SNI, a.config.Port
		route.EntryDomain = a.config.EntryDomain
		route.ChainKey, route.Certificate, route.CertificateKey = a.config.ChainKey, a.config.Certificate, a.config.CertificateKey
		route.LogLevel = a.config.LogLevel
		route.ObfsType, route.ObfsPassword = a.config.ObfsType, a.config.ObfsPassword
		route.RelayTransport = a.config.RelayTransport
		route.RelayTransportClientKey = a.config.RelayTransportClientKey
		route.RelayTransportClientPub = a.config.RelayTransportClientPub
		route.RelayTransportHostKey = a.config.RelayTransportHostKey
		route.RelayTransportHostPub = a.config.RelayTransportHostPub
		return
	}
}

func (a *App) loadActiveRouteLocked() {
	for _, route := range a.config.Routes {
		if route.ID != a.config.ActiveRouteID {
			continue
		}
		a.config.MapServers = append([]Server{}, route.MapServers...)
		a.config.DeployedServers = append([]Server{}, route.DeployedServers...)
		a.config.Users = append([]User{}, route.Users...)
		a.config.Edges = append([]Edge{}, route.Edges...)
		a.config.SNI, a.config.Port = route.SNI, route.Port
		a.config.EntryDomain = route.EntryDomain
		a.config.ChainKey, a.config.Certificate, a.config.CertificateKey = route.ChainKey, route.Certificate, route.CertificateKey
		a.config.LogLevel = route.LogLevel
		a.config.ObfsType, a.config.ObfsPassword = route.ObfsType, route.ObfsPassword
		a.config.RelayTransport = route.RelayTransport
		a.config.RelayTransportClientKey = route.RelayTransportClientKey
		a.config.RelayTransportClientPub = route.RelayTransportClientPub
		a.config.RelayTransportHostKey = route.RelayTransportHostKey
		a.config.RelayTransportHostPub = route.RelayTransportHostPub
		if a.config.RelayTransport == "" {
			a.config.RelayTransport = "udp"
		}
		a.links = linksForUsers(a.config.Users)
		return
	}
}

func (a *App) SelectRoute(id string) (AppConfig, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.storeActiveRouteLocked()
	found := false
	for _, route := range a.config.Routes {
		if route.ID == id {
			found = true
			break
		}
	}
	if !found {
		return a.config, fmt.Errorf("маршрут не найден")
	}
	a.config.ActiveRouteID = id
	a.loadActiveRouteLocked()
	if err := a.saveLocked(); err != nil {
		return a.config, err
	}
	return a.config, nil
}

// DeleteRoute removes the route's installation, then irreversibly deletes its
// saved map, settings, users, and connection links. Other routes are untouched.
func (a *App) DeleteRoute(id string) (AppConfig, error) {
	a.mu.Lock()
	if a.vaultLocked || len(a.vaultKey) == 0 {
		a.mu.Unlock()
		return a.GetState(), fmt.Errorf("разблокируйте настройки перед удалением маршрута")
	}
	a.storeActiveRouteLocked()
	var target Route
	found := false
	for _, route := range a.config.Routes {
		if route.ID == id {
			target, found = route, true
			break
		}
	}
	a.mu.Unlock()
	if !found {
		return a.GetState(), fmt.Errorf("маршрут не найден")
	}
	if len(target.DeployedServers) > 0 {
		if _, err := a.RemoveRouteService(id); err != nil {
			return a.GetState(), fmt.Errorf("не удалось удалить установленный сервис маршрута: %w", err)
		}
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	a.storeActiveRouteLocked()
	index := -1
	for i := range a.config.Routes {
		if a.config.Routes[i].ID == id {
			index = i
			break
		}
	}
	if index < 0 {
		return a.config, fmt.Errorf("маршрут уже удалён")
	}
	wasActive := a.config.ActiveRouteID == id
	a.config.Routes = append(a.config.Routes[:index], a.config.Routes[index+1:]...)
	if wasActive {
		if len(a.config.Routes) > 0 {
			a.config.ActiveRouteID = a.config.Routes[0].ID
			a.loadActiveRouteLocked()
		} else {
			a.config.ActiveRouteID = ""
			a.config.MapServers = []Server{}
			a.config.DeployedServers = []Server{}
			a.config.Users = []User{}
			a.config.Edges = []Edge{}
			a.config.EntryDomain = ""
			a.config.SNI = ""
			a.config.Port = defaultPort
			a.config.ChainKey = ""
			a.config.Certificate = ""
			a.config.CertificateKey = ""
			a.config.LogLevel = "warn"
			a.config.ObfsType = ""
			a.config.ObfsPassword = ""
			a.links = map[string]string{}
		}
	}
	if err := a.saveLocked(); err != nil {
		return a.config, err
	}
	return a.config, nil
}

func (a *App) CreateRoute(name string) (AppConfig, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	name = strings.TrimSpace(name)
	if name == "" {
		return a.config, fmt.Errorf("укажите название маршрута")
	}
	a.storeActiveRouteLocked()
	nextIndex := 1
	for _, existing := range a.config.Routes {
		if existing.Index >= nextIndex {
			nextIndex = existing.Index + 1
		}
	}
	route := Route{ID: newID(), Name: name, Index: nextIndex, MapServers: []Server{}, DeployedServers: []Server{}, Users: []User{}, Edges: []Edge{}, SNI: a.config.SNI, Port: a.config.Port, LogLevel: "warn", ObfsType: a.config.ObfsType, ObfsPassword: a.config.ObfsPassword}
	a.config.Routes = append(a.config.Routes, route)
	a.config.ActiveRouteID = route.ID
	a.loadActiveRouteLocked()
	if err := a.saveLocked(); err != nil {
		return a.config, err
	}
	return a.config, nil
}

func (a *App) RenameRoute(id, name string) (AppConfig, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	name = strings.TrimSpace(name)
	if name == "" {
		return a.config, fmt.Errorf("укажите название маршрута")
	}
	a.storeActiveRouteLocked()
	for i := range a.config.Routes {
		if a.config.Routes[i].ID == id {
			a.config.Routes[i].Name = name
			if err := a.saveLocked(); err != nil {
				return a.config, err
			}
			return a.config, nil
		}
	}
	return a.config, fmt.Errorf("маршрут не найден")
}
