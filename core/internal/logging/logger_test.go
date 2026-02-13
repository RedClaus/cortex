package logging

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogLevels(t *testing.T) {
	tests := []struct {
		level    Level
		expected string
	}{
		{LevelDebug, "DEBUG"},
		{LevelInfo, "INFO"},
		{LevelWarn, "WARN"},
		{LevelError, "ERROR"},
		{LevelFatal, "FATAL"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if tt.level.String() != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, tt.level.String())
			}
		})
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected Level
	}{
		{"debug", LevelDebug},
		{"DEBUG", LevelDebug},
		{"info", LevelInfo},
		{"INFO", LevelInfo},
		{"warn", LevelWarn},
		{"warning", LevelWarn},
		{"error", LevelError},
		{"fatal", LevelFatal},
		{"unknown", LevelInfo}, // Default
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ParseLevel(tt.input)
			if result != tt.expected {
				t.Errorf("ParseLevel(%s) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestLoggerOutput(t *testing.T) {
	var buf bytes.Buffer

	cfg := &Config{
		Level:      LevelDebug,
		Colored:    false,
		ShowCaller: false,
		ShowTime:   false,
	}

	logger := New(cfg)
	logger.output = &buf

	logger.Info("test message")

	output := buf.String()
	if !strings.Contains(output, "INFO") {
		t.Errorf("expected output to contain 'INFO', got: %s", output)
	}
	if !strings.Contains(output, "test message") {
		t.Errorf("expected output to contain 'test message', got: %s", output)
	}
}

func TestLoggerLevelFiltering(t *testing.T) {
	var buf bytes.Buffer

	cfg := &Config{
		Level:      LevelWarn,
		Colored:    false,
		ShowCaller: false,
		ShowTime:   false,
	}

	logger := New(cfg)
	logger.output = &buf

	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")

	output := buf.String()

	if strings.Contains(output, "debug message") {
		t.Error("debug message should be filtered")
	}
	if strings.Contains(output, "info message") {
		t.Error("info message should be filtered")
	}
	if !strings.Contains(output, "warn message") {
		t.Error("warn message should be present")
	}
	if !strings.Contains(output, "error message") {
		t.Error("error message should be present")
	}
}

func TestLoggerWithComponent(t *testing.T) {
	var buf bytes.Buffer

	cfg := &Config{
		Level:      LevelDebug,
		Colored:    false,
		ShowCaller: false,
		ShowTime:   false,
	}

	logger := New(cfg)
	logger.output = &buf

	componentLogger := logger.WithComponent("Router")
	componentLogger.output = &buf
	componentLogger.Info("routing request")

	output := buf.String()
	if !strings.Contains(output, "[Router]") {
		t.Errorf("expected output to contain '[Router]', got: %s", output)
	}
}

func TestLoggerWithFields(t *testing.T) {
	var buf bytes.Buffer

	cfg := &Config{
		Level:      LevelDebug,
		Colored:    false,
		ShowCaller: false,
		ShowTime:   false,
	}

	logger := New(cfg)
	logger.output = &buf

	fieldLogger := logger.WithField("user_id", "123")
	fieldLogger.output = &buf
	fieldLogger.Info("user action")

	output := buf.String()
	if !strings.Contains(output, "user_id=123") {
		t.Errorf("expected output to contain 'user_id=123', got: %s", output)
	}
}

func TestLoggerWithMultipleFields(t *testing.T) {
	var buf bytes.Buffer

	cfg := &Config{
		Level:      LevelDebug,
		Colored:    false,
		ShowCaller: false,
		ShowTime:   false,
	}

	logger := New(cfg)
	logger.output = &buf

	fieldLogger := logger.WithFields(map[string]interface{}{
		"request_id": "abc-123",
		"method":     "GET",
	})
	fieldLogger.output = &buf
	fieldLogger.Info("request received")

	output := buf.String()
	if !strings.Contains(output, "request_id=abc-123") {
		t.Errorf("expected output to contain 'request_id=abc-123', got: %s", output)
	}
	if !strings.Contains(output, "method=GET") {
		t.Errorf("expected output to contain 'method=GET', got: %s", output)
	}
}

func TestLoggerShowCaller(t *testing.T) {
	var buf bytes.Buffer

	cfg := &Config{
		Level:      LevelDebug,
		Colored:    false,
		ShowCaller: true,
		ShowTime:   false,
	}

	logger := New(cfg)
	logger.output = &buf

	logger.Info("test with caller")

	output := buf.String()
	// Should contain the test file name
	if !strings.Contains(output, "logger_test.go:") {
		t.Errorf("expected output to contain caller info, got: %s", output)
	}
}

func TestLoggerShowTime(t *testing.T) {
	var buf bytes.Buffer

	cfg := &Config{
		Level:      LevelDebug,
		Colored:    false,
		ShowCaller: false,
		ShowTime:   true,
	}

	logger := New(cfg)
	logger.output = &buf

	logger.Info("test with time")

	output := buf.String()
	// Should contain a timestamp pattern (YYYY-MM-DD)
	if !strings.Contains(output, "202") { // 2024, 2025, etc.
		t.Errorf("expected output to contain timestamp, got: %s", output)
	}
}

func TestLoggerFileOutput(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	cfg := &Config{
		Level:      LevelDebug,
		FilePath:   logPath,
		Colored:    false,
		ShowCaller: false,
		ShowTime:   false,
	}

	logger := New(cfg)
	defer logger.Close()

	logger.Info("file log test")

	// Read the log file
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	if !strings.Contains(string(content), "file log test") {
		t.Errorf("expected log file to contain message, got: %s", string(content))
	}
}

func TestGlobalLogger(t *testing.T) {
	var buf bytes.Buffer

	cfg := &Config{
		Level:      LevelDebug,
		Colored:    false,
		ShowCaller: false,
		ShowTime:   false,
	}

	logger := New(cfg)
	logger.output = &buf
	SetGlobal(logger)

	Info("global test message")

	output := buf.String()
	if !strings.Contains(output, "global test message") {
		t.Errorf("expected output to contain message, got: %s", output)
	}
}

func TestEnableVerbose(t *testing.T) {
	var buf bytes.Buffer

	cfg := &Config{
		Level:      LevelInfo, // Start with INFO
		Colored:    false,
		ShowCaller: false,
		ShowTime:   false,
	}

	logger := New(cfg)
	logger.output = &buf
	SetGlobal(logger)

	// Debug should be filtered
	Debug("should not appear")
	if strings.Contains(buf.String(), "should not appear") {
		t.Error("debug message should be filtered before EnableVerbose")
	}

	// Enable verbose
	EnableVerbose()

	Debug("should appear now")
	if !strings.Contains(buf.String(), "should appear now") {
		t.Errorf("debug message should appear after EnableVerbose, got: %s", buf.String())
	}
}

func TestTrace(t *testing.T) {
	var buf bytes.Buffer

	cfg := &Config{
		Level:      LevelDebug,
		Colored:    false,
		ShowCaller: false,
		ShowTime:   false,
	}

	logger := New(cfg)
	logger.output = &buf

	done := logger.Trace("TestFunction")
	done()

	output := buf.String()
	if !strings.Contains(output, "ENTER TestFunction") {
		t.Errorf("expected ENTER trace, got: %s", output)
	}
	if !strings.Contains(output, "EXIT  TestFunction") {
		t.Errorf("expected EXIT trace, got: %s", output)
	}
	if !strings.Contains(output, "took") {
		t.Errorf("expected duration in EXIT trace, got: %s", output)
	}
}

func TestStripANSI(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"\033[31mRed\033[0m", "Red"},
		{"\033[32mGreen\033[0m text", "Green text"},
		{"No colors", "No colors"},
		{"\033[1m\033[34mBold Blue\033[0m", "Bold Blue"},
	}

	for _, tt := range tests {
		result := stripANSI(tt.input)
		if result != tt.expected {
			t.Errorf("stripANSI(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Level != LevelInfo {
		t.Errorf("expected LevelInfo, got %v", cfg.Level)
	}
	if !cfg.Colored {
		t.Error("expected Colored to be true")
	}
	if cfg.ShowCaller {
		t.Error("expected ShowCaller to be false")
	}
	if !cfg.ShowTime {
		t.Error("expected ShowTime to be true")
	}
}

func TestVerboseConfig(t *testing.T) {
	cfg := VerboseConfig()

	if cfg.Level != LevelDebug {
		t.Errorf("expected LevelDebug, got %v", cfg.Level)
	}
	if !cfg.ShowCaller {
		t.Error("expected ShowCaller to be true for verbose")
	}
}

// Benchmarks

func BenchmarkLoggerInfo(b *testing.B) {
	var buf bytes.Buffer
	logger := New(&Config{
		Level:      LevelInfo,
		Colored:    false,
		ShowCaller: false,
		ShowTime:   false,
	})
	logger.output = &buf

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("benchmark message %d", i)
	}
}

func BenchmarkLoggerWithFields(b *testing.B) {
	var buf bytes.Buffer
	logger := New(&Config{
		Level:      LevelInfo,
		Colored:    false,
		ShowCaller: false,
		ShowTime:   false,
	})
	logger.output = &buf

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.WithField("iteration", i).Info("benchmark message")
	}
}
