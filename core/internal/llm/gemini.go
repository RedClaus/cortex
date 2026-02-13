package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// GeminiProvider implements the Provider interface for Google Gemini.
type GeminiProvider struct {
	baseProvider
}

// NewGeminiProvider creates a new Gemini provider.
func NewGeminiProvider(cfg *ProviderConfig) *GeminiProvider {
	return &GeminiProvider{
		baseProvider: newBaseProvider(cfg, "gemini"),
	}
}

// Chat sends a chat request to Gemini.
func (p *GeminiProvider) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	if p.config.APIKey == "" {
		return nil, fmt.Errorf("Gemini API key not configured")
	}

	start := time.Now()

	model := req.Model
	if model == "" {
		model = p.config.Model
	}

	// Build Gemini request
	geminiReq := geminiGenerateRequest{
		Contents: []geminiContent{},
	}

	// Set generation config
	geminiReq.GenerationConfig.MaxOutputTokens = req.MaxTokens
	if geminiReq.GenerationConfig.MaxOutputTokens == 0 {
		geminiReq.GenerationConfig.MaxOutputTokens = p.config.MaxTokens
	}
	geminiReq.GenerationConfig.Temperature = req.Temperature
	if geminiReq.GenerationConfig.Temperature == 0 {
		geminiReq.GenerationConfig.Temperature = p.config.Temperature
	}

	// Add system instruction if provided
	if req.SystemPrompt != "" {
		geminiReq.SystemInstruction = &geminiContent{
			Parts: []geminiPart{{Text: req.SystemPrompt}},
		}
	}

	// Convert messages to Gemini format
	for _, msg := range req.Messages {
		role := msg.Role
		// Gemini uses "user" and "model" instead of "assistant"
		if role == "assistant" {
			role = "model"
		}
		geminiReq.Contents = append(geminiReq.Contents, geminiContent{
			Role:  role,
			Parts: []geminiPart{{Text: msg.Content}},
		})
	}

	// Marshal request
	body, err := json.Marshal(geminiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Build URL without API key (key goes in header to prevent log exposure)
	url := fmt.Sprintf("%s/models/%s:generateContent", p.config.Endpoint, model)

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	// Use x-goog-api-key header instead of URL parameter to prevent API key exposure in logs
	httpReq.Header.Set("x-goog-api-key", p.config.APIKey)

	// Execute request
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := readLimitedBody(resp.Body, MaxErrorBodySize)
		return nil, fmt.Errorf("Gemini error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response
	var geminiResp geminiGenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&geminiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(geminiResp.Candidates) == 0 {
		return nil, fmt.Errorf("no candidates in response")
	}

	// Extract content
	var content string
	candidate := geminiResp.Candidates[0]
	for _, part := range candidate.Content.Parts {
		content += part.Text
	}

	// Calculate tokens
	promptTokens := 0
	completionTokens := 0
	if geminiResp.UsageMetadata.PromptTokenCount > 0 {
		promptTokens = geminiResp.UsageMetadata.PromptTokenCount
	}
	if geminiResp.UsageMetadata.CandidatesTokenCount > 0 {
		completionTokens = geminiResp.UsageMetadata.CandidatesTokenCount
	}

	return &ChatResponse{
		Content:          content,
		Model:            model,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TokensUsed:       promptTokens + completionTokens,
		Duration:         time.Since(start),
		FinishReason:     candidate.FinishReason,
	}, nil
}

// Gemini API types
type geminiGenerateRequest struct {
	Contents          []geminiContent        `json:"contents"`
	SystemInstruction *geminiContent         `json:"systemInstruction,omitempty"`
	GenerationConfig  geminiGenerationConfig `json:"generationConfig,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiGenerationConfig struct {
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
	Temperature     float64 `json:"temperature,omitempty"`
	TopP            float64 `json:"topP,omitempty"`
	TopK            int     `json:"topK,omitempty"`
}

type geminiGenerateResponse struct {
	Candidates []struct {
		Content struct {
			Parts []geminiPart `json:"parts"`
			Role  string       `json:"role"`
		} `json:"content"`
		FinishReason  string `json:"finishReason"`
		SafetyRatings []struct {
			Category    string `json:"category"`
			Probability string `json:"probability"`
		} `json:"safetyRatings"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
}
