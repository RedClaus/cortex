// Package logging provides structured logging with file and console output.
package logging

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// LogLevel represents logging levels
type LogLevel string

const (
	LevelDebug LogLevel = "debug"
	LevelInfo  LogLevel = "info"
	LevelWarn  LogLevel = "warn"
	LevelError LogLevel = "error"
)

// LogEntry represents a single log entry for the frontend
type LogEntry struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Component string `json:"component"`
	Message   string `json:"message"`
	Data      string `json:"data,omitempty"`
}

// Logger wraps zerolog with file output and log history
type Logger struct {
	zlog     zerolog.Logger
	file     *os.File
	logPath  string
	mu       sync.RWMutex
	history  []LogEntry
	maxHist  int
	onLog    func(LogEntry) // callback for real-time log streaming
}

// Config holds logger configuration
type Config struct {
	LogDir     string   // Directory for log files (default: ~/.cortexavatar/logs)
	Level      LogLevel // Minimum log level (default: debug)
	MaxHistory int      // Max entries to keep in memory (default: 1000)
	Console    bool     // Also log to console (default: true)
}

// DefaultConfig returns sensible defaults
func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	return &Config{
		LogDir:     filepath.Join(home, ".cortexavatar", "logs"),
		Level:      LevelDebug,
		MaxHistory: 1000,
		Console:    true,
	}
}

// New creates a new Logger with file and console output
func New(cfg *Config) (*Logger, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	// Ensure log directory exists
	if err := os.MkdirAll(cfg.LogDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Create log file with date-based name
	logFileName := fmt.Sprintf("cortexavatar_%s.log", time.Now().Format("2006-01-02"))
	logPath := filepath.Join(cfg.LogDir, logFileName)

	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	// Set up writers
	var writers []io.Writer
	writers = append(writers, file)

	if cfg.Console {
		consoleWriter := zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: "15:04:05",
		}
		writers = append(writers, consoleWriter)
	}

	multi := io.MultiWriter(writers...)

	// Set log level
	level := zerolog.DebugLevel
	switch cfg.Level {
	case LevelInfo:
		level = zerolog.InfoLevel
	case LevelWarn:
		level = zerolog.WarnLevel
	case LevelError:
		level = zerolog.ErrorLevel
	}
	zerolog.SetGlobalLevel(level)

	zlog := zerolog.New(multi).With().
		Timestamp().
		Str("app", "cortexavatar").
		Logger()

	logger := &Logger{
		zlog:    zlog,
		file:    file,
		logPath: logPath,
		history: make([]LogEntry, 0, cfg.MaxHistory),
		maxHist: cfg.MaxHistory,
	}

	// Log startup
	logger.Info("logging", "Logger initialized", map[string]interface{}{
		"logFile": logPath,
		"level":   string(cfg.Level),
	})

	return logger, nil
}

// SetOnLog sets a callback for real-time log streaming (to frontend)
func (l *Logger) SetOnLog(fn func(LogEntry)) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.onLog = fn
}

// addToHistory adds an entry to the in-memory log history
func (l *Logger) addToHistory(entry LogEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.history = append(l.history, entry)
	if len(l.history) > l.maxHist {
		// Remove oldest entries
		l.history = l.history[len(l.history)-l.maxHist:]
	}

	// Call callback if set
	if l.onLog != nil {
		go l.onLog(entry)
	}
}

// GetHistory returns recent log entries
func (l *Logger) GetHistory(limit int) []LogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if limit <= 0 || limit > len(l.history) {
		limit = len(l.history)
	}

	// Return most recent entries
	start := len(l.history) - limit
	if start < 0 {
		start = 0
	}

	result := make([]LogEntry, limit)
	copy(result, l.history[start:])
	return result
}

// GetLogPath returns the current log file path
func (l *Logger) GetLogPath() string {
	return l.logPath
}

// Close closes the log file
func (l *Logger) Close() error {
	l.Info("logging", "Logger shutting down", nil)
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// formatData converts data map to string for storage
func formatData(data map[string]interface{}) string {
	if data == nil || len(data) == 0 {
		return ""
	}
	result := ""
	for k, v := range data {
		if result != "" {
			result += ", "
		}
		result += fmt.Sprintf("%s=%v", k, v)
	}
	return result
}

// Debug logs a debug message
func (l *Logger) Debug(component, msg string, data map[string]interface{}) {
	event := l.zlog.Debug().Str("component", component)
	for k, v := range data {
		event = event.Interface(k, v)
	}
	event.Msg(msg)

	l.addToHistory(LogEntry{
		Timestamp: time.Now().Format("15:04:05.000"),
		Level:     "debug",
		Component: component,
		Message:   msg,
		Data:      formatData(data),
	})
}

// Info logs an info message
func (l *Logger) Info(component, msg string, data map[string]interface{}) {
	event := l.zlog.Info().Str("component", component)
	for k, v := range data {
		event = event.Interface(k, v)
	}
	event.Msg(msg)

	l.addToHistory(LogEntry{
		Timestamp: time.Now().Format("15:04:05.000"),
		Level:     "info",
		Component: component,
		Message:   msg,
		Data:      formatData(data),
	})
}

// Warn logs a warning message
func (l *Logger) Warn(component, msg string, data map[string]interface{}) {
	event := l.zlog.Warn().Str("component", component)
	for k, v := range data {
		event = event.Interface(k, v)
	}
	event.Msg(msg)

	l.addToHistory(LogEntry{
		Timestamp: time.Now().Format("15:04:05.000"),
		Level:     "warn",
		Component: component,
		Message:   msg,
		Data:      formatData(data),
	})
}

// Error logs an error message
func (l *Logger) Error(component, msg string, err error, data map[string]interface{}) {
	event := l.zlog.Error().Str("component", component)
	if err != nil {
		event = event.Err(err)
	}
	for k, v := range data {
		event = event.Interface(k, v)
	}
	event.Msg(msg)

	errStr := ""
	if err != nil {
		errStr = err.Error()
	}

	l.addToHistory(LogEntry{
		Timestamp: time.Now().Format("15:04:05.000"),
		Level:     "error",
		Component: component,
		Message:   msg,
		Data:      formatData(data) + " error=" + errStr,
	})
}

// Component returns a zerolog.Logger with the component field set
// This allows existing code using zerolog directly to work
func (l *Logger) Component(name string) zerolog.Logger {
	return l.zlog.With().Str("component", name).Logger()
}

// Zerolog returns the underlying zerolog.Logger for compatibility
func (l *Logger) Zerolog() zerolog.Logger {
	return l.zlog
}
