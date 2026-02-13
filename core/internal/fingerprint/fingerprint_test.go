package fingerprint

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestNewFingerprinter(t *testing.T) {
	fp := NewFingerprinter()
	if fp == nil {
		t.Fatal("expected non-nil fingerprinter")
	}
	if fp.timeout != 5*time.Second {
		t.Errorf("expected 5s timeout, got %v", fp.timeout)
	}
}

func TestFingerprinter_Detect(t *testing.T) {
	fp := NewFingerprinter()
	ctx := context.Background()

	result, err := fp.Detect(ctx)
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}

	// Verify platform
	expectedPlatform := Platform(runtime.GOOS)
	if result.Platform != expectedPlatform && result.Platform != PlatformUnknown {
		t.Errorf("expected platform %s, got %s", expectedPlatform, result.Platform)
	}

	// Verify arch
	if result.Arch != runtime.GOARCH {
		t.Errorf("expected arch %s, got %s", runtime.GOARCH, result.Arch)
	}

	// Verify shell is set
	if result.Shell == "" {
		t.Error("expected non-empty shell")
	}

	// Verify timestamp
	if result.FingerprintedAt.IsZero() {
		t.Error("expected non-zero fingerprint time")
	}
}

func TestFingerprinter_DetectProject(t *testing.T) {
	fp := NewFingerprinter()
	ctx := context.Background()

	// Create temp project structure
	tmpDir, err := os.MkdirTemp("", "cortex_test_")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create go.mod to simulate Go project
	goMod := `module test
go 1.21
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := fp.DetectProject(ctx, tmpDir)
	if err != nil {
		t.Fatalf("DetectProject failed: %v", err)
	}

	if result.ProjectType != ProjectGo {
		t.Errorf("expected project type %s, got %s", ProjectGo, result.ProjectType)
	}

	if result.PackageFile != "go.mod" {
		t.Errorf("expected package file 'go.mod', got %s", result.PackageFile)
	}
}

func TestFingerprinter_DetectProjectType(t *testing.T) {
	fp := NewFingerprinter()

	testCases := []struct {
		files    []string
		expected ProjectType
	}{
		{[]string{"go.mod"}, ProjectGo},
		{[]string{"package.json"}, ProjectNode},
		{[]string{"requirements.txt"}, ProjectPython},
		{[]string{"Cargo.toml"}, ProjectRust},
		{[]string{"Gemfile"}, ProjectRuby},
		{[]string{"pom.xml"}, ProjectJava},
		{[]string{"go.mod", "package.json"}, ProjectMultiple},
		{[]string{}, ProjectUnknown},
	}

	for _, tc := range testCases {
		t.Run(tc.expected.String(), func(t *testing.T) {
			tmpDir, _ := os.MkdirTemp("", "cortex_test_")
			defer os.RemoveAll(tmpDir)

			for _, f := range tc.files {
				os.WriteFile(filepath.Join(tmpDir, f), []byte(""), 0644)
			}

			result := fp.detectProjectType(tmpDir)
			if result != tc.expected {
				t.Errorf("expected %s, got %s", tc.expected, result)
			}
		})
	}
}

func TestFingerprinter_FindProjectRoot(t *testing.T) {
	fp := NewFingerprinter()

	// Create nested structure with .git at root
	tmpDir, _ := os.MkdirTemp("", "cortex_test_")
	defer os.RemoveAll(tmpDir)

	// Create .git marker
	os.Mkdir(filepath.Join(tmpDir, ".git"), 0755)

	// Create nested directory
	nested := filepath.Join(tmpDir, "a", "b", "c")
	os.MkdirAll(nested, 0755)

	// Should find root from nested dir
	root := fp.findProjectRoot(nested)
	if root != tmpDir {
		t.Errorf("expected root %s, got %s", tmpDir, root)
	}
}

func TestIsNetworkDevice(t *testing.T) {
	testCases := []struct {
		prompt   string
		isDevice bool
		vendor   string
	}{
		{"router#", false, ""},
		{"Cisco IOS XE", true, "cisco"},
		{"JUNOS 21.4R1", true, "juniper"},
		{"Arista EOS", true, "arista"},
		{"PaloAlto Networks", true, "paloalto"},
		{"FortiGate", true, "fortinet"},
		{"MikroTik RouterOS", true, "mikrotik"},
		{"Linux ubuntu 22.04", false, ""},
	}

	for _, tc := range testCases {
		t.Run(tc.prompt, func(t *testing.T) {
			isDevice, vendor := IsNetworkDevice(tc.prompt)
			if isDevice != tc.isDevice {
				t.Errorf("IsNetworkDevice(%q) = %v, expected %v", tc.prompt, isDevice, tc.isDevice)
			}
			if vendor != tc.vendor {
				t.Errorf("vendor = %q, expected %q", vendor, tc.vendor)
			}
		})
	}
}

func TestFingerprint_Summary(t *testing.T) {
	fp := &Fingerprint{
		Platform:    PlatformLinux,
		Arch:        "amd64",
		Shell:       "/bin/bash",
		Environment: EnvDevelopment,
		ProjectType: ProjectGo,
		GitBranch:   "main",
		GitDirty:    true,
	}

	summary := fp.Summary()

	expectedParts := []string{
		"linux/amd64",
		"/bin/bash",
		"development",
		"go",
		"main",
		"dirty",
	}

	for _, part := range expectedParts {
		if !containsSubstring(summary, part) {
			t.Errorf("expected %q in summary, got: %s", part, summary)
		}
	}
}

func TestFingerprint_JSON(t *testing.T) {
	fp := &Fingerprint{
		Platform:        PlatformDarwin,
		Arch:            "arm64",
		OS:              "darwin",
		Shell:           "/bin/zsh",
		Environment:     EnvDevelopment,
		FingerprintedAt: time.Now(),
	}

	data, err := fp.JSON()
	if err != nil {
		t.Fatalf("JSON() failed: %v", err)
	}

	if len(data) == 0 {
		t.Error("expected non-empty JSON")
	}

	// Should be valid JSON containing expected fields
	jsonStr := string(data)
	if !containsSubstring(jsonStr, "darwin") {
		t.Errorf("expected 'darwin' in JSON: %s", jsonStr)
	}
	if !containsSubstring(jsonStr, "arm64") {
		t.Errorf("expected 'arm64' in JSON: %s", jsonStr)
	}
}

func TestDetectEnvironment(t *testing.T) {
	fp := NewFingerprinter()

	// Save and restore environment
	originalCI := os.Getenv("CI")
	defer os.Setenv("CI", originalCI)

	// Test CI detection
	os.Setenv("CI", "true")
	env := fp.detectEnvironment()
	if env != EnvCI {
		t.Errorf("expected CI environment, got %s", env)
	}

	// Clear CI
	os.Unsetenv("CI")

	// Test development detection
	originalNodeEnv := os.Getenv("NODE_ENV")
	defer os.Setenv("NODE_ENV", originalNodeEnv)

	os.Setenv("NODE_ENV", "development")
	env = fp.detectEnvironment()
	if env != EnvDevelopment {
		t.Errorf("expected development environment, got %s", env)
	}
}

func (pt ProjectType) String() string {
	return string(pt)
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstringHelper(s, substr))
}

func containsSubstringHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ===========================================================================
// BENCHMARKS
// ===========================================================================

func BenchmarkFingerprinter_Detect(b *testing.B) {
	fp := NewFingerprinter()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fp.Detect(ctx)
	}
}

func BenchmarkIsNetworkDevice(b *testing.B) {
	prompts := []string{
		"Cisco IOS XE Software, Version 17.3.2",
		"Linux ubuntu 22.04",
		"router#",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, p := range prompts {
			IsNetworkDevice(p)
		}
	}
}
