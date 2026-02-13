// Package brain provides adapters to wire pkg/brain (Executive) with Cortex infrastructure.
// This enables the Brain's cognitive architecture to use Cortex's LLM providers and memory systems.
package brain

import (
	"context"
	"fmt"

	"github.com/normanking/cortex/internal/cognitive/router"
	"github.com/normanking/cortex/internal/llm"
	"github.com/normanking/cortex/internal/logging"
	"github.com/normanking/cortex/internal/memory"
	"github.com/normanking/cortex/pkg/brain"
	"github.com/normanking/cortex/pkg/brain/lobes"
	"github.com/normanking/cortex/pkg/types"
)

var log = logging.Global()

// MemorySystem defines the interface for memory operations.
// This is a local copy to avoid import cycles with internal/orchestrator.
type MemorySystem interface {
	SearchArchival(ctx context.Context, query string, limit int) ([]*types.KnowledgeItem, error)
	ExecuteTool(ctx context.Context, userID, toolName, argsJSON string) (*memory.ToolResult, error)
	MemoryTools() *memory.MemoryTools
}

// ═══════════════════════════════════════════════════════════════════════════════
// LLM PROVIDER ADAPTER
// ═══════════════════════════════════════════════════════════════════════════════

// LLMAdapter wraps an internal/llm.Provider to implement lobes.LLMProvider.
// This allows the Brain's lobes to use any Cortex LLM provider (Ollama, OpenAI, etc.).
type LLMAdapter struct {
	provider llm.Provider
}

// NewLLMAdapter creates an adapter from an llm.Provider.
func NewLLMAdapter(p llm.Provider) *LLMAdapter {
	return &LLMAdapter{provider: p}
}

// Chat implements lobes.LLMProvider interface.
func (a *LLMAdapter) Chat(ctx context.Context, req *llm.ChatRequest) (*llm.ChatResponse, error) {
	if a.provider == nil {
		log.Error("[Brain] LLMAdapter: provider not configured")
		return nil, fmt.Errorf("LLM provider not configured")
	}
	log.Debug("[Brain] LLMAdapter: Chat request with %d messages", len(req.Messages))
	resp, err := a.provider.Chat(ctx, req)
	if err != nil {
		log.Error("[Brain] LLMAdapter: Chat failed: %v", err)
		return nil, err
	}
	log.Debug("[Brain] LLMAdapter: Chat response received, content length=%d", len(resp.Content))
	return resp, nil
}

// Verify LLMAdapter implements lobes.LLMProvider at compile time.
var _ lobes.LLMProvider = (*LLMAdapter)(nil)

// ═══════════════════════════════════════════════════════════════════════════════
// MEMORY STORE ADAPTER
// ═══════════════════════════════════════════════════════════════════════════════

type MemoryAdapter struct {
	memSys MemorySystem
	userID string
}

func NewMemoryAdapter(ms MemorySystem, userID string) *MemoryAdapter {
	return &MemoryAdapter{
		memSys: ms,
		userID: userID,
	}
}

// Search implements lobes.MemoryStore interface.
// Searches archival memory and converts results to brain.Memory format.
func (a *MemoryAdapter) Search(ctx context.Context, query string, limit int) ([]brain.Memory, error) {
	if a.memSys == nil {
		log.Error("[Brain] MemoryAdapter: memory system not configured")
		return nil, fmt.Errorf("memory system not configured")
	}

	log.Debug("[Brain] MemoryAdapter: Search query=%q limit=%d", query, limit)

	// Search archival memory through the memory coordinator
	items, err := a.memSys.SearchArchival(ctx, query, limit)
	if err != nil {
		log.Error("[Brain] MemoryAdapter: Search failed: %v", err)
		return nil, fmt.Errorf("search archival: %w", err)
	}

	memories := make([]brain.Memory, 0, len(items))
	for _, item := range items {
		memories = append(memories, brain.Memory{
			ID:        item.ID,
			Content:   item.Content,
			Source:    string(item.Scope),
			Relevance: item.Confidence,
		})
	}

	log.Debug("[Brain] MemoryAdapter: Search returned %d memories", len(memories))
	return memories, nil
}

// Store implements lobes.MemoryStore interface.
// Stores content in archival memory with metadata.
func (a *MemoryAdapter) Store(ctx context.Context, content string, metadata map[string]string) error {
	if a.memSys == nil {
		log.Error("[Brain] MemoryAdapter: memory system not configured for store")
		return fmt.Errorf("memory system not configured")
	}

	log.Debug("[Brain] MemoryAdapter: Store content length=%d metadata=%v", len(content), metadata)

	// Use memory tools to store if available
	if a.memSys.MemoryTools() != nil {
		// Format as archival_memory_insert tool call
		argsJSON := fmt.Sprintf(`{"content": %q}`, content)
		_, err := a.memSys.ExecuteTool(ctx, a.userID, "archival_memory_insert", argsJSON)
		if err != nil {
			log.Error("[Brain] MemoryAdapter: Store via memory tool failed: %v", err)
			return fmt.Errorf("store via memory tool: %w", err)
		}
		log.Info("[Brain] MemoryAdapter: Successfully stored content")
		return nil
	}

	log.Warn("[Brain] MemoryAdapter: No memory tool available for storage")
	return fmt.Errorf("no memory tool available for storage")
}

// Verify MemoryAdapter implements lobes.MemoryStore at compile time.
var _ lobes.MemoryStore = (*MemoryAdapter)(nil)

// ═══════════════════════════════════════════════════════════════════════════════
// MULTI-PROVIDER LLM ADAPTER
// ═══════════════════════════════════════════════════════════════════════════════

// MultiLLMAdapter supports multiple providers with tier-based selection.
// This allows the Brain to use different models for different lobes.
type MultiLLMAdapter struct {
	providers map[string]llm.Provider // provider name -> provider
	primary   llm.Provider            // default/fallback provider
}

// NewMultiLLMAdapter creates an adapter that can route to multiple providers.
func NewMultiLLMAdapter(primary llm.Provider) *MultiLLMAdapter {
	return &MultiLLMAdapter{
		providers: make(map[string]llm.Provider),
		primary:   primary,
	}
}

// AddProvider registers a named provider.
func (a *MultiLLMAdapter) AddProvider(name string, p llm.Provider) {
	a.providers[name] = p
}

// Chat implements lobes.LLMProvider using the primary provider.
func (a *MultiLLMAdapter) Chat(ctx context.Context, req *llm.ChatRequest) (*llm.ChatResponse, error) {
	if a.primary == nil {
		log.Error("[Brain] MultiLLMAdapter: no primary provider configured")
		return nil, fmt.Errorf("no primary LLM provider configured")
	}
	log.Debug("[Brain] MultiLLMAdapter: Chat via primary provider")
	return a.primary.Chat(ctx, req)
}

// ChatWith routes to a specific named provider.
func (a *MultiLLMAdapter) ChatWith(ctx context.Context, providerName string, req *llm.ChatRequest) (*llm.ChatResponse, error) {
	p, ok := a.providers[providerName]
	if !ok {
		// Fallback to primary
		if a.primary == nil {
			log.Error("[Brain] MultiLLMAdapter: provider %q not found and no primary", providerName)
			return nil, fmt.Errorf("provider %q not found and no primary configured", providerName)
		}
		log.Debug("[Brain] MultiLLMAdapter: provider %q not found, using primary", providerName)
		return a.primary.Chat(ctx, req)
	}
	log.Debug("[Brain] MultiLLMAdapter: ChatWith provider=%s", providerName)
	return p.Chat(ctx, req)
}

// Verify MultiLLMAdapter implements lobes.LLMProvider at compile time.
var _ lobes.LLMProvider = (*MultiLLMAdapter)(nil)

// ═══════════════════════════════════════════════════════════════════════════════
// EMBEDDER ADAPTER
// ═══════════════════════════════════════════════════════════════════════════════

// EmbedderAdapter adapts the router.Embedder to the brain.Embedder interface.
type EmbedderAdapter struct {
	embedder router.Embedder
}

// NewEmbedderAdapter creates a new EmbedderAdapter.
func NewEmbedderAdapter(embedder router.Embedder) *EmbedderAdapter {
	return &EmbedderAdapter{
		embedder: embedder,
	}
}

// Embed generates an embedding for the given text and converts it to []float64.
func (a *EmbedderAdapter) Embed(ctx context.Context, text string) ([]float64, error) {
	if a.embedder == nil {
		log.Debug("[Brain] EmbedderAdapter: embedder not configured, returning nil")
		return nil, nil
	}

	log.Debug("[Brain] EmbedderAdapter: Embed text length=%d", len(text))
	embedding, err := a.embedder.Embed(ctx, text)
	if err != nil {
		log.Error("[Brain] EmbedderAdapter: Embed failed: %v", err)
		return nil, err
	}

	// Convert []float32 to []float64
	result := make([]float64, len(embedding))
	for i, v := range embedding {
		result[i] = float64(v)
	}

	log.Debug("[Brain] EmbedderAdapter: Embed returned %d dimensions", len(result))
	return result, nil
}

// Verify that EmbedderAdapter implements brain.Embedder
var _ brain.Embedder = (*EmbedderAdapter)(nil)
