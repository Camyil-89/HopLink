package desktop

import (
	"path/filepath"
	"testing"
)

func TestDeleteRoutePreservesOtherRoutesAndAllowsEmptyRouteList(t *testing.T) {
	app := &App{
		config: AppConfig{
			Routes: []Route{
				{ID: "first", Name: "Первый", Index: 1, SNI: "first.example", Port: 22033, MapServers: []Server{}, DeployedServers: []Server{}, Users: []User{}, Edges: []Edge{}},
				{ID: "second", Name: "Второй", Index: 2, SNI: "second.example", Port: 22034, MapServers: []Server{}, DeployedServers: []Server{}, Users: []User{}, Edges: []Edge{}},
			},
			ActiveRouteID: "first", Servers: []Server{{ID: "shared-server", Host: "shared.example"}},
		},
		vaultKey:   make([]byte, 32),
		vaultSalt:  []byte("0123456789abcdef"),
		configPath: filepath.Join(t.TempDir(), "settings.json"),
	}

	state, err := app.DeleteRoute("first")
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Routes) != 1 || state.Routes[0].ID != "second" || state.ActiveRouteID != "second" || state.Port != 22034 {
		t.Fatalf("other route was not preserved/selected: %#v", state)
	}
	if len(state.Servers) != 1 || state.Servers[0].ID != "shared-server" {
		t.Fatal("deleting a route removed a shared server")
	}

	state, err = app.DeleteRoute("second")
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Routes) != 0 || state.ActiveRouteID != "" {
		t.Fatalf("last route was not deleted: %#v", state.Routes)
	}
	if err := app.prepareRoutesLocked(); err != nil {
		t.Fatal(err)
	}
	if len(app.config.Routes) != 0 {
		t.Fatal("loading an empty saved route list recreated a deleted route")
	}
}

func TestPrepareRoutesKeepsExplicitlyEmptySNI(t *testing.T) {
	app := &App{config: AppConfig{
		SNI:           "",
		ActiveRouteID: "route",
		Routes:        []Route{{ID: "route", Name: "Без SNI", Port: defaultPort, MapServers: []Server{}, DeployedServers: []Server{}, Users: []User{}, Edges: []Edge{}}},
	}}
	if err := app.prepareRoutesLocked(); err != nil {
		t.Fatal(err)
	}
	if app.config.SNI != "" || app.config.Routes[0].SNI != "" {
		t.Fatalf("empty SNI was replaced: config=%q route=%q", app.config.SNI, app.config.Routes[0].SNI)
	}
}
