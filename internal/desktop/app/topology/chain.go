package topology

import "fmt"

type Node struct {
	ID   string
	Name string
	Role string
}

type Edge struct {
	Source string
	Target string
}

func Validate(nodes []Node, edges []Edge) error {
	ids := map[string]bool{}
	degree := map[string]int{}
	for _, node := range nodes {
		if node.ID == "" || ids[node.ID] {
			return fmt.Errorf("у серверов должны быть уникальные id")
		}
		ids[node.ID] = true
	}
	if len(nodes) == 0 && len(edges) > 0 {
		return fmt.Errorf("на карте нет серверов")
	}
	seen := map[string]bool{}
	for _, edge := range edges {
		if !ids[edge.Source] || !ids[edge.Target] || edge.Source == edge.Target {
			return fmt.Errorf("связь содержит некорректные серверы")
		}
		pair := edge.Source + "\x00" + edge.Target
		if seen[pair] || seen[edge.Target+"\x00"+edge.Source] {
			return fmt.Errorf("связь между серверами уже есть")
		}
		seen[pair] = true
		degree[edge.Source]++
		degree[edge.Target]++
	}
	for _, connections := range degree {
		if connections > 2 {
			return fmt.Errorf("карта должна быть линейной цепочкой: у сервера не может быть больше двух связей")
		}
	}
	for _, node := range nodes {
		connections := degree[node.ID]
		if connections == 0 && node.Role != "entry" && node.Role != "exit" {
			return fmt.Errorf("для сервера %s без связей выберите вход или выход", node.Name)
		}
		if connections == 1 && node.Role != "entry" && node.Role != "exit" {
			return fmt.Errorf("для крайнего сервера %s выберите вход или выход", node.Name)
		}
		if connections > 1 && node.Role != "" && node.Role != "relay" {
			return fmt.Errorf("сервер %s с двумя связями не может быть входом или выходом", node.Name)
		}
	}
	return nil
}

func Ordered(nodes []Node, edges []Edge) ([]string, error) {
	if err := Validate(nodes, edges); err != nil {
		return nil, err
	}
	if len(nodes) == 0 {
		return nil, nil
	}
	if len(nodes) == 1 {
		return []string{nodes[0].ID}, nil
	}
	byID := map[string]Node{}
	adjacent := map[string][]string{}
	for _, node := range nodes {
		byID[node.ID] = node
	}
	for _, edge := range edges {
		adjacent[edge.Source] = append(adjacent[edge.Source], edge.Target)
		adjacent[edge.Target] = append(adjacent[edge.Target], edge.Source)
	}
	start := ""
	for _, node := range nodes {
		if node.Role == "entry" {
			if start != "" {
				return nil, fmt.Errorf("укажите только одну входную точку")
			}
			start = node.ID
		}
	}
	if start == "" {
		for _, node := range nodes {
			if len(adjacent[node.ID]) < 2 {
				if start != "" {
					return nil, fmt.Errorf("выберите сервер входной точкой")
				}
				start = node.ID
			}
		}
	}
	if start == "" {
		return nil, fmt.Errorf("не найдена входная точка")
	}
	result := []string{}
	previous, current := "", start
	for current != "" {
		result = append(result, current)
		next := ""
		for _, neighbour := range adjacent[current] {
			if neighbour != previous {
				next = neighbour
				break
			}
		}
		previous, current = current, next
	}
	if len(result) != len(nodes) {
		return nil, fmt.Errorf("карта должна быть одной связной цепочкой")
	}
	if byID[result[len(result)-1]].Role != "exit" {
		return nil, fmt.Errorf("отметьте последний сервер как выход")
	}
	return result, nil
}
