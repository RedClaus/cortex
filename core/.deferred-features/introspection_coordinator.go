// Package orchestrator provides the introspection coordinator for CR-018.
// This coordinator handles metacognitive self-awareness, allowing Cortex
// to reason about its own knowledge, identify gaps, and autonomously acquire
// new knowledge when needed.
package orchestrator

import (
	"context"
	"fmt"

	"github.com/normanking/cortex/internal/bus"
	"github.com/normanking/cortex/internal/cognitive/introspection"
	"github.com/normanking/cortex/internal/logging"
	"github.com/normanking/cortex/internal/memory"
)

// IntrospectionCoordinator defines the interface for metacognitive operations.
// CR-018: Metacognitive Self-Awareness
type IntrospectionCoordinator interface {
	// Classify determines if input is an introspection query.
	Classify(ctx context.Context, input string) (*introspection.IntrospectionQuery, error)

	// QueryInventory searches all memory stores for a subject.
	QueryInventory(ctx context.Context, query *introspection.IntrospectionQuery) (*memory.InventoryResult, error)

	// AnalyzeGap determines knowledge gaps and options.
	AnalyzeGap(ctx context.Context, query *introspection.IntrospectionQuery, inventory *memory.InventoryResult) (*introspection.GapAnalysis, error)

	// GenerateResponse creates a natural language response.
	GenerateResponse(ctx context.Context, analysis *introspection.GapAnalysis, inventory *memory.InventoryResult) (string, error)

	// StartAcquisition begins knowledge acquisition.
	StartAcquisition(ctx context.Context, req *introspection.AcquisitionRequest) (*introspection.AcquisitionResult, error)

	// VerifyLearning confirms knowledge was successfully acquired.
	VerifyLearning(ctx context.Context, subject string, result *introspection.AcquisitionResult) (*introspection.LearningOutcome, error)

	// Enabled returns whether introspection is enabled.
	Enabled() bool
}

// IntrospectionCoordinatorConfig holds configuration for the coordinator.
type IntrospectionCoordinatorConfig struct {
	// Classifier detects introspection queries.
	Classifier *introspection.Classifier

	// Inventory searches all memory stores.
	Inventory *memory.KnowledgeInventory

	// Analyzer determines knowledge gaps.
	Analyzer *introspection.GapAnalyzer

	// Responder generates natural language responses.
	Responder *introspection.MetacognitiveResponder

	// Acquisition handles knowledge acquisition.
	Acquisition *introspection.AcquisitionEngine

	// Learning verifies and records learning patterns.
	Learning *introspection.LearningConfirmation

	// EventBus for publishing introspection events.
	EventBus *bus.EventBus

	// Enabled controls whether introspection is active.
	Enabled bool
}

// introspectionCoordinatorImpl is the concrete implementation.
type introspectionCoordinatorImpl struct {
	classifier  *introspection.Classifier
	inventory   *memory.KnowledgeInventory
	analyzer    *introspection.GapAnalyzer
	responder   *introspection.MetacognitiveResponder
	acquisition *introspection.AcquisitionEngine
	learning    *introspection.LearningConfirmation
	eventBus    *bus.EventBus
	enabled     bool
}

// NewIntrospectionCoordinator creates a new introspection coordinator.
func NewIntrospectionCoordinator(cfg *IntrospectionCoordinatorConfig) IntrospectionCoordinator {
	if cfg == nil {
		return &introspectionCoordinatorImpl{enabled: false}
	}

	return &introspectionCoordinatorImpl{
		classifier:  cfg.Classifier,
		inventory:   cfg.Inventory,
		analyzer:    cfg.Analyzer,
		responder:   cfg.Responder,
		acquisition: cfg.Acquisition,
		learning:    cfg.Learning,
		eventBus:    cfg.EventBus,
		enabled:     cfg.Enabled,
	}
}

// Enabled returns whether introspection is enabled.
func (ic *introspectionCoordinatorImpl) Enabled() bool {
	return ic.enabled && ic.classifier != nil
}

// Classify determines if input is an introspection query.
func (ic *introspectionCoordinatorImpl) Classify(ctx context.Context, input string) (*introspection.IntrospectionQuery, error) {
	log := logging.Global()

	if !ic.enabled || ic.classifier == nil {
		return &introspection.IntrospectionQuery{
			Type:          introspection.QueryTypeNotIntrospective,
			OriginalQuery: input,
			Confidence:    1.0,
		}, nil
	}

	query, err := ic.classifier.Classify(ctx, input)
	if err != nil {
		log.Warn("[Introspection] Classification failed: %v", err)
		return &introspection.IntrospectionQuery{
			Type:          introspection.QueryTypeNotIntrospective,
			OriginalQuery: input,
			Confidence:    0.5,
		}, nil
	}

	if query.Type != introspection.QueryTypeNotIntrospective {
		log.Info("[Introspection] Detected %s query for subject: %s", query.Type, query.Subject)

		// Publish event
		ic.publishEvent("introspection_detected", map[string]any{
			"query_type": string(query.Type),
			"subject":    query.Subject,
			"confidence": query.Confidence,
		})
	}

	return query, nil
}

// QueryInventory searches all memory stores for a subject.
func (ic *introspectionCoordinatorImpl) QueryInventory(ctx context.Context, query *introspection.IntrospectionQuery) (*memory.InventoryResult, error) {
	log := logging.Global()

	if ic.inventory == nil {
		return &memory.InventoryResult{
			Subject:      query.Subject,
			TotalMatches: 0,
			ByStore:      make(map[string]memory.StoreResult),
			TopResults:   make([]memory.InventoryItem, 0),
			Confidence:   0,
		}, nil
	}

	log.Debug("[Introspection] Querying inventory for: %s", query.Subject)

	result, err := ic.inventory.Query(ctx, query.Subject, query.SearchTerms)
	if err != nil {
		log.Warn("[Introspection] Inventory query failed: %v", err)
		return nil, fmt.Errorf("inventory query: %w", err)
	}

	log.Info("[Introspection] Inventory found %d matches for '%s' in %v",
		result.TotalMatches, query.Subject, result.QueryDuration)

	// Publish event
	ic.publishEvent("inventory_queried", map[string]any{
		"subject":       query.Subject,
		"total_matches": result.TotalMatches,
		"duration_ms":   result.QueryDuration.Milliseconds(),
	})

	return result, nil
}

// AnalyzeGap determines knowledge gaps and options.
func (ic *introspectionCoordinatorImpl) AnalyzeGap(ctx context.Context, query *introspection.IntrospectionQuery, inventory *memory.InventoryResult) (*introspection.GapAnalysis, error) {
	log := logging.Global()

	if ic.analyzer == nil {
		// Return a minimal analysis if analyzer not configured
		hasKnowledge := inventory != nil && inventory.TotalMatches > 0
		return &introspection.GapAnalysis{
			Subject:              query.Subject,
			HasStoredKnowledge:   hasKnowledge,
			StoredKnowledgeCount: 0,
			LLMCanAnswer:         true,
			LLMConfidence:        0.5,
			GapSeverity:          introspection.GapSeverityModerate,
			AcquisitionOptions:   []introspection.AcquisitionOption{},
			RecommendedAction:    "offer_llm_and_acquisition",
		}, nil
	}

	log.Debug("[Introspection] Analyzing gap for: %s", query.Subject)

	analysis, err := ic.analyzer.Analyze(ctx, query, inventory)
	if err != nil {
		log.Warn("[Introspection] Gap analysis failed: %v", err)
		return nil, fmt.Errorf("gap analysis: %w", err)
	}

	log.Info("[Introspection] Gap analysis: severity=%s, has_stored=%v, llm_confidence=%.2f",
		analysis.GapSeverity, analysis.HasStoredKnowledge, analysis.LLMConfidence)

	// Publish event
	ic.publishEvent("gap_analyzed", map[string]any{
		"subject":        query.Subject,
		"gap_severity":   string(analysis.GapSeverity),
		"has_stored":     analysis.HasStoredKnowledge,
		"stored_count":   analysis.StoredKnowledgeCount,
		"llm_can_answer": analysis.LLMCanAnswer,
		"llm_confidence": analysis.LLMConfidence,
		"recommended":    analysis.RecommendedAction,
	})

	return analysis, nil
}

// GenerateResponse creates a natural language response.
func (ic *introspectionCoordinatorImpl) GenerateResponse(ctx context.Context, analysis *introspection.GapAnalysis, inventory *memory.InventoryResult) (string, error) {
	log := logging.Global()

	if ic.responder == nil {
		// Generate a simple fallback response
		if analysis.HasStoredKnowledge {
			return fmt.Sprintf("I found %d items about '%s' in my memory.", analysis.StoredKnowledgeCount, analysis.Subject), nil
		}
		if analysis.LLMCanAnswer {
			return fmt.Sprintf("I don't have '%s' stored in my memory, but I can answer from my general knowledge.", analysis.Subject), nil
		}
		return fmt.Sprintf("I don't have '%s' in my memory and my knowledge is limited on this topic.", analysis.Subject), nil
	}

	// Select appropriate template
	template := ic.responder.SelectTemplate(analysis)
	log.Debug("[Introspection] Selected template: %s", template)

	// Build response context
	responseCtx := &introspection.ResponseContext{
		Subject:            analysis.Subject,
		MatchCount:         analysis.StoredKnowledgeCount,
		LLMCanAnswer:       analysis.LLMCanAnswer,
		LLMConfidence:      analysis.LLMConfidence,
		AcquisitionOptions: analysis.AcquisitionOptions,
	}

	if inventory != nil {
		// Convert memory.InventoryItem to introspection.InventoryItem
		for _, item := range inventory.TopResults {
			responseCtx.TopResults = append(responseCtx.TopResults, introspection.InventoryItem{
				ID:        item.ID,
				Source:    item.Source,
				Content:   item.Content,
				Summary:   item.Summary,
				Relevance: item.Relevance,
				Metadata:  item.Metadata,
			})
		}
		responseCtx.RelatedTopics = inventory.RelatedTopics
	}

	response, err := ic.responder.Generate(template, responseCtx)
	if err != nil {
		log.Warn("[Introspection] Response generation failed: %v", err)
		return "", fmt.Errorf("generate response: %w", err)
	}

	return response, nil
}

// StartAcquisition begins knowledge acquisition.
func (ic *introspectionCoordinatorImpl) StartAcquisition(ctx context.Context, req *introspection.AcquisitionRequest) (*introspection.AcquisitionResult, error) {
	log := logging.Global()

	if ic.acquisition == nil {
		return nil, fmt.Errorf("acquisition engine not configured")
	}

	log.Info("[Introspection] Starting acquisition: type=%s, subject=%s", req.Type, req.Subject)

	// Publish start event
	ic.publishEvent("acquisition_started", map[string]any{
		"type":    string(req.Type),
		"subject": req.Subject,
	})

	result, err := ic.acquisition.Acquire(ctx, req)
	if err != nil {
		log.Error("[Introspection] Acquisition failed: %v", err)

		ic.publishEvent("acquisition_failed", map[string]any{
			"type":    string(req.Type),
			"subject": req.Subject,
			"error":   err.Error(),
		})

		return nil, fmt.Errorf("acquisition: %w", err)
	}

	log.Info("[Introspection] Acquisition complete: %d items ingested in %v",
		result.ItemsIngested, result.Duration)

	// Publish completion event
	ic.publishEvent("acquisition_complete", map[string]any{
		"type":           string(req.Type),
		"subject":        req.Subject,
		"items_ingested": result.ItemsIngested,
		"duration_ms":    result.Duration.Milliseconds(),
		"topic_created":  result.TopicCreated,
	})

	return result, nil
}

// VerifyLearning confirms knowledge was successfully acquired.
func (ic *introspectionCoordinatorImpl) VerifyLearning(ctx context.Context, subject string, result *introspection.AcquisitionResult) (*introspection.LearningOutcome, error) {
	log := logging.Global()

	if ic.learning == nil {
		// Return a simple verification if learning not configured
		return &introspection.LearningOutcome{
			Subject:          subject,
			Verified:         result.Success && result.ItemsIngested > 0,
			ItemsRetrievable: result.ItemsIngested,
		}, nil
	}

	log.Debug("[Introspection] Verifying learning for: %s", subject)

	acquisitionType := "web_search"
	if len(result.Sources) > 0 {
		acquisitionType = result.Sources[0]
	}

	outcome, err := ic.learning.Verify(ctx, subject, acquisitionType, result)
	if err != nil {
		log.Warn("[Introspection] Learning verification failed: %v", err)
		return nil, fmt.Errorf("verify learning: %w", err)
	}

	log.Info("[Introspection] Learning verified: %v (%d/%d items retrievable)",
		outcome.Verified, outcome.ItemsRetrievable, result.ItemsIngested)

	// Publish event
	ic.publishEvent("learning_verified", map[string]any{
		"subject":           subject,
		"verified":          outcome.Verified,
		"items_retrievable": outcome.ItemsRetrievable,
		"test_count":        len(outcome.TestResults),
	})

	return outcome, nil
}

func (ic *introspectionCoordinatorImpl) publishEvent(eventType string, data map[string]any) {
	if ic.eventBus == nil {
		return
	}
	ic.eventBus.Publish(bus.NewIntrospectionEvent(eventType, data))
}

// ============================================================================
// ORCHESTRATOR INTEGRATION
// ============================================================================

// WithIntrospectionCoordinator sets the introspection coordinator.
// CR-018: Metacognitive Self-Awareness
func WithIntrospectionCoordinator(ic IntrospectionCoordinator) Option {
	return func(o *Orchestrator) {
		o.introspection = ic
	}
}

// IntrospectionResult holds the result of introspection processing.
type IntrospectionResult struct {
	Query         *introspection.IntrospectionQuery
	Inventory     *memory.InventoryResult
	GapAnalysis   *introspection.GapAnalysis
	Response      string
	IsHandled     bool
	ShouldAcquire bool
}

// ProcessIntrospection handles an introspection query through the full pipeline.
// This is called from the cognitive stage when an introspection query is detected.
func (o *Orchestrator) ProcessIntrospection(ctx context.Context, input string) (*IntrospectionResult, error) {
	if o.introspection == nil || !o.introspection.Enabled() {
		return &IntrospectionResult{IsHandled: false}, nil
	}

	log := logging.Global()
	result := &IntrospectionResult{}

	// Step 1: Classify
	query, err := o.introspection.Classify(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("classify: %w", err)
	}
	result.Query = query

	// If not introspective, don't handle
	if query.Type == introspection.QueryTypeNotIntrospective {
		return &IntrospectionResult{IsHandled: false}, nil
	}

	log.Info("[Orchestrator] Processing introspection query: type=%s, subject=%s",
		query.Type, query.Subject)

	// Step 2: Query inventory
	inventory, err := o.introspection.QueryInventory(ctx, query)
	if err != nil {
		log.Warn("[Orchestrator] Inventory query failed: %v", err)
		// Continue with empty inventory
		inventory = &memory.InventoryResult{
			Subject:      query.Subject,
			TotalMatches: 0,
			ByStore:      make(map[string]memory.StoreResult),
		}
	}
	result.Inventory = inventory

	// Step 3: Analyze gap
	analysis, err := o.introspection.AnalyzeGap(ctx, query, inventory)
	if err != nil {
		log.Warn("[Orchestrator] Gap analysis failed: %v", err)
		// Continue with default analysis
		analysis = &introspection.GapAnalysis{
			Subject:           query.Subject,
			GapSeverity:       introspection.GapSeverityModerate,
			LLMCanAnswer:      true,
			LLMConfidence:     0.5,
			RecommendedAction: "offer_llm_and_acquisition",
		}
	}
	result.GapAnalysis = analysis

	// Step 4: Generate response
	response, err := o.introspection.GenerateResponse(ctx, analysis, inventory)
	if err != nil {
		log.Warn("[Orchestrator] Response generation failed: %v", err)
		// Generate fallback response
		response = fmt.Sprintf("I found %d items about '%s' in my memory.",
			inventory.TotalMatches, query.Subject)
	}
	result.Response = response
	result.IsHandled = true

	// Determine if we should suggest acquisition
	result.ShouldAcquire = analysis.GapSeverity != introspection.GapSeverityNone

	return result, nil
}
