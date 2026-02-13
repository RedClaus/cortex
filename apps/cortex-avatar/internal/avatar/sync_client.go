// Package avatar provides avatar sync with CortexBrain via SSE.
package avatar

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// BrainAvatarState mirrors CortexBrain's AvatarState struct
type BrainAvatarState struct {
	Phoneme    string       `json:"phoneme"`
	Emotion    BrainEmotion `json:"emotion"`
	Gaze       BrainGaze    `json:"gaze"`
	Intensity  float64      `json:"intensity"`
	IsSpeaking bool         `json:"is_speaking"`
	IsThinking bool         `json:"is_thinking"`
	Timestamp  time.Time    `json:"timestamp"`
	SessionID  string       `json:"session_id,omitempty"`
}

// BrainEmotion mirrors CortexBrain's EmotionState
type BrainEmotion struct {
	Primary   string             `json:"primary"`
	Secondary map[string]float64 `json:"secondary,omitempty"`
	Valence   float64            `json:"valence"`
	Arousal   float64            `json:"arousal"`
}

// BrainGaze mirrors CortexBrain's GazeDirection
type BrainGaze struct {
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	BlinkRate float64 `json:"blink_rate"`
}

// SyncClient connects to CortexBrain's avatar state SSE endpoint
type SyncClient struct {
	baseURL    string
	controller *Controller
	logger     zerolog.Logger
	client     *http.Client

	mu        sync.RWMutex
	connected bool
	cancel    context.CancelFunc
}

// NewSyncClient creates a new sync client
func NewSyncClient(baseURL string, controller *Controller, logger zerolog.Logger) *SyncClient {
	return &SyncClient{
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		controller: controller,
		logger:     logger.With().Str("component", "avatar-sync").Logger(),
		client: &http.Client{
			Timeout: 0, // No timeout for SSE
		},
	}
}

// Connect starts the SSE connection to CortexBrain
func (s *SyncClient) Connect(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	s.cancel = cancel

	go s.connectLoop(ctx)
	return nil
}

// Disconnect stops the SSE connection
func (s *SyncClient) Disconnect() {
	if s.cancel != nil {
		s.cancel()
	}
	s.mu.Lock()
	s.connected = false
	s.mu.Unlock()
}

// IsConnected returns connection status
func (s *SyncClient) IsConnected() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.connected
}

// connectLoop maintains the SSE connection with reconnection
func (s *SyncClient) connectLoop(ctx context.Context) {
	backoff := 3 * time.Second
	maxBackoff := 60 * time.Second
	consecutiveFailures := 0

	for {
		select {
		case <-ctx.Done():
			return
		default:
			if err := s.connectSSE(ctx); err != nil {
				consecutiveFailures++
				s.mu.Lock()
				s.connected = false
				s.mu.Unlock()

				// If we've failed many times, the endpoint probably doesn't exist
				if consecutiveFailures >= 3 {
					// Only log once at warn level, then switch to debug
					if consecutiveFailures == 3 {
						s.logger.Warn().
							Err(err).
							Int("failures", consecutiveFailures).
							Msg("Avatar state SSE endpoint not available, will retry less frequently")
					} else {
						s.logger.Debug().
							Int("failures", consecutiveFailures).
							Msg("Avatar state SSE still unavailable")
					}
					backoff = maxBackoff
				} else {
					s.logger.Warn().Err(err).Msg("SSE connection failed, reconnecting...")
				}

				// Wait before reconnecting with exponential backoff
				select {
				case <-ctx.Done():
					return
				case <-time.After(backoff):
				}

				// Increase backoff
				if backoff < maxBackoff {
					backoff = backoff * 2
					if backoff > maxBackoff {
						backoff = maxBackoff
					}
				}
			} else {
				// Reset backoff on success
				backoff = 3 * time.Second
				consecutiveFailures = 0
			}
		}
	}
}

// connectSSE establishes SSE connection
func (s *SyncClient) connectSSE(ctx context.Context) error {
	url := s.baseURL + "/api/v1/avatar/state"
	s.logger.Info().Str("url", url).Msg("Connecting to avatar state SSE")

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	// Check Content-Type is SSE
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/event-stream") {
		return fmt.Errorf("unexpected content-type: %s (expected text/event-stream)", contentType)
	}

	s.mu.Lock()
	s.connected = true
	s.mu.Unlock()
	s.logger.Info().Msg("Connected to avatar state SSE")

	// Read SSE events
	scanner := bufio.NewScanner(resp.Body)
	var eventType string
	var dataLines []string

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "event:") {
			eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		} else if strings.HasPrefix(line, "data:") {
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			dataLines = append(dataLines, data)
		} else if line == "" && len(dataLines) > 0 {
			// End of event, process it
			fullData := strings.Join(dataLines, "\n")
			s.handleEvent(eventType, fullData)
			eventType = ""
			dataLines = nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}

	return scanner.Err()
}

// handleEvent processes an SSE event
func (s *SyncClient) handleEvent(eventType, data string) {
	switch eventType {
	case "state":
		var state BrainAvatarState
		if err := json.Unmarshal([]byte(data), &state); err != nil {
			s.logger.Warn().Err(err).Msg("Failed to parse state event")
			return
		}
		s.applyBrainState(&state)

	case "phoneme":
		var phonemeData struct {
			Phoneme   string  `json:"phoneme"`
			Intensity float64 `json:"intensity"`
		}
		if err := json.Unmarshal([]byte(data), &phonemeData); err != nil {
			s.logger.Warn().Err(err).Msg("Failed to parse phoneme event")
			return
		}
		s.controller.SetMouthShape(PhonemeToViseme(phonemeData.Phoneme))

	case "emotion":
		var emotion BrainEmotion
		if err := json.Unmarshal([]byte(data), &emotion); err != nil {
			s.logger.Warn().Err(err).Msg("Failed to parse emotion event")
			return
		}
		s.controller.SetEmotion(mapBrainEmotion(emotion.Primary))

	case "gaze":
		var gaze BrainGaze
		if err := json.Unmarshal([]byte(data), &gaze); err != nil {
			s.logger.Warn().Err(err).Msg("Failed to parse gaze event")
			return
		}
		s.controller.SetLookDirection(mapGazeToDirection(gaze))

	default:
		s.logger.Debug().Str("type", eventType).Msg("Unknown event type")
	}
}

// applyBrainState applies the full brain state to the controller
func (s *SyncClient) applyBrainState(state *BrainAvatarState) {
	// Update speaking/thinking state
	if state.IsSpeaking {
		s.controller.mu.Lock()
		s.controller.state.IsSpeaking = true
		s.controller.state.IsThinking = false
		s.controller.mu.Unlock()
	} else if state.IsThinking {
		s.controller.mu.Lock()
		s.controller.state.IsThinking = true
		s.controller.state.IsSpeaking = false
		s.controller.mu.Unlock()
	} else {
		s.controller.mu.Lock()
		s.controller.state.IsSpeaking = false
		s.controller.state.IsThinking = false
		s.controller.mu.Unlock()
	}

	// Update mouth shape from phoneme
	if state.Phoneme != "" {
		s.controller.SetMouthShape(PhonemeToViseme(state.Phoneme))
	}

	// Update emotion
	s.controller.SetEmotion(mapBrainEmotion(state.Emotion.Primary))

	// Update gaze
	s.controller.SetLookDirection(mapGazeToDirection(state.Gaze))
}

// mapBrainEmotion maps CortexBrain emotion to avatar EmotionState
func mapBrainEmotion(primary string) EmotionState {
	switch strings.ToLower(primary) {
	case "joy", "happy":
		return EmotionHappy
	case "sadness", "sad":
		return EmotionSad
	case "surprise", "surprised":
		return EmotionSurprised
	case "confusion", "confused":
		return EmotionConfused
	case "excitement", "excited":
		return EmotionExcited
	case "thinking":
		return EmotionThinking
	default:
		return EmotionNeutral
	}
}

// mapGazeToDirection converts gaze x,y to a look direction
func mapGazeToDirection(gaze BrainGaze) LookDirection {
	// Gaze x/y are -1 to 1
	if gaze.X < -0.3 {
		return LookLeft
	} else if gaze.X > 0.3 {
		return LookRight
	} else if gaze.Y > 0.3 {
		return LookUp
	} else if gaze.Y < -0.3 {
		return LookDown
	}
	return LookCenter
}

// SetLookDirection sets the gaze direction
func (c *Controller) SetLookDirection(direction LookDirection) {
	c.mu.Lock()
	c.state.LookDirection = direction
	state := c.state
	c.mu.Unlock()

	c.notifyStateChange(state)
}

// CheckHealth checks the avatar health endpoint
func (s *SyncClient) CheckHealth(ctx context.Context) error {
	url := s.baseURL + "/api/v1/avatar/health"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed: %d", resp.StatusCode)
	}

	return nil
}

// GetCurrentState fetches current state snapshot
func (s *SyncClient) GetCurrentState(ctx context.Context) (*BrainAvatarState, error) {
	url := s.baseURL + "/api/v1/avatar/current"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get current state failed: %d", resp.StatusCode)
	}

	var state BrainAvatarState
	if err := json.NewDecoder(resp.Body).Decode(&state); err != nil {
		return nil, err
	}

	return &state, nil
}
