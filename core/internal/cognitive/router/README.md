---
project: Cortex
component: Docs
phase: Archive
date_created: 2025-12-18T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:16:51.388881
---

# Semantic Router

The semantic router is a core component of Cortex's Cognitive Architecture v2.1. It uses embedding-based similarity search to match user requests against learned templates and determine the optimal model tier for processing.

## Overview

The router performs three key functions:

1. **Semantic Matching**: Generates embeddings for user input and finds similar templates using cosine similarity
2. **Model Selection**: Routes requests to the appropriate model tier (Local → Mid → Advanced → Frontier)
3. **Fallback Handling**: Uses keyword search when embeddings are unavailable

## Architecture

```
User Input
    │
    ▼
┌─────────────────┐
│  Embedder       │  Generate embedding via Ollama
│  (nomic-embed)  │  (768-dimensional vector)
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Index Search   │  Cosine similarity against
│  (EmbeddingIdx) │  active templates
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Threshold      │  High (≥0.85)  → Local
│  Analysis       │  Medium (0.70) → Mid
│                 │  Low (0.50)    → Advanced
│                 │  NoMatch (<0.50) → Frontier + Distill
└────────┬────────┘
         │
         ▼
   Routing Decision
```

## Components

### 1. Router (`router.go`)

Main routing logic with configurable thresholds.

**Key Methods:**
- `Route(ctx, input) → RoutingResult` - Main routing function
- `Initialize(ctx)` - Load active templates into index
- `RefreshIfNeeded(ctx)` - Periodic index refresh
- `AddTemplate(ctx, template)` - Real-time index update
- `Stats()` - Performance metrics

**Thresholds:**
```go
High:   0.85  // Strong match → Use template directly with local model
Medium: 0.70  // Moderate match → Use template with mid-tier model
Low:    0.50  // Weak match → Use advanced model, may need adjustment
// Below 0.50: Novel request → Frontier model + distillation
```

**Model Selection Logic:**
- High similarity (≥0.85) + High confidence (≥0.8) → **Local** (Ollama)
- High similarity (≥0.85) + Medium confidence (≥0.6) → **Mid** (Claude Haiku)
- Medium similarity (≥0.70) → **Mid**
- Low similarity (≥0.50) → **Advanced** (Claude Sonnet)
- No match (<0.50) → **Frontier** (Claude Sonnet) + Distillation

### 2. Embedder (`embedder.go`)

Generates text embeddings using Ollama's local models.

**Interface:**
```go
type Embedder interface {
    Embed(ctx, text) (Embedding, error)
    EmbedBatch(ctx, texts) ([]Embedding, error)
    Dimension() int
    ModelName() string
    Available() bool
}
```

**OllamaEmbedder:**
- Model: `nomic-embed-text` (768 dimensions)
- Endpoint: `http://localhost:11434` (default)
- Auto-pull: Optional automatic model download
- Availability checking: Periodic health checks with auto-recovery

**Features:**
- Connection pooling with 30s timeout
- Automatic availability detection
- Periodic re-checks (5min default)
- Graceful degradation to keyword search

### 3. Embedding Index (`index.go`)

In-memory vector similarity search using cosine similarity.

**Operations:**
- `Add(id, embedding, metadata)` - Insert/update entry
- `Remove(id)` - Delete entry
- `Search(query, k)` - Find k most similar entries
- `SearchWithThreshold(query, threshold)` - Find all above threshold
- `BatchAdd(entries)` - Bulk insertion
- `Clear()` - Reset index

**Performance:**
- Brute-force cosine similarity (efficient for <10K templates)
- Pre-normalized vectors for fast comparison
- Thread-safe with RWMutex
- O(n) search complexity

**Cosine Similarity:**
```go
similarity = dot(A, B) / (||A|| * ||B||)
// Range: -1 (opposite) to 1 (identical)
```

## Usage Example

```go
import (
    "context"
    "github.com/normanking/cortex/internal/cognitive/router"
)

// Create embedder
embedder := router.NewOllamaEmbedder(&router.OllamaEmbedderConfig{
    Host:     "http://localhost:11434",
    Model:    "nomic-embed-text",
    AutoPull: true,
})

// Create router
r := router.NewRouter(&router.RouterConfig{
    Embedder:      embedder,
    Registry:      templateRegistry,
    Thresholds:    router.DefaultThresholds(),
    RefreshPeriod: 5 * time.Minute,
})

// Initialize
if err := r.Initialize(ctx); err != nil {
    log.Fatal(err)
}

// Route a request
result, err := r.Route(ctx, "fix the authentication bug")
if err != nil {
    log.Fatal(err)
}

switch result.Decision {
case cognitive.RouteTemplate:
    // Use matched template
    template := result.Match.Template
    model := result.RecommendedModel

case cognitive.RouteNovel:
    // No match - use frontier model and distill
    model := result.RecommendedModel // Frontier tier

case cognitive.RouteFallback:
    // Embedder unavailable - used keyword search
}
```

## Integration Points

### Registry Interface

The router requires a `cognitive.Registry` implementation:

```go
type Registry interface {
    ListActive(ctx) ([]*Template, error)
    SearchByKeywords(ctx, query, limit) ([]*Template, error)
    // ... other methods
}
```

**Key Methods:**
- `ListActive()` - Get all promoted/validated templates for index
- `SearchByKeywords()` - Fallback when embeddings unavailable

### Template Structure

Templates must have pre-computed embeddings:

```go
type Template struct {
    ID              string
    Intent          string      // Natural language intent
    IntentEmbedding Embedding   // 768-dim vector
    ConfidenceScore float64     // Used for model tier selection
    // ... other fields
}
```

## Performance Characteristics

**Latency:**
- Embedding generation: ~50-200ms (depends on Ollama)
- Index search: <1ms for <1000 templates
- Total routing: ~50-250ms

**Memory:**
- Per template: ~3KB (768 floats * 4 bytes)
- 1000 templates: ~3MB
- Index overhead: minimal

**Scalability:**
- Efficient up to 10,000 templates
- Consider HNSW/FAISS for larger collections

## Error Handling

**Graceful Degradation:**
1. Ollama unavailable → Use keyword search fallback
2. No keyword matches → Route to frontier model
3. Registry errors → Log warning, continue with partial index

**Availability Checking:**
- Periodic checks (5min default)
- Automatic recovery when Ollama comes back online
- No hard failures - always returns a routing decision

## Metrics and Monitoring

**RouterStats:**
```go
stats := router.Stats()
// Returns:
// - IndexSize: Number of templates indexed
// - EmbedderAvailable: Embedder health status
// - EmbeddingModel: Model name (e.g., "nomic-embed-text")
// - EmbeddingDimension: Vector size (768)
// - LastRefresh: When index was last rebuilt
```

## Configuration

**Environment Variables:**
- `OLLAMA_HOST` - Ollama API endpoint (default: http://localhost:11434)
- `EMBEDDING_MODEL` - Model name (default: nomic-embed-text)

**Tunable Parameters:**
- Similarity thresholds (High/Medium/Low)
- Refresh period (how often to reload templates)
- Availability check interval (Ollama health checks)
- Model tier selection logic

## Testing

Run tests:
```bash
go test -v ./internal/cognitive/router/...
```

**Test Coverage:**
- Embedding index operations (add/remove/search)
- Cosine similarity computation
- Normalization
- Model tier selection
- Router with null embedder (fallback path)
- Embedding serialization

## Future Enhancements

1. **Approximate Nearest Neighbors**: Use HNSW/FAISS for >10K templates
2. **Batch Embedding**: Parallel embedding generation for multiple requests
3. **Caching**: Cache user input embeddings for repeat queries
4. **Multi-Model Support**: Support for different embedding models
5. **A/B Testing**: Compare different threshold configurations
6. **Adaptive Thresholds**: Learn optimal thresholds from feedback

## Dependencies

- `github.com/normanking/cortex/internal/cognitive` - Core types
- `github.com/normanking/cortex/internal/logging` - Logging
- Ollama API - Embedding generation (external service)

## Related Components

- **Template Registry** (`internal/cognitive/registry.go`) - Template storage
- **Distillation Engine** (`internal/cognitive/distillation/`) - Frontier model learning
- **Feedback Loop** (`internal/cognitive/feedback/`) - Quality grading
- **Orchestrator** (`internal/orchestrator/`) - Request coordinator
