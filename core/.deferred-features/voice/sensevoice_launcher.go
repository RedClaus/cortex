// Package voice provides voice processing capabilities for Cortex.
// sensevoice_launcher.go manages the SenseVoice Python sidecar lifecycle.
package voice

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// SenseVoiceConfig holds launcher configuration.
type SenseVoiceConfig struct {
	// InstallDir is the directory where SenseVoice is installed
	InstallDir string

	// Host and port for the SenseVoice server
	Host string
	Port int

	// Timeouts
	StartupTimeout  time.Duration
	HealthTimeout   time.Duration
	ShutdownTimeout time.Duration

	// Feature flags
	EnableEmotion bool
	EnableEvents  bool
}

// DefaultSenseVoiceConfig returns sensible defaults.
func DefaultSenseVoiceConfig() SenseVoiceConfig {
	homeDir, _ := os.UserHomeDir()
	return SenseVoiceConfig{
		InstallDir:      filepath.Join(homeDir, ".cortex", "sensevoice"),
		Host:            "127.0.0.1",
		Port:            8881, // Different from VoiceBox (8880)
		StartupTimeout:  30 * time.Second, // SenseVoice model loading can be slow
		HealthTimeout:   2 * time.Second,
		ShutdownTimeout: 5 * time.Second,
		EnableEmotion:   true,
		EnableEvents:    true,
	}
}

// SenseVoiceLauncher manages the SenseVoice Python sidecar lifecycle.
// Brain Alignment: Like a dedicated neural pathway, this launcher ensures
// the voice emotion processing channel is available when needed.
type SenseVoiceLauncher struct {
	config     SenseVoiceConfig
	client     *SenseVoiceClient
	httpClient *SenseVoiceClient

	// Process management
	cmd     *exec.Cmd
	running bool
	mu      sync.RWMutex

	// Lazy initialization
	initialized bool
	initOnce    sync.Once
	initErr     error
}

// NewSenseVoiceLauncher creates a new launcher instance.
func NewSenseVoiceLauncher(config SenseVoiceConfig) *SenseVoiceLauncher {
	clientConfig := &STTBackendConfig{
		Enabled:       true,
		Endpoint:      fmt.Sprintf("http://%s:%d", config.Host, config.Port),
		EnableEmotion: config.EnableEmotion,
		EnableEvents:  config.EnableEvents,
		Timeout:       config.HealthTimeout,
	}

	return &SenseVoiceLauncher{
		config: config,
		client: NewSenseVoiceClient(clientConfig),
	}
}

// Endpoint returns the base URL for SenseVoice API.
func (l *SenseVoiceLauncher) Endpoint() string {
	return fmt.Sprintf("http://%s:%d", l.config.Host, l.config.Port)
}

// IsInstalled checks if SenseVoice has been installed.
func (l *SenseVoiceLauncher) IsInstalled() bool {
	serverScript := filepath.Join(l.config.InstallDir, "server.py")
	pythonBin := filepath.Join(l.config.InstallDir, "bin", "python")

	_, err1 := os.Stat(serverScript)
	_, err2 := os.Stat(pythonBin)

	return err1 == nil && err2 == nil
}

// IsHealthy checks if SenseVoice is running and responding.
func (l *SenseVoiceLauncher) IsHealthy() bool {
	ctx, cancel := context.WithTimeout(context.Background(), l.config.HealthTimeout)
	defer cancel()

	return l.client.IsAvailable(ctx)
}

// EnsureRunning starts SenseVoice if not already running.
// This is the primary entry point - call this before making STT requests.
func (l *SenseVoiceLauncher) EnsureRunning(ctx context.Context) error {
	// Fast path: already healthy
	if l.IsHealthy() {
		return nil
	}

	// Check if installed
	if !l.IsInstalled() {
		return fmt.Errorf("SenseVoice not installed. Run: ./scripts/install-sensevoice.sh")
	}

	// Try to start
	return l.Start(ctx)
}

// Start launches the SenseVoice server.
func (l *SenseVoiceLauncher) Start(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Double-check health under lock
	if l.IsHealthy() {
		l.running = true
		return nil
	}

	// Check for existing process via PID file
	if l.isRunningViaPIDFile() {
		// Process exists but not responding - kill it
		l.killViaPIDFile()
		time.Sleep(500 * time.Millisecond)
	}

	// Start via shell script (preferred - handles logging, PID file)
	startScript := filepath.Join(l.config.InstallDir, "start.sh")
	if _, err := os.Stat(startScript); err == nil {
		log.Info().Str("script", startScript).Msg("starting SenseVoice via script")

		cmd := exec.CommandContext(ctx, "bash", startScript)
		cmd.Dir = l.config.InstallDir

		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("start script failed: %w\nOutput: %s", err, string(output))
		}

		// Wait for healthy
		if err := l.waitForHealthy(ctx); err != nil {
			return err
		}

		l.running = true
		log.Info().Str("endpoint", l.Endpoint()).Msg("SenseVoice started successfully")
		return nil
	}

	// Fallback: direct Python invocation
	return l.startDirect(ctx)
}

// startDirect launches Python directly without shell script.
func (l *SenseVoiceLauncher) startDirect(ctx context.Context) error {
	pythonBin := filepath.Join(l.config.InstallDir, "bin", "python")
	serverScript := filepath.Join(l.config.InstallDir, "server.py")

	log.Info().
		Str("python", pythonBin).
		Str("server", serverScript).
		Msg("starting SenseVoice directly")

	l.cmd = exec.Command(pythonBin, serverScript)
	l.cmd.Dir = l.config.InstallDir
	l.cmd.Env = append(os.Environ(),
		fmt.Sprintf("SENSEVOICE_HOST=%s", l.config.Host),
		fmt.Sprintf("SENSEVOICE_PORT=%d", l.config.Port),
	)

	// Redirect output to log file
	logFile := filepath.Join(l.config.InstallDir, "sensevoice.log")
	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		l.cmd.Stdout = f
		l.cmd.Stderr = f
	}

	if err := l.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start SenseVoice: %w", err)
	}

	// Write PID file
	pidFile := filepath.Join(l.config.InstallDir, "sensevoice.pid")
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(l.cmd.Process.Pid)), 0644); err != nil {
		log.Warn().Err(err).Msg("failed to write PID file")
	}

	// Wait for healthy
	if err := l.waitForHealthy(ctx); err != nil {
		if l.cmd.Process != nil {
			l.cmd.Process.Kill()
		}
		return err
	}

	l.running = true
	log.Info().
		Int("pid", l.cmd.Process.Pid).
		Str("endpoint", l.Endpoint()).
		Msg("SenseVoice started successfully")
	return nil
}

// waitForHealthy polls the health endpoint until success or timeout.
func (l *SenseVoiceLauncher) waitForHealthy(ctx context.Context) error {
	deadline := time.Now().Add(l.config.StartupTimeout)
	ticker := time.NewTicker(500 * time.Millisecond) // Slower polling for SenseVoice
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if l.IsHealthy() {
				return nil
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("SenseVoice failed to become healthy within %v", l.config.StartupTimeout)
			}
		}
	}
}

// Stop gracefully shuts down SenseVoice.
func (l *SenseVoiceLauncher) Stop() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Try stop script first
	stopScript := filepath.Join(l.config.InstallDir, "stop.sh")
	if _, err := os.Stat(stopScript); err == nil {
		cmd := exec.Command("bash", stopScript)
		cmd.Dir = l.config.InstallDir
		if err := cmd.Run(); err == nil {
			l.running = false
			log.Info().Msg("SenseVoice stopped via script")
			return nil
		}
	}

	// Kill via PID file
	if l.killViaPIDFile() {
		l.running = false
		log.Info().Msg("SenseVoice stopped via PID file")
		return nil
	}

	// Kill direct process
	if l.cmd != nil && l.cmd.Process != nil {
		if err := l.cmd.Process.Kill(); err == nil {
			l.running = false
			log.Info().Msg("SenseVoice process killed")
			return nil
		}
	}

	return fmt.Errorf("SenseVoice not running")
}

// isRunningViaPIDFile checks if process from PID file is alive.
func (l *SenseVoiceLauncher) isRunningViaPIDFile() bool {
	pidFile := filepath.Join(l.config.InstallDir, "sensevoice.pid")
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return false
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return false
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// On Unix, FindProcess always succeeds, so we need to send signal 0
	err = process.Signal(os.Signal(nil))
	return err == nil
}

// killViaPIDFile kills the process from PID file.
func (l *SenseVoiceLauncher) killViaPIDFile() bool {
	pidFile := filepath.Join(l.config.InstallDir, "sensevoice.pid")
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return false
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return false
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	if err := process.Kill(); err != nil {
		return false
	}

	os.Remove(pidFile)
	return true
}

// Client returns the SenseVoice client for transcription requests.
func (l *SenseVoiceLauncher) Client() *SenseVoiceClient {
	return l.client
}

// Config returns the launcher configuration.
func (l *SenseVoiceLauncher) Config() SenseVoiceConfig {
	return l.config
}

// GetHealth returns detailed health information.
func (l *SenseVoiceLauncher) GetHealth(ctx context.Context) (*SenseVoiceHealth, error) {
	return l.client.GetHealth(ctx)
}

// ─────────────────────────────────────────────────────────────────────────────
// Global launcher singleton for convenience
// ─────────────────────────────────────────────────────────────────────────────

var (
	globalSenseVoiceLauncher     *SenseVoiceLauncher
	globalSenseVoiceLauncherOnce sync.Once
)

// GetSenseVoiceLauncher returns a global launcher instance.
func GetSenseVoiceLauncher() *SenseVoiceLauncher {
	globalSenseVoiceLauncherOnce.Do(func() {
		globalSenseVoiceLauncher = NewSenseVoiceLauncher(DefaultSenseVoiceConfig())
	})
	return globalSenseVoiceLauncher
}
