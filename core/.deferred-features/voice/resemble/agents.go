// Package resemble provides Resemble.ai API clients for TTS and Voice Agents.
// This file implements the Voice Agents API for CR-016.
package resemble

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/normanking/cortex/internal/logging"
)

const (
	// AgentsAPIBase is the base URL for the Resemble Agents API
	AgentsAPIBase = "https://app.resemble.ai/api/v2"
	// AgentsTimeout is the default timeout for agent API requests
	AgentsTimeout = 30 * time.Second
)

// Agent represents a Resemble Voice Agent.
// This is a flattened structure combining data from the API's nested response.
type Agent struct {
	UUID        string   `json:"uuid"`
	Name        string   `json:"name"`
	Status      string   `json:"status"`
	Languages   []string `json:"languages,omitempty"`
	CallsToday  int      `json:"calls_today,omitempty"`
	VoiceUUID   string   `json:"voice_uuid,omitempty"`
	VoiceName   string   `json:"voice_name,omitempty"`
	LLMProvider string   `json:"llm_provider,omitempty"`
	LLMModel    string   `json:"llm_model,omitempty"`
	LLMPrompt   string   `json:"llm_prompt,omitempty"`
	ASRProvider string   `json:"asr_provider,omitempty"`
	ASRModel    string   `json:"asr_model,omitempty"`
	ToolsCount  int      `json:"tools_count,omitempty"`
	Tools       []Tool   `json:"tools,omitempty"`
	CreatedAt   string   `json:"created_at,omitempty"`
	UpdatedAt   string   `json:"updated_at,omitempty"`
}

// agentCoreInfo is the nested "agent" object in the API response.
type agentCoreInfo struct {
	UUID       string   `json:"uuid"`
	Name       string   `json:"name"`
	Status     string   `json:"status"`
	Languages  []string `json:"languages"`
	CallsToday int      `json:"calls_today"`
	CreatedAt  string   `json:"created_at"`
	UpdatedAt  string   `json:"updated_at"`
}

// agentTTSConfig is the nested "tts" object in the API response.
type agentTTSConfig struct {
	VoiceUUID string `json:"voice_uuid"`
	VoiceName string `json:"voice_name"`
}

// agentLLMConfig is the nested "llm" object in the API response.
type agentLLMConfig struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Prompt   string `json:"prompt"`
}

// agentASRConfig is the nested "asr" object in the API response.
type agentASRConfig struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

// agentListItem is the structure of each item in the list response.
type agentListItem struct {
	Agent         agentCoreInfo  `json:"agent"`
	TTS           agentTTSConfig `json:"tts"`
	LLM           agentLLMConfig `json:"llm"`
	ASR           agentASRConfig `json:"asr"`
	ToolsCount    int            `json:"tools_count"`
	WebhooksCount int            `json:"webhooks_count"`
}

// agentGetItem is the structure of the item in the get response.
type agentGetItem struct {
	Agent         agentCoreInfo  `json:"agent"`
	TTS           agentTTSConfig `json:"tts"`
	LLM           agentLLMConfig `json:"llm"`
	ASR           agentASRConfig `json:"asr"`
	ToolsCount    int            `json:"tools_count"`
	WebhooksCount int            `json:"webhooks_count"`
	Tools         []Tool         `json:"tools,omitempty"`
}

// Tool represents a tool registered with a Resemble Agent.
type Tool struct {
	Name        string               `json:"name"`
	Description string               `json:"description"`
	ToolType    string               `json:"tool_type"` // "webhook", "builtin"
	Active      bool                 `json:"active"`
	Parameters  map[string]ToolParam `json:"parameters,omitempty"`
	APISchema   *ToolAPISchema       `json:"api_schema,omitempty"`
}

// ToolParam describes a parameter for a tool.
type ToolParam struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Required    bool   `json:"required,omitempty"`
}

// ToolAPISchema defines the webhook configuration for a tool.
type ToolAPISchema struct {
	URL            string            `json:"url"`
	Method         string            `json:"method"`
	RequestHeaders map[string]string `json:"request_headers,omitempty"`
	RequestBody    map[string]string `json:"request_body,omitempty"`
}

// Capabilities represents the supported ASR/LLM/TTS options for agents.
type Capabilities struct {
	ASRProviders []string `json:"asr_providers"`
	LLMModels    []string `json:"llm_models"`
	TTSVoices    []string `json:"tts_voices"`
	Languages    []string `json:"languages"`
}

// AgentsClient is the client for the Resemble Agents API.
type AgentsClient struct {
	apiKey  string
	client  *http.Client
	baseURL string
	log     *logging.Logger
}

// NewAgentsClient creates a new Resemble Agents API client.
func NewAgentsClient(apiKey string) *AgentsClient {
	return &AgentsClient{
		apiKey:  apiKey,
		client:  &http.Client{Timeout: AgentsTimeout},
		baseURL: AgentsAPIBase,
		log:     logging.Global(),
	}
}

// agentsResponse wraps the API response for listing agents.
type agentsResponse struct {
	Success bool            `json:"success"`
	Items   []agentListItem `json:"items"`
	Count   int             `json:"count"`
}

// agentResponse wraps the API response for a single agent.
type agentResponse struct {
	Success bool         `json:"success"`
	Item    agentGetItem `json:"item"`
	Error   string       `json:"error,omitempty"`
}

// capabilitiesResponse wraps the API response for capabilities.
type capabilitiesResponse struct {
	Success      bool         `json:"success"`
	Capabilities Capabilities `json:"capabilities"`
}

// ListAgents fetches all Voice Agents from the Resemble API.
func (c *AgentsClient) ListAgents(ctx context.Context) ([]Agent, error) {
	url := c.baseURL + "/agents"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("resemble agents: failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")

	c.log.Info("[ResembleAgents] Listing agents from %s", url)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("resemble agents: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("resemble agents: failed to read response: %w", err)
	}

	c.log.Info("[ResembleAgents] Response status=%d body=%s", resp.StatusCode, string(body))

	if resp.StatusCode != http.StatusOK {
		c.log.Error("[ResembleAgents] API error: status=%d body=%s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("resemble agents: API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result agentsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("resemble agents: failed to parse response: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("resemble agents: API returned unsuccessful response")
	}

	// Convert API response items to Agent structs
	agents := make([]Agent, len(result.Items))
	for i, item := range result.Items {
		agents[i] = Agent{
			UUID:        item.Agent.UUID,
			Name:        item.Agent.Name,
			Status:      item.Agent.Status,
			Languages:   item.Agent.Languages,
			CallsToday:  item.Agent.CallsToday,
			VoiceUUID:   item.TTS.VoiceUUID,
			VoiceName:   item.TTS.VoiceName,
			LLMProvider: item.LLM.Provider,
			LLMModel:    item.LLM.Model,
			LLMPrompt:   item.LLM.Prompt,
			ASRProvider: item.ASR.Provider,
			ASRModel:    item.ASR.Model,
			ToolsCount:  item.ToolsCount,
			CreatedAt:   item.Agent.CreatedAt,
			UpdatedAt:   item.Agent.UpdatedAt,
		}
	}

	c.log.Info("[ResembleAgents] Found %d agents", len(agents))
	return agents, nil
}

// GetAgent fetches a specific Voice Agent by UUID.
func (c *AgentsClient) GetAgent(ctx context.Context, uuid string) (*Agent, error) {
	url := c.baseURL + "/agents/" + uuid

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("resemble agents: failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")

	c.log.Info("[ResembleAgents] Getting agent %s", uuid)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("resemble agents: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("resemble agents: failed to read response: %w", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("resemble agents: agent not found: %s", uuid)
	}

	if resp.StatusCode != http.StatusOK {
		c.log.Error("[ResembleAgents] API error: status=%d body=%s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("resemble agents: API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result agentResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("resemble agents: failed to parse response: %w", err)
	}

	if !result.Success {
		errMsg := result.Error
		if errMsg == "" {
			errMsg = "unknown error"
		}
		return nil, fmt.Errorf("resemble agents: %s", errMsg)
	}

	// Convert API response to Agent struct
	item := result.Item
	agent := &Agent{
		UUID:        item.Agent.UUID,
		Name:        item.Agent.Name,
		Status:      item.Agent.Status,
		Languages:   item.Agent.Languages,
		CallsToday:  item.Agent.CallsToday,
		VoiceUUID:   item.TTS.VoiceUUID,
		VoiceName:   item.TTS.VoiceName,
		LLMProvider: item.LLM.Provider,
		LLMModel:    item.LLM.Model,
		LLMPrompt:   item.LLM.Prompt,
		ASRProvider: item.ASR.Provider,
		ASRModel:    item.ASR.Model,
		ToolsCount:  item.ToolsCount,
		Tools:       item.Tools,
		CreatedAt:   item.Agent.CreatedAt,
		UpdatedAt:   item.Agent.UpdatedAt,
	}

	return agent, nil
}

// GetCapabilities fetches the available ASR/LLM/TTS options for agents.
func (c *AgentsClient) GetCapabilities(ctx context.Context) (*Capabilities, error) {
	url := c.baseURL + "/agents/capabilities"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("resemble agents: failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")

	c.log.Info("[ResembleAgents] Getting capabilities")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("resemble agents: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("resemble agents: failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		c.log.Error("[ResembleAgents] API error: status=%d body=%s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("resemble agents: API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result capabilitiesResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("resemble agents: failed to parse response: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("resemble agents: failed to fetch capabilities")
	}

	return &result.Capabilities, nil
}

// Health checks if the Agents API is accessible.
func (c *AgentsClient) Health(ctx context.Context) error {
	_, err := c.ListAgents(ctx)
	if err != nil {
		return fmt.Errorf("resemble agents: health check failed: %w", err)
	}
	return nil
}

// AgentInfo provides a simplified view of an agent for display.
type AgentInfo struct {
	UUID        string
	Name        string
	Description string
	VoiceName   string
	LLMModel    string
	Status      string
	HasTools    bool
}

// ToAgentInfo converts an Agent to AgentInfo for display purposes.
func (a *Agent) ToAgentInfo() AgentInfo {
	// Build description from LLM model and voice info
	description := ""
	if a.LLMModel != "" {
		description = a.LLMModel
	}
	if a.VoiceName != "" {
		if description != "" {
			description += " • "
		}
		description += a.VoiceName
	}

	return AgentInfo{
		UUID:        a.UUID,
		Name:        a.Name,
		Description: description,
		VoiceName:   a.VoiceName,
		LLMModel:    a.LLMModel,
		Status:      a.Status,
		HasTools:    a.ToolsCount > 0 || len(a.Tools) > 0,
	}
}
