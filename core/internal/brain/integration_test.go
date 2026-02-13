//go:build integration

package brain

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/normanking/cortex/internal/llm"
	"github.com/normanking/cortex/pkg/brain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func skipIfNoOllama(t *testing.T) {
	endpoint := getOllamaEndpoint()
	models, err := llm.FetchOllamaModels(endpoint)
	if err != nil || len(models) == 0 {
		t.Skip("Ollama not available or no models installed")
	}
}

func getOllamaEndpoint() string {
	if endpoint := os.Getenv("OLLAMA_ENDPOINT"); endpoint != "" {
		return endpoint
	}
	return "http://127.0.0.1:11434"
}

func createOllamaProvider(endpoint, model string) llm.Provider {
	cfg := &llm.ProviderConfig{
		Name:     "ollama",
		Endpoint: endpoint,
		Model:    model,
	}
	return llm.NewOllamaProvider(cfg)
}

func TestIntegration_BrainExecutive_WithOllama(t *testing.T) {
	skipIfNoOllama(t)

	provider := createOllamaProvider(getOllamaEndpoint(), "llama3.2:1b")

	cfg := FactoryConfig{
		LLMProvider: provider,
		UserID:      "test-user",
	}

	exec := NewExecutive(cfg)
	require.NotNil(t, exec)

	exec.Start()
	defer exec.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := exec.Process(ctx, "What is 2 + 2?")
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.NotNil(t, result.Classification)
	assert.NotNil(t, result.Strategy)
	assert.NotEmpty(t, result.FinalContent)
}

func TestIntegration_ClassificationAccuracy(t *testing.T) {
	skipIfNoOllama(t)

	provider := createOllamaProvider(getOllamaEndpoint(), "llama3.2:1b")

	testCases := []struct {
		input        string
		expectedLobe brain.LobeID
	}{
		{"Write a Python function to sort a list", "coding"},
		{"What is the capital of France?", "memory"},
		{"Analyze this logical argument for fallacies", "reasoning"},
		{"Create a poem about the ocean", "creativity"},
	}

	classifier := &ClassifierLLMAdapter{provider: provider}
	candidates := []brain.LobeID{"coding", "reasoning", "memory", "creativity", "planning"}

	for _, tc := range testCases {
		t.Run(tc.input[:30], func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			result, err := classifier.Classify(ctx, tc.input, candidates)
			require.NoError(t, err)
			assert.NotEmpty(t, result.PrimaryLobe)
			t.Logf("Input: %q -> Classified as: %s (expected: %s)", tc.input, result.PrimaryLobe, tc.expectedLobe)
		})
	}
}

func TestIntegration_MultiLLMAdapter_Fallback(t *testing.T) {
	skipIfNoOllama(t)

	endpoint := getOllamaEndpoint()
	primary := createOllamaProvider(endpoint, "llama3.2:1b")

	adapter := NewMultiLLMAdapter(primary)

	models, _ := llm.FetchOllamaModels(endpoint)
	if len(models) > 1 {
		secondary := createOllamaProvider(endpoint, models[1].Name)
		adapter.AddProvider("secondary", secondary)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req := &llm.ChatRequest{
		Messages: []llm.Message{{Role: "user", Content: "Say 'test'"}},
	}

	resp, err := adapter.Chat(ctx, req)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Content)
}

func TestIntegration_ExecutiveLifecycle(t *testing.T) {
	skipIfNoOllama(t)

	provider := createOllamaProvider(getOllamaEndpoint(), "llama3.2:1b")

	cfg := FactoryConfig{
		LLMProvider: provider,
		UserID:      "test-user",
	}

	exec := NewExecutive(cfg)
	require.NotNil(t, exec)

	exec.Start()
	defer exec.Stop()

	registry := exec.Registry()
	require.NotNil(t, registry)

	lobes := registry.All()
	assert.NotEmpty(t, lobes)
	t.Logf("Registered %d lobes", len(lobes))

	metrics := exec.GetMetrics()
	assert.NotNil(t, metrics)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := exec.Process(ctx, "Hello!")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotNil(t, result.Classification)
}
