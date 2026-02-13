// Package orchestrator provides the central coordination layer for Cortex.
package orchestrator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/normanking/cortex/internal/cognitive"
	cogdecomp "github.com/normanking/cortex/internal/cognitive/decomposer"
	cogdistill "github.com/normanking/cortex/internal/cognitive/distillation"
	cogfeedback "github.com/normanking/cortex/internal/cognitive/feedback"
	cogrouter "github.com/normanking/cortex/internal/cognitive/router"
	cogtemplates "github.com/normanking/cortex/internal/cognitive/templates"
	"github.com/normanking/cortex/internal/logging"
)

// ═══════════════════════════════════════════════════════════════════════════════
// COGNITIVE ARCHITECTURE INTERFACE
// CR-017: Phase 2 - Cognitive Coordinator Extraction
// ═══════════════════════════════════════════════════════════════════════════════

// CognitiveArchitecture defines the interface for the cognitive subsystem.
// It provides semantic routing, template rendering, distillation, and feedback
// capabilities in a decoupled manner.
type CognitiveArchitecture interface {
	// Route performs semantic routing for a user request.
	// Returns a routing result with template match (if found) and model tier recommendation.
	Route(ctx context.Context, input string) (*cognitive.RoutingResult, error)

	// RenderTemplate renders a template with the given variables.
	// Returns the rendered output string.
	RenderTemplate(ctx context.Context, templateID string, vars map[string]any) (string, error)

	// RenderTemplateSimple renders a template body directly with variables.
	// This is a convenience method for pipeline stage use.
	RenderTemplateSimple(templateBody string, vars map[string]any) (string, error)

	// Distill handles a novel request by calling a frontier model and
	// extracting a reusable template from the response.
	Distill(ctx context.Context, input string, taskType cognitive.TaskType) (*cognitive.DistillationResult, error)

	// RecordFeedback records the outcome of a template execution.
	// success=true for successful execution, false for failure.
	RecordFeedback(ctx context.Context, templateID string, userInput, renderedOutput string, success bool, latencyMs int) error

	// Analyze evaluates task complexity without calling LLM.
	// Returns analysis with complexity score and decomposition recommendation.
	Analyze(input string, taskType cognitive.TaskType) *cogdecomp.DecompositionResult

	// Decompose breaks a complex task into executable steps using LLM.
	Decompose(ctx context.Context, input string, taskType cognitive.TaskType) (*cogdecomp.DecompositionResult, error)

	// Enabled returns whether the cognitive architecture is enabled.
	Enabled() bool

	// Stats returns cognitive system statistics.
	Stats() *CognitiveStats
}

// CognitiveStats contains statistics about the cognitive subsystem.
type CognitiveStats struct {
	Enabled           bool   `json:"enabled"`
	RouterAvailable   bool   `json:"router_available"`
	TemplatesIndexed  int    `json:"templates_indexed"`
	EmbedderAvailable bool   `json:"embedder_available"`
	EmbeddingModel    string `json:"embedding_model,omitempty"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// COGNITIVE COORDINATOR IMPLEMENTATION
// ═══════════════════════════════════════════════════════════════════════════════

// CognitiveCoordinator coordinates all cognitive architecture components.
// It encapsulates the semantic router, template engine, distillation engine,
// decomposer, and feedback loop into a single coherent subsystem.
type CognitiveCoordinator struct {
	// Core cognitive components
	router      *cogrouter.Router
	registry    cognitive.Registry
	templateEng *cogtemplates.Engine
	distiller   *cogdistill.Engine
	decomposer  *cogdecomp.Decomposer
	feedback    *cogfeedback.Loop
	promptMgr   *cognitive.PromptManager

	// Configuration
	enabled bool
	log     *logging.Logger

	// State
	mu          sync.RWMutex
	initialized bool
}

// CognitiveConfig configures the CognitiveCoordinator.
type CognitiveConfig struct {
	// Router is the semantic router for template matching.
	Router *cogrouter.Router

	// Registry is the template registry for storage.
	Registry cognitive.Registry

	// TemplateEngine renders templates with variables.
	TemplateEngine *cogtemplates.Engine

	// Distiller extracts templates from frontier model responses.
	Distiller *cogdistill.Engine

	// Decomposer breaks complex tasks into steps.
	Decomposer *cogdecomp.Decomposer

	// Feedback is the learning feedback loop.
	Feedback *cogfeedback.Loop

	// PromptManager provides tier-optimized prompts.
	PromptManager *cognitive.PromptManager

	// Enabled toggles the cognitive architecture.
	Enabled bool
}

// NewCognitiveCoordinator creates a new cognitive coordinator.
func NewCognitiveCoordinator(cfg *CognitiveConfig) *CognitiveCoordinator {
	if cfg == nil {
		cfg = &CognitiveConfig{}
	}

	cc := &CognitiveCoordinator{
		router:      cfg.Router,
		registry:    cfg.Registry,
		templateEng: cfg.TemplateEngine,
		distiller:   cfg.Distiller,
		decomposer:  cfg.Decomposer,
		feedback:    cfg.Feedback,
		promptMgr:   cfg.PromptManager,
		enabled:     cfg.Enabled,
		log:         logging.Global(),
	}

	// Create default template engine if not provided
	if cc.templateEng == nil {
		cc.templateEng = cogtemplates.NewEngine()
	}

	return cc
}

// Verify CognitiveCoordinator implements CognitiveArchitecture at compile time.
var _ CognitiveArchitecture = (*CognitiveCoordinator)(nil)

// ═══════════════════════════════════════════════════════════════════════════════
// ROUTING
// ═══════════════════════════════════════════════════════════════════════════════

// Route performs semantic routing for a user request.
func (c *CognitiveCoordinator) Route(ctx context.Context, input string) (*cognitive.RoutingResult, error) {
	// Check enabled status and router under read lock
	c.mu.RLock()
	enabled := c.enabled
	router := c.router
	c.mu.RUnlock()

	if !enabled || router == nil {
		return &cognitive.RoutingResult{
			Decision:        cognitive.RouteNovel,
			RecommendedTier: cognitive.TierFrontier,
		}, nil
	}

	c.log.Debug("[CognitiveCoordinator] Routing request")
	return router.Route(ctx, input)
}

// ═══════════════════════════════════════════════════════════════════════════════
// TEMPLATE RENDERING
// ═══════════════════════════════════════════════════════════════════════════════

// RenderTemplate renders a template by ID with the given variables.
func (c *CognitiveCoordinator) RenderTemplate(ctx context.Context, templateID string, vars map[string]any) (string, error) {
	if c.templateEng == nil {
		return "", fmt.Errorf("template engine not configured")
	}

	if c.registry == nil {
		return "", fmt.Errorf("registry not configured")
	}

	// Get the template from registry
	template, err := c.registry.Get(ctx, templateID)
	if err != nil {
		return "", fmt.Errorf("get template: %w", err)
	}

	// Render the template
	result, err := c.templateEng.Render(ctx, template, vars)
	if err != nil {
		return "", fmt.Errorf("render template: %w", err)
	}

	return result.Output, nil
}

// RenderTemplateSimple renders a template body directly with variables.
func (c *CognitiveCoordinator) RenderTemplateSimple(templateBody string, vars map[string]any) (string, error) {
	if c.templateEng == nil {
		return "", fmt.Errorf("template engine not configured")
	}

	return c.templateEng.RenderSimple(templateBody, vars)
}

// ═══════════════════════════════════════════════════════════════════════════════
// DISTILLATION
// ═══════════════════════════════════════════════════════════════════════════════

// Distill handles a novel request by calling a frontier model and
// extracting a reusable template from the response.
func (c *CognitiveCoordinator) Distill(ctx context.Context, input string, taskType cognitive.TaskType) (*cognitive.DistillationResult, error) {
	if !c.enabled || c.distiller == nil {
		return &cognitive.DistillationResult{
			Solution: "", // Let caller handle without distillation
		}, nil
	}

	c.log.Debug("[CognitiveCoordinator] Starting distillation for task type: %s", taskType)
	return c.distiller.SolveAndTeach(ctx, input, taskType)
}

// ═══════════════════════════════════════════════════════════════════════════════
// FEEDBACK
// ═══════════════════════════════════════════════════════════════════════════════

// RecordFeedback records the outcome of a template execution.
func (c *CognitiveCoordinator) RecordFeedback(ctx context.Context, templateID string, userInput, renderedOutput string, success bool, latencyMs int) error {
	if !c.enabled || c.feedback == nil {
		return nil // Silently skip if not enabled
	}

	if success {
		return c.feedback.RecordSuccess(ctx, templateID, userInput, renderedOutput, latencyMs)
	}
	return c.feedback.RecordFailure(ctx, templateID, userInput, renderedOutput, latencyMs)
}

// ═══════════════════════════════════════════════════════════════════════════════
// DECOMPOSITION
// ═══════════════════════════════════════════════════════════════════════════════

// Analyze evaluates task complexity without calling LLM.
func (c *CognitiveCoordinator) Analyze(input string, taskType cognitive.TaskType) *cogdecomp.DecompositionResult {
	if !c.enabled || c.decomposer == nil {
		// Return simple result indicating no decomposition needed
		return &cogdecomp.DecompositionResult{
			OriginalInput: input,
			Complexity: &cogdecomp.ComplexityResult{
				Score:       0,
				NeedsDecomp: false,
			},
		}
	}

	return c.decomposer.Analyze(input, taskType)
}

// Decompose breaks a complex task into executable steps using LLM.
func (c *CognitiveCoordinator) Decompose(ctx context.Context, input string, taskType cognitive.TaskType) (*cogdecomp.DecompositionResult, error) {
	if !c.enabled || c.decomposer == nil {
		return &cogdecomp.DecompositionResult{
			OriginalInput: input,
			Complexity: &cogdecomp.ComplexityResult{
				Score:       0,
				NeedsDecomp: false,
			},
		}, nil
	}

	return c.decomposer.Decompose(ctx, input, taskType)
}

// ═══════════════════════════════════════════════════════════════════════════════
// CONFIGURATION & STATUS
// ═══════════════════════════════════════════════════════════════════════════════

// Enabled returns whether the cognitive architecture is enabled.
func (c *CognitiveCoordinator) Enabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.enabled
}

// SetEnabled enables or disables the cognitive architecture.
func (c *CognitiveCoordinator) SetEnabled(enabled bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.enabled = enabled
	c.log.Info("[CognitiveCoordinator] Enabled set to: %v", enabled)
}

// Stats returns cognitive system statistics.
func (c *CognitiveCoordinator) Stats() *CognitiveStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := &CognitiveStats{
		Enabled: c.enabled,
	}

	if c.router != nil {
		routerStats := c.router.Stats()
		stats.RouterAvailable = routerStats.Initialized
		stats.TemplatesIndexed = routerStats.IndexSize
		stats.EmbedderAvailable = routerStats.EmbedderAvailable
		stats.EmbeddingModel = routerStats.EmbeddingModel
	}

	return stats
}

// ═══════════════════════════════════════════════════════════════════════════════
// INITIALIZATION
// ═══════════════════════════════════════════════════════════════════════════════

// Initialize initializes the cognitive coordinator and its components.
func (c *CognitiveCoordinator) Initialize(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.initialized {
		return nil
	}

	c.log.Info("[CognitiveCoordinator] Initializing...")
	start := time.Now()

	// Initialize router if present
	if c.router != nil {
		if err := c.router.Initialize(ctx); err != nil {
			c.log.Warn("[CognitiveCoordinator] Router initialization failed: %v", err)
			// Continue anyway - router can work in fallback mode
		}
	}

	// Start feedback loop if present
	if c.feedback != nil {
		if err := c.feedback.Start(ctx); err != nil {
			c.log.Warn("[CognitiveCoordinator] Feedback loop start failed: %v", err)
		}
	}

	c.initialized = true
	c.log.Info("[CognitiveCoordinator] Initialized in %v", time.Since(start))

	return nil
}

// Shutdown gracefully shuts down the cognitive coordinator.
func (c *CognitiveCoordinator) Shutdown() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.initialized {
		return
	}

	c.log.Info("[CognitiveCoordinator] Shutting down...")

	// Stop feedback loop if present
	if c.feedback != nil {
		c.feedback.Stop()
	}

	c.initialized = false
	c.log.Info("[CognitiveCoordinator] Shutdown complete")
}

// ═══════════════════════════════════════════════════════════════════════════════
// COMPONENT ACCESS (for advanced use cases)
// ═══════════════════════════════════════════════════════════════════════════════

// Router returns the underlying semantic router (may be nil).
func (c *CognitiveCoordinator) Router() *cogrouter.Router {
	return c.router
}

// TemplateEngine returns the underlying template engine.
func (c *CognitiveCoordinator) TemplateEngine() *cogtemplates.Engine {
	return c.templateEng
}

// Registry returns the underlying template registry (may be nil).
func (c *CognitiveCoordinator) Registry() cognitive.Registry {
	return c.registry
}

// PromptManager returns the prompt manager (may be nil).
func (c *CognitiveCoordinator) PromptManager() *cognitive.PromptManager {
	return c.promptMgr
}
