// Package vision provides camera and screen capture for CortexAvatar.
package vision

import (
	"errors"
	"time"
)

// Common errors
var (
	ErrCameraNotAvailable  = errors.New("camera not available")
	ErrScreenNotAvailable  = errors.New("screen capture not available")
	ErrCaptureNotStarted   = errors.New("capture not started")
	ErrPermissionDenied    = errors.New("capture permission denied")
)

// CaptureType identifies the capture source
type CaptureType string

const (
	CaptureTypeCamera CaptureType = "camera"
	CaptureTypeScreen CaptureType = "screen"
)

// Config holds vision capture configuration
type Config struct {
	CameraEnabled bool   `json:"camera_enabled"`
	ScreenEnabled bool   `json:"screen_enabled"`
	CameraID      string `json:"camera_id"`       // Camera device ID
	MaxFPS        int    `json:"max_fps"`         // Max frames per second
	Quality       int    `json:"quality"`         // JPEG quality (1-100)
	MaxWidth      int    `json:"max_width"`       // Max image width
	MaxHeight     int    `json:"max_height"`      // Max image height
}

// DefaultConfig returns sensible defaults
func DefaultConfig() *Config {
	return &Config{
		CameraEnabled: false,
		ScreenEnabled: false,
		CameraID:      "default",
		MaxFPS:        1,  // 1 frame per second for A2A
		Quality:       70, // Reasonable quality/size balance
		MaxWidth:      1280,
		MaxHeight:     720,
	}
}

// Frame represents a captured image frame
type Frame struct {
	Data        []byte      `json:"data"`         // Image bytes (JPEG)
	Width       int         `json:"width"`
	Height      int         `json:"height"`
	Format      string      `json:"format"`       // jpeg, png
	Timestamp   time.Time   `json:"timestamp"`
	CaptureType CaptureType `json:"capture_type"` // camera or screen
}

// CameraInfo describes an available camera
type CameraInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	IsDefault   bool   `json:"is_default"`
	MaxWidth    int    `json:"max_width"`
	MaxHeight   int    `json:"max_height"`
}

// ScreenInfo describes an available screen/display
type ScreenInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	IsPrimary bool   `json:"is_primary"`
}
