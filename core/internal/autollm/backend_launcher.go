package autollm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/normanking/cortex/internal/logging"
)

// ═══════════════════════════════════════════════════════════════════════════════
// BACKEND LAUNCHER - Auto-starts the best available LLM backend
// ═══════════════════════════════════════════════════════════════════════════════

// BackendLauncherConfig holds configuration for backend auto-start.
type BackendLauncherConfig struct {
	// Endpoints for each backend
	MLXEndpoint    string
	OllamaEndpoint string
	DnetEndpoint   string

	// Default model for MLX (if no model specified)
	MLXDefaultModel string

	// Timeouts
	StartupTimeout time.Duration
	HealthTimeout  time.Duration
}

// DefaultBackendLauncherConfig returns sensible defaults.
func DefaultBackendLauncherConfig() BackendLauncherConfig {
	return BackendLauncherConfig{
		MLXEndpoint:     "http://127.0.0.1:8081",
		OllamaEndpoint:  "http://127.0.0.1:11434",
		DnetEndpoint:    "http://127.0.0.1:9080",
		MLXDefaultModel: "mlx-community/Llama-3.2-3B-Instruct-4bit",
		StartupTimeout:  60 * time.Second, // MLX model loading can take time
		HealthTimeout:   3 * time.Second,
	}
}

// BackendLauncher manages auto-starting LLM backends.
// Priority order: MLX (fastest on Apple Silicon) > Ollama > dnet
type BackendLauncher struct {
	config     BackendLauncherConfig
	httpClient *http.Client
	log        *logging.Logger

	// Process management
	cmd     *exec.Cmd
	running BackendType
	mu      sync.RWMutex
}

// NewBackendLauncher creates a new backend launcher.
func NewBackendLauncher(config BackendLauncherConfig) *BackendLauncher {
	return &BackendLauncher{
		config: config,
		httpClient: &http.Client{
			Timeout: config.HealthTimeout,
		},
		log: logging.Global(),
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// INSTALLATION CHECKS
// ═══════════════════════════════════════════════════════════════════════════════

// IsMLXInstalled checks if mlx-lm is installed and functional.
func (l *BackendLauncher) IsMLXInstalled() bool {
	// Check if mlx_lm.server command exists
	_, err := exec.LookPath("mlx_lm.server")
	if err != nil {
		// Also check common pip install locations
		home, _ := os.UserHomeDir()
		paths := []string{
			"/Library/Frameworks/Python.framework/Versions/3.11/bin/mlx_lm.server",
			"/Library/Frameworks/Python.framework/Versions/3.12/bin/mlx_lm.server",
			filepath.Join(home, ".local/bin/mlx_lm.server"),
			"/opt/homebrew/bin/mlx_lm.server",
			"/usr/local/bin/mlx_lm.server",
		}
		for _, p := range paths {
			if _, err := os.Stat(p); err == nil {
				return true
			}
		}
		return false
	}
	return true
}

// GetMLXServerPath returns the path to mlx_lm.server executable.
func (l *BackendLauncher) GetMLXServerPath() string {
	if path, err := exec.LookPath("mlx_lm.server"); err == nil {
		return path
	}
	home, _ := os.UserHomeDir()
	paths := []string{
		"/Library/Frameworks/Python.framework/Versions/3.11/bin/mlx_lm.server",
		"/Library/Frameworks/Python.framework/Versions/3.12/bin/mlx_lm.server",
		filepath.Join(home, ".local/bin/mlx_lm.server"),
		"/opt/homebrew/bin/mlx_lm.server",
		"/usr/local/bin/mlx_lm.server",
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// IsOllamaInstalled checks if Ollama is installed.
func (l *BackendLauncher) IsOllamaInstalled() bool {
	_, err := exec.LookPath("ollama")
	return err == nil
}

// IsDnetInstalled checks if dnet is installed.
func (l *BackendLauncher) IsDnetInstalled() bool {
	// Check for dnet-api or dnet command
	if _, err := exec.LookPath("dnet-api"); err == nil {
		return true
	}
	if _, err := exec.LookPath("dnet"); err == nil {
		return true
	}
	// Check if it's a Python package
	cmd := exec.Command("python3", "-c", "import dnet; print('ok')")
	return cmd.Run() == nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// HEALTH CHECKS
// ═══════════════════════════════════════════════════════════════════════════════

// IsMLXHealthy checks if MLX server is running and responding.
func (l *BackendLauncher) IsMLXHealthy() bool {
	ctx, cancel := context.WithTimeout(context.Background(), l.config.HealthTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", l.config.MLXEndpoint+"/v1/models", nil)
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

// IsOllamaHealthy checks if Ollama is running and has models.
func (l *BackendLauncher) IsOllamaHealthy() bool {
	ctx, cancel := context.WithTimeout(context.Background(), l.config.HealthTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", l.config.OllamaEndpoint+"/api/tags", nil)
	if err != nil {
		return false
	}

	resp, err := l.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false
	}

	// Check if Ollama has at least one model
	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false
	}

	return len(result.Models) > 0
}

// IsDnetHealthy checks if dnet is running.
func (l *BackendLauncher) IsDnetHealthy() bool {
	ctx, cancel := context.WithTimeout(context.Background(), l.config.HealthTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", l.config.DnetEndpoint+"/v1/models", nil)
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

// ═══════════════════════════════════════════════════════════════════════════════
// AUTO-START LOGIC
// ═══════════════════════════════════════════════════════════════════════════════

// EnsureBackendRunning checks if any backend is running, and if not, starts the best available one.
// Returns the backend type that is running and its endpoint.
func (l *BackendLauncher) EnsureBackendRunning(ctx context.Context) (*BackendInfo, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// First, check if any backend is already running
	if l.IsMLXHealthy() {
		l.log.Info("[BackendLauncher] MLX already running at %s", l.config.MLXEndpoint)
		return &BackendInfo{
			Type:      BackendMLX,
			Endpoint:  l.config.MLXEndpoint,
			Available: true,
		}, nil
	}

	if l.IsOllamaHealthy() {
		l.log.Info("[BackendLauncher] Ollama already running at %s", l.config.OllamaEndpoint)
		models := l.getOllamaModels()
		return &BackendInfo{
			Type:      BackendOllama,
			Endpoint:  l.config.OllamaEndpoint,
			Available: true,
			Models:    models,
		}, nil
	}

	if l.IsDnetHealthy() {
		l.log.Info("[BackendLauncher] dnet already running at %s", l.config.DnetEndpoint)
		return &BackendInfo{
			Type:      BackendDnet,
			Endpoint:  l.config.DnetEndpoint,
			Available: true,
		}, nil
	}

	// No backend running - try to start one
	// Priority: MLX > Ollama > dnet (MLX is 5-10x faster on Apple Silicon)

	if l.IsMLXInstalled() {
		l.log.Info("[BackendLauncher] Starting MLX server (fastest on Apple Silicon)...")
		if err := l.startMLX(ctx); err != nil {
			l.log.Warn("[BackendLauncher] Failed to start MLX: %v", err)
		} else {
			return &BackendInfo{
				Type:      BackendMLX,
				Endpoint:  l.config.MLXEndpoint,
				Available: true,
			}, nil
		}
	}

	if l.IsOllamaInstalled() {
		l.log.Info("[BackendLauncher] Starting Ollama...")
		if err := l.startOllama(ctx); err != nil {
			l.log.Warn("[BackendLauncher] Failed to start Ollama: %v", err)
		} else {
			models := l.getOllamaModels()
			return &BackendInfo{
				Type:      BackendOllama,
				Endpoint:  l.config.OllamaEndpoint,
				Available: true,
				Models:    models,
			}, nil
		}
	}

	if l.IsDnetInstalled() {
		l.log.Info("[BackendLauncher] Starting dnet...")
		if err := l.startDnet(ctx); err != nil {
			l.log.Warn("[BackendLauncher] Failed to start dnet: %v", err)
		} else {
			return &BackendInfo{
				Type:      BackendDnet,
				Endpoint:  l.config.DnetEndpoint,
				Available: true,
			}, nil
		}
	}

	return nil, fmt.Errorf("no LLM backend available - install ollama, mlx-lm, or dnet")
}

// ═══════════════════════════════════════════════════════════════════════════════
// BACKEND STARTERS
// ═══════════════════════════════════════════════════════════════════════════════

// startMLX starts the mlx-lm server.
func (l *BackendLauncher) startMLX(ctx context.Context) error {
	mlxPath := l.GetMLXServerPath()
	if mlxPath == "" {
		return fmt.Errorf("mlx_lm.server not found")
	}

	// Extract port from endpoint
	port := "8081"
	if strings.Contains(l.config.MLXEndpoint, ":") {
		parts := strings.Split(l.config.MLXEndpoint, ":")
		port = parts[len(parts)-1]
	}

	// Start mlx_lm.server with default model
	l.cmd = exec.Command(mlxPath,
		"--model", l.config.MLXDefaultModel,
		"--port", port,
	)

	// Redirect output to log file
	home, _ := os.UserHomeDir()
	logFile, err := os.OpenFile(
		filepath.Join(home, ".cortex", "logs", "mlx_server.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0644,
	)
	if err == nil {
		l.cmd.Stdout = logFile
		l.cmd.Stderr = logFile
	}

	l.log.Info("[BackendLauncher] Launching: %s --model %s --port %s",
		mlxPath, l.config.MLXDefaultModel, port)

	if err := l.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start mlx_lm.server: %w", err)
	}

	l.running = BackendMLX

	// Wait for server to be ready
	return l.waitForBackend(ctx, l.IsMLXHealthy, "MLX")
}

// startOllama starts the Ollama server.
func (l *BackendLauncher) startOllama(ctx context.Context) error {
	ollamaPath, err := exec.LookPath("ollama")
	if err != nil {
		return fmt.Errorf("ollama not found: %w", err)
	}

	// Start ollama serve
	l.cmd = exec.Command(ollamaPath, "serve")

	// Redirect output to log file
	home, _ := os.UserHomeDir()
	logFile, err := os.OpenFile(
		filepath.Join(home, ".cortex", "logs", "ollama_server.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0644,
	)
	if err == nil {
		l.cmd.Stdout = logFile
		l.cmd.Stderr = logFile
	}

	l.log.Info("[BackendLauncher] Launching: ollama serve")

	if err := l.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start ollama: %w", err)
	}

	l.running = BackendOllama

	// Wait for server to be ready
	if err := l.waitForBackend(ctx, l.isOllamaResponding, "Ollama"); err != nil {
		return err
	}

	// Check if Ollama has models
	if !l.IsOllamaHealthy() {
		l.log.Warn("[BackendLauncher] Ollama running but has no models. Run 'ollama pull <model>' to add one.")
	}

	return nil
}

// isOllamaResponding checks if Ollama is responding (even without models).
func (l *BackendLauncher) isOllamaResponding() bool {
	ctx, cancel := context.WithTimeout(context.Background(), l.config.HealthTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", l.config.OllamaEndpoint+"/api/tags", nil)
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

// startDnet starts the dnet server.
func (l *BackendLauncher) startDnet(ctx context.Context) error {
	// Try dnet-api first, then dnet
	var cmdPath string
	if path, err := exec.LookPath("dnet-api"); err == nil {
		cmdPath = path
	} else if path, err := exec.LookPath("dnet"); err == nil {
		cmdPath = path
	} else {
		return fmt.Errorf("dnet not found")
	}

	// Extract port from endpoint
	port := "9080"
	if strings.Contains(l.config.DnetEndpoint, ":") {
		parts := strings.Split(l.config.DnetEndpoint, ":")
		port = parts[len(parts)-1]
	}

	l.cmd = exec.Command(cmdPath, "--http-port", port)

	// Redirect output to log file
	home, _ := os.UserHomeDir()
	logFile, err := os.OpenFile(
		filepath.Join(home, ".cortex", "logs", "dnet_server.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0644,
	)
	if err == nil {
		l.cmd.Stdout = logFile
		l.cmd.Stderr = logFile
	}

	l.log.Info("[BackendLauncher] Launching: %s --http-port %s", cmdPath, port)

	if err := l.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start dnet: %w", err)
	}

	l.running = BackendDnet

	// Wait for server to be ready
	return l.waitForBackend(ctx, l.IsDnetHealthy, "dnet")
}

// waitForBackend waits for a backend to become healthy.
func (l *BackendLauncher) waitForBackend(ctx context.Context, healthCheck func() bool, name string) error {
	deadline := time.Now().Add(l.config.StartupTimeout)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if healthCheck() {
			l.log.Info("[BackendLauncher] %s is ready", name)
			return nil
		}

		l.log.Debug("[BackendLauncher] Waiting for %s to start...", name)
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("%s failed to start within %v", name, l.config.StartupTimeout)
}

// ═══════════════════════════════════════════════════════════════════════════════
// HELPERS
// ═══════════════════════════════════════════════════════════════════════════════

// getOllamaModels returns the list of models available in Ollama.
func (l *BackendLauncher) getOllamaModels() []string {
	ctx, cancel := context.WithTimeout(context.Background(), l.config.HealthTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", l.config.OllamaEndpoint+"/api/tags", nil)
	if err != nil {
		return nil
	}

	resp, err := l.httpClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil
	}

	models := make([]string, len(result.Models))
	for i, m := range result.Models {
		models[i] = m.Name
	}
	return models
}

// Stop stops any running backend that was started by this launcher.
func (l *BackendLauncher) Stop() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.cmd != nil && l.cmd.Process != nil {
		l.log.Info("[BackendLauncher] Stopping %s", l.running)
		if err := l.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to stop %s: %w", l.running, err)
		}
		l.cmd = nil
		l.running = BackendNone
	}

	return nil
}

// Running returns the currently running backend type.
func (l *BackendLauncher) Running() BackendType {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.running
}

// Status returns a summary of backend availability.
func (l *BackendLauncher) Status() map[string]interface{} {
	return map[string]interface{}{
		"mlx_installed":    l.IsMLXInstalled(),
		"mlx_healthy":      l.IsMLXHealthy(),
		"ollama_installed": l.IsOllamaInstalled(),
		"ollama_healthy":   l.IsOllamaHealthy(),
		"dnet_installed":   l.IsDnetInstalled(),
		"dnet_healthy":     l.IsDnetHealthy(),
		"running":          string(l.Running()),
	}
}
