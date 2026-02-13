// brain/persona.go - Persona client for CortexBrain API
// This file adds persona loading capabilities to the Cortex-Gateway brain client

package brain

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// PersonaSummary represents a persona summary from registry
type PersonaSummary struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	DisplayName string   `json:"display_name"`
	Role        string   `json:"role"`
	Category    string   `json:"category"`
	Version     string   `json:"version"`
	Status      string   `json:"status"`
	Path        string   `json:"path"`
	Traits      Traits   `json:"traits"`
	Tags        []string `json:"tags"`
}

// PersonaDetail represents a full persona with prompts
type PersonaDetail struct {
	PersonaSummary
	Capabilities []string        `json:"capabilities"`
	Style        Style           `json:"style"`
	Prompts      Prompts         `json:"prompts"`
	Metadata     PersonaMetadata `json:"metadata"`
}

// Traits represents persona personality traits
type Traits struct {
	Conscientiousness      float64 `json:"conscientiousness"`
	Neuroticism           float64 `json:"neuroticism"`
	Efficiency            float64 `json:"efficiency,omitempty"`
	Precision             float64 `json:"precision,omitempty"`
	Coordination          float64 `json:"coordination,omitempty"`
	GPUAwareness         float64 `json:"gpu_awareness,omitempty"`
	Flexibility          float64 `json:"flexibility,omitempty"`
	Organization         float64 `json:"organization,omitempty"`
	ArchitecturalThinking float64 `json:"architectural_thinking,omitempty"`
	Proactivity          float64 `json:"proactivity,omitempty"`
}

// Style represents communication style
type Style struct {
	Verbosity               string  `json:"verbosity"`
	Tone                   string  `json:"tone"`
	PronounUsage           string  `json:"pronoun_usage"`
	PreferBulletPoints     bool    `json:"prefer_bullet_points"`
	IncludeSummaries       bool    `json:"include_summaries"`
	ExplainReasoning       bool    `json:"explain_reasoning"`
	AskClarifyingQuestions  bool    `json:"ask_clarifying_questions"`
	UseEmoji               bool    `json:"use_emoji"`
	EmojiFrequency         float64 `json:"emoji_frequency"`
}

// Prompts represents persona prompt sections
type Prompts struct {
	Identity       string `json:"identity"`
	CoreDirectives string `json:"core_directives"`
	StyleGuide     string `json:"style_guide"`
	Constraints    string `json:"constraints"`
}

// PersonaMetadata represents persona metadata
type PersonaMetadata struct {
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`
	Author    string   `json:"author"`
	BestFor   []string `json:"best_for"`
}

// PersonasListResponse represents the personas list response
type PersonasListResponse struct {
	Personas   map[string]PersonaSummary `json:"personas"`
	Total       int                       `json:"total"`
	Version     string                    `json:"version"`
	Categories  map[string]CategoryInfo    `json:"categories"`
	Roles       map[string]RoleInfo        `json:"roles"`
}

// CategoryInfo represents a persona category
type CategoryInfo struct {
	Description string   `json:"description"`
	Personas   []string `json:"personas"`
}

// RoleInfo represents a persona role
type RoleInfo struct {
	Description string   `json:"description"`
	Personas   []string `json:"personas"`
}

// PersonaResponse wraps a persona detail response
type PersonaResponse struct {
	Persona PersonaDetail `json:"persona"`
}

// ListPersonas retrieves the list of all personas from CortexBrain
func (c *Client) ListPersonas() (*PersonasListResponse, error) {
	resp, err := c.doRequest(http.MethodGet, "/api/personas", nil)
	if err != nil {
		return nil, fmt.Errorf("list personas failed: %w", err)
	}
	defer resp.Body.Close()

	var listResp PersonasListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, fmt.Errorf("failed to decode personas list: %w", err)
	}

	return &listResp, nil
}

// GetPersona retrieves a specific persona by ID from CortexBrain
func (c *Client) GetPersona(personaID string) (*PersonaDetail, error) {
	resp, err := c.doRequest(http.MethodGet, "/api/personas/"+personaID, nil)
	if err != nil {
		return nil, fmt.Errorf("get persona failed: %w", err)
	}
	defer resp.Body.Close()

	var personaResp PersonaResponse
	if err := json.NewDecoder(resp.Body).Decode(&personaResp); err != nil {
		return nil, fmt.Errorf("failed to decode persona: %w", err)
	}

	return &personaResp.Persona, nil
}

// GetPersonasByCategory retrieves personas filtered by category
func (c *Client) GetPersonasByCategory(category string) (*PersonasListResponse, error) {
	resp, err := c.doRequest(http.MethodGet, "/api/personas?category="+category, nil)
	if err != nil {
		return nil, fmt.Errorf("get personas by category failed: %w", err)
	}
	defer resp.Body.Close()

	var listResp PersonasListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, fmt.Errorf("failed to decode personas by category: %w", err)
	}

	return &listResp, nil
}

// GetPersonasCategories retrieves all persona categories
func (c *Client) GetPersonasCategories() (map[string]CategoryInfo, error) {
	resp, err := c.doRequest(http.MethodGet, "/api/personas/categories", nil)
	if err != nil {
		return nil, fmt.Errorf("get personas categories failed: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Categories map[string]CategoryInfo `json:"categories"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode categories: %w", err)
	}

	return result.Categories, nil
}

// GetPersonasRoles retrieves all persona roles
func (c *Client) GetPersonasRoles() (map[string]RoleInfo, error) {
	resp, err := c.doRequest(http.MethodGet, "/api/personas/roles", nil)
	if err != nil {
		return nil, fmt.Errorf("get personas roles failed: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Roles map[string]RoleInfo `json:"roles"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode roles: %w", err)
	}

	return result.Roles, nil
}

// InferWithPersona sends an inference request with persona injection
func (c *Client) InferWithPersona(req *InferRequest, personaID string) (*InferResponse, error) {
	// Load persona
	persona, err := c.GetPersona(personaID)
	if err != nil {
		return nil, fmt.Errorf("failed to load persona %s: %w", personaID, err)
	}

	// Prepend persona identity to prompt
	if persona.Prompts.Identity != "" {
		req.Prompt = persona.Prompts.Identity + "\n\n" + req.Prompt
	}

	// Send inference with enhanced prompt
	return c.Infer(req)
}

// GetPersonaIdentity returns just the identity prompt for a persona
func (c *Client) GetPersonaIdentity(personaID string) (string, error) {
	persona, err := c.GetPersona(personaID)
	if err != nil {
		return "", fmt.Errorf("failed to load persona %s: %w", personaID, err)
	}

	return persona.Prompts.Identity, nil
}

// GetPersonaSystemPrompt returns a complete system prompt from a persona
func (c *Client) GetPersonaSystemPrompt(personaID string) (string, error) {
	persona, err := c.GetPersona(personaID)
	if err != nil {
		return "", fmt.Errorf("failed to load persona %s: %w", personaID, err)
	}

	// Build complete system prompt
	systemPrompt := ""
	if persona.Prompts.Identity != "" {
		systemPrompt += persona.Prompts.Identity + "\n\n"
	}
	if persona.Prompts.CoreDirectives != "" {
		systemPrompt += persona.Prompts.CoreDirectives + "\n\n"
	}
	if persona.Prompts.StyleGuide != "" {
		systemPrompt += persona.Prompts.StyleGuide + "\n\n"
	}
	if persona.Prompts.Constraints != "" {
		systemPrompt += persona.Prompts.Constraints
	}

	return systemPrompt, nil
}
