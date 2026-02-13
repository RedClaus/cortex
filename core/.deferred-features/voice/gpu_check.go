package voice

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// AcceleratorType represents the type of ML acceleration available.
type AcceleratorType string

const (
	AcceleratorNVIDIA       AcceleratorType = "nvidia"        // NVIDIA CUDA GPU
	AcceleratorAppleSilicon AcceleratorType = "apple_silicon" // Apple M1/M2/M3/M4
	AcceleratorMetal        AcceleratorType = "metal"         // Apple Metal (Intel Mac)
	AcceleratorCPU          AcceleratorType = "cpu"           // CPU only
)

// GPUStatus contains GPU availability and metadata.
type GPUStatus struct {
	Available     bool            `json:"available"`
	DriverVersion string          `json:"driver_version,omitempty"`
	DeviceName    string          `json:"device_name,omitempty"`
	Memory        string          `json:"memory,omitempty"`
	Error         string          `json:"error,omitempty"`
	CheckedAt     time.Time       `json:"checked_at"`
	Accelerator   AcceleratorType `json:"accelerator,omitempty"`
}

// MLXStatus contains Apple Silicon MLX availability information.
type MLXStatus struct {
	Available    bool      `json:"available"`
	ChipName     string    `json:"chip_name,omitempty"`
	MLXInstalled bool      `json:"mlx_installed"`
	MetalSupport bool      `json:"metal_support"`
	NeuralEngine bool      `json:"neural_engine"`
	Error        string    `json:"error,omitempty"`
	CheckedAt    time.Time `json:"checked_at"`
}

// GPUChecker provides cached GPU availability checks.
type GPUChecker struct {
	mu           sync.RWMutex
	cachedStatus *GPUStatus
	cacheTTL     time.Duration
}

// NewGPUChecker creates a new GPU checker with 5-minute cache.
func NewGPUChecker() *GPUChecker {
	return &GPUChecker{
		cacheTTL: 5 * time.Minute,
	}
}

// IsGPUAvailable returns cached GPU status if available, otherwise performs check.
func (c *GPUChecker) IsGPUAvailable(ctx context.Context) (*GPUStatus, error) {
	// Check cache first
	c.mu.RLock()
	if c.cachedStatus != nil && time.Since(c.cachedStatus.CheckedAt) < c.cacheTTL {
		cached := c.cachedStatus
		c.mu.RUnlock()
		log.Debug().
			Bool("available", cached.Available).
			Str("device", cached.DeviceName).
			Msg("using cached GPU status")
		return cached, nil
	}
	c.mu.RUnlock()

	// Perform fresh check
	status, err := CheckGPUAvailability(ctx)
	if err != nil {
		return status, err
	}

	// Update cache
	c.mu.Lock()
	c.cachedStatus = status
	c.mu.Unlock()

	return status, nil
}

// InvalidateCache clears the cached GPU status.
func (c *GPUChecker) InvalidateCache() {
	c.mu.Lock()
	c.cachedStatus = nil
	c.mu.Unlock()
	log.Debug().Msg("GPU status cache invalidated")
}

// CheckGPUAvailability performs pre-flight GPU detection using multiple methods.
// This runs BEFORE attempting to start the XTTS container.
func CheckGPUAvailability(ctx context.Context) (*GPUStatus, error) {
	log.Debug().Msg("checking GPU availability")

	status := &GPUStatus{
		CheckedAt: time.Now(),
	}

	// Method 1: Try nvidia-smi directly
	if gpuInfo, err := checkNvidiaSMI(ctx); err == nil {
		status.Available = true
		status.DriverVersion = gpuInfo.driverVersion
		status.DeviceName = gpuInfo.deviceName
		status.Memory = gpuInfo.memory
		log.Info().
			Str("driver", status.DriverVersion).
			Str("device", status.DeviceName).
			Str("memory", status.Memory).
			Msg("GPU detected via nvidia-smi")
		return status, nil
	}

	// Method 2: Try via Docker container
	if gpuInfo, err := checkGPUViaDocker(ctx); err == nil {
		status.Available = true
		status.DriverVersion = gpuInfo.driverVersion
		status.DeviceName = gpuInfo.deviceName
		status.Memory = gpuInfo.memory
		log.Info().
			Str("driver", status.DriverVersion).
			Str("device", status.DeviceName).
			Msg("GPU detected via Docker")
		return status, nil
	}

	// Method 3: Check /dev/nvidia0 device file (Linux)
	if checkNvidiaDeviceFile() {
		status.Available = true
		status.DeviceName = "NVIDIA GPU (device file detected)"
		log.Info().Msg("GPU detected via /dev/nvidia0")
		return status, nil
	}

	// No GPU detected
	status.Available = false
	status.Error = "no GPU detected via nvidia-smi, Docker, or device files"
	log.Warn().Msg(status.Error)
	return status, nil
}

// gpuInfo contains parsed GPU information.
type gpuInfo struct {
	driverVersion string
	deviceName    string
	memory        string
}

// checkNvidiaSMI attempts to query nvidia-smi for GPU information.
func checkNvidiaSMI(ctx context.Context) (*gpuInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "nvidia-smi", "--query-gpu=driver_version,name,memory.total", "--format=csv,noheader")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		log.Debug().
			Err(err).
			Str("stderr", stderr.String()).
			Msg("nvidia-smi check failed")
		return nil, fmt.Errorf("nvidia-smi failed: %w", err)
	}

	// Parse output: "driver_version, device_name, memory"
	output := strings.TrimSpace(stdout.String())
	if output == "" {
		return nil, fmt.Errorf("nvidia-smi returned empty output")
	}

	parts := strings.Split(output, ",")
	if len(parts) < 3 {
		return nil, fmt.Errorf("unexpected nvidia-smi output format: %s", output)
	}

	return &gpuInfo{
		driverVersion: strings.TrimSpace(parts[0]),
		deviceName:    strings.TrimSpace(parts[1]),
		memory:        strings.TrimSpace(parts[2]),
	}, nil
}

// checkGPUViaDocker attempts to detect GPU via Docker nvidia runtime.
func checkGPUViaDocker(ctx context.Context) (*gpuInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Check if Docker is available
	if _, err := exec.LookPath("docker"); err != nil {
		return nil, fmt.Errorf("docker not found: %w", err)
	}

	// Try to run nvidia-smi in a Docker container with GPU support
	cmd := exec.CommandContext(ctx, "docker", "run", "--rm",
		"--gpus", "all",
		"nvidia/cuda:12.0.0-base-ubuntu22.04",
		"nvidia-smi", "--query-gpu=driver_version,name,memory.total", "--format=csv,noheader")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		log.Debug().
			Err(err).
			Str("stderr", stderr.String()).
			Msg("Docker GPU check failed")
		return nil, fmt.Errorf("docker GPU check failed: %w", err)
	}

	// Parse output: "driver_version, device_name, memory"
	output := strings.TrimSpace(stdout.String())
	if output == "" {
		return nil, fmt.Errorf("docker nvidia-smi returned empty output")
	}

	parts := strings.Split(output, ",")
	if len(parts) < 3 {
		return nil, fmt.Errorf("unexpected docker nvidia-smi output format: %s", output)
	}

	return &gpuInfo{
		driverVersion: strings.TrimSpace(parts[0]),
		deviceName:    strings.TrimSpace(parts[1]),
		memory:        strings.TrimSpace(parts[2]),
	}, nil
}

// checkNvidiaDeviceFile checks if /dev/nvidia0 exists (Linux only).
func checkNvidiaDeviceFile() bool {
	if _, err := os.Stat("/dev/nvidia0"); err == nil {
		return true
	}
	return false
}

// ============================================================================
// Apple Silicon / MLX Detection (CR-012-A)
// ============================================================================

// IsAppleSilicon returns true if running on Apple Silicon (M1/M2/M3/M4).
func IsAppleSilicon() bool {
	return runtime.GOOS == "darwin" && runtime.GOARCH == "arm64"
}

// CheckMLXAvailability checks if MLX is available for Apple Silicon acceleration.
// Returns detailed status including chip info and MLX installation state.
func CheckMLXAvailability(ctx context.Context) (*MLXStatus, error) {
	log.Debug().Msg("checking MLX availability")

	status := &MLXStatus{
		CheckedAt: time.Now(),
	}

	// First check: Must be Apple Silicon
	if !IsAppleSilicon() {
		status.Available = false
		if runtime.GOOS == "darwin" {
			status.Error = "MLX requires Apple Silicon (M1/M2/M3/M4). Intel Mac detected."
		} else {
			status.Error = fmt.Sprintf("MLX only available on macOS Apple Silicon. Current: %s/%s", runtime.GOOS, runtime.GOARCH)
		}
		log.Debug().Str("error", status.Error).Msg("MLX not available")
		return status, nil
	}

	// Get chip name
	status.ChipName = getAppleChipName(ctx)
	status.MetalSupport = true // All Apple Silicon supports Metal
	status.NeuralEngine = true // All Apple Silicon has Neural Engine

	// Check if MLX Python package is installed
	status.MLXInstalled = checkMLXInstalled(ctx)

	// MLX is available if we're on Apple Silicon with MLX installed
	status.Available = status.MLXInstalled

	if status.Available {
		log.Info().
			Str("chip", status.ChipName).
			Bool("mlx", status.MLXInstalled).
			Msg("MLX available on Apple Silicon")
	} else {
		status.Error = "MLX Python package not installed. Run: pip install mlx mlx-whisper"
		log.Warn().
			Str("chip", status.ChipName).
			Str("error", status.Error).
			Msg("Apple Silicon detected but MLX not installed")
	}

	return status, nil
}

// getAppleChipName returns the Apple Silicon chip name (e.g., "Apple M1 Pro").
func getAppleChipName(ctx context.Context) string {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sysctl", "-n", "machdep.cpu.brand_string")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		log.Debug().Err(err).Msg("failed to get Apple chip name")
		return "Apple Silicon"
	}

	chipName := strings.TrimSpace(stdout.String())
	if chipName == "" {
		return "Apple Silicon"
	}
	return chipName
}

// checkMLXInstalled checks if the MLX Python package is available.
func checkMLXInstalled(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Try to import mlx in Python
	cmd := exec.CommandContext(ctx, "python3", "-c", "import mlx; print('ok')")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		log.Debug().
			Err(err).
			Str("stderr", stderr.String()).
			Msg("MLX Python import failed")
		return false
	}

	return strings.TrimSpace(stdout.String()) == "ok"
}

// checkMLXWhisperInstalled checks if mlx-whisper is available.
func checkMLXWhisperInstalled(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "python3", "-c", "import mlx_whisper; print('ok')")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		log.Debug().
			Err(err).
			Str("stderr", stderr.String()).
			Msg("mlx-whisper Python import failed")
		return false
	}

	return strings.TrimSpace(stdout.String()) == "ok"
}

// GetOptimalBackend returns the recommended TTS/STT backend for the current platform.
// Returns: backend name, model recommendation, and whether MLX should be used.
func GetOptimalBackend(ctx context.Context) (backend string, sttModel string, useMLX bool) {
	// Check for Apple Silicon with MLX first (optimal path)
	if IsAppleSilicon() {
		mlxStatus, _ := CheckMLXAvailability(ctx)
		if mlxStatus != nil && mlxStatus.Available {
			// Check if mlx-whisper is also installed for STT
			if checkMLXWhisperInstalled(ctx) {
				log.Info().
					Str("chip", mlxStatus.ChipName).
					Msg("using MLX backend for Apple Silicon")
				return "mlx", "mlx-community/whisper-large-v3-turbo", true
			}
			// MLX available but not mlx-whisper - use MLX for TTS, fallback for STT
			log.Info().
				Str("chip", mlxStatus.ChipName).
				Msg("using MLX for TTS, whisper.cpp for STT")
			return "mlx", "large-v3-turbo", true
		}
		// Apple Silicon without MLX - use CPU/whisper.cpp
		log.Info().Msg("Apple Silicon without MLX - using CPU backend")
		return "cpu", "large-v3-turbo", false
	}

	// Check for NVIDIA GPU (Linux/Windows)
	gpuStatus, _ := CheckGPUAvailability(ctx)
	if gpuStatus != nil && gpuStatus.Available {
		log.Info().
			Str("gpu", gpuStatus.DeviceName).
			Msg("using CUDA backend for NVIDIA GPU")
		return "cuda", "large-v3-turbo", false
	}

	// Fallback to CPU
	log.Info().Msg("no GPU acceleration available - using CPU backend")
	return "cpu", "base", false
}

// MLXChecker provides cached MLX availability checks.
type MLXChecker struct {
	mu           sync.RWMutex
	cachedStatus *MLXStatus
	cacheTTL     time.Duration
}

// NewMLXChecker creates a new MLX checker with 5-minute cache.
func NewMLXChecker() *MLXChecker {
	return &MLXChecker{
		cacheTTL: 5 * time.Minute,
	}
}

// IsMLXAvailable returns cached MLX status if available, otherwise performs check.
func (c *MLXChecker) IsMLXAvailable(ctx context.Context) (*MLXStatus, error) {
	// Check cache first
	c.mu.RLock()
	if c.cachedStatus != nil && time.Since(c.cachedStatus.CheckedAt) < c.cacheTTL {
		cached := c.cachedStatus
		c.mu.RUnlock()
		log.Debug().
			Bool("available", cached.Available).
			Str("chip", cached.ChipName).
			Msg("using cached MLX status")
		return cached, nil
	}
	c.mu.RUnlock()

	// Perform fresh check
	status, err := CheckMLXAvailability(ctx)
	if err != nil {
		return status, err
	}

	// Update cache
	c.mu.Lock()
	c.cachedStatus = status
	c.mu.Unlock()

	return status, nil
}

// InvalidateCache clears the cached MLX status.
func (c *MLXChecker) InvalidateCache() {
	c.mu.Lock()
	c.cachedStatus = nil
	c.mu.Unlock()
	log.Debug().Msg("MLX status cache invalidated")
}
