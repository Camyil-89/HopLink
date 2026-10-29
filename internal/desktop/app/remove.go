package desktop

import (
	"context"
	"fmt"
	"hoplink/internal/desktop/remote"
)

type routeRemovalTarget struct {
	routeID    string
	routeName  string
	port       int
	installDir string
	unitName   string
	oldLayouts []remoteLayout
}

func allRouteRemovalTargets(routes []Route) []routeRemovalTarget {
	targets := make([]routeRemovalTarget, 0, len(routes))
	for _, route := range routes {
		installDir, unitName := indexedRouteLayout(route.Index)
		targets = append(targets, routeRemovalTarget{
			routeID: route.ID, routeName: route.Name, port: route.Port,
			installDir: installDir, unitName: unitName,
			oldLayouts: previousRouteLayouts(route.ID, routes),
		})
	}
	return targets
}

func removeServerDeployment(route *Route, server Server) {
	wasEntry := routeIsEntry(*route, server.ID)
	remaining := make([]Server, 0, len(route.DeployedServers))
	for _, deployed := range route.DeployedServers {
		if deployed.ID != server.ID && !(deployed.Host == server.Host && deployed.SSHPort == server.SSHPort) {
			remaining = append(remaining, deployed)
		}
	}
	route.DeployedServers = remaining
	if wasEntry {
		for i := range route.Users {
			route.Users[i].Link = ""
		}
	}
	if len(remaining) == 0 {
		route.LegacyLayout = false
		route.LegacyIDLayout = false
	}
}

// RemoveHoplink cleans every configured route on each selected server. Route
// definitions and maps stay intact, while deployment state is saved per route
// so a later SSH failure does not restore already removed services in the UI.
func (a *App) RemoveHoplink(ids []string) (AppConfig, error) {
	a.mu.Lock()
	if a.vaultLocked || len(a.vaultKey) == 0 {
		a.mu.Unlock()
		return a.GetState(), fmt.Errorf("разблокируйте настройки перед удалением сервиса")
	}
	a.storeActiveRouteLocked()
	known := make(map[string]Server, len(a.config.Servers))
	for _, server := range a.config.Servers {
		known[server.ID] = server
	}
	targets := allRouteRemovalTargets(a.config.Routes)
	a.mu.Unlock()

	if len(ids) == 0 {
		return a.GetState(), fmt.Errorf("выберите серверы для удаления")
	}
	if len(targets) == 0 {
		return a.GetState(), fmt.Errorf("нет маршрутов для удаления")
	}
	selected := make([]Server, 0, len(ids))
	seen := map[string]bool{}
	for _, id := range ids {
		server, ok := known[id]
		if !ok {
			return a.GetState(), fmt.Errorf("сервер не найден")
		}
		if !seen[id] {
			selected = append(selected, server)
			seen[id] = true
		}
	}

	for _, server := range selected {
		for _, target := range targets {
			a.log(fmt.Sprintf("Удаляю маршрут «%s» на %s…", target.routeName, server.Host))
			if err := a.removeOneAtContext(context.Background(), server, target.port, target.installDir, target.unitName); err != nil {
				return a.GetState(), fmt.Errorf("удаление маршрута «%s» на %s: %w", target.routeName, server.Host, err)
			}
			if err := a.removePreviousLayoutsContext(context.Background(), server, target.port, target.oldLayouts); err != nil {
				return a.GetState(), fmt.Errorf("удаление старой службы маршрута «%s» на %s: %w", target.routeName, server.Host, err)
			}
			a.mu.Lock()
			for i := range a.config.Routes {
				if a.config.Routes[i].ID == target.routeID {
					removeServerDeployment(&a.config.Routes[i], server)
					break
				}
			}
			a.loadActiveRouteLocked()
			saveErr := a.saveLocked()
			a.mu.Unlock()
			if saveErr != nil {
				return a.GetState(), fmt.Errorf("маршрут «%s» удалён на %s, но локальный статус не сохранён: %w", target.routeName, server.Host, saveErr)
			}
			a.log(fmt.Sprintf("Маршрут «%s» удалён на %s", target.routeName, server.Host))
		}
	}
	return a.GetState(), nil
}

// RemoveRouteService removes only the selected route's installation from its deployed servers.
// The route definition, map, settings and users remain available for a later deployment.
func (a *App) RemoveRouteService(routeID string) (AppConfig, error) {
	a.mu.Lock()
	if a.vaultLocked || len(a.vaultKey) == 0 {
		a.mu.Unlock()
		return a.GetState(), fmt.Errorf("разблокируйте настройки перед удалением сервиса")
	}
	a.storeActiveRouteLocked()
	var route Route
	found := false
	for _, candidate := range a.config.Routes {
		if candidate.ID == routeID {
			route, found = candidate, true
			break
		}
	}
	installDir, unitName := routeRemoteLayout(routeID, a.config.Routes)
	oldLayouts := previousRouteLayouts(routeID, a.config.Routes)
	a.mu.Unlock()
	if !found {
		return a.GetState(), fmt.Errorf("маршрут не найден")
	}
	if len(route.DeployedServers) == 0 {
		return a.GetState(), fmt.Errorf("сервис маршрута не установлен")
	}
	for _, server := range route.DeployedServers {
		a.log("Удаляю сервис маршрута «" + route.Name + "» с " + server.Host + "…")
		if err := a.removeOneAtContext(context.Background(), server, route.Port, installDir, unitName); err != nil {
			a.log("Ошибка удаления на " + server.Host + ": " + err.Error())
			return a.GetState(), err
		}
		if err := a.removePreviousLayoutsContext(context.Background(), server, route.Port, oldLayouts); err != nil {
			a.log("Ошибка удаления старой службы на " + server.Host + ": " + err.Error())
			return a.GetState(), err
		}
		a.mu.Lock()
		for i := range a.config.Routes {
			if a.config.Routes[i].ID != routeID {
				continue
			}
			current := &a.config.Routes[i]
			remaining := make([]Server, 0, len(current.DeployedServers))
			for _, deployed := range current.DeployedServers {
				if deployed.ID != server.ID {
					remaining = append(remaining, deployed)
				}
			}
			current.DeployedServers = remaining
			for userIndex := range current.Users {
				current.Users[userIndex].Link = ""
			}
			if len(remaining) == 0 {
				current.LegacyLayout = false
				current.LegacyIDLayout = false
			}
			if a.config.ActiveRouteID == routeID {
				a.config.DeployedServers = append([]Server{}, remaining...)
				a.clearUserLinksLocked()
			}
			break
		}
		saveErr := a.saveLocked()
		a.mu.Unlock()
		if saveErr != nil {
			return a.GetState(), fmt.Errorf("сервис удалён с %s, но локальный статус не сохранён: %w", server.Host, saveErr)
		}
		a.log("Сервис маршрута удалён: " + server.Host)
	}
	return a.GetState(), nil
}

// DeleteServer removes the remote HopLink installation, if present, then removes the server from the library and map.
func (a *App) DeleteServer(id string) (AppConfig, error) {
	a.mu.Lock()
	a.storeActiveRouteLocked()
	var server Server
	found := false
	for _, s := range a.config.Servers {
		if s.ID == id {
			server, found = s, true
			break
		}
	}
	type deployment struct {
		port       int
		dir        string
		unit       string
		oldLayouts []remoteLayout
	}
	deployments := []deployment{}
	for _, route := range a.config.Routes {
		for _, deployed := range route.DeployedServers {
			if deployed.ID == id {
				dir, unit := routeRemoteLayout(route.ID, a.config.Routes)
				deployments = append(deployments, deployment{port: route.Port, dir: dir, unit: unit, oldLayouts: previousRouteLayouts(route.ID, a.config.Routes)})
				break
			}
		}
	}
	a.mu.Unlock()
	if !found {
		return a.GetState(), fmt.Errorf("сервер не найден")
	}
	for _, deployment := range deployments {
		a.log("Удаляю HopLink перед удалением сервера из списка: " + server.Host)
		if err := a.removeOneAtContext(context.Background(), server, deployment.port, deployment.dir, deployment.unit); err != nil {
			a.log("Ошибка удаления HopLink на " + server.Host + ": " + err.Error())
			return a.GetState(), err
		}
		if err := a.removePreviousLayoutsContext(context.Background(), server, deployment.port, deployment.oldLayouts); err != nil {
			a.log("Ошибка удаления предыдущего сервиса HopLink на " + server.Host + ": " + err.Error())
			return a.GetState(), err
		}
		a.log("Сервис и файлы HopLink удалены: " + server.Host)
	}
	a.mu.Lock()
	servers := a.config.Servers[:0]
	for _, s := range a.config.Servers {
		if s.ID != id {
			servers = append(servers, s)
		}
	}
	a.config.Servers = servers
	for routeIndex := range a.config.Routes {
		route := &a.config.Routes[routeIndex]
		wasEntry := routeIsEntry(*route, id)
		mapServers := route.MapServers[:0]
		for _, mapped := range route.MapServers {
			if mapped.ID != id {
				mapServers = append(mapServers, mapped)
			}
		}
		route.MapServers = append([]Server{}, mapServers...)
		edges := route.Edges[:0]
		for _, edge := range route.Edges {
			if edge.Source != id && edge.Target != id {
				edges = append(edges, edge)
			}
		}
		route.Edges = append([]Edge{}, edges...)
		deployedServers := route.DeployedServers[:0]
		for _, deployed := range route.DeployedServers {
			if deployed.ID != id {
				deployedServers = append(deployedServers, deployed)
			}
		}
		route.DeployedServers = append([]Server{}, deployedServers...)
		if len(deployedServers) == 0 {
			route.LegacyLayout = false
			route.LegacyIDLayout = false
		}
		if wasEntry {
			for userIndex := range route.Users {
				route.Users[userIndex].Link = ""
			}
		}
	}
	a.loadActiveRouteLocked()
	if err := a.saveLocked(); err != nil {
		a.mu.Unlock()
		return a.GetState(), err
	}
	config := a.config
	a.mu.Unlock()
	a.log("Сервер удалён из библиотеки и маршрутов: " + server.Host)
	return config, nil
}
func (a *App) removeOne(s Server, port int) error {
	return a.removeOneContext(context.Background(), s, port)
}

func (a *App) removeOneContext(ctx context.Context, s Server, port int) error {
	return a.removeOneAtContext(ctx, s, port, remoteDir, serviceName)
}

func (a *App) removeOneAtContext(ctx context.Context, s Server, port int, installDir, unitName string) error {
	c, err := a.sshConnectContext(ctx, s)
	if err != nil {
		return fmt.Errorf("remote SSH operation failed: %w", err)
	}
	defer c.Close()
	if _, err = runServerContext(ctx, c, s, remote.CleanupCommandAt(port, installDir, unitName), nil); err != nil {
		return fmt.Errorf("remote SSH operation failed: %w", err)
	}
	return nil
}

func (a *App) removePreviousLayoutsContext(ctx context.Context, s Server, port int, layouts []remoteLayout) error {
	for _, layout := range layouts {
		if err := a.removeOneAtContext(ctx, s, port, layout.dir, layout.unit); err != nil {
			return err
		}
	}
	return nil
}
