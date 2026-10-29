package route

import "hoplink/internal/desktop/app/topology"

type Server struct {
	ID   string
	Name string
	Role string
}

type Edge struct {
	Source string
	Target string
}

type View struct {
	MapServers      []Server
	DeployedServers []Server
	Edges           []Edge
}

func ValidateMap(servers []Server, edges []Edge) error {
	nodes, connections := topologyInput(servers, edges)
	return topology.Validate(nodes, connections)
}

func OrderedChain(servers []Server, edges []Edge) ([]Server, error) {
	nodes, connections := topologyInput(servers, edges)
	ids, err := topology.Ordered(nodes, connections)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]Server, len(servers))
	for _, server := range servers {
		byID[server.ID] = server
	}
	ordered := make([]Server, 0, len(ids))
	for _, id := range ids {
		ordered = append(ordered, byID[id])
	}
	return ordered, nil
}

func IsEntry(route View, id string) bool {
	for i, server := range route.DeployedServers {
		if server.ID == id {
			return i == 0
		}
	}
	incoming, outgoing := false, false
	for _, edge := range route.Edges {
		if edge.Target == id {
			incoming = true
		}
		if edge.Source == id {
			outgoing = true
		}
	}
	if !incoming && outgoing {
		return true
	}
	for _, server := range route.MapServers {
		if server.ID == id && server.Role == "entry" {
			return true
		}
	}
	return false
}

func topologyInput(servers []Server, edges []Edge) ([]topology.Node, []topology.Edge) {
	nodes := make([]topology.Node, 0, len(servers))
	connections := make([]topology.Edge, 0, len(edges))
	for _, server := range servers {
		nodes = append(nodes, topology.Node{ID: server.ID, Name: server.Name, Role: server.Role})
	}
	for _, edge := range edges {
		connections = append(connections, topology.Edge{Source: edge.Source, Target: edge.Target})
	}
	return nodes, connections
}
