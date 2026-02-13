// Package logging provides structured logging for Pinky
package logging

import (
	"io"
	"log/slog"
	"os"
)

// Logger wraps slog for Pinky
type Logger struct {
	*slog.Logger
	file *os.File // Keep reference to close later
}

// New creates a new logger with default settings
func New() *Logger {
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	return &Logger{Logger: slog.New(handler)}
}

// NewWithConfig creates a logger from configuration
func NewWithConfig(level, format, filePath string) *Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "info":
		lvl = slog.LevelInfo
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: lvl}

	// Determine output destination
	var output io.Writer = os.Stdout
	var logFile *os.File

	if filePath != "" {
		f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err == nil {
			output = f
			logFile = f
		}
		// If file open fails, fall back to stdout silently
	}

	var handler slog.Handler
	if format == "json" {
		handler = slog.NewJSONHandler(output, opts)
	} else {
		handler = slog.NewTextHandler(output, opts)
	}

	return &Logger{Logger: slog.New(handler), file: logFile}
}

// Close closes the log file if one is open
func (l *Logger) Close() error {
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}
