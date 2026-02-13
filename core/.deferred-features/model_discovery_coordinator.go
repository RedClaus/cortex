// Package orchestrator provides the model discovery coordinator for CR-025.
// This coordinator handles automatic model discovery during idle periods,
// checking for newer/better models and recommending upgrades.
package orchestrator

import (
	"context"
	"fmt"
	"time"

	"github.com/normanking/cortex/internal/bus"
	"github.com/normanking/cortex/internal/logging"
	"github.com/normanking/cortex/internal/models"
)

// ModelDiscoveryCoordinator defines the interface for model discovery operations.
// CR-025: Automatic Model Discovery System
type ModelDiscoveryCoordinator interface {
	// RunIfNeeded runs discovery if enough time has passed.
	RunIfNeeded(ctx context.Context) (*models.DiscoveryResult, error)

	// ForceRun forces a discovery run regardless of timing.
	ForceRun(ctx context.Context) (*models.DiscoveryResult, error)

	// GetRecommendations returns current model recommendations.
	GetRecommendations() []models.ModelRecommendation

	// GetInstalledModels returns currently installed models.
	GetInstalledModels() []models.ModelInfo

	// SetRecommendationCallback sets the callback for user prompting.
	SetRecommendationCallback(cb models.RecommendationCallback)

	// Enabled returns whether model discovery is enabled.
	Enabled() bool

	// GetLastRun returns when discovery was last run.
	GetLastRun() time.Time

	// GetStats returns model discovery statistics.
	GetStats() ModelDiscoveryStats
}

// ModelDiscoveryStats holds statistics about model discovery.
type ModelDiscoveryStats struct {
	LastRun             time.Time
	InstalledCount      int
	AvailableCount      int
	RecommendationCount int
	SystemRAMGB         float64
	MaxModelSizeGB      float64
}

// ModelDiscoveryConfig holds configuration for the model discovery coordinator.
type ModelDiscoveryConfig struct {
	// Enabled controls whether model discovery is active.
	Enabled bool

	// CheckInterval is how often to check for new models (default: 24h).
	CheckInterval time.Duration

	// Config holds discovery configuration.
	Config *models.DiscoveryConfig

	// EventBus for publishing discovery events.
	EventBus *bus.EventBus
}

// modelDiscoveryCoordinatorImpl is the concrete implementation.
type modelDiscoveryCoordinatorImpl struct {
	coordinator *models.DiscoveryCoordinator
	eventBus    *bus.EventBus
	enabled     bool
	lastResult  *models.DiscoveryResult
	log         *logging.Logger
}

// NewModelDiscoveryCoordinator creates a new model discovery coordinator.
func NewModelDiscoveryCoordinator(cfg *ModelDiscoveryConfig) ModelDiscoveryCoordinator {
	log := logging.Global()

	if cfg == nil || !cfg.Enabled {
		log.Info("[ModelDiscovery] Model discovery coordinator disabled")
		return &modelDiscoveryCoordinatorImpl{enabled: false, log: log}
	}

	// Use default config if not provided
	config := cfg.Config
	if config == nil {
		config = models.DefaultDiscoveryConfig()
	}

	// Create the underlying coordinator
	coordinator := models.NewDiscoveryCoordinator(config)

	log.Info("[ModelDiscovery] Model discovery coordinator initialized (check_interval=%v)",
		cfg.CheckInterval)

	return &modelDiscoveryCoordinatorImpl{
		coordinator: coordinator,
		eventBus:    cfg.EventBus,
		enabled:     true,
		log:         log,
	}
}

// Enabled returns whether model discovery is enabled.
func (mdc *modelDiscoveryCoordinatorImpl) Enabled() bool {
	return mdc.enabled && mdc.coordinator != nil
}

// RunIfNeeded runs discovery if enough time has passed.
func (mdc *modelDiscoveryCoordinatorImpl) RunIfNeeded(ctx context.Context) (*models.DiscoveryResult, error) {
	if !mdc.enabled || mdc.coordinator == nil {
		return nil, nil
	}

	result, err := mdc.coordinator.RunIfNeeded(ctx)
	if err != nil {
		mdc.publishEvent("discovery_failed", map[string]any{
			"error": err.Error(),
		})
		return nil, err
	}

	if result != nil {
		mdc.lastResult = result
		mdc.publishEvent("discovery_completed", map[string]any{
			"installed_count":      len(result.InstalledModels),
			"available_count":      len(result.AvailableModels),
			"recommendation_count": len(result.Recommendations),
		})
	}

	return result, nil
}

// ForceRun forces a discovery run regardless of timing.
func (mdc *modelDiscoveryCoordinatorImpl) ForceRun(ctx context.Context) (*models.DiscoveryResult, error) {
	if !mdc.enabled || mdc.coordinator == nil {
		return nil, fmt.Errorf("model discovery coordinator not enabled")
	}

	mdc.log.Info("[ModelDiscovery] Starting forced model discovery...")

	mdc.publishEvent("discovery_started", map[string]any{
		"forced": true,
	})

	result, err := mdc.coordinator.ForceRun(ctx)
	if err != nil {
		mdc.log.Warn("[ModelDiscovery] Discovery failed: %v", err)
		mdc.publishEvent("discovery_failed", map[string]any{
			"error": err.Error(),
		})
		return nil, err
	}

	mdc.lastResult = result

	mdc.log.Info("[ModelDiscovery] Discovery complete: %d installed, %d available, %d recommendations",
		len(result.InstalledModels), len(result.AvailableModels), len(result.Recommendations))

	mdc.publishEvent("discovery_completed", map[string]any{
		"installed_count":      len(result.InstalledModels),
		"available_count":      len(result.AvailableModels),
		"recommendation_count": len(result.Recommendations),
	})

	return result, nil
}

// GetRecommendations returns current model recommendations.
func (mdc *modelDiscoveryCoordinatorImpl) GetRecommendations() []models.ModelRecommendation {
	if !mdc.enabled || mdc.coordinator == nil {
		return nil
	}
	return mdc.coordinator.GetRecommendations()
}

// GetInstalledModels returns currently installed models.
func (mdc *modelDiscoveryCoordinatorImpl) GetInstalledModels() []models.ModelInfo {
	if !mdc.enabled || mdc.coordinator == nil {
		return nil
	}
	return mdc.coordinator.GetInstalledModels()
}

// SetRecommendationCallback sets the callback for user prompting.
func (mdc *modelDiscoveryCoordinatorImpl) SetRecommendationCallback(cb models.RecommendationCallback) {
	if !mdc.enabled || mdc.coordinator == nil {
		return
	}
	mdc.coordinator.SetRecommendationCallback(cb)
}

// GetLastRun returns when discovery was last run.
func (mdc *modelDiscoveryCoordinatorImpl) GetLastRun() time.Time {
	if mdc.lastResult != nil {
		return mdc.lastResult.DiscoveryTime
	}
	return time.Time{}
}

// GetStats returns model discovery statistics.
func (mdc *modelDiscoveryCoordinatorImpl) GetStats() ModelDiscoveryStats {
	if !mdc.enabled || mdc.lastResult == nil {
		return ModelDiscoveryStats{}
	}

	return ModelDiscoveryStats{
		LastRun:             mdc.lastResult.DiscoveryTime,
		InstalledCount:      len(mdc.lastResult.InstalledModels),
		AvailableCount:      len(mdc.lastResult.AvailableModels),
		RecommendationCount: len(mdc.lastResult.Recommendations),
		SystemRAMGB:         mdc.lastResult.SystemRAMGB,
		MaxModelSizeGB:      mdc.lastResult.MaxModelSizeGB,
	}
}

func (mdc *modelDiscoveryCoordinatorImpl) publishEvent(eventType string, data map[string]any) {
	if mdc.eventBus == nil {
		return
	}
	// Use DMN event since model discovery is an idle/background task
	mdc.eventBus.Publish(bus.NewDMNEvent("model_discovery."+eventType, data))
}

// ============================================================================
// ORCHESTRATOR INTEGRATION
// ============================================================================

// WithModelDiscovery sets the model discovery coordinator.
// CR-025: Automatic Model Discovery System
func WithModelDiscovery(mdc ModelDiscoveryCoordinator) Option {
	return func(o *Orchestrator) {
		o.modelDiscovery = mdc
	}
}

// ModelDiscovery returns the model discovery coordinator.
func (o *Orchestrator) ModelDiscovery() ModelDiscoveryCoordinator {
	return o.modelDiscovery
}

// CheckModelDiscovery runs model discovery if conditions are met.
// This should be called during idle periods (similar to sleep checks).
func (o *Orchestrator) CheckModelDiscovery(ctx context.Context) (*models.DiscoveryResult, error) {
	if o.modelDiscovery == nil || !o.modelDiscovery.Enabled() {
		return nil, nil
	}

	return o.modelDiscovery.RunIfNeeded(ctx)
}

// GetModelRecommendations returns current model recommendations.
func (o *Orchestrator) GetModelRecommendations() []models.ModelRecommendation {
	if o.modelDiscovery == nil {
		return nil
	}
	return o.modelDiscovery.GetRecommendations()
}
