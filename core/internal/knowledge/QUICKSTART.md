---
project: Cortex
component: UI
phase: Archive
date_created: 2025-12-18T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:16:51.559279
---

# Quick Start Guide - Knowledge Search & Merge

## 5-Minute Integration

### 1. Initialize Searcher

```go
package main

import (
    "context"
    "database/sql"
    "log"

    "github.com/normanking/cortex/internal/knowledge"
    "github.com/normanking/cortex/pkg/types"
    _ "modernc.org/sqlite"
)

func main() {
    // Open database
    db, err := sql.Open("sqlite", "~/.cortex/knowledge.db")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // Create searcher
    searcher := knowledge.NewFTS5Searcher(db)

    // Search for knowledge
    results, err := searcher.Search(context.Background(),
        "cisco vlan configuration",
        types.SearchOptions{
            Limit:    10,
            MinTrust: 0.5,
        },
    )

    if err != nil {
        log.Fatal(err)
    }

    // Display results
    for _, result := range results {
        log.Printf("%.2f | %s", result.Relevance, result.Item.Title)
    }
}
```

### 2. Resolve Sync Conflicts

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/normanking/cortex/internal/knowledge"
    "github.com/normanking/cortex/pkg/types"
)

func main() {
    // Create merger
    merger := knowledge.NewTrustWeightedMerge()

    // Example conflict: Team item with different trust scores
    local := &types.KnowledgeItem{
        ID:         "item-123",
        Scope:      types.ScopeTeam,
        Content:    "Local version",
        TrustScore: 0.85,
        UpdatedAt:  time.Now(),
    }

    remote := &types.KnowledgeItem{
        ID:         "item-123",
        Scope:      types.ScopeTeam,
        Content:    "Remote version",
        TrustScore: 0.60,
        UpdatedAt:  time.Now().Add(-1 * time.Hour),
    }

    // Resolve conflict
    result, err := merger.Resolve(context.Background(), local, remote)
    if err != nil {
        log.Fatal(err)
    }

    // Log result
    log.Printf("Winner: %s", result.Resolution)
    log.Printf("Reason: %s", result.Reason)

    // Apply winner
    if result.Winner == local {
        log.Println("Keep local version")
    } else {
        log.Println("Update to remote version")
    }
}
```

## Common Use Cases

### Search Patterns

#### 1. Basic Search
```go
results, _ := searcher.Search(ctx, "error handling", types.SearchOptions{
    Limit: 10,
})
```

#### 2. Phrase Search
```go
results, _ := searcher.Search(ctx, `"show interfaces trunk"`, types.SearchOptions{
    Limit: 5,
})
```

#### 3. Scope-Filtered Search (Team Only)
```go
results, _ := searcher.Search(ctx, "deployment procedure", types.SearchOptions{
    Tiers: []types.Scope{types.ScopeTeam},
    Limit: 20,
})
```

#### 4. High-Trust Search
```go
results, _ := searcher.Search(ctx, "security policy", types.SearchOptions{
    MinTrust: 0.8,  // Only highly trusted items
    Limit:    10,
})
```

#### 5. Tag-Filtered Search
```go
results, _ := searcher.Search(ctx, "network configuration", types.SearchOptions{
    Tags: []string{"cisco", "ios-xe"},  // Must have BOTH tags
    Limit: 15,
})
```

#### 6. Type-Filtered Search (SOPs Only)
```go
results, _ := searcher.Search(ctx, "backup procedure", types.SearchOptions{
    Types: []string{string(types.TypeSOP)},
    Limit: 10,
})
```

### Merge Patterns

#### 1. Global Scope (Remote Always Wins)
```go
local := &types.KnowledgeItem{
    ID:    "policy-001",
    Scope: types.ScopeGlobal,
    // ... local changes
}
remote := &types.KnowledgeItem{
    ID:    "policy-001",
    Scope: types.ScopeGlobal,
    // ... remote version
}

result, _ := merger.Resolve(ctx, local, remote)
// result.Winner == remote (always)
// result.Reason == "Global scope: admin authority. Remote always wins."
```

#### 2. Personal Scope (Local Always Wins)
```go
local := &types.KnowledgeItem{
    ID:    "note-001",
    Scope: types.ScopePersonal,
    // ... your notes
}
remote := &types.KnowledgeItem{
    ID:    "note-001",
    Scope: types.ScopePersonal,
    // ... shouldn't exist but handle gracefully
}

result, _ := merger.Resolve(ctx, local, remote)
// result.Winner == local (always)
// result.Reason == "Personal scope: private data. Local always wins."
```

#### 3. Team Scope (Trust-Weighted)
```go
// Higher trust wins
local := &types.KnowledgeItem{
    ID:         "sop-001",
    Scope:      types.ScopeTeam,
    TrustScore: 0.85,  // Expert user
}
remote := &types.KnowledgeItem{
    ID:         "sop-001",
    Scope:      types.ScopeTeam,
    TrustScore: 0.55,  // Less experienced
}

result, _ := merger.Resolve(ctx, local, remote)
// result.Winner == local (higher trust)
```

#### 4. Team Scope with Equal Trust (Timestamp Wins)
```go
local := &types.KnowledgeItem{
    ID:         "lesson-001",
    Scope:      types.ScopeTeam,
    TrustScore: 0.75,
    UpdatedAt:  time.Now().Add(-2 * time.Hour),  // Older
}
remote := &types.KnowledgeItem{
    ID:         "lesson-001",
    Scope:      types.ScopeTeam,
    TrustScore: 0.76,  // Within 5% (0.75 vs 0.76)
    UpdatedAt:  time.Now(),  // Newer
}

result, _ := merger.Resolve(ctx, local, remote)
// result.Winner == remote (more recent)
```

#### 5. Batch Resolution
```go
conflicts := []knowledge.ConflictPair{
    {Local: item1Local, Remote: item1Remote},
    {Local: item2Local, Remote: item2Remote},
    {Local: item3Local, Remote: item3Remote},
}

results, err := merger.BatchResolve(ctx, conflicts)

// Process results
for i, result := range results {
    log.Printf("Conflict %d: %s (%s)", i+1, result.Resolution, result.Reason)

    // Apply winner to database
    db.Update(result.Winner)
}

// Summary
summary := knowledge.SummarizeBatch(results)
log.Printf("Total: %d | Local: %d | Remote: %d",
    summary.TotalConflicts, summary.LocalWins, summary.RemoteWins)
```

## HTTP API Integration

### Search Endpoint

```go
// GET /api/knowledge/search?q=cisco+vlan&scope=team&min_trust=0.7
func handleSearch(w http.ResponseWriter, r *http.Request) {
    query := r.URL.Query().Get("q")
    scope := r.URL.Query().Get("scope")
    minTrust, _ := strconv.ParseFloat(r.URL.Query().Get("min_trust"), 64)
    limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
    if limit == 0 {
        limit = 10
    }

    opts := types.SearchOptions{
        Limit:    limit,
        MinTrust: minTrust,
    }

    if scope != "" {
        opts.Tiers = []types.Scope{types.Scope(scope)}
    }

    searcher := knowledge.NewFTS5Searcher(db)
    results, err := searcher.Search(r.Context(), query, opts)

    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(results)
}
```

### Sync Endpoint

```go
// POST /api/knowledge/sync
func handleSync(w http.ResponseWriter, r *http.Request) {
    // 1. Fetch remote changes
    remoteItems := fetchFromRemote()

    // 2. Find conflicts
    var conflicts []knowledge.ConflictPair
    for _, remote := range remoteItems {
        local := db.FindByID(remote.ID)
        if local != nil && local.Version != remote.Version {
            conflicts = append(conflicts, knowledge.ConflictPair{
                Local:  local,
                Remote: remote,
            })
        }
    }

    // 3. Resolve conflicts
    merger := knowledge.NewTrustWeightedMerge()
    results, err := merger.BatchResolve(r.Context(), conflicts)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    // 4. Apply winners
    for _, result := range results {
        db.Update(result.Winner)
        logConflict(result)
    }

    // 5. Return summary
    summary := knowledge.SummarizeBatch(results)
    json.NewEncoder(w).Encode(summary)
}
```

## CLI Commands

### Search Command

```go
// cortex knowledge search "cisco vlan" --scope team --min-trust 0.7
var searchCmd = &cobra.Command{
    Use:   "search [query]",
    Short: "Search knowledge base",
    Args:  cobra.ExactArgs(1),
    Run: func(cmd *cobra.Command, args []string) {
        query := args[0]
        scope, _ := cmd.Flags().GetString("scope")
        minTrust, _ := cmd.Flags().GetFloat64("min-trust")
        limit, _ := cmd.Flags().GetInt("limit")

        searcher := knowledge.NewFTS5Searcher(db)
        results, err := searcher.Search(context.Background(), query, types.SearchOptions{
            Tiers:    []types.Scope{types.Scope(scope)},
            MinTrust: minTrust,
            Limit:    limit,
        })

        if err != nil {
            log.Fatal(err)
        }

        for _, r := range results {
            fmt.Printf("%.2f | %s\n", r.Relevance, r.Item.Title)
        }
    },
}

func init() {
    searchCmd.Flags().String("scope", "", "Filter by scope (global/team/personal)")
    searchCmd.Flags().Float64("min-trust", 0.0, "Minimum trust score")
    searchCmd.Flags().Int("limit", 10, "Max results")
}
```

### Sync Command

```go
// cortex knowledge sync --resolve-conflicts
var syncCmd = &cobra.Command{
    Use:   "sync",
    Short: "Synchronize knowledge with remote",
    Run: func(cmd *cobra.Command, args []string) {
        // Fetch and resolve conflicts
        conflicts := fetchConflicts()

        merger := knowledge.NewTrustWeightedMerge()
        results, err := merger.BatchResolve(context.Background(), conflicts)
        if err != nil {
            log.Fatal(err)
        }

        // Apply winners
        for _, result := range results {
            applyWinner(result.Winner)
        }

        // Summary
        summary := knowledge.SummarizeBatch(results)
        fmt.Printf("Synced: %d conflicts resolved\n", summary.TotalConflicts)
        fmt.Printf("  Local wins: %d\n", summary.LocalWins)
        fmt.Printf("  Remote wins: %d\n", summary.RemoteWins)
    },
}
```

## Testing

### Run Examples
```bash
cd /Users/normanking/ServerProjectsMac/Cortex
go test -v ./internal/knowledge -run Example
```

### Manual Testing
```bash
# 1. Initialize database with test data
sqlite3 ~/.cortex/knowledge.db < internal/data/migrations/001_initial_schema.sql

# 2. Insert test items
sqlite3 ~/.cortex/knowledge.db "INSERT INTO knowledge_items ..."

# 3. Test search
go run examples/search.go

# 4. Test merge
go run examples/merge.go
```

## Troubleshooting

### "no such table: knowledge_fts"
```bash
# Run migrations
sqlite3 ~/.cortex/knowledge.db < internal/data/migrations/001_initial_schema.sql
```

### "fts5: syntax error"
```bash
# Check SQLite version (need 3.9.0+)
sqlite3 --version

# Verify FTS5 is enabled
echo "SELECT * FROM pragma_compile_options WHERE compile_options LIKE '%FTS5%';" | sqlite3 ~/.cortex/knowledge.db
```

### No search results
```bash
# Check FTS index is populated
echo "SELECT count(*) FROM knowledge_fts;" | sqlite3 ~/.cortex/knowledge.db

# Rebuild index if needed
echo "INSERT INTO knowledge_fts(knowledge_fts) VALUES('rebuild');" | sqlite3 ~/.cortex/knowledge.db
```

## Next Steps

1. ✅ Search implementation complete
2. ✅ Merge implementation complete
3. ⏳ Wire up HTTP API endpoints
4. ⏳ Add CLI commands
5. ⏳ Build UI components
6. ⏳ Add to sync service

See `README.md` for detailed documentation.
