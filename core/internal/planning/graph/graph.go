// Package graph provides dependency graph algorithms.
// Cycle detection and topological sort algorithms adapted from TaskWing
// (https://github.com/josephgoksu/TaskWing) under MIT License.
package graph

import (
	"fmt"
	"strings"
)

// Graph represents a directed graph for task dependencies
type Graph struct {
	nodes map[string]bool
	edges map[string][]string // adjacency list: node -> dependencies
}

// NewGraph creates a new empty graph
func NewGraph() *Graph {
	return &Graph{
		nodes: make(map[string]bool),
		edges: make(map[string][]string),
	}
}

// AddNode adds a node to the graph
func (g *Graph) AddNode(id string) {
	g.nodes[id] = true
	if _, exists := g.edges[id]; !exists {
		g.edges[id] = []string{}
	}
}

// AddEdge adds a directed edge from 'from' to 'to'
// Represents: 'from' depends on 'to' (to must complete before from)
func (g *Graph) AddEdge(from, to string) error {
	// Ensure both nodes exist
	g.AddNode(from)
	g.AddNode(to)

	// Check if this would create a cycle
	if g.WouldCreateCycle(from, to) {
		hasCycle, path := g.HasCycleAfterEdge(from, to)
		if hasCycle {
			return &CycleError{Path: path}
		}
	}

	// Add edge
	g.edges[from] = append(g.edges[from], to)
	return nil
}

// HasCycle performs DFS-based cycle detection
// Returns true if cycle exists, and the cycle path
func (g *Graph) HasCycle() (bool, []string) {
	visited := make(map[string]bool)
	recStack := make(map[string]bool)
	parent := make(map[string]string)

	var dfs func(node string) (bool, []string)
	dfs = func(node string) (bool, []string) {
		visited[node] = true
		recStack[node] = true

		for _, neighbor := range g.edges[node] {
			if !visited[neighbor] {
				parent[neighbor] = node
				if hasCycle, path := dfs(neighbor); hasCycle {
					return true, path
				}
			} else if recStack[neighbor] {
				// Found cycle - reconstruct path
				cycle := []string{neighbor}
				current := node
				for current != neighbor {
					cycle = append([]string{current}, cycle...)
					current = parent[current]
				}
				cycle = append([]string{neighbor}, cycle...)
				return true, cycle
			}
		}

		recStack[node] = false
		return false, nil
	}

	for node := range g.nodes {
		if !visited[node] {
			if hasCycle, path := dfs(node); hasCycle {
				return true, path
			}
		}
	}

	return false, nil
}

// HasCycleAfterEdge checks if adding edge would create cycle
func (g *Graph) HasCycleAfterEdge(from, to string) (bool, []string) {
	// Temporarily add the edge
	original := make([]string, len(g.edges[from]))
	copy(original, g.edges[from])
	g.edges[from] = append(g.edges[from], to)

	hasCycle, path := g.HasCycle()

	// Restore original edges
	g.edges[from] = original

	return hasCycle, path
}

// WouldCreateCycle checks if adding edge would create a cycle (lightweight check)
func (g *Graph) WouldCreateCycle(from, to string) bool {
	// If 'to' can reach 'from', adding 'from'->'to' creates cycle
	return g.canReach(to, from)
}

// canReach performs BFS to check if 'from' can reach 'to'
func (g *Graph) canReach(from, to string) bool {
	if from == to {
		return true
	}

	visited := make(map[string]bool)
	queue := []string{from}
	visited[from] = true

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for _, neighbor := range g.edges[current] {
			if neighbor == to {
				return true
			}
			if !visited[neighbor] {
				visited[neighbor] = true
				queue = append(queue, neighbor)
			}
		}
	}

	return false
}

// TopologicalSort returns nodes in dependency-first order using Kahn's algorithm
// Returns error if graph has cycles
func (g *Graph) TopologicalSort() ([]string, error) {
	// Calculate in-degrees
	inDegree := make(map[string]int)
	for node := range g.nodes {
		inDegree[node] = 0
	}
	for _, neighbors := range g.edges {
		for _, neighbor := range neighbors {
			inDegree[neighbor]++
		}
	}

	// Queue nodes with no dependencies
	queue := []string{}
	for node, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, node)
		}
	}

	result := []string{}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)

		// Reduce in-degree of neighbors
		for _, neighbor := range g.edges[current] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	// If not all nodes processed, there's a cycle
	if len(result) != len(g.nodes) {
		hasCycle, path := g.HasCycle()
		if hasCycle {
			return nil, &CycleError{Path: path}
		}
		return nil, fmt.Errorf("topological sort failed: graph may contain cycle")
	}

	return result, nil
}

// GetBlockers returns all transitive dependencies (blockers) for a given node
// Uses BFS to find all nodes that must complete before 'nodeID'
func (g *Graph) GetBlockers(nodeID string) []string {
	if !g.nodes[nodeID] {
		return []string{}
	}

	visited := make(map[string]bool)
	queue := []string{nodeID}
	visited[nodeID] = true
	blockers := []string{}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for _, dependency := range g.edges[current] {
			if !visited[dependency] {
				visited[dependency] = true
				blockers = append(blockers, dependency)
				queue = append(queue, dependency)
			}
		}
	}

	return blockers
}

// GetDependents returns all nodes that depend on the given node
func (g *Graph) GetDependents(nodeID string) []string {
	dependents := []string{}
	for node, deps := range g.edges {
		for _, dep := range deps {
			if dep == nodeID {
				dependents = append(dependents, node)
				break
			}
		}
	}
	return dependents
}

// RemoveNode removes a node and all its edges
func (g *Graph) RemoveNode(nodeID string) {
	delete(g.nodes, nodeID)
	delete(g.edges, nodeID)

	// Remove edges pointing to this node
	for node, deps := range g.edges {
		newDeps := []string{}
		for _, dep := range deps {
			if dep != nodeID {
				newDeps = append(newDeps, dep)
			}
		}
		g.edges[node] = newDeps
	}
}

// CycleError represents a circular dependency error
type CycleError struct {
	Path []string
}

func (e *CycleError) Error() string {
	if len(e.Path) == 0 {
		return "circular dependency detected"
	}
	return fmt.Sprintf("circular dependency detected: %s", strings.Join(e.Path, " -> "))
}

// IsCycleError checks if an error is a CycleError
func IsCycleError(err error) bool {
	_, ok := err.(*CycleError)
	return ok
}
