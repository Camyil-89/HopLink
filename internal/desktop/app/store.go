package desktop

import (
	"encoding/json"
	"fmt"
	"os"
)

func (a *App) GetState() AppConfig {
	a.mu.Lock()
	defer a.mu.Unlock()
	cfg := a.config
	cfg.CertificateLifetime = formatCertificateLifetime(cfg.Certificate)
	return cfg
}
func (a *App) GetVaultStatus() VaultStatus {
	a.mu.Lock()
	defer a.mu.Unlock()
	return VaultStatus{Locked: a.vaultLocked, NeedsSetup: a.vaultNeedsSetup, Error: a.vaultError, HasData: a.legacyConfig || (a.vaultLocked && !a.vaultNeedsSetup) || len(a.config.Servers) > 0}
}
func (a *App) GetLinks() map[string]string {
	a.mu.Lock()
	defer a.mu.Unlock()
	links := make(map[string]string, len(a.links))
	for k, v := range a.links {
		links[k] = v
	}
	return links
}
func (a *App) saveLocked() error {
	if a.vaultLocked || len(a.vaultKey) == 0 {
		return fmt.Errorf("задайте пароль шифрования настроек перед сохранением")
	}
	a.storeActiveRouteLocked()
	data, e := json.MarshalIndent(a.config, "", "  ")
	if e != nil {
		return e
	}
	sealed, e := encryptVault(data, a.vaultKey, a.vaultSalt)
	if e != nil {
		return e
	}
	tmp := a.configPath + ".tmp"
	if e = os.WriteFile(tmp, sealed, 0600); e != nil {
		return e
	}
	if e = os.Rename(tmp, a.configPath); e != nil {
		_ = os.Remove(a.configPath)
		if e = os.Rename(tmp, a.configPath); e != nil {
			return e
		}
	}
	if a.legacyPath != "" && a.legacyPath != a.configPath {
		_ = os.Remove(a.legacyPath)
	}
	if a.legacyMapPath != "" {
		_ = os.Remove(a.legacyMapPath)
	}
	if a.mapPath != "" && a.mapPath != a.legacyMapPath {
		_ = os.Remove(a.mapPath)
	}
	a.legacyPath = ""
	a.legacyMapPath = ""
	a.legacyConfig = false
	a.vaultNeedsSetup = false
	a.vaultSourcePath = a.configPath
	return nil
}
func (a *App) saveMapLocked() error {
	// Maps are stored inside encrypted settings.json with their route.
	return nil
}
