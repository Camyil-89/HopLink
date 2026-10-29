package desktop

import (
	"context"
	"fmt"
	"strings"
	"time"

	"hoplink/internal/desktop/app/remote"
)

// GetRouteJournal returns the selected route unit's journal for an explicit time range.
func (a *App) GetRouteJournal(routeID, serverID, since, until string) (string, error) {
	start, err := time.Parse(time.RFC3339, since)
	if err != nil {
		return "", fmt.Errorf("неверное время начала")
	}
	end, err := time.Parse(time.RFC3339, until)
	if err != nil {
		return "", fmt.Errorf("неверное время окончания")
	}
	if !end.After(start) {
		return "", fmt.Errorf("время окончания должно быть позже времени начала")
	}
	if end.Sub(start) > 31*24*time.Hour {
		return "", fmt.Errorf("период журнала не может превышать 31 день")
	}
	a.mu.Lock()
	if a.vaultLocked {
		a.mu.Unlock()
		return "", fmt.Errorf("разблокируйте настройки перед чтением журнала")
	}
	var selected *Route
	for i := range a.config.Routes {
		if a.config.Routes[i].ID == routeID {
			copy := a.config.Routes[i]
			selected = &copy
			break
		}
	}
	var server *Server
	for i := range a.config.Servers {
		if a.config.Servers[i].ID == serverID {
			copy := a.config.Servers[i]
			server = &copy
			break
		}
	}
	a.mu.Unlock()
	if selected == nil || server == nil {
		return "", fmt.Errorf("маршрут или сервер не найден")
	}
	deployed := false
	for _, candidate := range selected.DeployedServers {
		if candidate.ID == serverID {
			deployed = true
			break
		}
	}
	if !deployed {
		return "", fmt.Errorf("сервис этого маршрута не установлен на выбранном сервере")
	}
	_, unit := routeRemoteLayout(routeID, []Route{*selected})
	if selected.LegacyLayout {
		unit = serviceName
	}
	if selected.LegacyIDLayout {
		id := selected.ID
		if len(id) > 32 {
			id = id[:32]
		}
		unit = "hoplink-" + id + ".service"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	client, err := a.sshConnectContext(ctx, *server)
	if err != nil {
		return "", fmt.Errorf("SSH %s: %w", server.Host, err)
	}
	defer client.Close()
	command := "journalctl -u " + remote.ShellQuote(unit) + " --since " + remote.ShellQuote(fmt.Sprintf("@%d", start.Unix())) + " --until " + remote.ShellQuote(fmt.Sprintf("@%d", end.Unix())) + " --no-pager -o short-iso-precise"
	output, err := runServerContext(ctx, client, *server, command, nil)
	if err != nil {
		detail := strings.TrimSpace(output)
		if detail != "" {
			return detail, fmt.Errorf("journalctl %s на %s: %w: %s", unit, server.Host, err, detail)
		}
		return "", fmt.Errorf("journalctl %s на %s: %w", unit, server.Host, err)
	}
	if strings.TrimSpace(output) == "" {
		return "Записей за выбранный период нет.", nil
	}
	return output, nil
}
