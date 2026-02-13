package knowledge_test

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/normanking/cortex/internal/knowledge"
	"github.com/normanking/cortex/pkg/types"
)

// ExampleFTS5Searcher_Search demonstrates basic full-text search usage.
func ExampleFTS5Searcher_Search() {
	// Assume db is already initialized
	var db *sql.DB

	searcher := knowledge.NewFTS5Searcher(db)

	// Basic search
	results, err := searcher.Search(context.Background(), "cisco vlan configuration", types.SearchOptions{
		Limit:    10,
		MinTrust: 0.5,
	})
	if err != nil {
		fmt.Printf("Search failed: %v\n", err)
		return
	}

	for _, result := range results {
		fmt.Printf("Relevance: %.3f | Trust: %.2f | Title: %s\n",
			result.Relevance, result.Item.TrustScore, result.Item.Title)
	}
}

// ExampleFTS5Searcher_Search_phraseQuery demonstrates phrase search.
func ExampleFTS5Searcher_Search_phraseQuery() {
	var db *sql.DB
	searcher := knowledge.NewFTS5Searcher(db)

	// Exact phrase search using quotes
	results, err := searcher.Search(context.Background(), `"trunk mode" cisco`, types.SearchOptions{
		Limit: 5,
	})
	if err != nil {
		fmt.Printf("Search failed: %v\n", err)
		return
	}

	fmt.Printf("Found %d results matching phrase\n", len(results))
}

// ExampleFTS5Searcher_Search_filtered demonstrates filtered search.
func ExampleFTS5Searcher_Search_filtered() {
	var db *sql.DB
	searcher := knowledge.NewFTS5Searcher(db)

	// Search only team-scope SOPs with specific tags
	results, err := searcher.Search(context.Background(), "network troubleshooting", types.SearchOptions{
		Tiers:    []types.Scope{types.ScopeTeam},
		Types:    []string{string(types.TypeSOP), string(types.TypeLesson)},
		Tags:     []string{"cisco", "layer2"},
		MinTrust: 0.7,
		Limit:    20,
	})
	if err != nil {
		fmt.Printf("Search failed: %v\n", err)
		return
	}

	fmt.Printf("Found %d high-trust team SOPs\n", len(results))
}

// ExampleFTS5Searcher_Index demonstrates FTS5 index optimization.
func ExampleFTS5Searcher_Index() {
	var db *sql.DB
	searcher := knowledge.NewFTS5Searcher(db)

	// Optimize the FTS5 index (useful after bulk imports)
	err := searcher.Index(context.Background())
	if err != nil {
		fmt.Printf("Index optimization failed: %v\n", err)
		return
	}

	fmt.Println("FTS5 index optimized successfully")
}

// ExampleTrustWeightedMerge_Resolve demonstrates conflict resolution.
func ExampleTrustWeightedMerge_Resolve() {
	merger := knowledge.NewTrustWeightedMerge()

	// Scenario 1: Global scope - remote always wins
	local := &types.KnowledgeItem{
		ID:         "item-001",
		Scope:      types.ScopeGlobal,
		Content:    "Local version",
		TrustScore: 0.9,
		UpdatedAt:  time.Now(),
	}
	remote := &types.KnowledgeItem{
		ID:         "item-001",
		Scope:      types.ScopeGlobal,
		Content:    "Remote version",
		TrustScore: 0.5,
		UpdatedAt:  time.Now().Add(-1 * time.Hour),
	}

	result, _ := merger.Resolve(context.Background(), local, remote)
	fmt.Printf("Global scope: %s (%s)\n", result.Resolution, result.Reason)
	// Output: Global scope: remote_wins (Global scope: admin authority. Remote always wins.)
}

// ExampleTrustWeightedMerge_Resolve_personal demonstrates personal scope resolution.
func ExampleTrustWeightedMerge_Resolve_personal() {
	merger := knowledge.NewTrustWeightedMerge()

	// Scenario 2: Personal scope - local always wins
	local := &types.KnowledgeItem{
		ID:         "item-002",
		Scope:      types.ScopePersonal,
		Content:    "My personal notes",
		TrustScore: 0.5,
		UpdatedAt:  time.Now().Add(-1 * time.Hour),
	}
	remote := &types.KnowledgeItem{
		ID:         "item-002",
		Scope:      types.ScopePersonal,
		Content:    "Someone else's version (shouldn't exist)",
		TrustScore: 0.9,
		UpdatedAt:  time.Now(),
	}

	result, _ := merger.Resolve(context.Background(), local, remote)
	fmt.Printf("Personal scope: %s\n", result.Resolution)
	// Output: Personal scope: local_wins
}

// ExampleTrustWeightedMerge_Resolve_team demonstrates team scope trust-based resolution.
func ExampleTrustWeightedMerge_Resolve_team() {
	merger := knowledge.NewTrustWeightedMerge()

	// Scenario: Team scope - higher trust wins
	local := &types.KnowledgeItem{
		ID:         "item-003",
		Scope:      types.ScopeTeam,
		Content:    "Local team knowledge",
		TrustScore: 0.85, // Significantly higher
		UpdatedAt:  time.Now().Add(-1 * time.Hour),
	}
	remote := &types.KnowledgeItem{
		ID:         "item-003",
		Scope:      types.ScopeTeam,
		Content:    "Remote team knowledge",
		TrustScore: 0.60,
		UpdatedAt:  time.Now(),
	}

	result, _ := merger.Resolve(context.Background(), local, remote)
	fmt.Printf("Team scope (trust diff): %s\n", result.Resolution)
	// Output: Team scope (trust diff): local_wins
}

// ExampleNewTrustWeightedMergeWithBias demonstrates local bias.
func ExampleNewTrustWeightedMergeWithBias() {
	// Create merger with 30% local bias
	merger, _ := knowledge.NewTrustWeightedMergeWithBias(0.3)

	local := &types.KnowledgeItem{
		ID:         "item-004",
		Scope:      types.ScopeTeam,
		TrustScore: 0.70,
		UpdatedAt:  time.Now(),
	}
	remote := &types.KnowledgeItem{
		ID:         "item-004",
		Scope:      types.ScopeTeam,
		TrustScore: 0.80,
		UpdatedAt:  time.Now(),
	}

	// Without bias: remote wins (0.80 > 0.70)
	// With 30% bias: local effective trust = 0.70 * 1.3 = 0.91 > 0.80, local wins

	result, _ := merger.Resolve(context.Background(), local, remote)
	fmt.Printf("With local bias: %s\n", result.Resolution)
	// Output: With local bias: local_wins
}

// ExampleTrustWeightedMerge_BatchResolve demonstrates batch conflict resolution.
func ExampleTrustWeightedMerge_BatchResolve() {
	merger := knowledge.NewTrustWeightedMerge()

	conflicts := []knowledge.ConflictPair{
		{
			Local: &types.KnowledgeItem{
				ID: "item-001", Scope: types.ScopeGlobal,
				TrustScore: 0.9, UpdatedAt: time.Now(),
			},
			Remote: &types.KnowledgeItem{
				ID: "item-001", Scope: types.ScopeGlobal,
				TrustScore: 0.5, UpdatedAt: time.Now(),
			},
		},
		{
			Local: &types.KnowledgeItem{
				ID: "item-002", Scope: types.ScopePersonal,
				TrustScore: 0.5, UpdatedAt: time.Now(),
			},
			Remote: &types.KnowledgeItem{
				ID: "item-002", Scope: types.ScopePersonal,
				TrustScore: 0.9, UpdatedAt: time.Now(),
			},
		},
		{
			Local: &types.KnowledgeItem{
				ID: "item-003", Scope: types.ScopeTeam,
				TrustScore: 0.85, UpdatedAt: time.Now(),
			},
			Remote: &types.KnowledgeItem{
				ID: "item-003", Scope: types.ScopeTeam,
				TrustScore: 0.60, UpdatedAt: time.Now(),
			},
		},
	}

	results, err := merger.BatchResolve(context.Background(), conflicts)
	if err != nil {
		fmt.Printf("Batch resolve failed: %v\n", err)
		return
	}

	summary := knowledge.SummarizeBatch(results)
	fmt.Printf("Resolved %d conflicts: %d local wins, %d remote wins\n",
		summary.TotalConflicts, summary.LocalWins, summary.RemoteWins)
	// Output: Resolved 3 conflicts: 2 local wins, 1 remote wins
}

// ExampleIsContentDifferent demonstrates content comparison.
func ExampleIsContentDifferent() {
	local := &types.KnowledgeItem{
		Title:   "VLAN Config",
		Content: "Configure VLANs...",
		Tags:    []string{"cisco", "network"},
	}
	remote := &types.KnowledgeItem{
		Title:   "VLAN Config",
		Content: "Configure VLANs...",
		Tags:    []string{"network", "cisco"}, // Same tags, different order
	}

	isDifferent := knowledge.IsContentDifferent(local, remote)
	fmt.Printf("Content different: %v\n", isDifferent)
	// Output: Content different: false
}
