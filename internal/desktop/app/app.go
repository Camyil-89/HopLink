package desktop

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
)

type App struct {
	ctx                 context.Context
	mu                  sync.Mutex
	config              AppConfig
	links               map[string]string
	applyCancel         context.CancelFunc
	configPath, mapPath string
	vaultKey, vaultSalt []byte
	vaultLocked         bool
	vaultNeedsSetup     bool
	vaultError          string
	legacyConfig        bool
	legacyPath          string
	legacyMapPath       string
	vaultSourcePath     string
}

func NewApp() *App { return &App{} }
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	executable, err := os.Executable()
	dir := "."
	if err == nil {
		dir = filepath.Dir(executable)
	} else {
		if workingDir, cwdErr := os.Getwd(); cwdErr == nil {
			dir = workingDir
		}
		log.Printf("resolve HopLink executable path: %v; using %s for local data", err, dir)
	}
	a.configPath = filepath.Join(dir, "settings.json")
	a.mapPath = filepath.Join(dir, "map.json")
	legacyDir := ""
	if configDir, configErr := os.UserConfigDir(); configErr == nil {
		legacyDir = filepath.Join(configDir, "HopLink")
	}
	configSourcePath := a.configPath
	a.config = defaultConfig()
	b, e := os.ReadFile(a.configPath)
	if os.IsNotExist(e) && legacyDir != "" {
		configSourcePath = filepath.Join(legacyDir, "settings.json")
		b, e = os.ReadFile(configSourcePath)
		if e == nil {
			log.Printf("loading existing HopLink settings from %s; they will now be saved next to the executable", legacyDir)
		}
	}
	if e == nil {
		var envelope vaultEnvelope
		if json.Unmarshal(b, &envelope) == nil && envelope.Format == vaultFormat {
			a.vaultLocked = true
			a.vaultSourcePath = configSourcePath
			if configSourcePath != a.configPath {
				a.legacyPath = configSourcePath
			}
			a.vaultSalt, _ = base64.RawStdEncoding.DecodeString(envelope.Salt)
		} else if err := json.Unmarshal(b, &a.config); err != nil {
			a.vaultLocked = true
			a.vaultError = "Файл настроек не распознан или повреждён."
			log.Printf("read settings.json: %v", err)
		} else {
			a.vaultNeedsSetup = true
			a.legacyConfig = true
			a.legacyPath = configSourcePath
		}
	} else if !os.IsNotExist(e) {
		a.vaultLocked = true
		a.vaultError = "Не удалось прочитать файл настроек."
		log.Printf("read settings.json: %v", e)
	} else {
		a.vaultNeedsSetup = true
	}
	if a.config.Port == 0 {
		a.config.Port = defaultPort
	}
	if a.config.KnownHosts == nil {
		a.config.KnownHosts = map[string]string{}
	}
	if a.config.Servers == nil {
		a.config.Servers = []Server{}
	}
	if a.config.DeployedServers == nil {
		a.config.DeployedServers = []Server{}
	}
	if a.config.Users == nil {
		a.config.Users = []User{}
	}
	if a.config.Edges == nil {
		a.config.Edges = []Edge{}
	}
	mapSourcePath := a.mapPath
	mapData, mapErr := os.ReadFile(a.mapPath)
	if os.IsNotExist(mapErr) && legacyDir != "" {
		mapSourcePath = filepath.Join(legacyDir, "map.json")
		mapData, mapErr = os.ReadFile(mapSourcePath)
		if mapErr == nil {
			log.Printf("loading existing HopLink map from %s; it will now be saved next to the executable", legacyDir)
		}
	}
	if mapErr == nil {
		var saved MapState
		if err := json.Unmarshal(mapData, &saved); err == nil {
			a.legacyConfig = true
			a.legacyMapPath = mapSourcePath
			a.config.MapServers, a.config.Edges = saved.Servers, saved.Edges
		} else {
			log.Printf("read map.json: %v", err)
		}
	} else if len(a.config.Servers) > 0 {
		// Migrate the initial single-list layout without losing its topology.
		a.config.MapServers = append([]Server(nil), a.config.Servers...)
	}
	if err := a.prepareRoutesLocked(); err != nil {
		log.Printf("prepare routes: %v", err)
	}
	if a.legacyConfig {
		a.vaultLocked = true
	}
}
