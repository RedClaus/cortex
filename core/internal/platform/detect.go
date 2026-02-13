// Package platform provides system platform detection for Cortex.
// It detects the operating system, architecture, and available ML acceleration.
package platform

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

// Platform represents the detected system platform.
type Platform string

const (
	PlatformAppleSilicon Platform = "apple_silicon" // macOS on M1/M2/M3/M4
	PlatformMacOSIntel   Platform = "macos_intel"   // macOS on Intel
	PlatformLinuxCUDA    Platform = "linux_cuda"    // Linux with NVIDIA GPU
	PlatformLinuxCPU     Platform = "linux_cpu"     // Linux without GPU
	PlatformWindows      Platform = "windows"       // Windows (limited support)
	PlatformUnknown      Platform = "unknown"       // Unknown platform
)

// MLBackend represents the ML acceleration backend to use.
type MLBackend string

const (
	BackendMLX        MLBackend = "mlx"         // Apple MLX (Apple Silicon only)
	BackendCUDA       MLBackend = "cuda"        // NVIDIA CUDA
	BackendMetal      MLBackend = "metal"       // Apple Metal (Intel Mac)
	BackendCPU        MLBackend = "cpu"         // CPU fallback
	BackendWhisperCpp MLBackend = "whisper_cpp" // Whisper.cpp (cross-platform)
)

// PlatformInfo contains detailed platform detection results.
type PlatformInfo struct {
	// Core platform identification
	Platform Platform `json:"platform"`
	OS       string   `json:"os"`   // darwin, linux, windows
	Arch     string   `json:"arch"` // arm64, amd64

	// Apple Silicon specific
	IsAppleSilicon bool   `json:"is_apple_silicon"`
	ChipName       string `json:"chip_name,omitempty"` // e.g., "Apple M1 Pro"

	// ML Acceleration
	MLXAvailable   bool `json:"mlx_available"`
	CUDAAvailable  bool `json:"cuda_available"`
	MetalAvailable bool `json:"metal_available"`

	// System Resources
	TotalRAMBytes int64   `json:"total_ram_bytes"` // Total system RAM in bytes
	TotalRAMGB    float64 `json:"total_ram_gb"`    // Total system RAM in GB
	MaxModelGB    float64 `json:"max_model_gb"`    // Maximum recommended model size

	// Recommended backends
	TTSBackend MLBackend `json:"tts_backend"`
	STTBackend MLBackend `json:"stt_backend"`
	STTModel   string    `json:"stt_model"`

	// Detection metadata
	DetectedAt time.Time `json:"detected_at"`
	Error      string    `json:"error,omitempty"`
}

// String returns a human-readable description of the platform.
func (p *PlatformInfo) String() string {
	if p.IsAppleSilicon {
		return fmt.Sprintf("Apple Silicon (%s) - MLX: %v", p.ChipName, p.MLXAvailable)
	}
	return fmt.Sprintf("%s/%s - CUDA: %v, Metal: %v", p.OS, p.Arch, p.CUDAAvailable, p.MetalAvailable)
}

// SupportsMLX returns true if the platform can use MLX acceleration.
func (p *PlatformInfo) SupportsMLX() bool {
	return p.IsAppleSilicon && p.MLXAvailable
}

// SupportsCUDA returns true if the platform has NVIDIA GPU support.
func (p *PlatformInfo) SupportsCUDA() bool {
	return p.CUDAAvailable
}

// Detector provides cached platform detection.
type Detector struct {
	mu       sync.RWMutex
	cached   *PlatformInfo
	cacheTTL time.Duration
}

// NewDetector creates a new platform detector with default 10-minute cache.
func NewDetector() *Detector {
	return &Detector{
		cacheTTL: 10 * time.Minute,
	}
}

var (
	globalDetector     *Detector
	globalDetectorOnce sync.Once
)

// GetDetector returns the global platform detector singleton.
func GetDetector() *Detector {
	globalDetectorOnce.Do(func() {
		globalDetector = NewDetector()
	})
	return globalDetector
}

// Detect performs platform detection, using cache if available.
func (d *Detector) Detect(ctx context.Context) (*PlatformInfo, error) {
	// Check cache
	d.mu.RLock()
	if d.cached != nil && time.Since(d.cached.DetectedAt) < d.cacheTTL {
		cached := d.cached
		d.mu.RUnlock()
		log.Debug().
			Str("platform", string(cached.Platform)).
			Bool("mlx", cached.MLXAvailable).
			Msg("using cached platform info")
		return cached, nil
	}
	d.mu.RUnlock()

	// Perform fresh detection
	info, err := DetectPlatform(ctx)
	if err != nil {
		return info, err
	}

	// Update cache
	d.mu.Lock()
	d.cached = info
	d.mu.Unlock()

	return info, nil
}

// InvalidateCache clears the cached platform info.
func (d *Detector) InvalidateCache() {
	d.mu.Lock()
	d.cached = nil
	d.mu.Unlock()
	log.Debug().Msg("platform cache invalidated")
}

// DetectPlatform performs comprehensive platform detection.
func DetectPlatform(ctx context.Context) (*PlatformInfo, error) {
	log.Debug().Msg("detecting platform capabilities")

	info := &PlatformInfo{
		OS:         runtime.GOOS,
		Arch:       runtime.GOARCH,
		DetectedAt: time.Now(),
	}

	// Determine base platform
	switch runtime.GOOS {
	case "darwin":
		detectDarwin(ctx, info)
	case "linux":
		detectLinux(ctx, info)
	case "windows":
		info.Platform = PlatformWindows
		info.TTSBackend = BackendCPU
		info.STTBackend = BackendWhisperCpp
		info.STTModel = "base"
	default:
		info.Platform = PlatformUnknown
		info.TTSBackend = BackendCPU
		info.STTBackend = BackendCPU
		info.STTModel = "base"
	}

	if ramBytes, err := GetSystemRAM(ctx); err == nil {
		const GB = 1024 * 1024 * 1024
		info.TotalRAMBytes = ramBytes
		info.TotalRAMGB = float64(ramBytes) / float64(GB)
		info.MaxModelGB = GetMaxModelSizeGB(ramBytes)
	} else {
		log.Debug().Err(err).Msg("failed to detect system RAM, using safe defaults")
		info.TotalRAMGB = 8.0
		info.MaxModelGB = 5.0
	}

	log.Info().
		Str("platform", string(info.Platform)).
		Bool("apple_silicon", info.IsAppleSilicon).
		Bool("mlx", info.MLXAvailable).
		Bool("cuda", info.CUDAAvailable).
		Float64("ram_gb", info.TotalRAMGB).
		Float64("max_model_gb", info.MaxModelGB).
		Str("tts_backend", string(info.TTSBackend)).
		Str("stt_backend", string(info.STTBackend)).
		Msg("platform detected")

	return info, nil
}

// detectDarwin handles macOS platform detection.
func detectDarwin(ctx context.Context, info *PlatformInfo) {
	// Check for Apple Silicon (arm64)
	if runtime.GOARCH == "arm64" {
		info.IsAppleSilicon = true
		info.Platform = PlatformAppleSilicon
		info.MetalAvailable = true // All Apple Silicon has Metal

		// Get chip name
		info.ChipName = getAppleChipName(ctx)

		// Check MLX availability
		info.MLXAvailable = checkMLXAvailable(ctx)

		if info.MLXAvailable {
			// Use MLX for both TTS and STT on Apple Silicon
			info.TTSBackend = BackendMLX
			info.STTBackend = BackendMLX
			info.STTModel = "mlx-community/whisper-large-v3-turbo"
		} else {
			// Fall back to CPU/whisper.cpp if MLX not available
			info.TTSBackend = BackendCPU
			info.STTBackend = BackendWhisperCpp
			info.STTModel = "large-v3-turbo"
		}
	} else {
		// Intel Mac
		info.Platform = PlatformMacOSIntel
		info.MetalAvailable = checkMetalAvailable(ctx)
		info.TTSBackend = BackendCPU
		info.STTBackend = BackendWhisperCpp
		info.STTModel = "large-v3-turbo"
	}
}

// detectLinux handles Linux platform detection.
func detectLinux(ctx context.Context, info *PlatformInfo) {
	// Check for NVIDIA GPU
	info.CUDAAvailable = checkCUDAAvailable(ctx)

	if info.CUDAAvailable {
		info.Platform = PlatformLinuxCUDA
		info.TTSBackend = BackendCUDA
		info.STTBackend = BackendCUDA
		info.STTModel = "large-v3-turbo"
	} else {
		info.Platform = PlatformLinuxCPU
		info.TTSBackend = BackendCPU
		info.STTBackend = BackendCPU
		info.STTModel = "base" // Smaller model for CPU
	}
}

// getAppleChipName returns the Apple Silicon chip name (e.g., "Apple M1 Pro").
func getAppleChipName(ctx context.Context) string {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sysctl", "-n", "machdep.cpu.brand_string")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		log.Debug().Err(err).Msg("failed to get chip name")
		return "Apple Silicon"
	}

	chipName := strings.TrimSpace(stdout.String())
	if chipName == "" {
		return "Apple Silicon"
	}
	return chipName
}

// checkMLXAvailable checks if MLX framework is available.
// This checks if Python with mlx package is installed.
func checkMLXAvailable(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// First check if we're on Apple Silicon (required for MLX)
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
		return false
	}

	// Check if mlx Python package is available
	cmd := exec.CommandContext(ctx, "python3", "-c", "import mlx; print('ok')")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		log.Debug().
			Err(err).
			Str("stderr", stderr.String()).
			Msg("MLX not available (python mlx import failed)")
		return false
	}

	if strings.TrimSpace(stdout.String()) == "ok" {
		log.Debug().Msg("MLX framework available")
		return true
	}

	return false
}

// checkMetalAvailable checks if Metal GPU framework is available (macOS).
func checkMetalAvailable(ctx context.Context) bool {
	if runtime.GOOS != "darwin" {
		return false
	}

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	// Check for Metal support via system_profiler
	cmd := exec.CommandContext(ctx, "system_profiler", "SPDisplaysDataType")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return false
	}

	output := stdout.String()
	return strings.Contains(output, "Metal") || strings.Contains(output, "Apple")
}

// checkCUDAAvailable checks if NVIDIA CUDA is available.
func checkCUDAAvailable(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Try nvidia-smi
	cmd := exec.CommandContext(ctx, "nvidia-smi", "--query-gpu=name", "--format=csv,noheader")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		// Also check for device file on Linux
		if _, err := os.Stat("/dev/nvidia0"); err == nil {
			return true
		}
		return false
	}

	return strings.TrimSpace(stdout.String()) != ""
}

// IsAppleSilicon is a quick check for Apple Silicon without full detection.
func IsAppleSilicon() bool {
	return runtime.GOOS == "darwin" && runtime.GOARCH == "arm64"
}

// GetSystemRAM returns the total system RAM in bytes.
func GetSystemRAM(ctx context.Context) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	switch runtime.GOOS {
	case "darwin":
		cmd := exec.CommandContext(ctx, "sysctl", "-n", "hw.memsize")
		var stdout bytes.Buffer
		cmd.Stdout = &stdout
		if err := cmd.Run(); err != nil {
			return 0, fmt.Errorf("sysctl hw.memsize: %w", err)
		}
		var memBytes int64
		_, err := fmt.Sscanf(strings.TrimSpace(stdout.String()), "%d", &memBytes)
		if err != nil {
			return 0, fmt.Errorf("parse memsize: %w", err)
		}
		return memBytes, nil

	case "linux":
		data, err := os.ReadFile("/proc/meminfo")
		if err != nil {
			return 0, fmt.Errorf("read meminfo: %w", err)
		}
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "MemTotal:") {
				var kbytes int64
				_, err := fmt.Sscanf(line, "MemTotal: %d kB", &kbytes)
				if err != nil {
					return 0, fmt.Errorf("parse meminfo: %w", err)
				}
				return kbytes * 1024, nil
			}
		}
		return 0, fmt.Errorf("MemTotal not found in /proc/meminfo")

	default:
		return 0, fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

// GetMaxModelSizeGB returns the maximum recommended model size based on system RAM.
// Rule: Model should be < 70% of available RAM to leave room for OS and other processes.
func GetMaxModelSizeGB(totalRAMBytes int64) float64 {
	const GB = 1024 * 1024 * 1024
	ramGB := float64(totalRAMBytes) / float64(GB)
	maxModelGB := ramGB * 0.70
	return maxModelGB
}

// QuickDetect performs a minimal detection for common use cases.
// Use Detect() for full platform information.
func QuickDetect() Platform {
	switch runtime.GOOS {
	case "darwin":
		if runtime.GOARCH == "arm64" {
			return PlatformAppleSilicon
		}
		return PlatformMacOSIntel
	case "linux":
		// Quick CUDA check
		if _, err := exec.LookPath("nvidia-smi"); err == nil {
			return PlatformLinuxCUDA
		}
		return PlatformLinuxCPU
	case "windows":
		return PlatformWindows
	default:
		return PlatformUnknown
	}
}
