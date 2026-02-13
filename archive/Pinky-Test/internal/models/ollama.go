package models

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const (
	DefaultOllamaURL = "http://localhost:11434"
)

// OllamaProvider fetches models dynamically from Ollama API
type OllamaProvider struct {
	baseURL string
	client  *http.Client
}

// OllamaTagsResponse represents the response from /api/tags
type OllamaTagsResponse struct {
	Models []OllamaModel `json:"models"`
}

// OllamaModel represents a single model from Ollama's API
type OllamaModel struct {
	Name       string `json:"name"`
	Model      string `json:"model"`
	ModifiedAt string `json:"modified_at"`
	Size       int64  `json:"size"`
	Digest     string `json:"digest"`
	Details    struct {
		ParentModel       string   `json:"parent_model"`
		Format            string   `json:"format"`
		Family            string   `json:"family"`
		Families          []string `json:"families"`
		ParameterSize     string   `json:"parameter_size"`
		QuantizationLevel string   `json:"quantization_level"`
	} `json:"details"`
}

// NewOllamaProvider creates a new Ollama provider with the given base URL
func NewOllamaProvider(baseURL string) *OllamaProvider {
	if baseURL == "" {
		baseURL = DefaultOllamaURL
	}

	return &OllamaProvider{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Engine returns the provider engine name
func (p *OllamaProvider) Engine() string {
	return "ollama"
}

// ListModels fetches available models from Ollama's API
func (p *OllamaProvider) ListModels(ctx context.Context) ([]ModelInfo, error) {
	url := fmt.Sprintf("%s/api/tags", p.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch models from Ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Ollama returned status %d", resp.StatusCode)
	}

	var tagsResp OllamaTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tagsResp); err != nil {
		return nil, fmt.Errorf("failed to decode Ollama response: %w", err)
	}

	models := make([]ModelInfo, 0, len(tagsResp.Models))
	for _, m := range tagsResp.Models {
		modelInfo := ModelInfo{
			ID:          m.Name,
			Name:        formatOllamaName(m.Name),
			Description: formatOllamaDescription(m),
			ContextSize: estimateContextSize(m),
		}
		models = append(models, modelInfo)
	}

	return models, nil
}

// ValidateModel checks if a model exists in Ollama
func (p *OllamaProvider) ValidateModel(model string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	models, err := p.ListModels(ctx)
	if err != nil {
		return false
	}

	for _, m := range models {
		if m.ID == model {
			return true
		}
	}
	return false
}

// formatOllamaName creates a display name from the model ID
func formatOllamaName(name string) string {
	// Convert "llama3.2:3b" to "Llama 3.2 (3B)"
	return name
}

// formatOllamaDescription creates a description from model details
func formatOllamaDescription(m OllamaModel) string {
	if m.Details.ParameterSize != "" {
		if m.Details.Family != "" {
			return fmt.Sprintf("%s family, %s", m.Details.Family, m.Details.ParameterSize)
		}
		return m.Details.ParameterSize
	}
	if m.Details.Family != "" {
		return m.Details.Family
	}
	return "Local Ollama model"
}

// estimateContextSize returns an estimated context window for common models
func estimateContextSize(m OllamaModel) int {
	// Common context window sizes for known models
	contextSizes := map[string]int{
		"llama3":     128000,
		"llama3.1":   128000,
		"llama3.2":   128000,
		"mistral":    32768,
		"mixtral":    32768,
		"qwen2":      128000,
		"qwen2.5":    128000,
		"phi3":       128000,
		"phi4":       128000,
		"gemma2":     8192,
		"codellama":  16384,
		"deepseek":   128000,
	}

	for family, size := range contextSizes {
		if m.Details.Family == family {
			return size
		}
	}

	// Default context size if unknown
	return 4096
}
