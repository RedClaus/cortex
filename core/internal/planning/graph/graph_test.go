package graph

import (
	"testing"
)

func TestGraph_AddNode(t *testing.T) {
	g := NewGraph()

	g.AddNode("A")
	g.AddNode("B")

	if !g.nodes["A"] {
		t.Error("Node A should exist")
	}
	if !g.nodes["B"] {
		t.Error("Node B should exist")
	}
}

func TestGraph_AddEdge(t *testing.T) {
	g := NewGraph()

	// A depends on B (B must complete before A)
	err := g.AddEdge("A", "B")
	if err != nil {
		t.Fatalf("AddEdge failed: %v", err)
	}

	if len(g.edges["A"]) != 1 || g.edges["A"][0] != "B" {
		t.Error("Edge A->B should exist")
	}
}

func TestGraph_CycleDetection(t *testing.T) {
	g := NewGraph()

	// Create: A -> B -> C
	g.AddEdge("A", "B")
	g.AddEdge("B", "C")

	// No cycle yet
	hasCycle, _ := g.HasCycle()
	if hasCycle {
		t.Error("Should not detect cycle in A->B->C")
	}

	// Adding C -> A creates a cycle
	if !g.WouldCreateCycle("C", "A") {
		t.Error("Adding C->A should create a cycle")
	}

	// Adding A -> C should not create a cycle (same direction)
	if g.WouldCreateCycle("A", "C") {
		t.Error("Adding A->C should NOT create a cycle")
	}
}

func TestGraph_CycleError(t *testing.T) {
	g := NewGraph()

	// A -> B -> C
	g.AddEdge("A", "B")
	g.AddEdge("B", "C")

	// Try to create C -> A (cycle)
	err := g.AddEdge("C", "A")
	if err == nil {
		t.Error("Should return error for cycle")
	}

	if !IsCycleError(err) {
		t.Errorf("Error should be CycleError, got: %T", err)
	}
}

func TestGraph_TopologicalSort(t *testing.T) {
	g := NewGraph()

	// Edge semantics: AddEdge(from, to) = "from depends on to"
	// But TopologicalSort treats edges as execution order arrows
	// So we add edges in reverse: dependency -> dependent
	// To get execution order C -> B -> A (where A depends on B depends on C):
	g.AddEdge("C", "B") // C must complete before B
	g.AddEdge("B", "A") // B must complete before A

	order, err := g.TopologicalSort()
	if err != nil {
		t.Fatalf("TopologicalSort failed: %v", err)
	}

	// C should come before B, B before A
	indexOf := func(id string) int {
		for i, v := range order {
			if v == id {
				return i
			}
		}
		return -1
	}

	if indexOf("C") > indexOf("B") {
		t.Error("C should come before B")
	}
	if indexOf("B") > indexOf("A") {
		t.Error("B should come before A")
	}
}

func TestGraph_GetBlockers(t *testing.T) {
	g := NewGraph()

	// A depends on B and C
	// B depends on D
	g.AddEdge("A", "B")
	g.AddEdge("A", "C")
	g.AddEdge("B", "D")

	blockers := g.GetBlockers("A")

	// A's blockers should include B, C, and D (transitive)
	contains := func(slice []string, item string) bool {
		for _, v := range slice {
			if v == item {
				return true
			}
		}
		return false
	}

	if !contains(blockers, "B") {
		t.Error("B should be a blocker of A")
	}
	if !contains(blockers, "C") {
		t.Error("C should be a blocker of A")
	}
	if !contains(blockers, "D") {
		t.Error("D should be a blocker of A (transitive through B)")
	}
}

func TestGraph_GetDependents(t *testing.T) {
	g := NewGraph()

	// A depends on B, C depends on B
	g.AddEdge("A", "B")
	g.AddEdge("C", "B")

	dependents := g.GetDependents("B")

	if len(dependents) != 2 {
		t.Errorf("B should have 2 dependents, got %d", len(dependents))
	}
}

func TestGraph_RemoveNode(t *testing.T) {
	g := NewGraph()

	g.AddEdge("A", "B")
	g.AddEdge("B", "C")

	g.RemoveNode("B")

	if g.nodes["B"] {
		t.Error("Node B should be removed")
	}
	if len(g.edges["A"]) != 0 {
		t.Error("Edge A->B should be removed")
	}
}

func BenchmarkCycleDetection(b *testing.B) {
	// Create a larger graph for benchmarking
	g := NewGraph()
	for i := 0; i < 100; i++ {
		g.AddNode(string(rune('A' + i%26)) + string(rune('0'+i/26)))
	}

	// Add edges (linear chain)
	for i := 0; i < 99; i++ {
		from := string(rune('A'+i%26)) + string(rune('0'+i/26))
		to := string(rune('A'+(i+1)%26)) + string(rune('0'+(i+1)/26))
		g.edges[from] = append(g.edges[from], to)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.HasCycle()
	}
}
