package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/normanking/cortex/internal/logging"
)

// SkillStore interface for skill memory operations.
// This matches the existing MemoryStoreInterface patterns.
type SkillStore interface {
	// SearchSkills finds similar skills based on intent.
	SearchSkills(ctx context.Context, userID, query string, limit int) ([]Skill, error)

	// StoreSkill saves a successful execution as a skill.
	StoreSkill(ctx context.Context, userID, intent, tool string, params map[string]string, success bool) error
}

// Router decides which brain to use for a given request.
type Router struct {
	localBrain    BrainInterface
	frontierBrain BrainInterface
	skillStore    SkillStore
	log           *logging.Logger

	// Config
	skillConfidenceThreshold float64 // Min confidence to use skill (default: 0.8)
	complexityThreshold      int     // Query length that suggests complexity
	preferLocal              bool    // Prefer local brain when possible
}

// RouterConfig holds configuration for the Router.
type RouterConfig struct {
	SkillConfidenceThreshold float64
	ComplexityThreshold      int
	PreferLocal              bool
}

// DefaultRouterConfig returns sensible defaults.
func DefaultRouterConfig() RouterConfig {
	return RouterConfig{
		SkillConfidenceThreshold: 0.8,
		ComplexityThreshold:      500, // Queries > 500 chars likely complex
		PreferLocal:              true,
	}
}

// NewRouter creates a new Router instance.
func NewRouter(local, frontier BrainInterface, skills SkillStore, cfg RouterConfig) *Router {
	return &Router{
		localBrain:               local,
		frontierBrain:            frontier,
		skillStore:               skills,
		log:                      logging.Global(),
		skillConfidenceThreshold: cfg.SkillConfidenceThreshold,
		complexityThreshold:      cfg.ComplexityThreshold,
		preferLocal:              cfg.PreferLocal,
	}
}

// RouteDecision explains why a particular brain was chosen.
type RouteDecision struct {
	Brain       string  `json:"brain"`       // "local" or "frontier"
	Reason      string  `json:"reason"`      // Why this brain was chosen
	MatchedSkill *Skill `json:"matched_skill,omitempty"` // If skill was matched
	Confidence  float64 `json:"confidence"`  // Routing confidence
}

// Route determines which brain should handle the request.
func (r *Router) Route(ctx context.Context, userID string, input *BrainInput) (*RouteDecision, BrainInterface) {
	decision := &RouteDecision{
		Brain:      "local",
		Confidence: 0.5,
	}

	// 1. Check if frontier brain is available
	if r.frontierBrain == nil || !r.frontierBrain.Available() {
		decision.Reason = "frontier unavailable, using local"
		r.log.Info("[Router] %s", decision.Reason)
		return decision, r.localBrain
	}

	// 2. Check skill memory for similar past successes
	if r.skillStore != nil && userID != "" {
		skills, err := r.skillStore.SearchSkills(ctx, userID, input.Query, 3)
		if err == nil && len(skills) > 0 {
			bestSkill := skills[0]

			// Check if skill has high enough success rate
			if bestSkill.SuccessRate >= r.skillConfidenceThreshold {
				decision.Brain = "local"
				decision.Reason = fmt.Sprintf("matched skill '%s' (%.0f%% success)",
					truncate(bestSkill.Intent, 30), bestSkill.SuccessRate*100)
				decision.MatchedSkill = &bestSkill
				decision.Confidence = bestSkill.SuccessRate
				r.log.Info("[Router] %s", decision.Reason)
				return decision, r.localBrain
			}
		}
	}

	// 3. Classify query complexity
	complexity := r.classifyComplexity(input.Query)

	switch complexity {
	case "trivial", "simple":
		decision.Brain = "local"
		decision.Reason = fmt.Sprintf("simple query (complexity: %s)", complexity)
		decision.Confidence = 0.8
		r.log.Info("[Router] %s", decision.Reason)
		return decision, r.localBrain

	case "moderate":
		if r.preferLocal {
			decision.Brain = "local"
			decision.Reason = "moderate complexity, trying local first"
			decision.Confidence = 0.6
		} else {
			decision.Brain = "frontier"
			decision.Reason = "moderate complexity, using frontier for quality"
			decision.Confidence = 0.7
		}
		r.log.Info("[Router] %s", decision.Reason)
		if decision.Brain == "local" {
			return decision, r.localBrain
		}
		return decision, r.frontierBrain

	case "complex", "novel":
		decision.Brain = "frontier"
		decision.Reason = fmt.Sprintf("complex query (complexity: %s)", complexity)
		decision.Confidence = 0.9
		r.log.Info("[Router] %s", decision.Reason)
		return decision, r.frontierBrain
	}

	// Default to local if prefer local, otherwise frontier
	if r.preferLocal {
		decision.Reason = "default: prefer local"
		return decision, r.localBrain
	}
	decision.Brain = "frontier"
	decision.Reason = "default: using frontier"
	return decision, r.frontierBrain
}

// Process routes the request to the appropriate brain and handles the result.
func (r *Router) Process(ctx context.Context, userID string, input *BrainInput) (*BrainResult, *RouteDecision, error) {
	// Route to appropriate brain
	decision, brain := r.Route(ctx, userID, input)

	// Process with selected brain
	result, err := brain.Process(ctx, input)
	if err != nil {
		// If local failed and frontier is available, try frontier
		if decision.Brain == "local" && r.frontierBrain != nil && r.frontierBrain.Available() {
			r.log.Info("[Router] Local brain failed, falling back to frontier: %v", err)
			decision.Brain = "frontier"
			decision.Reason = "fallback after local failure"
			result, err = r.frontierBrain.Process(ctx, input)
		}
	}

	// If successful and used frontier, store as skill for future
	if err == nil && result.Success && decision.Brain == "frontier" {
		r.captureSkill(ctx, userID, input, result)
	}

	return result, decision, err
}

// captureSkill stores a successful frontier execution as a skill.
func (r *Router) captureSkill(ctx context.Context, userID string, input *BrainInput, result *BrainResult) {
	if r.skillStore == nil || userID == "" {
		return
	}

	// Extract tool info from result if available
	tool := "reasoning" // Default tool type
	params := make(map[string]string)

	if len(result.ToolCalls) > 0 {
		tool = result.ToolCalls[0].Name
		for k, v := range result.ToolCalls[0].Params {
			if s, ok := v.(string); ok {
				params[k] = s
			}
		}
	}

	// Store the skill
	err := r.skillStore.StoreSkill(ctx, userID, input.Query, tool, params, true)
	if err != nil {
		r.log.Debug("[Router] Failed to store skill: %v", err)
	} else {
		r.log.Info("[Router] Skill captured: '%s' -> %s (source: frontier)",
			truncate(input.Query, 40), tool)
	}
}

// classifyComplexity determines the complexity level of a query.
func (r *Router) classifyComplexity(query string) string {
	queryLower := strings.ToLower(query)
	queryLen := len(query)

	// Trivial: Very short, simple commands
	if queryLen < 50 {
		trivialPatterns := []string{
			"hello", "hi", "help", "what time", "date",
			"pwd", "ls", "list", "show",
		}
		for _, p := range trivialPatterns {
			if strings.Contains(queryLower, p) {
				return "trivial"
			}
		}
	}

	// Complex: Long queries, PRDs, multi-step requests
	if queryLen > r.complexityThreshold {
		return "complex"
	}

	// Complex: Keywords indicating multi-step work
	complexKeywords := []string{
		"create an application", "build an app", "implement",
		"prd", "requirements", "architecture",
		"refactor", "redesign", "migrate",
		"step by step", "multi-step", "phases",
	}
	for _, kw := range complexKeywords {
		if strings.Contains(queryLower, kw) {
			return "complex"
		}
	}

	// Novel: Questions about cutting-edge topics
	novelKeywords := []string{
		"latest", "newest", "2026", "recent",
		"cutting edge", "state of the art",
	}
	for _, kw := range novelKeywords {
		if strings.Contains(queryLower, kw) {
			return "novel"
		}
	}

	// Simple: Single actions, clear requests
	simpleKeywords := []string{
		"create a file", "write a function", "fix this",
		"explain", "what is", "how do",
	}
	for _, kw := range simpleKeywords {
		if strings.Contains(queryLower, kw) {
			return "simple"
		}
	}

	// Default to moderate
	return "moderate"
}

// truncate shortens a string to max length with ellipsis.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

// Stats returns routing statistics.
type RouterStats struct {
	LocalCalls    int64         `json:"local_calls"`
	FrontierCalls int64         `json:"frontier_calls"`
	SkillMatches  int64         `json:"skill_matches"`
	Fallbacks     int64         `json:"fallbacks"`
	AvgLatency    time.Duration `json:"avg_latency"`
}
