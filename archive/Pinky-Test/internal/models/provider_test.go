package models

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestModelInfo_Struct(t *testing.T) {
	m := ModelInfo{
		ID:          "gpt-4o",
		Name:        "GPT-4o",
		Description: "Test model",
		ContextSize: 128000,
	}

	if m.ID != "gpt-4o" {
		t.Errorf("Expected ID 'gpt-4o', got '%s'", m.ID)
	}
	if m.ContextSize != 128000 {
		t.Errorf("Expected ContextSize 128000, got %d", m.ContextSize)
	}
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	registry := NewRegistry()

	// Register a static provider
	provider := NewStaticProvider("test", []ModelInfo{
		{ID: "model-1", Name: "Model 1"},
	})
	registry.Register(provider)

	// Test Get
	p, ok := registry.Get("test")
	if !ok {
		t.Error("Expected to find provider 'test'")
	}
	if p.Engine() != "test" {
		t.Errorf("Expected engine 'test', got '%s'", p.Engine())
	}

	// Test Get for non-existent provider
	_, ok = registry.Get("nonexistent")
	if ok {
		t.Error("Expected not to find provider 'nonexistent'")
	}
}

func TestRegistry_ListModels(t *testing.T) {
	registry := NewRegistry()

	// Register providers
	registry.Register(NewStaticProvider("provider1", []ModelInfo{
		{ID: "p1-m1", Name: "Provider1 Model1"},
		{ID: "p1-m2", Name: "Provider1 Model2"},
	}))
	registry.Register(NewStaticProvider("provider2", []ModelInfo{
		{ID: "p2-m1", Name: "Provider2 Model1"},
	}))

	ctx := context.Background()

	// Test ListModels for provider1
	models, err := registry.ListModels(ctx, "provider1")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(models) != 2 {
		t.Errorf("Expected 2 models, got %d", len(models))
	}

	// Test ListModels for unknown provider
	_, err = registry.ListModels(ctx, "unknown")
	if err == nil {
		t.Error("Expected error for unknown provider")
	}
}

func TestRegistry_ValidateModel(t *testing.T) {
	registry := NewRegistry()
	registry.Register(NewStaticProvider("test", []ModelInfo{
		{ID: "valid-model", Name: "Valid Model"},
	}))

	if !registry.ValidateModel("test", "valid-model") {
		t.Error("Expected 'valid-model' to be valid")
	}
	if registry.ValidateModel("test", "invalid-model") {
		t.Error("Expected 'invalid-model' to be invalid")
	}
	if registry.ValidateModel("unknown", "valid-model") {
		t.Error("Expected validation to fail for unknown engine")
	}
}

func TestRegistry_Engines(t *testing.T) {
	registry := NewRegistry()
	
	if len(registry.Engines()) != 0 {
		t.Error("Expected empty registry to have no engines")
	}

	registry.Register(NewStaticProvider("engine1", []ModelInfo{}))
	registry.Register(NewStaticProvider("engine2", []ModelInfo{}))

	engines := registry.Engines()
	if len(engines) != 2 {
		t.Errorf("Expected 2 engines, got %d", len(engines))
	}
}

func TestStaticProvider_Engine(t *testing.T) {
	p := NewStaticProvider("my-engine", []ModelInfo{})
	if p.Engine() != "my-engine" {
		t.Errorf("Expected engine 'my-engine', got '%s'", p.Engine())
	}
}

func TestStaticProvider_ListModels(t *testing.T) {
	original := []ModelInfo{
		{ID: "model-1", Name: "Model 1", ContextSize: 1000},
		{ID: "model-2", Name: "Model 2", ContextSize: 2000},
	}
	p := NewStaticProvider("test", original)

	ctx := context.Background()
	models, err := p.ListModels(ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(models) != 2 {
		t.Errorf("Expected 2 models, got %d", len(models))
	}

	// Verify modifying returned slice doesn't affect original
	models[0].ID = "modified"
	models2, _ := p.ListModels(ctx)
	if models2[0].ID == "modified" {
		t.Error("ListModels should return a copy, not the original")
	}
}

func TestStaticProvider_ValidateModel(t *testing.T) {
	p := NewStaticProvider("test", []ModelInfo{
		{ID: "valid-model", Name: "Valid"},
		{ID: "another-model", Name: "Another"},
	})

	if !p.ValidateModel("valid-model") {
		t.Error("Expected 'valid-model' to be valid")
	}
	if !p.ValidateModel("another-model") {
		t.Error("Expected 'another-model' to be valid")
	}
	if p.ValidateModel("invalid-model") {
		t.Error("Expected 'invalid-model' to be invalid")
	}
}

func TestNewAnthropicProvider(t *testing.T) {
	p := NewAnthropicProvider()
	if p.Engine() != "anthropic" {
		t.Errorf("Expected engine 'anthropic', got '%s'", p.Engine())
	}
	if len(p.models) == 0 {
		t.Error("Expected Anthropic provider to have models")
	}
}

func TestNewOpenAIProvider(t *testing.T) {
	p := NewOpenAIProvider()
	if p.Engine() != "openai" {
		t.Errorf("Expected engine 'openai', got '%s'", p.Engine())
	}
	if len(p.models) == 0 {
		t.Error("Expected OpenAI provider to have models")
	}
}

func TestNewGroqProvider(t *testing.T) {
	p := NewGroqProvider()
	if p.Engine() != "groq" {
		t.Errorf("Expected engine 'groq', got '%s'", p.Engine())
	}
	if len(p.models) == 0 {
		t.Error("Expected Groq provider to have models")
	}
}

func TestOllamaProvider_Engine(t *testing.T) {
	p := NewOllamaProvider("")
	if p.Engine() != "ollama" {
		t.Errorf("Expected engine 'ollama', got '%s'", p.Engine())
	}
}

func TestOllamaProvider_DefaultURL(t *testing.T) {
	p := NewOllamaProvider("")
	if p.baseURL != DefaultOllamaURL {
		t.Errorf("Expected default URL '%s', got '%s'", DefaultOllamaURL, p.baseURL)
	}
}

func TestOllamaProvider_CustomURL(t *testing.T) {
	customURL := "http://192.168.1.100:11434"
	p := NewOllamaProvider(customURL)
	if p.baseURL != customURL {
		t.Errorf("Expected custom URL '%s', got '%s'", customURL, p.baseURL)
	}
}

func TestOllamaProvider_ListModels_Success(t *testing.T) {
	// Create mock server
	mockResponse := OllamaTagsResponse{
		Models: []OllamaModel{
			{
				Name:   "llama3.2:3b",
				Model:  "llama3.2:3b",
				Size:   2000000000,
				Digest: "abc123",
				Details: struct {
					ParentModel       string   `json:"parent_model"`
					Format            string   `json:"format"`
					Family            string   `json:"family"`
					Families          []string `json:"families"`
					ParameterSize     string   `json:"parameter_size"`
					QuantizationLevel string   `json:"quantization_level"`
				}{
					Family:        "llama3.2",
					ParameterSize: "3B",
				},
			},
			{
				Name:   "mistral:latest",
				Model:  "mistral:latest",
				Size:   4000000000,
				Details: struct {
					ParentModel       string   `json:"parent_model"`
					Format            string   `json:"format"`
					Family            string   `json:"family"`
					Families          []string `json:"families"`
					ParameterSize     string   `json:"parameter_size"`
					QuantizationLevel string   `json:"quantization_level"`
				}{
					Family:        "mistral",
					ParameterSize: "7B",
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			t.Errorf("Expected path '/api/tags', got '%s'", r.URL.Path)
		}
		if r.Method != "GET" {
			t.Errorf("Expected GET method, got '%s'", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	p := NewOllamaProvider(server.URL)
	ctx := context.Background()
	models, err := p.ListModels(ctx)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(models) != 2 {
		t.Errorf("Expected 2 models, got %d", len(models))
	}
	if models[0].ID != "llama3.2:3b" {
		t.Errorf("Expected model ID 'llama3.2:3b', got '%s'", models[0].ID)
	}
	if models[1].ContextSize != 32768 {
		t.Errorf("Expected mistral context size 32768, got %d", models[1].ContextSize)
	}
}

func TestOllamaProvider_ListModels_Error(t *testing.T) {
	// Test with server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	p := NewOllamaProvider(server.URL)
	ctx := context.Background()
	_, err := p.ListModels(ctx)

	if err == nil {
		t.Error("Expected error for failed request")
	}
}

func TestOllamaProvider_ListModels_Timeout(t *testing.T) {
	// Test with server that never responds
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Don't respond
	}))
	defer server.Close()

	p := NewOllamaProvider(server.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 1)
	defer cancel()

	_, err := p.ListModels(ctx)
	if err == nil {
		t.Error("Expected timeout error")
	}
}

func TestOllamaProvider_ValidateModel(t *testing.T) {
	mockResponse := OllamaTagsResponse{
		Models: []OllamaModel{
			{Name: "llama3.2:3b"},
			{Name: "mistral:latest"},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	p := NewOllamaProvider(server.URL)

	if !p.ValidateModel("llama3.2:3b") {
		t.Error("Expected 'llama3.2:3b' to be valid")
	}
	if !p.ValidateModel("mistral:latest") {
		t.Error("Expected 'mistral:latest' to be valid")
	}
	if p.ValidateModel("nonexistent") {
		t.Error("Expected 'nonexistent' to be invalid")
	}
}

func TestOllamaProvider_ValidateModel_Unreachable(t *testing.T) {
	// Test validation when Ollama is not running
	p := NewOllamaProvider("http://localhost:59999")
	if p.ValidateModel("any-model") {
		t.Error("Expected validation to fail when Ollama is unreachable")
	}
}

func TestEstimateContextSize(t *testing.T) {
	tests := []struct {
		family   string
		expected int
	}{
		{"llama3", 128000},
		{"llama3.1", 128000},
		{"llama3.2", 128000},
		{"mistral", 32768},
		{"mixtral", 32768},
		{"qwen2", 128000},
		{"phi3", 128000},
		{"phi4", 128000},
		{"gemma2", 8192},
		{"codellama", 16384},
		{"deepseek", 128000},
		{"unknown", 4096},
	}

	for _, tt := range tests {
		m := OllamaModel{
			Details: struct {
				ParentModel       string   `json:"parent_model"`
				Format            string   `json:"format"`
				Family            string   `json:"family"`
				Families          []string `json:"families"`
				ParameterSize     string   `json:"parameter_size"`
				QuantizationLevel string   `json:"quantization_level"`
			}{
				Family: tt.family,
			},
		}
		result := estimateContextSize(m)
		if result != tt.expected {
			t.Errorf("For family '%s', expected %d, got %d", tt.family, tt.expected, result)
		}
	}
}

func TestFormatOllamaDescription(t *testing.T) {
	tests := []struct {
		model    OllamaModel
		expected string
	}{
		{
			OllamaModel{Details: struct {
				ParentModel       string   `json:"parent_model"`
				Format            string   `json:"format"`
				Family            string   `json:"family"`
				Families          []string `json:"families"`
				ParameterSize     string   `json:"parameter_size"`
				QuantizationLevel string   `json:"quantization_level"`
			}{Family: "llama3.2", ParameterSize: "3B"}},
			"llama3.2 family, 3B",
		},
		{
			OllamaModel{Details: struct {
				ParentModel       string   `json:"parent_model"`
				Format            string   `json:"format"`
				Family            string   `json:"family"`
				Families          []string `json:"families"`
				ParameterSize     string   `json:"parameter_size"`
				QuantizationLevel string   `json:"quantization_level"`
			}{ParameterSize: "7B"}},
			"7B",
		},
		{
			OllamaModel{Details: struct {
				ParentModel       string   `json:"parent_model"`
				Format            string   `json:"format"`
				Family            string   `json:"family"`
				Families          []string `json:"families"`
				ParameterSize     string   `json:"parameter_size"`
				QuantizationLevel string   `json:"quantization_level"`
			}{Family: "mistral"}},
			"mistral",
		},
		{
			OllamaModel{},
			"Local Ollama model",
		},
	}

	for _, tt := range tests {
		result := formatOllamaDescription(tt.model)
		if result != tt.expected {
			t.Errorf("Expected '%s', got '%s'", tt.expected, result)
		}
	}
}
