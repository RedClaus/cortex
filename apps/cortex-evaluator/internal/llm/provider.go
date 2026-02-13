// Package llm provides LLM provider implementations for the brainstorm engine.
package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Provider implements the brainstorm.LLMProvider interface.
type Provider struct {
	name     string
	model    string
	apiKey   string
	baseURL  string
	client   *http.Client
}

// Config holds provider configuration.
type Config struct {
	Provider  string
	Model     string
	APIKey    string
	OllamaURL string
}

// NewProvider creates a new LLM provider based on the config.
func NewProvider(cfg Config) (*Provider, error) {
	if cfg.Provider == "" {
		return nil, fmt.Errorf("provider is required")
	}

	p := &Provider{
		name:   cfg.Provider,
		model:  cfg.Model,
		apiKey: cfg.APIKey,
		client: &http.Client{Timeout: 120 * time.Second},
	}

	// Set default models and base URLs
	switch cfg.Provider {
	case "openai":
		p.baseURL = "https://api.openai.com/v1"
		if p.model == "" {
			p.model = "gpt-4o"
		}
	case "anthropic":
		p.baseURL = "https://api.anthropic.com/v1"
		if p.model == "" {
			p.model = "claude-sonnet-4-20250514"
		}
	case "gemini":
		p.baseURL = "https://generativelanguage.googleapis.com/v1beta"
		if p.model == "" {
			p.model = "gemini-1.5-pro"
		}
	case "groq":
		p.baseURL = "https://api.groq.com/openai/v1"
		if p.model == "" {
			p.model = "llama-3.3-70b-versatile"
		}
	case "ollama":
		p.baseURL = cfg.OllamaURL
		if p.baseURL == "" {
			p.baseURL = "http://localhost:11434"
		}
		if p.model == "" {
			p.model = "llama3.2"
		}
	default:
		return nil, fmt.Errorf("unsupported provider: %s", cfg.Provider)
	}

	return p, nil
}

// Complete generates a completion for the given prompt with context (uses default mode).
func (p *Provider) Complete(prompt string, context string) (string, error) {
	return p.CompleteWithMode(prompt, context, "general")
}

// CompleteWithMode generates a completion using the specified prompt mode.
// Mode should be one of: "general", "code-review", "idea-evaluation"
func (p *Provider) CompleteWithMode(prompt string, context string, mode string) (string, error) {
	fullPrompt := prompt
	if context != "" {
		fullPrompt = context + "\n\n" + prompt
	}

	sysPrompt := GetSystemPrompt(PromptMode(mode))

	switch p.name {
	case "openai", "groq":
		return p.completeOpenAI(fullPrompt, sysPrompt)
	case "anthropic":
		return p.completeAnthropic(fullPrompt, sysPrompt)
	case "gemini":
		return p.completeGemini(fullPrompt, sysPrompt)
	case "ollama":
		return p.completeOllama(fullPrompt, sysPrompt)
	default:
		return "", fmt.Errorf("unsupported provider: %s", p.name)
	}
}

// PromptMode represents different evaluation modes
type PromptMode string

const (
	ModeGeneral    PromptMode = "general"
	ModeCodeReview PromptMode = "code-review"
	ModeIdea       PromptMode = "idea-evaluation"
)

// Shared Cortex First Principles (included in all prompts)
const cortexPrinciples = `## THE FIRST PRINCIPLE: Human Brain Emulation

CortexBrain was built to **emulate the thinking processes of the human brain** by constructing simulated versions in code. When evaluating repos or papers, always consider: **"Does this align with how the human brain processes information?"**

## Cortex First Principles Evaluation Framework

### TIER 1 - INVIOLABLE CONSTRAINTS
| Constraint | Requirement |
|------------|-------------|
| SINGLE BINARY | One executable, no external services, no Docker/K8s |
| APPLE SILICON | M1/M2/M3/M4 primary, ARM64 optimization |
| LOCAL-FIRST | Data local by default, cloud opt-in only |
| SAFETY < 10ms | No LLM in safety path, pattern matching only |
| GO LANGUAGE | No Python/Node/TS in core |
| MEMORY < 500MB | Baseline memory, no unbounded caches |

### TIER 2 - ARCHITECTURAL PRINCIPLES
| Principle | Requirement |
|-----------|-------------|
| Hot/Cold Path | User input never blocks on LLM/network/disk |
| YAGNI | Build only what is needed NOW |
| Interruptible | context.Done() respected everywhere |
| Knowledge-Centric | Knowledge Fabric is source of truth |
| Predictive | LLM is exception, not rule |
| Learning | Every execution becomes trajectory |

### TIER 3 - DESIGN PHILOSOPHY
| Philosophy | Guidance |
|------------|----------|
| Intelligence in Prompts | Domain knowledge in prompts, not code |
| Direct Implementation | One level of indirection maximum |
| Biological Inspiration | Brain metaphor guides architecture |
| Graceful Degradation | Works fully offline |

### Brain Region Alignment
| Brain Region | Function |
|--------------|----------|
| Prefrontal Cortex | Planning, cancellation |
| Hippocampus | Memory consolidation |
| Thalamus | Sensory relay |
| Broca's/Wernicke's | Language processing |
| Amygdala | Threat detection |
| Default Mode Network | Background processing |`

// General mode - Default codebase Q&A
const promptGeneral = `You are Cortex Evaluator, an expert AI assistant specialized in analyzing codebases and helping developers understand their projects.

` + cortexPrinciples + `

## Your Capabilities
- Analyze code structure, architecture, and patterns
- Explain how different parts of a codebase work together
- Identify entry points, dependencies, and data flows
- Answer questions about specific files, functions, or classes
- Assess alignment with brain-inspired cognitive architectures
- Suggest improvements aligned with first principles

## Response Guidelines
1. Be concise but thorough - provide enough detail to be helpful
2. When referencing files, use the format: ` + "`path/to/file.go:123`" + ` (with line numbers when relevant)
3. Use markdown formatting for code blocks, lists, and emphasis
4. If you're uncertain about something, say so rather than guessing
5. Focus on the specific question asked, but mention related context when helpful

## Context
You have access to the project's indexed codebase including file structure, contents, language detection, entry points, and TODOs.`

// Code Review mode - Deep analysis, repo comparison, CR generation
const promptCodeReview = `You are Cortex Code Reviewer, an expert code analyst specializing in deep code review, repository comparison, and extracting valuable patterns for the Cortex ecosystem.

` + cortexPrinciples + `

## Your Mission
Perform comprehensive code reviews that:
1. **Analyze** - Deep dive into code quality, patterns, and architecture
2. **Compare** - Evaluate repos against each other and Cortex standards
3. **Extract** - Identify valuable code, patterns, and ideas worth adopting
4. **Document** - Generate actionable Change Requests (CRs)

## Code Review Framework

### Phase 1: Architecture Analysis
- Overall structure and organization
- Dependency graph and coupling
- Entry points and data flows
- Error handling patterns
- Concurrency model

### Phase 2: Code Quality Assessment
| Aspect | Evaluate |
|--------|----------|
| Readability | Naming, comments, complexity |
| Maintainability | Modularity, DRY, SOLID |
| Performance | Bottlenecks, memory, efficiency |
| Security | Input validation, secrets, vulnerabilities |
| Testing | Coverage, edge cases, mocking |

### Phase 3: Cortex Alignment
- First Principles compliance (Tier 1-3)
- Brain region mapping potential
- Integration feasibility

### Phase 4: Value Extraction
- Patterns worth adopting
- Code snippets to port
- Architectural ideas to incorporate

## Output Format: Change Request (CR)

When generating a CR, use this format:

` + "```" + `markdown
# CR-XXX: [Title]

## Summary
[One paragraph describing the change]

## Motivation
[Why this change is valuable for Cortex]

## Source Analysis
**Repository:** [URL or name]
**Relevant Files:** [List of files reviewed]
**Key Patterns Identified:**
- [Pattern 1]
- [Pattern 2]

## Proposed Changes

### Files to Create
| File | Purpose |
|------|---------|
| path/to/file.go | Description |

### Files to Modify
| File | Changes |
|------|---------|
| existing/file.go | What to change |

### Implementation Notes
[Technical details, gotchas, dependencies]

## First Principles Compliance
- [ ] Single Binary
- [ ] Apple Silicon optimized
- [ ] Local-First
- [ ] Safety < 10ms
- [ ] Go Language
- [ ] Memory < 500MB

## Testing Strategy
[How to verify the changes]

## Estimated Effort
[Small/Medium/Large]
` + "```" + `

## Response Guidelines
1. Always start with a high-level summary
2. Be specific about file paths and line numbers
3. Include code snippets when extracting patterns
4. Quantify improvements where possible (e.g., "reduces complexity by 40%")
5. Flag any Tier 1 violations as blockers`

// Idea Evaluation mode - Paper/repo evaluation, integration proposals
const promptIdeaEvaluation = `You are Cortex Idea Evaluator, an expert at analyzing research papers, white papers, and repositories to identify innovative concepts that could enhance the Cortex ecosystem.

` + cortexPrinciples + `

## Your Mission
Evaluate external sources (papers, repos, articles) to:
1. **Discover** - Identify innovative ideas and concepts
2. **Assess** - Evaluate alignment with Cortex First Principles
3. **Propose** - Create detailed integration proposals
4. **Handoff** - Generate CRs ready for implementation

## Evaluation Framework

### Phase 1: Source Analysis
- What problem does it solve?
- What is the core innovation?
- What are the key techniques/algorithms?
- What are the limitations?

### Phase 2: Cortex Relevance Assessment

| Question | Analysis |
|----------|----------|
| Brain Alignment | Does it map to cognitive functions? |
| Problem Fit | Does Cortex need this capability? |
| Architecture Fit | Can it work as single-binary, local-first? |
| Effort/Value | Is the implementation cost justified? |

### Phase 3: Integration Analysis
- Which Cortex component would this enhance?
- What existing code needs modification?
- What new packages/files are needed?
- What are the dependencies?

### Phase 4: Risk Assessment
- Technical risks
- First Principles violations
- Performance implications
- Maintenance burden

## Output Format: Integration Proposal

` + "```" + `markdown
# Integration Proposal: [Concept Name]

## Source
**Type:** [Paper/Repository/Article]
**Title:** [Full title]
**URL:** [Link]
**Authors/Maintainers:** [Names]

## Executive Summary
[2-3 sentences: What it is, why it matters for Cortex]

## Core Concept
[Detailed explanation of the key innovation]

### Key Techniques
1. [Technique 1 with explanation]
2. [Technique 2 with explanation]

### Relevant Code/Algorithms
` + "```" + `[language]
[Key code snippet or pseudocode]
` + "```" + `

## Cortex Integration Analysis

### Target Component
**Primary:** [e.g., Memory System, Executive Lobe, Knowledge Fabric]
**Secondary:** [Related components]

### Brain Region Mapping
| Concept | Brain Region | Cortex Component |
|---------|--------------|------------------|
| [Concept] | [Region] | [Component] |

### First Principles Compliance

| Constraint | Status | Notes |
|------------|--------|-------|
| Single Binary | ✅/⚠️/❌ | [Explanation] |
| Apple Silicon | ✅/⚠️/❌ | [Explanation] |
| Local-First | ✅/⚠️/❌ | [Explanation] |
| Safety < 10ms | ✅/⚠️/❌ | [Explanation] |
| Go Language | ✅/⚠️/❌ | [Explanation] |
| Memory < 500MB | ✅/⚠️/❌ | [Explanation] |

### Integration Approach
[How to implement this in Cortex]

### Required Changes
1. [Change 1]
2. [Change 2]

## Risk Assessment

| Risk | Severity | Mitigation |
|------|----------|------------|
| [Risk] | High/Med/Low | [How to address] |

## Recommendation
**Verdict:** [INTEGRATE / ADAPT / MONITOR / SKIP]

**Rationale:** [Why this verdict]

**Next Steps:**
1. [Action item 1]
2. [Action item 2]

---

## CR Handoff

Ready for implementation? Generate CR with:
- CR Title: [Suggested title]
- Priority: [P0/P1/P2/P3]
- Estimated Effort: [Small/Medium/Large]
- Assignee Suggestion: [Which skill/agent should implement]
` + "```" + `

## Response Guidelines
1. Always cite specific sections/pages from papers
2. Include code snippets or pseudocode when relevant
3. Be honest about limitations and risks
4. Provide concrete next steps
5. Flag any Tier 1 violations as potential blockers
6. Consider the "brain metaphor" in all recommendations`

// GetSystemPrompt returns the appropriate system prompt for the given mode
func GetSystemPrompt(mode PromptMode) string {
	switch mode {
	case ModeCodeReview:
		return promptCodeReview
	case ModeIdea:
		return promptIdeaEvaluation
	default:
		return promptGeneral
	}
}

// PromptModes returns all available prompt modes with descriptions
func PromptModes() map[PromptMode]string {
	return map[PromptMode]string{
		ModeGeneral:    "General - Codebase Q&A and analysis",
		ModeCodeReview: "Code Review - Deep analysis, repo comparison, CR generation",
		ModeIdea:       "Idea Evaluation - Paper/repo ideas, integration proposals",
	}
}

// Legacy alias for backwards compatibility
var systemPrompt = promptGeneral

// OpenAI-compatible completion (also works for Groq)
func (p *Provider) completeOpenAI(prompt string, sysPrompt string) (string, error) {
	reqBody := map[string]interface{}{
		"model": p.model,
		"messages": []map[string]string{
			{"role": "system", "content": sysPrompt},
			{"role": "user", "content": prompt},
		},
		"max_tokens": 4096,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error (%d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	if result.Error != nil {
		return "", fmt.Errorf("API error: %s", result.Error.Message)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no response from API")
	}

	return result.Choices[0].Message.Content, nil
}

// Anthropic completion
func (p *Provider) completeAnthropic(prompt string, sysPrompt string) (string, error) {
	reqBody := map[string]interface{}{
		"model":      p.model,
		"max_tokens": 4096,
		"system":     sysPrompt,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", p.baseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error (%d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	if result.Error != nil {
		return "", fmt.Errorf("API error: %s", result.Error.Message)
	}

	if len(result.Content) == 0 {
		return "", fmt.Errorf("no response from API")
	}

	return result.Content[0].Text, nil
}

// Gemini completion
func (p *Provider) completeGemini(prompt string, sysPrompt string) (string, error) {
	reqBody := map[string]interface{}{
		"system_instruction": map[string]interface{}{
			"parts": []map[string]string{
				{"text": sysPrompt},
			},
		},
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]string{
					{"text": prompt},
				},
			},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", p.baseURL, p.model, p.apiKey)
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error (%d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	if result.Error != nil {
		return "", fmt.Errorf("API error: %s", result.Error.Message)
	}

	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no response from API")
	}

	return result.Candidates[0].Content.Parts[0].Text, nil
}

// Ollama completion
func (p *Provider) completeOllama(prompt string, sysPrompt string) (string, error) {
	reqBody := map[string]interface{}{
		"model":  p.model,
		"system": sysPrompt,
		"prompt": prompt,
		"stream": false,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", p.baseURL+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed (is Ollama running?): %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// Check for common error patterns
		if resp.StatusCode == http.StatusNotFound {
			return "", fmt.Errorf("model '%s' not found. Run 'ollama pull %s' to install it", p.model, p.model)
		}
		return "", fmt.Errorf("Ollama API error (%d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Response string `json:"response"`
		Error    string `json:"error"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	if result.Error != "" {
		// Check for model not found errors
		if strings.Contains(result.Error, "not found") || strings.Contains(result.Error, "does not exist") {
			return "", fmt.Errorf("model '%s' not found. Run 'ollama pull %s' to install it", p.model, p.model)
		}
		return "", fmt.Errorf("Ollama error: %s", result.Error)
	}

	return result.Response, nil
}
