package memcell

import (
	"context"
	"strings"
	"time"
)

// ══════════════════════════════════════════════════════════════════════════════
// BOUNDARY DETECTOR
// Detects event boundaries in conversation streams for episode segmentation
// ══════════════════════════════════════════════════════════════════════════════

// BoundaryDetector detects event boundaries between conversation turns.
type BoundaryDetector struct {
	config   BoundaryConfig
	embedder EmbedderFunc

	// State for tracking episodes
	lastTurnTime    time.Time
	lastTurnContent string
	lastEmbedding   []float32
}

// NewBoundaryDetector creates a new boundary detector.
func NewBoundaryDetector(embedder EmbedderFunc) *BoundaryDetector {
	return &BoundaryDetector{
		config:   DefaultBoundaryConfig(),
		embedder: embedder,
	}
}

// NewBoundaryDetectorWithConfig creates a boundary detector with custom config.
func NewBoundaryDetectorWithConfig(embedder EmbedderFunc, config BoundaryConfig) *BoundaryDetector {
	return &BoundaryDetector{
		config:   config,
		embedder: embedder,
	}
}

// DetectBoundary determines if the current turn marks an event boundary.
// Returns true if a new episode should start.
func (d *BoundaryDetector) DetectBoundary(ctx context.Context, turn ConversationTurn) (bool, BoundaryReason) {
	// First turn is always a boundary (starts first episode)
	if d.lastTurnContent == "" {
		d.updateState(ctx, turn)
		return true, BoundaryReasonFirstTurn
	}

	// Check time gap
	if !d.lastTurnTime.IsZero() && !turn.Timestamp.IsZero() {
		gap := turn.Timestamp.Sub(d.lastTurnTime)
		if gap > d.config.TimeGap {
			d.updateState(ctx, turn)
			return true, BoundaryReasonTimeGap
		}
	}

	// Check explicit transition patterns
	if d.hasTransitionPattern(turn.Content) {
		d.updateState(ctx, turn)
		return true, BoundaryReasonTransition
	}

	// Check completion signals in previous turn
	if d.hasCompletionSignal(d.lastTurnContent) {
		d.updateState(ctx, turn)
		return true, BoundaryReasonCompletion
	}

	// Check semantic distance (if embedder available)
	if d.embedder != nil {
		distance := d.computeSemanticDistance(ctx, turn.Content)
		if distance > d.config.EmbeddingThreshold {
			d.updateState(ctx, turn)
			return true, BoundaryReasonSemantic
		}
	}

	// No boundary detected
	d.updateState(ctx, turn)
	return false, BoundaryReasonNone
}

// Reset clears the detector state (for testing or new conversations).
func (d *BoundaryDetector) Reset() {
	d.lastTurnTime = time.Time{}
	d.lastTurnContent = ""
	d.lastEmbedding = nil
}

// ══════════════════════════════════════════════════════════════════════════════
// BOUNDARY REASONS
// ══════════════════════════════════════════════════════════════════════════════

// BoundaryReason explains why a boundary was detected.
type BoundaryReason string

const (
	BoundaryReasonNone       BoundaryReason = ""
	BoundaryReasonFirstTurn  BoundaryReason = "first_turn"
	BoundaryReasonTimeGap    BoundaryReason = "time_gap"
	BoundaryReasonTransition BoundaryReason = "transition_phrase"
	BoundaryReasonCompletion BoundaryReason = "completion_signal"
	BoundaryReasonSemantic   BoundaryReason = "semantic_distance"
)

// ══════════════════════════════════════════════════════════════════════════════
// DETECTION HELPERS
// ══════════════════════════════════════════════════════════════════════════════

func (d *BoundaryDetector) hasTransitionPattern(content string) bool {
	lower := strings.ToLower(content)

	// Check if content STARTS with a transition pattern
	for _, pattern := range d.config.TransitionPatterns {
		if strings.HasPrefix(lower, pattern) {
			return true
		}
	}

	// Also check for "anyway" and "by the way" after punctuation
	midPatterns := []string{". anyway", "! anyway", "? anyway", ". by the way", "! by the way", ", anyway"}
	for _, pattern := range midPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}

	return false
}

func (d *BoundaryDetector) hasCompletionSignal(content string) bool {
	lower := strings.ToLower(content)

	completionSignals := []string{
		"thanks", "thank you", "thx",
		"that works", "that fixed it", "perfect",
		"got it", "understood", "makes sense",
		"solved", "done", "finished", "complete",
		"great", "awesome", "excellent",
	}

	for _, signal := range completionSignals {
		if strings.Contains(lower, signal) {
			return true
		}
	}

	return false
}

func (d *BoundaryDetector) computeSemanticDistance(ctx context.Context, content string) float64 {
	if d.embedder == nil {
		return 0
	}

	// Get embedding for current content
	currentEmb, err := d.embedder(ctx, content)
	if err != nil {
		return 0
	}

	// If no previous embedding, store this one and return 0
	if d.lastEmbedding == nil {
		return 0
	}

	// Compute cosine similarity
	similarity := cosineSimilarity(d.lastEmbedding, currentEmb)

	// Return distance (1 - similarity)
	// High distance = low similarity = likely different topic
	return 1.0 - similarity
}

func (d *BoundaryDetector) updateState(ctx context.Context, turn ConversationTurn) {
	d.lastTurnTime = turn.Timestamp
	d.lastTurnContent = turn.Content

	// Update embedding if embedder available
	if d.embedder != nil {
		if emb, err := d.embedder(ctx, turn.Content); err == nil {
			d.lastEmbedding = emb
		}
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// EPISODE MANAGER
// Tracks active episodes and manages episode lifecycle
// ══════════════════════════════════════════════════════════════════════════════

// EpisodeManager manages episode state during extraction.
type EpisodeManager struct {
	detector       *BoundaryDetector
	currentEpisode *Episode
	episodes       []*Episode
}

// Episode represents a conversation segment.
type Episode struct {
	ID        string
	StartedAt time.Time
	EndedAt   time.Time
	TurnCount int
	Topic     string // Inferred topic (optional)
	Outcome   string // success, failure, ongoing
}

// NewEpisodeManager creates a new episode manager.
func NewEpisodeManager(detector *BoundaryDetector) *EpisodeManager {
	return &EpisodeManager{
		detector: detector,
	}
}

// ProcessTurn processes a conversation turn and returns the episode ID.
func (m *EpisodeManager) ProcessTurn(ctx context.Context, turn ConversationTurn) (string, bool) {
	isBoundary, _ := m.detector.DetectBoundary(ctx, turn)

	if isBoundary || m.currentEpisode == nil {
		// Close current episode if exists
		if m.currentEpisode != nil {
			m.currentEpisode.EndedAt = turn.Timestamp
			m.currentEpisode.Outcome = "completed"
		}

		// Start new episode
		m.currentEpisode = &Episode{
			ID:        generateUUID(),
			StartedAt: turn.Timestamp,
			TurnCount: 0,
			Outcome:   "ongoing",
		}
		m.episodes = append(m.episodes, m.currentEpisode)
	}

	m.currentEpisode.TurnCount++
	return m.currentEpisode.ID, isBoundary
}

// CurrentEpisode returns the current active episode.
func (m *EpisodeManager) CurrentEpisode() *Episode {
	return m.currentEpisode
}

// Episodes returns all episodes.
func (m *EpisodeManager) Episodes() []*Episode {
	return m.episodes
}

// Reset clears all episode state.
func (m *EpisodeManager) Reset() {
	m.detector.Reset()
	m.currentEpisode = nil
	m.episodes = nil
}

// Helper for UUID generation
func generateUUID() string {
	// Simple UUID-like string
	return strings.ReplaceAll(time.Now().Format("20060102150405.000000"), ".", "-")
}
