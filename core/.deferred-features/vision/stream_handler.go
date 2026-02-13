// CR-005: Vision Stream Handler for Phase 2
// Real-time video stream analysis with rate limiting and buffering.
//
// Architecture:
// - Ingest frames at user-controlled FPS (1-5 fps default)
// - Buffer frames in circular buffer (30 frames default)
// - Analyze every Nth frame using vision router
// - Non-blocking analysis with result channel
//
// Key Features:
// - Rate limiting to prevent overwhelming Ollama
// - Frame dropping when ingestion too fast
// - Periodic analysis based on MinFPS/AnalyzeEvery
// - Reuses existing vision router (Fast/Smart lane logic)
package vision

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
)

// StreamHandler manages real-time video stream processing.
// It coordinates frame ingestion, buffering, and periodic analysis.
type StreamHandler struct {
	buffer *FrameBuffer
	router *Router // Existing vision router

	config StreamConfig

	// Rate limiting
	lastFrame   time.Time
	minInterval time.Duration // Calculated from MaxFPS

	// Analysis loop
	analysisCtx    context.Context
	analysisCancel context.CancelFunc
	analysisCh     chan *AnalysisResult
	analysisDone   chan struct{}

	// Stats
	framesReceived atomic.Int64
	framesDropped  atomic.Int64
	analysisCount  atomic.Int64
	totalLatencyMs atomic.Int64 // Sum of all analysis latencies

	// State
	isRunning atomic.Bool
	mu        sync.RWMutex
}

// StreamConfig configures the stream handler behavior.
type StreamConfig struct {
	MinFPS         float64       `json:"min_fps"`          // Minimum FPS (default: 1)
	MaxFPS         float64       `json:"max_fps"`          // Maximum FPS (default: 5)
	BufferSize     int           `json:"buffer_size"`      // Frame buffer size (default: 30)
	AnalyzeEvery   int           `json:"analyze_every"`    // Analyze every N frames (default: 5)
	AnalysisPrompt string        `json:"analysis_prompt"`  // Default prompt for analysis
	Timeout        time.Duration `json:"timeout"`          // Analysis timeout (default: 10s)
}

// DefaultStreamConfig returns sensible defaults for streaming.
func DefaultStreamConfig() StreamConfig {
	return StreamConfig{
		MinFPS:         1.0,
		MaxFPS:         5.0,
		BufferSize:     30,
		AnalyzeEvery:   5,
		AnalysisPrompt: "What changed in this frame?",
		Timeout:        10 * time.Second,
	}
}

// Validate checks and applies defaults to the stream config.
func (c *StreamConfig) Validate() {
	if c.MinFPS <= 0 {
		c.MinFPS = 1.0
	}
	if c.MaxFPS <= 0 || c.MaxFPS < c.MinFPS {
		c.MaxFPS = 5.0
	}
	if c.BufferSize <= 0 {
		c.BufferSize = 30
	}
	if c.AnalyzeEvery <= 0 {
		c.AnalyzeEvery = 5
	}
	if c.AnalysisPrompt == "" {
		c.AnalysisPrompt = "What changed in this frame?"
	}
	if c.Timeout <= 0 {
		c.Timeout = 10 * time.Second
	}
}

// AnalysisResult contains the result of analyzing a video frame.
type AnalysisResult struct {
	FrameSequence int64            `json:"frame_sequence"` // Frame sequence number
	Analysis      *AnalyzeResponse `json:"analysis"`       // Vision analysis response
	Timestamp     time.Time        `json:"timestamp"`      // Analysis completion time
	LatencyMs     int64            `json:"latency_ms"`     // Analysis duration
	Error         error            `json:"error,omitempty"` // Error if analysis failed
}

// NewStreamHandler creates a new stream handler.
func NewStreamHandler(router *Router, config StreamConfig) *StreamHandler {
	config.Validate()

	// Calculate minimum interval between frames based on MaxFPS
	minInterval := time.Duration(float64(time.Second) / config.MaxFPS)

	return &StreamHandler{
		buffer:       NewFrameBuffer(FrameBufferConfig{Capacity: config.BufferSize}),
		router:       router,
		config:       config,
		minInterval:  minInterval,
		analysisCh:   make(chan *AnalysisResult, 10), // Buffered channel for results
		analysisDone: make(chan struct{}),
	}
}

// Start begins the analysis loop in a background goroutine.
func (h *StreamHandler) Start(ctx context.Context) error {
	if h.isRunning.Load() {
		return fmt.Errorf("stream handler already running")
	}

	h.analysisCtx, h.analysisCancel = context.WithCancel(ctx)
	h.isRunning.Store(true)

	go h.analysisLoop(h.analysisCtx)

	log.Info().
		Float64("min_fps", h.config.MinFPS).
		Float64("max_fps", h.config.MaxFPS).
		Int("buffer_size", h.config.BufferSize).
		Int("analyze_every", h.config.AnalyzeEvery).
		Msg("vision stream handler started")

	return nil
}

// Stop gracefully shuts down the stream handler.
func (h *StreamHandler) Stop() error {
	if !h.isRunning.Load() {
		return fmt.Errorf("stream handler not running")
	}

	h.analysisCancel()
	<-h.analysisDone // Wait for analysis loop to finish
	h.isRunning.Store(false)

	log.Info().
		Int64("frames_received", h.framesReceived.Load()).
		Int64("frames_dropped", h.framesDropped.Load()).
		Int64("frames_analyzed", h.analysisCount.Load()).
		Msg("vision stream handler stopped")

	return nil
}

// IngestFrame accepts a new frame with rate limiting.
// Returns error if rate limit exceeded or stream handler not running.
func (h *StreamHandler) IngestFrame(ctx context.Context, frame *Frame) error {
	if !h.isRunning.Load() {
		return fmt.Errorf("stream handler not running")
	}

	// Check rate limit
	if !h.shouldAcceptFrame() {
		h.framesDropped.Add(1)
		return fmt.Errorf("rate limit exceeded (max %.1f fps)", h.config.MaxFPS)
	}

	// Push to buffer (buffer handles sequence assignment)
	dropped := h.buffer.Push(frame)
	h.framesReceived.Add(1)

	if dropped {
		h.framesDropped.Add(1)
	}

	log.Debug().
		Int64("sequence", frame.Sequence).
		Str("mime_type", frame.MimeType).
		Int("buffer_size", h.buffer.Len()).
		Msg("frame ingested")

	return nil
}

// GetAnalysisChannel returns a receive-only channel for analysis results.
// Consumers should read from this channel to get analysis results.
func (h *StreamHandler) GetAnalysisChannel() <-chan *AnalysisResult {
	return h.analysisCh
}

// GetStats returns current streaming statistics.
func (h *StreamHandler) GetStats() StreamStats {
	analysisCount := h.analysisCount.Load()
	totalLatency := h.totalLatencyMs.Load()

	var avgAnalysisMs float64
	if analysisCount > 0 {
		avgAnalysisMs = float64(totalLatency) / float64(analysisCount)
	}

	// Calculate current FPS based on recent frame ingestion
	var currentFPS float64
	h.mu.RLock()
	if !h.lastFrame.IsZero() {
		elapsed := time.Since(h.lastFrame)
		if elapsed > 0 && elapsed < 5*time.Second {
			currentFPS = 1.0 / elapsed.Seconds()
		}
	}
	h.mu.RUnlock()

	bufferSize := h.buffer.Len()
	bufferCap := h.buffer.Capacity()
	bufferUtilization := 0.0
	if bufferCap > 0 {
		bufferUtilization = float64(bufferSize) / float64(bufferCap)
	}

	return StreamStats{
		IsRunning:         h.isRunning.Load(),
		FramesReceived:    h.framesReceived.Load(),
		FramesDropped:     h.framesDropped.Load(),
		FramesAnalyzed:    analysisCount,
		CurrentFPS:        currentFPS,
		BufferUtilization: bufferUtilization,
		AvgAnalysisMs:     avgAnalysisMs,
	}
}

// shouldAcceptFrame checks if enough time has passed since last frame.
func (h *StreamHandler) shouldAcceptFrame() bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	now := time.Now()
	if !h.lastFrame.IsZero() && now.Sub(h.lastFrame) < h.minInterval {
		return false
	}
	h.lastFrame = now
	return true
}

// analysisLoop runs in background, periodically analyzing buffered frames.
func (h *StreamHandler) analysisLoop(ctx context.Context) {
	defer close(h.analysisDone)
	defer close(h.analysisCh)

	// Calculate analysis interval based on MinFPS
	// MinFPS = 1 means analyze once per second
	analysisInterval := time.Duration(float64(time.Second) / h.config.MinFPS)
	ticker := time.NewTicker(analysisInterval)
	defer ticker.Stop()

	framesSinceAnalysis := 0

	log.Debug().
		Dur("analysis_interval", analysisInterval).
		Int("analyze_every", h.config.AnalyzeEvery).
		Msg("analysis loop started")

	for {
		select {
		case <-ctx.Done():
			log.Debug().Msg("analysis loop stopped")
			return

		case <-ticker.C:
			framesSinceAnalysis++
			if framesSinceAnalysis >= h.config.AnalyzeEvery {
				h.triggerAnalysis(ctx)
				framesSinceAnalysis = 0
			}
		}
	}
}

// triggerAnalysis picks the latest frame and sends to vision router.
func (h *StreamHandler) triggerAnalysis(ctx context.Context) {
	frame := h.buffer.PeekLatest()
	if frame == nil {
		log.Debug().Msg("no frames available for analysis")
		return
	}

	// Use existing vision router for analysis
	req := &AnalyzeRequest{
		Image:    frame.Data,
		MimeType: frame.MimeType,
		Prompt:   h.config.AnalysisPrompt,
	}

	// Apply timeout
	analysisCtx, cancel := context.WithTimeout(ctx, h.config.Timeout)
	defer cancel()

	start := time.Now()
	resp, err := h.router.Analyze(analysisCtx, req)
	latency := time.Since(start)

	// Track statistics
	h.analysisCount.Add(1)
	h.totalLatencyMs.Add(latency.Milliseconds())

	result := &AnalysisResult{
		FrameSequence: frame.Sequence,
		Analysis:      resp,
		Timestamp:     time.Now(),
		LatencyMs:     latency.Milliseconds(),
		Error:         err,
	}

	if err != nil {
		log.Warn().
			Err(err).
			Int64("frame_sequence", frame.Sequence).
			Int64("latency_ms", latency.Milliseconds()).
			Msg("frame analysis failed")
	} else {
		log.Debug().
			Int64("frame_sequence", frame.Sequence).
			Str("provider", resp.Provider).
			Int64("latency_ms", latency.Milliseconds()).
			Bool("used_fallback", resp.UsedFallback).
			Msg("frame analysis completed")
	}

	// Send result to channel (non-blocking)
	select {
	case h.analysisCh <- result:
	default:
		// Channel full, drop result
		log.Warn().
			Int64("frame_sequence", frame.Sequence).
			Msg("analysis result dropped (channel full)")
	}
}

// StreamStats contains streaming statistics.
type StreamStats struct {
	IsRunning         bool    `json:"is_running"`
	FramesReceived    int64   `json:"frames_received"`
	FramesDropped     int64   `json:"frames_dropped"`
	FramesAnalyzed    int64   `json:"frames_analyzed"`
	CurrentFPS        float64 `json:"current_fps"`
	BufferUtilization float64 `json:"buffer_utilization"` // 0.0-1.0
	AvgAnalysisMs     float64 `json:"avg_analysis_ms"`
}

// GetConfig returns the current stream configuration.
func (h *StreamHandler) GetConfig() StreamConfig {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.config
}

// UpdateConfig updates the stream configuration.
// Changes take effect on the next analysis cycle.
func (h *StreamHandler) UpdateConfig(config StreamConfig) {
	config.Validate()

	h.mu.Lock()
	defer h.mu.Unlock()

	h.config = config
	h.minInterval = time.Duration(float64(time.Second) / config.MaxFPS)

	log.Info().
		Float64("min_fps", config.MinFPS).
		Float64("max_fps", config.MaxFPS).
		Int("analyze_every", config.AnalyzeEvery).
		Msg("stream config updated")
}

// IsRunning returns true if the stream handler is currently running.
func (h *StreamHandler) IsRunning() bool {
	return h.isRunning.Load()
}

// GetBuffer returns the underlying frame buffer (read-only access).
func (h *StreamHandler) GetBuffer() *FrameBuffer {
	return h.buffer
}
