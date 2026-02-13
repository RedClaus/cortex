// Package models provides stub types for model discovery.
// This is a minimal stub to allow compilation. Full implementation TBD.
package models

import "time"

// ModelInfo represents information about a model.
type ModelInfo struct {
	Name        string
	Provider    string
	Size        int64
	Format      string
	Capabilities []string
	InstalledAt time.Time
}

// ModelRecommendation represents a model recommendation.
type ModelRecommendation struct {
	CurrentModel    string
	RecommendedModel string
	Reason          string
	Priority        int
}

// DiscoveryResult represents the result of model discovery.
type DiscoveryResult struct {
	Timestamp        time.Time
	InstalledModels  []ModelInfo
	AvailableModels  []ModelInfo
	Recommendations  []ModelRecommendation
	SystemInfo       SystemInfo
}

// SystemInfo represents system information for model selection.
type SystemInfo struct {
	RAMGB      float64
	CPUCores   int
	HasGPU     bool
	GPUMemoryGB float64
}

// RecommendationCallback is called when recommendations are available.
type RecommendationCallback func(recommendations []ModelRecommendation)
