package brain

import (
	"context"
	"testing"
	"time"

	"github.com/normanking/cortex/internal/llm"
	pkgbrain "github.com/normanking/cortex/pkg/brain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockLLMProvider struct {
	response *llm.ChatResponse
	err      error
}

func (m *mockLLMProvider) Chat(ctx context.Context, req *llm.ChatRequest) (*llm.ChatResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.response, nil
}

func (m *mockLLMProvider) Name() string    { return "mock" }
func (m *mockLLMProvider) Available() bool { return true }

func TestLLMAdapter_Chat(t *testing.T) {
	mockResp := &llm.ChatResponse{
		Content:    "Hello, world!",
		Model:      "test-model",
		TokensUsed: 10,
		Duration:   100 * time.Millisecond,
	}

	adapter := NewLLMAdapter(&mockLLMProvider{response: mockResp})

	req := &llm.ChatRequest{
		Messages: []llm.Message{{Role: "user", Content: "Hi"}},
	}

	resp, err := adapter.Chat(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "Hello, world!", resp.Content)
	assert.Equal(t, "test-model", resp.Model)
}

func TestLLMAdapter_NilProvider(t *testing.T) {
	adapter := NewLLMAdapter(nil)

	req := &llm.ChatRequest{
		Messages: []llm.Message{{Role: "user", Content: "Hi"}},
	}

	_, err := adapter.Chat(context.Background(), req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not configured")
}

func TestMultiLLMAdapter_Primary(t *testing.T) {
	mockResp := &llm.ChatResponse{Content: "primary response"}
	adapter := NewMultiLLMAdapter(&mockLLMProvider{response: mockResp})

	req := &llm.ChatRequest{
		Messages: []llm.Message{{Role: "user", Content: "test"}},
	}

	resp, err := adapter.Chat(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "primary response", resp.Content)
}

func TestMultiLLMAdapter_NamedProvider(t *testing.T) {
	primaryResp := &llm.ChatResponse{Content: "primary"}
	secondaryResp := &llm.ChatResponse{Content: "secondary"}

	adapter := NewMultiLLMAdapter(&mockLLMProvider{response: primaryResp})
	adapter.AddProvider("fast", &mockLLMProvider{response: secondaryResp})

	req := &llm.ChatRequest{
		Messages: []llm.Message{{Role: "user", Content: "test"}},
	}

	resp, err := adapter.ChatWith(context.Background(), "fast", req)
	require.NoError(t, err)
	assert.Equal(t, "secondary", resp.Content)

	resp, err = adapter.ChatWith(context.Background(), "unknown", req)
	require.NoError(t, err)
	assert.Equal(t, "primary", resp.Content)
}

func TestSimpleCache(t *testing.T) {
	cache := NewSimpleCache()

	_, ok := cache.Get("missing")
	assert.False(t, ok)

	cache.Set("key1", &pkgbrain.ClassificationResult{
		PrimaryLobe: "coding",
		Confidence:  0.9,
	})

	result, ok := cache.Get("key1")
	assert.True(t, ok)
	assert.Equal(t, pkgbrain.LobeID("coding"), result.PrimaryLobe)
	assert.Equal(t, 0.9, result.Confidence)
}

func TestClassifierLLMAdapter_Classify(t *testing.T) {
	mockResp := &llm.ChatResponse{Content: "coding"}
	adapter := &ClassifierLLMAdapter{provider: &mockLLMProvider{response: mockResp}}

	candidates := []pkgbrain.LobeID{"coding", "reasoning", "memory"}
	result, err := adapter.Classify(context.Background(), "write a function", candidates)

	require.NoError(t, err)
	assert.Equal(t, pkgbrain.LobeID("coding"), result.PrimaryLobe)
	assert.Equal(t, "llm", result.Method)
}

func TestClassifierLLMAdapter_NilProvider(t *testing.T) {
	adapter := &ClassifierLLMAdapter{provider: nil}

	candidates := []pkgbrain.LobeID{"coding", "reasoning"}
	result, err := adapter.Classify(context.Background(), "test", candidates)

	require.NoError(t, err)
	assert.Equal(t, pkgbrain.LobeID("coding"), result.PrimaryLobe)
	assert.Equal(t, "default", result.Method)
}
