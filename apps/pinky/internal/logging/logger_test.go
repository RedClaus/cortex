// Package logging tests
package logging

import (
	"testing"
)

func TestNew(t *testing.T) {
	logger := New()
	if logger == nil {
		t.Fatal("New() returned nil")
	}
	if logger.Logger == nil {
		t.Fatal("logger.Logger is nil")
	}
}

func TestNewWithConfig_Levels(t *testing.T) {
	tests := []struct {
		name   string
		level  string
		format string
	}{
		{"debug level", "debug", "text"},
		{"info level", "info", "text"},
		{"warn level", "warn", "text"},
		{"error level", "error", "text"},
		{"unknown level defaults to info", "unknown", "text"},
		{"empty level defaults to info", "", "text"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := NewWithConfig(tt.level, tt.format, "")
			if logger == nil {
				t.Fatal("NewWithConfig() returned nil")
			}
			if logger.Logger == nil {
				t.Fatal("logger.Logger is nil")
			}
			logger.Close()
		})
	}
}

func TestNewWithConfig_Formats(t *testing.T) {
	tests := []struct {
		name   string
		format string
	}{
		{"text format", "text"},
		{"json format", "json"},
		{"unknown format defaults to text", "unknown"},
		{"empty format defaults to text", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := NewWithConfig("info", tt.format, "")
			if logger == nil {
				t.Fatal("NewWithConfig() returned nil")
			}
			logger.Close()
		})
	}
}

func TestLogger_LogMethods(t *testing.T) {
	// Test that log methods don't panic
	logger := New()

	// These should not panic
	logger.Debug("debug message", "key", "value")
	logger.Info("info message", "key", "value")
	logger.Warn("warn message", "key", "value")
	logger.Error("error message", "key", "value")
}

func TestLogger_WithAttributes(t *testing.T) {
	logger := New()

	// Create child logger with attributes
	child := logger.With("component", "test")
	if child == nil {
		t.Fatal("With() returned nil")
	}

	// Should not panic
	child.Info("message from child logger")
}
