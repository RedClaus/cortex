package voice

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// VoiceBoxConfig holds launcher configuration
type VoiceBoxConfig struct {
	// InstallDir is the directory where Voice Box is installed
	InstallDir string

	// Host and port for the Voice Box server
	Host string
	Port int

	// Timeouts
	StartupTimeout  time.Duration
	HealthTimeout   time.Duration
	ShutdownTimeout time.Duration
}

// DefaultVoiceBoxConfig returns sensible defaults
func DefaultVoiceBoxConfig() VoiceBoxConfig {
	homeDir, _ := os.UserHomeDir()
	return VoiceBoxConfig{
		InstallDir:      filepath.Join(homeDir, ".cortex", "voicebox"),
		Host:            "127.0.0.1",
		Port:            8880,
		StartupTimeout:  15 * time.Second,
		HealthTimeout:   2 * time.Second,
		ShutdownTimeout: 5 * time.Second,
	}
}

// VoiceBoxLauncher manages the Python TTS sidecar lifecycle
type VoiceBoxLauncher struct {
	config     VoiceBoxConfig
	httpClient *http.Client

	// Process management
	cmd     *exec.Cmd
	running bool
	mu      sync.RWMutex

	// Lazy initialization
	initialized bool
	initOnce    sync.Once
	initErr     error
}

// NewVoiceBoxLauncher creates a new launcher instance
func NewVoiceBoxLauncher(config VoiceBoxConfig) *VoiceBoxLauncher {
	return &VoiceBoxLauncher{
		config: config,
		httpClient: &http.Client{
			Timeout: config.HealthTimeout,
		},
	}
}

// Endpoint returns the base URL for Voice Box API
func (l *VoiceBoxLauncher) Endpoint() string {
	return fmt.Sprintf("http://%s:%d", l.config.Host, l.config.Port)
}

// SpeechEndpoint returns the URL for the speech synthesis endpoint
func (l *VoiceBoxLauncher) SpeechEndpoint() string {
	return l.Endpoint() + "/v1/audio/speech"
}

// IsInstalled checks if Voice Box has been installed
func (l *VoiceBoxLauncher) IsInstalled() bool {
	serverScript := filepath.Join(l.config.InstallDir, "server.py")
	pythonBin := filepath.Join(l.config.InstallDir, "bin", "python")

	_, err1 := os.Stat(serverScript)
	_, err2 := os.Stat(pythonBin)

	return err1 == nil && err2 == nil
}

// IsHealthy checks if Voice Box is running and responding
func (l *VoiceBoxLauncher) IsHealthy() bool {
	ctx, cancel := context.WithTimeout(context.Background(), l.config.HealthTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", l.Endpoint()+"/health", nil)
	if err != nil {
		return false
	}

	resp, err := l.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// EnsureRunning starts Voice Box if not already running
// This is the primary entry point - call this before making TTS requests
func (l *VoiceBoxLauncher) EnsureRunning(ctx context.Context) error {
	// Fast path: already healthy
	if l.IsHealthy() {
		return nil
	}

	// Check if installed
	if !l.IsInstalled() {
		return fmt.Errorf("Voice Box not installed. Run: cortex voice install")
	}

	// Try to start
	return l.Start(ctx)
}

// Start launches the Voice Box server
func (l *VoiceBoxLauncher) Start(ctx context.Context) error {
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
		log.Info().Str("script", startScript).Msg("starting Voice Box via script")

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
		log.Info().Str("endpoint", l.Endpoint()).Msg("Voice Box started successfully")
		return nil
	}

	// Fallback: direct Python invocation
	return l.startDirect(ctx)
}

// startDirect launches Python directly without shell script
func (l *VoiceBoxLauncher) startDirect(ctx context.Context) error {
	pythonBin := filepath.Join(l.config.InstallDir, "bin", "python")
	serverScript := filepath.Join(l.config.InstallDir, "server.py")

	log.Info().
		Str("python", pythonBin).
		Str("server", serverScript).
		Msg("starting Voice Box directly")

	l.cmd = exec.Command(pythonBin, serverScript)
	l.cmd.Dir = l.config.InstallDir
	l.cmd.Env = append(os.Environ(),
		fmt.Sprintf("VOICEBOX_HOST=%s", l.config.Host),
		fmt.Sprintf("VOICEBOX_PORT=%d", l.config.Port),
	)

	// Redirect output to log file
	logFile := filepath.Join(l.config.InstallDir, "voicebox.log")
	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		l.cmd.Stdout = f
		l.cmd.Stderr = f
	}

	if err := l.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start Voice Box: %w", err)
	}

	// Write PID file
	pidFile := filepath.Join(l.config.InstallDir, "voicebox.pid")
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
		Msg("Voice Box started successfully")
	return nil
}

// waitForHealthy polls the health endpoint until success or timeout
func (l *VoiceBoxLauncher) waitForHealthy(ctx context.Context) error {
	deadline := time.Now().Add(l.config.StartupTimeout)
	ticker := time.NewTicker(250 * time.Millisecond)
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
				return fmt.Errorf("Voice Box failed to become healthy within %v", l.config.StartupTimeout)
			}
		}
	}
}

// Stop gracefully shuts down Voice Box
func (l *VoiceBoxLauncher) Stop() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Try stop script first
	stopScript := filepath.Join(l.config.InstallDir, "stop.sh")
	if _, err := os.Stat(stopScript); err == nil {
		cmd := exec.Command("bash", stopScript)
		cmd.Dir = l.config.InstallDir
		if err := cmd.Run(); err == nil {
			l.running = false
			log.Info().Msg("Voice Box stopped via script")
			return nil
		}
	}

	// Kill via PID file
	if l.killViaPIDFile() {
		l.running = false
		log.Info().Msg("Voice Box stopped via PID file")
		return nil
	}

	// Kill direct process
	if l.cmd != nil && l.cmd.Process != nil {
		if err := l.cmd.Process.Kill(); err == nil {
			l.running = false
			log.Info().Msg("Voice Box process killed")
			return nil
		}
	}

	return fmt.Errorf("Voice Box not running")
}

// isRunningViaPIDFile checks if process from PID file is alive
func (l *VoiceBoxLauncher) isRunningViaPIDFile() bool {
	pidFile := filepath.Join(l.config.InstallDir, "voicebox.pid")
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
	// to check if the process exists
	err = process.Signal(os.Signal(nil))
	return err == nil
}

// killViaPIDFile kills the process from PID file
func (l *VoiceBoxLauncher) killViaPIDFile() bool {
	pidFile := filepath.Join(l.config.InstallDir, "voicebox.pid")
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

// GetVoices fetches available voices from Voice Box
func (l *VoiceBoxLauncher) GetVoices(ctx context.Context) ([]VoiceBoxVoiceInfo, error) {
	if err := l.EnsureRunning(ctx); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "GET", l.Endpoint()+"/v1/audio/voices", nil)
	if err != nil {
		return nil, err
	}

	resp, err := l.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get voices: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get voices failed: %s", string(body))
	}

	var result struct {
		Voices []VoiceBoxVoiceInfo `json:"voices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode voices response: %w", err)
	}

	return result.Voices, nil
}

// VoiceBoxVoiceInfo represents a voice option from Voice Box
type VoiceBoxVoiceInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Gender      string `json:"gender"`
	Accent      string `json:"accent"`
	Description string `json:"description,omitempty"`
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status  string `json:"status"`
	Model   string `json:"model"`
	Version string `json:"version"`
}

// GetHealth returns detailed health information
func (l *VoiceBoxLauncher) GetHealth(ctx context.Context) (*HealthResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", l.Endpoint()+"/health", nil)
	if err != nil {
		return nil, err
	}

	resp, err := l.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("health check returned status %d", resp.StatusCode)
	}

	var health HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return nil, fmt.Errorf("failed to decode health response: %w", err)
	}

	return &health, nil
}

// Config returns the launcher configuration
func (l *VoiceBoxLauncher) Config() VoiceBoxConfig {
	return l.config
}

// ══════════════════════════════════════════════════════════════════════════════
// Global launcher singleton for convenience
// ══════════════════════════════════════════════════════════════════════════════

var (
	globalLauncher     *VoiceBoxLauncher
	globalLauncherOnce sync.Once
)

// GetVoiceBoxLauncher returns a global launcher instance
func GetVoiceBoxLauncher() *VoiceBoxLauncher {
	globalLauncherOnce.Do(func() {
		globalLauncher = NewVoiceBoxLauncher(DefaultVoiceBoxConfig())
	})
	return globalLauncher
}
