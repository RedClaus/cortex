package cognitive

import (
	"sort"
	"sync"
	"time"
)

// PipelineMetrics tracks performance and routing statistics for the cognitive pipeline.
type PipelineMetrics struct {
	mu sync.RWMutex

	// Counters
	fastLaneCount   int64
	smartLaneCount  int64
	thinkingCount   int64
	errorCount      int64

	// Latency tracking (rolling window of last 100 requests per lane)
	fastLaneLatencies  []time.Duration
	smartLaneLatencies []time.Duration
	maxSamples         int
}

// NewPipelineMetrics creates a new metrics collector.
func NewPipelineMetrics() *PipelineMetrics {
	return &PipelineMetrics{
		fastLaneLatencies:  make([]time.Duration, 0, 100),
		smartLaneLatencies: make([]time.Duration, 0, 100),
		maxSamples:         100,
	}
}

// RecordRequest records metrics for a completed request.
func (m *PipelineMetrics) RecordRequest(lane Lane, latency time.Duration, usedThinking bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Update counters
	if lane == FastLane {
		m.fastLaneCount++
		m.fastLaneLatencies = append(m.fastLaneLatencies, latency)
		// Keep only last maxSamples
		if len(m.fastLaneLatencies) > m.maxSamples {
			m.fastLaneLatencies = m.fastLaneLatencies[1:]
		}
	} else {
		m.smartLaneCount++
		m.smartLaneLatencies = append(m.smartLaneLatencies, latency)
		// Keep only last maxSamples
		if len(m.smartLaneLatencies) > m.maxSamples {
			m.smartLaneLatencies = m.smartLaneLatencies[1:]
		}
	}

	if usedThinking {
		m.thinkingCount++
	}
}

// RecordError records a pipeline error.
func (m *PipelineMetrics) RecordError() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errorCount++
}

// GetStats returns current metrics statistics.
func (m *PipelineMetrics) GetStats() MetricsStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := MetricsStats{
		FastLaneCount:  m.fastLaneCount,
		SmartLaneCount: m.smartLaneCount,
		ThinkingCount:  m.thinkingCount,
		ErrorCount:     m.errorCount,
	}

	// Calculate fast lane stats
	if len(m.fastLaneLatencies) > 0 {
		stats.FastLaneAvgMs = calculateAvg(m.fastLaneLatencies)
		stats.FastLaneP95Ms = calculateP95(m.fastLaneLatencies)
	}

	// Calculate smart lane stats
	if len(m.smartLaneLatencies) > 0 {
		stats.SmartLaneAvgMs = calculateAvg(m.smartLaneLatencies)
		stats.SmartLaneP95Ms = calculateP95(m.smartLaneLatencies)
	}

	// Calculate local rate (percentage of requests using fast lane)
	totalRequests := m.fastLaneCount + m.smartLaneCount
	if totalRequests > 0 {
		stats.LocalRate = float64(m.fastLaneCount) / float64(totalRequests)
	}

	return stats
}

// MetricsStats contains aggregated metrics.
type MetricsStats struct {
	FastLaneCount  int64   `json:"fast_lane_count"`
	SmartLaneCount int64   `json:"smart_lane_count"`
	ThinkingCount  int64   `json:"thinking_count"`
	ErrorCount     int64   `json:"error_count"`
	FastLaneAvgMs  float64 `json:"fast_lane_avg_ms"`
	SmartLaneAvgMs float64 `json:"smart_lane_avg_ms"`
	FastLaneP95Ms  float64 `json:"fast_lane_p95_ms"`
	SmartLaneP95Ms float64 `json:"smart_lane_p95_ms"`
	LocalRate      float64 `json:"local_rate"` // Percentage using local models (0.0-1.0)
}

// calculateAvg calculates the average latency in milliseconds.
func calculateAvg(latencies []time.Duration) float64 {
	if len(latencies) == 0 {
		return 0
	}

	var sum time.Duration
	for _, lat := range latencies {
		sum += lat
	}

	return float64(sum.Milliseconds()) / float64(len(latencies))
}

// calculateP95 calculates the 95th percentile latency in milliseconds.
func calculateP95(latencies []time.Duration) float64 {
	if len(latencies) == 0 {
		return 0
	}

	// Create a sorted copy
	sorted := make([]time.Duration, len(latencies))
	copy(sorted, latencies)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

	// Calculate P95 index
	p95Index := int(float64(len(sorted)) * 0.95)
	if p95Index >= len(sorted) {
		p95Index = len(sorted) - 1
	}

	return float64(sorted[p95Index].Milliseconds())
}

// Reset clears all metrics (useful for testing).
func (m *PipelineMetrics) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.fastLaneCount = 0
	m.smartLaneCount = 0
	m.thinkingCount = 0
	m.errorCount = 0
	m.fastLaneLatencies = make([]time.Duration, 0, 100)
	m.smartLaneLatencies = make([]time.Duration, 0, 100)
}
