package platform

import (
	"context"
	"runtime"
	"testing"
	"time"
)

func TestIsAppleSilicon(t *testing.T) {
	result := IsAppleSilicon()

	// This test will have different expected results based on where it runs
	if runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" {
		if !result {
			t.Error("Expected IsAppleSilicon() to return true on Apple Silicon Mac")
		}
	} else {
		if result {
			t.Errorf("Expected IsAppleSilicon() to return false on %s/%s", runtime.GOOS, runtime.GOARCH)
		}
	}
}

func TestQuickDetect(t *testing.T) {
	platform := QuickDetect()

	// Should return a valid platform
	validPlatforms := map[Platform]bool{
		PlatformAppleSilicon: true,
		PlatformMacOSIntel:   true,
		PlatformLinuxCUDA:    true,
		PlatformLinuxCPU:     true,
		PlatformWindows:      true,
		PlatformUnknown:      true,
	}

	if !validPlatforms[platform] {
		t.Errorf("QuickDetect() returned invalid platform: %s", platform)
	}

	// Platform should match runtime
	switch runtime.GOOS {
	case "darwin":
		if runtime.GOARCH == "arm64" {
			if platform != PlatformAppleSilicon {
				t.Errorf("Expected PlatformAppleSilicon on darwin/arm64, got %s", platform)
			}
		} else {
			if platform != PlatformMacOSIntel {
				t.Errorf("Expected PlatformMacOSIntel on darwin/%s, got %s", runtime.GOARCH, platform)
			}
		}
	case "windows":
		if platform != PlatformWindows {
			t.Errorf("Expected PlatformWindows on windows, got %s", platform)
		}
	case "linux":
		if platform != PlatformLinuxCUDA && platform != PlatformLinuxCPU {
			t.Errorf("Expected Linux platform on linux, got %s", platform)
		}
	}
}

func TestDetectPlatform(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	info, err := DetectPlatform(ctx)
	if err != nil {
		t.Fatalf("DetectPlatform() error: %v", err)
	}

	// Basic validation
	if info.OS != runtime.GOOS {
		t.Errorf("Expected OS=%s, got %s", runtime.GOOS, info.OS)
	}

	if info.Arch != runtime.GOARCH {
		t.Errorf("Expected Arch=%s, got %s", runtime.GOARCH, info.Arch)
	}

	// Platform-specific validation
	if runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" {
		if !info.IsAppleSilicon {
			t.Error("Expected IsAppleSilicon=true on darwin/arm64")
		}
		if info.Platform != PlatformAppleSilicon {
			t.Errorf("Expected Platform=apple_silicon, got %s", info.Platform)
		}
		if info.ChipName == "" {
			t.Error("Expected non-empty ChipName on Apple Silicon")
		}
	}

	// Backends should be set
	if info.TTSBackend == "" {
		t.Error("Expected TTSBackend to be set")
	}
	if info.STTBackend == "" {
		t.Error("Expected STTBackend to be set")
	}
	if info.STTModel == "" {
		t.Error("Expected STTModel to be set")
	}

	t.Logf("Platform Info: %+v", info)
}

func TestDetector_Caching(t *testing.T) {
	detector := NewDetector()
	ctx := context.Background()

	// First detection
	info1, err := detector.Detect(ctx)
	if err != nil {
		t.Fatalf("First Detect() error: %v", err)
	}

	// Second detection should use cache
	info2, err := detector.Detect(ctx)
	if err != nil {
		t.Fatalf("Second Detect() error: %v", err)
	}

	// Should return same platform
	if info1.Platform != info2.Platform {
		t.Errorf("Expected cached platform to match: %s vs %s", info1.Platform, info2.Platform)
	}

	// Invalidate cache
	detector.InvalidateCache()

	// Third detection should refresh
	info3, err := detector.Detect(ctx)
	if err != nil {
		t.Fatalf("Third Detect() error: %v", err)
	}

	// Should still return same platform (just refreshed)
	if info1.Platform != info3.Platform {
		t.Errorf("Expected same platform after refresh: %s vs %s", info1.Platform, info3.Platform)
	}
}

func TestPlatformInfo_String(t *testing.T) {
	info := &PlatformInfo{
		Platform:       PlatformAppleSilicon,
		OS:             "darwin",
		Arch:           "arm64",
		IsAppleSilicon: true,
		ChipName:       "Apple M1 Pro",
		MLXAvailable:   true,
	}

	str := info.String()
	if str == "" {
		t.Error("Expected non-empty string representation")
	}

	if !containsAny(str, "Apple Silicon", "M1 Pro", "MLX") {
		t.Errorf("Expected string to contain platform info, got: %s", str)
	}
}

func TestPlatformInfo_SupportsMLX(t *testing.T) {
	tests := []struct {
		name     string
		info     PlatformInfo
		expected bool
	}{
		{
			name: "Apple Silicon with MLX",
			info: PlatformInfo{
				IsAppleSilicon: true,
				MLXAvailable:   true,
			},
			expected: true,
		},
		{
			name: "Apple Silicon without MLX",
			info: PlatformInfo{
				IsAppleSilicon: true,
				MLXAvailable:   false,
			},
			expected: false,
		},
		{
			name: "Non-Apple Silicon",
			info: PlatformInfo{
				IsAppleSilicon: false,
				MLXAvailable:   false,
			},
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.info.SupportsMLX()
			if result != tc.expected {
				t.Errorf("SupportsMLX() = %v, want %v", result, tc.expected)
			}
		})
	}
}

func TestGetDetector(t *testing.T) {
	d1 := GetDetector()
	d2 := GetDetector()

	if d1 != d2 {
		t.Error("Expected GetDetector() to return same singleton instance")
	}
}

func containsAny(s string, substrs ...string) bool {
	for _, substr := range substrs {
		if contains(s, substr) {
			return true
		}
	}
	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr, 0))
}

func containsAt(s, substr string, start int) bool {
	for i := start; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
