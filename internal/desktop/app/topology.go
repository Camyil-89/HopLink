package desktop

import (
	"hoplink/internal/desktop/app/route"
)

func (a *App) isEntryServerLocked(id string) bool {
	return routeIsEntry(a.config, id)
}

func routeIsEntry(cfg AppConfig, id string) bool {
	return route.IsEntry(routeView(cfg), id)
}

func routeIsEntryRoute(r Route, id string) bool {
	return route.IsEntry(routeViewFromRoute(r), id)
}

func routeView(cfg AppConfig) route.View {
	return route.View{
		MapServers:      mapRouteServers(cfg.MapServers),
		DeployedServers: mapRouteServers(cfg.DeployedServers),
		Edges:           mapRouteEdges(cfg.Edges),
	}
}

func routeViewFromRoute(r Route) route.View {
	return route.View{
		MapServers:      mapRouteServers(r.MapServers),
		DeployedServers: mapRouteServers(r.DeployedServers),
		Edges:           mapRouteEdges(r.Edges),
	}
}

func mapRouteServers(servers []Server) []route.Server {
	out := make([]route.Server, len(servers))
	for i, server := range servers {
		out[i] = route.Server{ID: server.ID, Name: server.Name, Role: server.Role}
	}
	return out
}

func mapRouteEdges(edges []Edge) []route.Edge {
	out := make([]route.Edge, len(edges))
	for i, edge := range edges {
		out[i] = route.Edge{Source: edge.Source, Target: edge.Target}
	}
	return out
}

func validateMap(servers []Server, edges []Edge) error {
	return route.ValidateMap(mapRouteServers(servers), mapRouteEdges(edges))
}

func orderedChain(servers []Server, edges []Edge) ([]Server, error) {
	ordered, err := route.OrderedChain(mapRouteServers(servers), mapRouteEdges(edges))
	if err != nil {
		return nil, err
	}
	out := make([]Server, len(ordered))
	for i, server := range ordered {
		out[i] = Server{ID: server.ID, Name: server.Name, Role: server.Role}
		for _, original := range servers {
			if original.ID == server.ID {
				out[i] = original
				break
			}
		}
	}
	return out, nil
}
