package desktop

import "testing"

func TestAllRouteRemovalTargetsIncludesEveryRoute(t *testing.T) {
	routes := []Route{
		{ID: "one", Name: "Первый", Index: 1, Port: 22033},
		{ID: "two", Name: "Второй", Index: 2, Port: 22034},
	}
	targets := allRouteRemovalTargets(routes)
	if len(targets) != 2 {
		t.Fatalf("wanted both routes, got %d", len(targets))
	}
	if targets[0].unitName != "hoplink-1.service" || targets[1].unitName != "hoplink-2.service" {
		t.Fatalf("wrong service units: %q and %q", targets[0].unitName, targets[1].unitName)
	}
	if targets[0].port != 22033 || targets[1].port != 22034 {
		t.Fatal("route-specific ports were lost")
	}
}

func TestRemoveServerDeploymentKeepsMapAndClearsEntryLinks(t *testing.T) {
	entry := Server{ID: "entry", Host: "entry.example", SSHPort: 22}
	exit := Server{ID: "exit", Host: "exit.example", SSHPort: 22}
	route := Route{
		MapServers:      []Server{entry, exit},
		DeployedServers: []Server{entry, exit},
		Edges:           []Edge{{Source: entry.ID, Target: exit.ID}},
		Users:           []User{{Name: "user", Link: "hy2://example"}},
	}
	removeServerDeployment(&route, entry)
	if len(route.DeployedServers) != 1 || route.DeployedServers[0].ID != exit.ID {
		t.Fatalf("unexpected deployment state: %#v", route.DeployedServers)
	}
	if route.Users[0].Link != "" {
		t.Fatal("entry connection link should be cleared")
	}
	if len(route.MapServers) != 2 || len(route.Edges) != 1 {
		t.Fatal("saved route map should remain unchanged")
	}
}
