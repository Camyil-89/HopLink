package desktop

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"hoplink/internal/desktop/remote"
)

type ServerServiceStatus struct {
	ServerID string `json:"serverId"`
	Running  bool   `json:"running"`
	Error    string `json:"error,omitempty"`
}

type RouteServiceStatus struct {
	RouteID string                `json:"routeId"`
	Running int                   `json:"running"`
	Total   int                   `json:"total"`
	Servers []ServerServiceStatus `json:"servers"`
}

// CheckRouteServices is called only by the UI's explicit Check button. It does
// not save the result or change the route's deployment metadata.
func (a *App) CheckRouteServices() ([]RouteServiceStatus, error) {
	a.mu.Lock()
	if a.vaultLocked {
		a.mu.Unlock()
		return nil, fmt.Errorf("разблокируйте настройки перед проверкой сервисов")
	}
	routes := append([]Route(nil), a.config.Routes...)
	knownServers := make(map[string]Server, len(a.config.Servers))
	for _, server := range a.config.Servers {
		knownServers[server.ID] = server
	}
	a.mu.Unlock()

	statuses := make([]RouteServiceStatus, len(routes))
	var workers sync.WaitGroup
	limit := make(chan struct{}, 4)
	for routeIndex, route := range routes {
		statuses[routeIndex] = RouteServiceStatus{
			RouteID: route.ID,
			Total:   len(route.MapServers),
			Servers: make([]ServerServiceStatus, len(route.MapServers)),
		}
		_, unit := indexedRouteLayout(route.Index)
		units := []string{unit}
		for _, previous := range previousRouteLayouts(route.ID, routes) {
			units = append(units, previous.unit)
		}
		for serverIndex, mapped := range route.MapServers {
			workers.Add(1)
			limit <- struct{}{}
			go func(routeIndex, serverIndex int, mapped Server, units []string) {
				defer workers.Done()
				defer func() { <-limit }()
				server, ok := knownServers[mapped.ID]
				if !ok {
					statuses[routeIndex].Servers[serverIndex] = ServerServiceStatus{ServerID: mapped.ID, Error: "Сервер удалён из библиотеки"}
					return
				}
				statuses[routeIndex].Servers[serverIndex] = a.checkServerService(server, units)
			}(routeIndex, serverIndex, mapped, units)
		}
	}
	workers.Wait()
	for i := range statuses {
		for _, server := range statuses[i].Servers {
			if server.Running {
				statuses[i].Running++
			}
		}
	}
	return statuses, nil
}

func (a *App) checkServerService(server Server, units []string) ServerServiceStatus {
	status := ServerServiceStatus{ServerID: server.ID}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, err := a.sshConnectContext(ctx, server)
	if err != nil {
		status.Error = fmt.Sprintf("SSH: %v", err)
		return status
	}
	defer client.Close()
	commands := make([]string, 0, len(units))
	for _, unit := range units {
		commands = append(commands, "systemctl is-active --quiet "+remote.ShellQuote(unit))
	}
	command := strings.Join(commands, " || ")
	_, err = runServerContext(ctx, client, server, command, nil)
	if err != nil {
		if ctx.Err() != nil {
			status.Error = "Время проверки истекло"
		} else {
			status.Error = "Сервис не запущен"
		}
		return status
	}
	status.Running = true
	return status
}
