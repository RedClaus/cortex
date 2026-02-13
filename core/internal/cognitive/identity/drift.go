package identity

import (
	"math"
	"sync"
	"time"
)

// DriftDetector performs deterministic drift detection using embedding similarity.
// All operations complete in <10ms as required by the architecture.
type DriftDetector struct {
	config  *Config
	creed   *CreedManager
	history *driftHistory

	mu sync.RWMutex
}

// driftHistory tracks recent responses and drift events.
type driftHistory struct {
	recentResponses   []responseRecord
	recentEvents      []DriftEvent
	maxResponses      int
	maxEvents         int
	responseSinceCheck int
	totalChecks       int64
	driftEvents       int64
	repairsApplied    int64
	runningDriftSum   float64
	runningDriftCount int64
}

type responseRecord struct {
	content   string
	embedding []float32
	timestamp time.Time
}

// NewDriftDetector creates a new drift detector.
func NewDriftDetector(cfg *Config, creed *CreedManager) *DriftDetector {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	return &DriftDetector{
		config: cfg,
		creed:  creed,
		history: &driftHistory{
			recentResponses: make([]responseRecord, 0, cfg.WindowSize),
			recentEvents:    make([]DriftEvent, 0, 100),
			maxResponses:    cfg.WindowSize,
			maxEvents:       100,
		},
	}
}

// RecordResponse records a response for drift tracking.
// The embedding should be pre-computed to keep this operation fast.
func (d *DriftDetector) RecordResponse(content string, embedding []float32) {
	d.mu.Lock()
	defer d.mu.Unlock()

	record := responseRecord{
		content:   content,
		embedding: embedding,
		timestamp: time.Now(),
	}

	// Add to history (ring buffer behavior)
	if len(d.history.recentResponses) >= d.history.maxResponses {
		d.history.recentResponses = d.history.recentResponses[1:]
	}
	d.history.recentResponses = append(d.history.recentResponses, record)
	d.history.responseSinceCheck++
}

// ShouldCheck returns true if it's time to check for drift.
func (d *DriftDetector) ShouldCheck() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.history.responseSinceCheck >= d.config.CheckInterval
}

// DetectDrift performs drift detection using embedding similarity.
// This is deterministic and completes in <10ms (pure math, no LLM calls).
func (d *DriftDetector) DetectDrift() *DriftAnalysis {
	startTime := time.Now()

	d.mu.Lock()
	defer d.mu.Unlock()

	// Reset check counter
	d.history.responseSinceCheck = 0
	d.history.totalChecks++

	analysis := &DriftAnalysis{
		PerStatementDrift: make(map[string]float64),
		DriftTrend:        "stable",
		Confidence:        0.0,
	}

	// Get creed embeddings
	creedEmbeddings := d.creed.GetEmbeddings()
	creedStatements := d.creed.GetStatements()
	combinedCreed := d.creed.GetCombinedEmbedding()

	if len(creedEmbeddings) == 0 || len(d.history.recentResponses) == 0 {
		analysis.Duration = time.Since(startTime)
		return analysis
	}

	// Compute average response embedding
	var responseEmbeddings [][]float32
	for _, r := range d.history.recentResponses {
		if len(r.embedding) > 0 {
			responseEmbeddings = append(responseEmbeddings, r.embedding)
		}
	}

	if len(responseEmbeddings) == 0 {
		analysis.Duration = time.Since(startTime)
		return analysis
	}

	avgResponseEmb := averageEmbeddings(responseEmbeddings)

	// Compute similarity to combined creed
	overallSimilarity := cosineSimilarity(avgResponseEmb, combinedCreed)
	analysis.RecentResponseSimilarity = overallSimilarity
	analysis.OverallDrift = 1.0 - overallSimilarity

	// Compute per-statement drift
	var maxDrift float64
	var maxDriftStatement string
	for i, creedEmb := range creedEmbeddings {
		similarity := cosineSimilarity(avgResponseEmb, creedEmb)
		drift := 1.0 - similarity
		statement := creedStatements[i]
		analysis.PerStatementDrift[statement] = drift

		if drift > maxDrift {
			maxDrift = drift
			maxDriftStatement = statement
		}
	}

	// Compute confidence based on sample size
	analysis.Confidence = math.Min(1.0, float64(len(responseEmbeddings))/float64(d.config.WindowSize))

	// Update running average
	d.history.runningDriftSum += analysis.OverallDrift
	d.history.runningDriftCount++

	// Determine trend by comparing to running average
	avgDrift := d.history.runningDriftSum / float64(d.history.runningDriftCount)
	if analysis.OverallDrift > avgDrift+0.05 {
		analysis.DriftTrend = "increasing"
	} else if analysis.OverallDrift < avgDrift-0.05 {
		analysis.DriftTrend = "decreasing"
	}

	// Record drift event if threshold exceeded
	if analysis.OverallDrift > d.config.DriftThreshold {
		d.history.driftEvents++
		event := DriftEvent{
			Timestamp:     time.Now(),
			DriftScore:    analysis.OverallDrift,
			ResponseCount: len(d.history.recentResponses),
			Triggered:     true,
			MostDrifted:   maxDriftStatement,
		}
		d.recordEvent(event)
	}

	analysis.Duration = time.Since(startTime)
	return analysis
}

// GetRecentDriftScore returns the most recent drift score without full analysis.
// This is a fast operation for status checks.
func (d *DriftDetector) GetRecentDriftScore() float64 {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if len(d.history.recentEvents) == 0 {
		return 0.0
	}

	return d.history.recentEvents[len(d.history.recentEvents)-1].DriftScore
}

// GetDriftHistory returns recent drift events.
func (d *DriftDetector) GetDriftHistory(limit int) []DriftEvent {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if limit <= 0 || limit > len(d.history.recentEvents) {
		limit = len(d.history.recentEvents)
	}

	// Return most recent events
	start := len(d.history.recentEvents) - limit
	result := make([]DriftEvent, limit)
	copy(result, d.history.recentEvents[start:])

	return result
}

// GetAverageDrift returns the running average drift score.
func (d *DriftDetector) GetAverageDrift() float64 {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if d.history.runningDriftCount == 0 {
		return 0.0
	}

	return d.history.runningDriftSum / float64(d.history.runningDriftCount)
}

// GetStats returns drift detection statistics.
func (d *DriftDetector) GetStats() *IdentityStats {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var lastCheck time.Time
	if len(d.history.recentEvents) > 0 {
		lastCheck = d.history.recentEvents[len(d.history.recentEvents)-1].Timestamp
	}

	avgDrift := 0.0
	if d.history.runningDriftCount > 0 {
		avgDrift = d.history.runningDriftSum / float64(d.history.runningDriftCount)
	}

	return &IdentityStats{
		Enabled:             d.config.Enabled,
		TotalChecks:         d.history.totalChecks,
		DriftEvents:         d.history.driftEvents,
		RepairsApplied:      d.history.repairsApplied,
		AverageDrift:        avgDrift,
		LastCheckTime:       lastCheck,
		ResponsesSinceCheck: d.history.responseSinceCheck,
	}
}

// recordEvent adds a drift event to history.
func (d *DriftDetector) recordEvent(event DriftEvent) {
	if len(d.history.recentEvents) >= d.history.maxEvents {
		d.history.recentEvents = d.history.recentEvents[1:]
	}
	d.history.recentEvents = append(d.history.recentEvents, event)
}

// RecordRepair records that a repair was applied.
func (d *DriftDetector) RecordRepair() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.history.repairsApplied++

	if len(d.history.recentEvents) > 0 {
		d.history.recentEvents[len(d.history.recentEvents)-1].RepairApplied = true
	}
}

// cosineSimilarity computes cosine similarity between two vectors.
func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0.0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0 || normB == 0 {
		return 0.0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}
