package vision

import (
	"context"
	"encoding/base64"
	"sync"
	"time"

	"github.com/normanking/cortexavatar/internal/bus"
	"github.com/rs/zerolog"
)

// Manager coordinates camera and screen capture.
// Actual capture happens in the browser; this manages state and coordination.
type Manager struct {
	config      *Config
	eventBus    *bus.EventBus
	logger      zerolog.Logger
	ctx         context.Context
	cancel      context.CancelFunc

	// State
	cameraActive bool
	screenActive bool
	stateMu      sync.RWMutex

	// Last captured frames
	lastCameraFrame *Frame
	lastScreenFrame *Frame
	frameMu         sync.RWMutex

	// Callbacks
	onCameraFrame func(*Frame)
	onScreenFrame func(*Frame)
	callbackMu    sync.RWMutex
}

// NewManager creates a new vision manager
func NewManager(config *Config, eventBus *bus.EventBus, logger zerolog.Logger) *Manager {
	if config == nil {
		config = DefaultConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Manager{
		config:   config,
		eventBus: eventBus,
		logger:   logger.With().Str("component", "vision").Logger(),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Start initializes the vision manager
func (m *Manager) Start() error {
	m.logger.Info().Msg("Vision manager started")
	return nil
}

// Stop shuts down the vision manager
func (m *Manager) Stop() {
	m.cancel()
	m.logger.Info().Msg("Vision manager stopped")
}

// IsCameraActive returns whether camera capture is active
func (m *Manager) IsCameraActive() bool {
	m.stateMu.RLock()
	defer m.stateMu.RUnlock()
	return m.cameraActive
}

// IsScreenActive returns whether screen capture is active
func (m *Manager) IsScreenActive() bool {
	m.stateMu.RLock()
	defer m.stateMu.RUnlock()
	return m.screenActive
}

// EnableCamera enables camera capture
func (m *Manager) EnableCamera() {
	m.stateMu.Lock()
	m.cameraActive = true
	m.stateMu.Unlock()

	m.logger.Info().Msg("Camera enabled")

	if m.eventBus != nil {
		m.eventBus.Publish(bus.Event{
			Type: bus.EventTypeCameraEnabled,
			Data: map[string]any{"camera_id": m.config.CameraID},
		})
	}
}

// DisableCamera disables camera capture
func (m *Manager) DisableCamera() {
	m.stateMu.Lock()
	m.cameraActive = false
	m.stateMu.Unlock()

	m.logger.Info().Msg("Camera disabled")

	if m.eventBus != nil {
		m.eventBus.Publish(bus.Event{
			Type: bus.EventTypeCameraDisabled,
		})
	}
}

// EnableScreen enables screen capture
func (m *Manager) EnableScreen() {
	m.stateMu.Lock()
	m.screenActive = true
	m.stateMu.Unlock()

	m.logger.Info().Msg("Screen capture enabled")

	if m.eventBus != nil {
		m.eventBus.Publish(bus.Event{
			Type: bus.EventTypeScreenShareEnabled,
		})
	}
}

// DisableScreen disables screen capture
func (m *Manager) DisableScreen() {
	m.stateMu.Lock()
	m.screenActive = false
	m.stateMu.Unlock()

	m.logger.Info().Msg("Screen capture disabled")

	if m.eventBus != nil {
		m.eventBus.Publish(bus.Event{
			Type: bus.EventTypeScreenShareDisabled,
		})
	}
}

// OnCameraFrame registers a callback for camera frames
func (m *Manager) OnCameraFrame(callback func(*Frame)) {
	m.callbackMu.Lock()
	defer m.callbackMu.Unlock()
	m.onCameraFrame = callback
}

// OnScreenFrame registers a callback for screen frames
func (m *Manager) OnScreenFrame(callback func(*Frame)) {
	m.callbackMu.Lock()
	defer m.callbackMu.Unlock()
	m.onScreenFrame = callback
}

// ProcessCameraFrame handles incoming camera frame from frontend
// imageBase64 is base64-encoded JPEG image data
func (m *Manager) ProcessCameraFrame(imageBase64 string, width, height int) {
	if !m.IsCameraActive() {
		return
	}

	// Decode image
	imageData, err := base64.StdEncoding.DecodeString(imageBase64)
	if err != nil {
		m.logger.Error().Err(err).Msg("Failed to decode camera frame")
		return
	}

	frame := &Frame{
		Data:        imageData,
		Width:       width,
		Height:      height,
		Format:      "jpeg",
		Timestamp:   time.Now(),
		CaptureType: CaptureTypeCamera,
	}

	// Store last frame
	m.frameMu.Lock()
	m.lastCameraFrame = frame
	m.frameMu.Unlock()

	// Invoke callback
	m.callbackMu.RLock()
	callback := m.onCameraFrame
	m.callbackMu.RUnlock()

	if callback != nil {
		go callback(frame)
	}

	// Publish event
	if m.eventBus != nil {
		m.eventBus.Publish(bus.Event{
			Type: bus.EventTypeFrameCaptured,
			Data: map[string]any{
				"type":   "camera",
				"width":  width,
				"height": height,
				"size":   len(imageData),
			},
		})
	}

	m.logger.Debug().Int("width", width).Int("height", height).Int("bytes", len(imageData)).Msg("Camera frame processed")
}

// ProcessScreenFrame handles incoming screen capture from frontend
// imageBase64 is base64-encoded JPEG image data
func (m *Manager) ProcessScreenFrame(imageBase64 string, width, height int) {
	if !m.IsScreenActive() {
		return
	}

	// Decode image
	imageData, err := base64.StdEncoding.DecodeString(imageBase64)
	if err != nil {
		m.logger.Error().Err(err).Msg("Failed to decode screen frame")
		return
	}

	frame := &Frame{
		Data:        imageData,
		Width:       width,
		Height:      height,
		Format:      "jpeg",
		Timestamp:   time.Now(),
		CaptureType: CaptureTypeScreen,
	}

	// Store last frame
	m.frameMu.Lock()
	m.lastScreenFrame = frame
	m.frameMu.Unlock()

	// Invoke callback
	m.callbackMu.RLock()
	callback := m.onScreenFrame
	m.callbackMu.RUnlock()

	if callback != nil {
		go callback(frame)
	}

	// Publish event
	if m.eventBus != nil {
		m.eventBus.Publish(bus.Event{
			Type: bus.EventTypeFrameCaptured,
			Data: map[string]any{
				"type":   "screen",
				"width":  width,
				"height": height,
				"size":   len(imageData),
			},
		})
	}

	m.logger.Debug().Int("width", width).Int("height", height).Int("bytes", len(imageData)).Msg("Screen frame processed")
}

// GetLastCameraFrame returns the most recent camera frame
func (m *Manager) GetLastCameraFrame() *Frame {
	m.frameMu.RLock()
	defer m.frameMu.RUnlock()
	return m.lastCameraFrame
}

// GetLastScreenFrame returns the most recent screen frame
func (m *Manager) GetLastScreenFrame() *Frame {
	m.frameMu.RLock()
	defer m.frameMu.RUnlock()
	return m.lastScreenFrame
}

// GetLastCameraBase64 returns the last camera frame as base64
func (m *Manager) GetLastCameraBase64() string {
	frame := m.GetLastCameraFrame()
	if frame == nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(frame.Data)
}

// GetLastScreenBase64 returns the last screen frame as base64
func (m *Manager) GetLastScreenBase64() string {
	frame := m.GetLastScreenFrame()
	if frame == nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(frame.Data)
}

// GetConfig returns the current configuration
func (m *Manager) GetConfig() *Config {
	return m.config
}

// UpdateConfig updates the configuration
func (m *Manager) UpdateConfig(config *Config) {
	m.config = config
	m.logger.Info().Interface("config", config).Msg("Vision config updated")
}
