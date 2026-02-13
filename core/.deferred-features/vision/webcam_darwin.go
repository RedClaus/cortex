// +build darwin

// Package vision provides webcam capture functionality for macOS.
// CR-023: CortexEyes - Webcam Integration for Apple Silicon
package vision

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/normanking/cortex/internal/logging"
)

// WebcamCapturer captures frames from a webcam on macOS using FFmpeg.
type WebcamCapturer struct {
	cortexEyes  *CortexEyes
	interval    time.Duration
	stopCh      chan struct{}
	wg          sync.WaitGroup
	running     bool
	mu          sync.Mutex
	log         *logging.Logger
	cameraIndex int    // AVFoundation camera index (0 = FaceTime, etc.)
	ffmpegPath  string // Path to ffmpeg binary
}

// WebcamConfig configures the webcam capturer.
type WebcamConfig struct {
	Enabled     bool    // Enable webcam capture
	CameraIndex int     // Camera index (0 = default/FaceTime)
	FPS         float64 // Frames per second (default: 0.5 = 1 every 2 seconds)
	FFmpegPath  string  // Path to ffmpeg (default: auto-detect)
}

// DefaultWebcamConfig returns default webcam configuration.
func DefaultWebcamConfig() *WebcamConfig {
	return &WebcamConfig{
		Enabled:     false, // Disabled by default for privacy
		CameraIndex: 0,     // FaceTime HD Camera
		FPS:         0.5,   // 1 frame every 2 seconds
		FFmpegPath:  "",    // Auto-detect
	}
}

// NewWebcamCapturer creates a new webcam capturer for macOS.
func NewWebcamCapturer(eyes *CortexEyes, config *WebcamConfig) *WebcamCapturer {
	if config == nil {
		config = DefaultWebcamConfig()
	}

	fps := config.FPS
	if fps <= 0 {
		fps = 0.5 // Default: 1 capture every 2 seconds
	}
	interval := time.Duration(float64(time.Second) / fps)

	// Find ffmpeg
	ffmpegPath := config.FFmpegPath
	if ffmpegPath == "" {
		// Try common locations
		paths := []string{
			"/opt/homebrew/bin/ffmpeg", // Apple Silicon Homebrew
			"/usr/local/bin/ffmpeg",    // Intel Homebrew
			"ffmpeg",                   // PATH
		}
		for _, p := range paths {
			if _, err := exec.LookPath(p); err == nil {
				ffmpegPath = p
				break
			}
		}
	}

	return &WebcamCapturer{
		cortexEyes:  eyes,
		interval:    interval,
		stopCh:      make(chan struct{}),
		log:         logging.Global(),
		cameraIndex: config.CameraIndex,
		ffmpegPath:  ffmpegPath,
	}
}

// Start begins the webcam capture loop.
func (wc *WebcamCapturer) Start(ctx context.Context) error {
	wc.mu.Lock()
	if wc.running {
		wc.mu.Unlock()
		return fmt.Errorf("webcam capturer already running")
	}

	if wc.ffmpegPath == "" {
		wc.mu.Unlock()
		return fmt.Errorf("ffmpeg not found - required for webcam capture")
	}

	wc.running = true
	wc.mu.Unlock()

	wc.wg.Add(1)
	go wc.captureLoop(ctx)

	wc.log.Info("[WebcamCapture] Started (camera=%d, interval=%v)", wc.cameraIndex, wc.interval)
	return nil
}

// Stop stops the webcam capture loop.
func (wc *WebcamCapturer) Stop() {
	wc.mu.Lock()
	if !wc.running {
		wc.mu.Unlock()
		return
	}
	wc.running = false
	wc.mu.Unlock()

	close(wc.stopCh)
	wc.wg.Wait()
	wc.log.Info("[WebcamCapture] Stopped")
}

// captureLoop is the main capture loop.
func (wc *WebcamCapturer) captureLoop(ctx context.Context) {
	defer wc.wg.Done()

	ticker := time.NewTicker(wc.interval)
	defer ticker.Stop()

	// Capture immediately on start
	wc.captureAndProcess(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-wc.stopCh:
			return
		case <-ticker.C:
			wc.captureAndProcess(ctx)
		}
	}
}

// captureAndProcess captures a frame from the webcam and sends it to CortexEyes.
func (wc *WebcamCapturer) captureAndProcess(ctx context.Context) {
	// Create temp file for output
	tmpFile := fmt.Sprintf("/tmp/cortex_webcam_%d.jpg", time.Now().UnixNano())
	defer func() {
		_ = os.Remove(tmpFile)
	}()

	// Capture single frame using FFmpeg with AVFoundation
	// -f avfoundation: Use AVFoundation input
	// -framerate 30: Camera framerate (required)
	// -i "0": Camera index
	// -frames:v 1: Capture only 1 frame
	// -f image2: Output as image
	// -y: Overwrite output
	cmd := exec.CommandContext(ctx, wc.ffmpegPath,
		"-f", "avfoundation",
		"-framerate", "30",
		"-i", strconv.Itoa(wc.cameraIndex),
		"-frames:v", "1",
		"-f", "image2",
		"-y",
		tmpFile,
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	start := time.Now()
	if err := cmd.Run(); err != nil {
		// Check for common errors
		errStr := stderr.String()
		if strings.Contains(errStr, "Permission") || strings.Contains(errStr, "access") {
			wc.log.Warn("[WebcamCapture] Camera access denied - grant permission in System Preferences > Privacy > Camera")
		} else if strings.Contains(errStr, "busy") || strings.Contains(errStr, "in use") {
			wc.log.Debug("[WebcamCapture] Camera busy (in use by another app)")
		} else {
			wc.log.Debug("[WebcamCapture] Failed: %v", err)
		}
		return
	}

	// Read the captured image
	imageData, err := os.ReadFile(tmpFile)
	if err != nil {
		wc.log.Debug("[WebcamCapture] Failed to read capture: %v", err)
		return
	}

	captureTime := time.Since(start)

	if len(imageData) == 0 {
		wc.log.Debug("[WebcamCapture] Empty capture")
		return
	}

	wc.log.Info("[WebcamCapture] Captured %d bytes in %v", len(imageData), captureTime)

	// Create frame and send to CortexEyes
	// Use "webcam" as app name to distinguish from screen captures
	frame := &Frame{
		Data:      imageData,
		MimeType:  "image/jpeg",
		Timestamp: time.Now(),
		Sequence:  time.Now().UnixNano(),
	}

	if err := wc.cortexEyes.ProcessFrame(ctx, frame, "webcam", "user_presence"); err != nil {
		wc.log.Debug("[WebcamCapture] ProcessFrame error: %v", err)
	}
}

// IsRunning returns true if the capturer is running.
func (wc *WebcamCapturer) IsRunning() bool {
	wc.mu.Lock()
	defer wc.mu.Unlock()
	return wc.running
}

// SetCamera changes the camera index.
func (wc *WebcamCapturer) SetCamera(index int) {
	wc.mu.Lock()
	defer wc.mu.Unlock()
	wc.cameraIndex = index
	wc.log.Info("[WebcamCapture] Camera changed to index %d", index)
}

// ListCameras returns available cameras on the system.
func ListCameras() ([]CameraInfo, error) {
	// Find ffmpeg
	ffmpegPath := ""
	paths := []string{
		"/opt/homebrew/bin/ffmpeg",
		"/usr/local/bin/ffmpeg",
		"ffmpeg",
	}
	for _, p := range paths {
		if _, err := exec.LookPath(p); err == nil {
			ffmpegPath = p
			break
		}
	}

	if ffmpegPath == "" {
		return nil, fmt.Errorf("ffmpeg not found")
	}

	// List devices
	cmd := exec.Command(ffmpegPath, "-f", "avfoundation", "-list_devices", "true", "-i", "")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	_ = cmd.Run() // This always "fails" because -i "" is invalid, but we get device list in stderr

	// Parse output for video devices
	var cameras []CameraInfo
	lines := strings.Split(stderr.String(), "\n")
	inVideoSection := false

	for _, line := range lines {
		if strings.Contains(line, "AVFoundation video devices:") {
			inVideoSection = true
			continue
		}
		if strings.Contains(line, "AVFoundation audio devices:") {
			inVideoSection = false
			continue
		}

		if inVideoSection && strings.Contains(line, "[") {
			// Parse line like: [AVFoundation indev @ 0x...] [0] FaceTime HD Camera
			parts := strings.SplitN(line, "] [", 2)
			if len(parts) >= 2 {
				rest := parts[1]
				// Extract index and name
				if idx := strings.Index(rest, "] "); idx > 0 {
					indexStr := rest[:idx]
					name := strings.TrimSpace(rest[idx+2:])
					if index, err := strconv.Atoi(indexStr); err == nil {
						cameras = append(cameras, CameraInfo{
							Index: index,
							Name:  name,
						})
					}
				}
			}
		}
	}

	return cameras, nil
}

// CameraInfo contains information about an available camera.
type CameraInfo struct {
	Index int    // AVFoundation device index
	Name  string // Device name (e.g., "FaceTime HD Camera")
}
