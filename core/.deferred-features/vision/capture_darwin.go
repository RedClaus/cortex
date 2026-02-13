// +build darwin

// Package vision provides screen capture functionality for macOS.
// CR-023: CortexEyes - Screen Capture for Apple Silicon
package vision

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/normanking/cortex/internal/logging"
)

// ScreenCapturer captures screenshots on macOS using the screencapture command.
type ScreenCapturer struct {
	cortexEyes *CortexEyes
	interval   time.Duration
	stopCh     chan struct{}
	wg         sync.WaitGroup
	running    bool
	mu         sync.Mutex
	log        *logging.Logger
}

// NewScreenCapturer creates a new screen capturer for macOS.
func NewScreenCapturer(eyes *CortexEyes, fps float64) *ScreenCapturer {
	if fps <= 0 {
		fps = 0.2 // Default: 1 capture every 5 seconds
	}
	interval := time.Duration(float64(time.Second) / fps)

	return &ScreenCapturer{
		cortexEyes: eyes,
		interval:   interval,
		stopCh:     make(chan struct{}),
		log:        logging.Global(),
	}
}

// Start begins the screen capture loop.
func (sc *ScreenCapturer) Start(ctx context.Context) error {
	sc.mu.Lock()
	if sc.running {
		sc.mu.Unlock()
		return fmt.Errorf("screen capturer already running")
	}
	sc.running = true
	sc.mu.Unlock()

	sc.wg.Add(1)
	go sc.captureLoop(ctx)

	sc.log.Info("[ScreenCapture] Started (interval=%v)", sc.interval)
	return nil
}

// Stop stops the screen capture loop.
func (sc *ScreenCapturer) Stop() {
	sc.mu.Lock()
	if !sc.running {
		sc.mu.Unlock()
		return
	}
	sc.running = false
	sc.mu.Unlock()

	close(sc.stopCh)
	sc.wg.Wait()
	sc.log.Info("[ScreenCapture] Stopped")
}

// captureLoop is the main capture loop.
func (sc *ScreenCapturer) captureLoop(ctx context.Context) {
	defer sc.wg.Done()

	ticker := time.NewTicker(sc.interval)
	defer ticker.Stop()

	// Capture immediately on start
	sc.captureAndProcess(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-sc.stopCh:
			return
		case <-ticker.C:
			sc.captureAndProcess(ctx)
		}
	}
}

// captureAndProcess captures the screen and sends it to CortexEyes.
func (sc *ScreenCapturer) captureAndProcess(ctx context.Context) {
	// Capture screen using macOS screencapture command
	// -x: no sound, -C: capture cursor, -t png: PNG format
	// Note: stdout (-) doesn't work reliably on macOS, use temp file instead
	tmpFile := fmt.Sprintf("/tmp/cortex_capture_%d.png", time.Now().UnixNano())
	defer func() {
		// Clean up temp file
		_ = exec.Command("rm", "-f", tmpFile).Run()
	}()

	cmd := exec.CommandContext(ctx, "screencapture", "-x", "-C", "-t", "png", tmpFile)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	start := time.Now()
	if err := cmd.Run(); err != nil {
		sc.log.Debug("[ScreenCapture] Failed: %v (stderr: %s)", err, stderr.String())
		return
	}

	// Read the captured image from temp file
	imageData, err := os.ReadFile(tmpFile)
	if err != nil {
		sc.log.Debug("[ScreenCapture] Failed to read capture: %v", err)
		return
	}

	captureTime := time.Since(start)

	if len(imageData) == 0 {
		sc.log.Debug("[ScreenCapture] Empty capture")
		return
	}

	sc.log.Info("[ScreenCapture] Captured %d bytes in %v", len(imageData), captureTime)

	// Get active application info (optional, for context)
	appName, windowTitle := sc.getActiveWindow()

	// Create frame and send to CortexEyes
	frame := &Frame{
		Data:      imageData,
		MimeType:  "image/png",
		Timestamp: time.Now(),
		Sequence:  time.Now().UnixNano(),
	}

	if err := sc.cortexEyes.ProcessFrame(ctx, frame, appName, windowTitle); err != nil {
		sc.log.Debug("[ScreenCapture] ProcessFrame error: %v", err)
	}
}

// getActiveWindow gets the currently active application and window title on macOS.
// Uses timeout context to prevent hanging if AppleScript doesn't respond.
func (sc *ScreenCapturer) getActiveWindow() (appName, windowTitle string) {
	// Create timeout context - AppleScript should respond within 2 seconds
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Use AppleScript to get the frontmost application
	script := `
		tell application "System Events"
			set frontApp to name of first application process whose frontmost is true
			return frontApp
		end tell
	`
	cmd := exec.CommandContext(ctx, "osascript", "-e", script)
	output, err := cmd.Output()
	if err == nil {
		appName = string(bytes.TrimSpace(output))
	} else {
		sc.log.Debug("[ScreenCapture] Failed to get app name: %v", err)
	}

	// Check if context was cancelled
	if ctx.Err() != nil {
		return appName, ""
	}

	// Get window title with fresh timeout
	ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel2()

	windowScript := `
		tell application "System Events"
			tell (first application process whose frontmost is true)
				if (count of windows) > 0 then
					return name of front window
				else
					return ""
				end if
			end tell
		end tell
	`
	cmd = exec.CommandContext(ctx2, "osascript", "-e", windowScript)
	output, err = cmd.Output()
	if err == nil {
		windowTitle = string(bytes.TrimSpace(output))
	} else {
		sc.log.Debug("[ScreenCapture] Failed to get window title: %v", err)
	}

	return appName, windowTitle
}

// IsRunning returns true if the capturer is running.
func (sc *ScreenCapturer) IsRunning() bool {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return sc.running
}
