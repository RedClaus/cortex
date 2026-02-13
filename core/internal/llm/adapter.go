package llm

import (
	"context"

	"github.com/normanking/cortex/pkg/types"
)

// TypesAdapter adapts our Provider interface to the shared types.LLMProvider interface.
type TypesAdapter struct {
	provider Provider
}

// NewTypesAdapter creates an adapter that bridges our LLM provider
// to the shared types interface (used by orchestrator and introspection).
func NewTypesAdapter(p Provider) *TypesAdapter {
	return &TypesAdapter{provider: p}
}

// Chat implements types-compatible LLM interface.
func (a *TypesAdapter) Chat(ctx context.Context, req *types.LLMRequest) (*types.LLMResponse, error) {
	chatReq := &ChatRequest{
		Model:        req.Model,
		SystemPrompt: req.SystemPrompt,
		MaxTokens:    req.MaxTokens,
		Temperature:  req.Temperature,
	}

	for _, msg := range req.Messages {
		chatReq.Messages = append(chatReq.Messages, Message{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	resp, err := a.provider.Chat(ctx, chatReq)
	if err != nil {
		return nil, err
	}

	return &types.LLMResponse{
		Content:    resp.Content,
		Model:      resp.Model,
		TokensUsed: resp.TokensUsed,
		Duration:   resp.Duration,
	}, nil
}

// Name returns the provider identifier.
func (a *TypesAdapter) Name() string {
	return a.provider.Name()
}

// Available returns true if the provider is configured.
func (a *TypesAdapter) Available() bool {
	return a.provider.Available()
}
