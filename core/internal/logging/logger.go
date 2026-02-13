// Package logging provides a verbose logging system for Cortex troubleshooting.
// It supports multiple log levels, colored output, caller information, and
// optional file logging for persistent debugging.
package logging

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// ═══════════════════════════════════════════════════════════════════════════════
// LOG LEVELS
// ═══════════════════════════════════════════════════════════════════════════════

// Level represents the severity of a log message.
type Level int

const (
	LevelDebug Level = iota // Detailed debugging information
	LevelInfo               // General operational information
	LevelWarn               // Warning conditions
	LevelError              // Error conditions
	LevelFatal              // Fatal errors (will exit)
)

// String returns the string representation of a log level.
func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	case LevelFatal:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// Color returns the ANSI color code for each level.
func (l Level) Color() string {
	switch l {
	case LevelDebug:
		return "\033[36m" // Cyan
	case LevelInfo:
		return "\033[32m" // Green
	case LevelWarn:
		return "\033[33m" // Yellow
	case LevelError:
		return "\033[31m" // Red
	case LevelFatal:
		return "\033[35m" // Magenta
	default:
		return "\033[0m" // Reset
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// LOGGER
// ═══════════════════════════════════════════════════════════════════════════════

// Logger is the main logging instance for Cortex.
type Logger struct {
	mu          sync.Mutex
	level       Level
	output      io.Writer
	fileOutput  io.Writer
	file        *os.File
	colored     bool
	showCaller  bool
	showTime    bool
	component   string
	fields      map[string]interface{}
}

// Config configures the logger behavior.
type Config struct {
	Level      Level  // Minimum level to log
	FilePath   string // Optional file path for persistent logs
	Colored    bool   // Enable colored output
	ShowCaller bool   // Show file:line of caller
	ShowTime   bool   // Show timestamp
	Component  string // Component name prefix
}

// DefaultConfig returns a sensible default configuration.
func DefaultConfig() *Config {
	return &Config{
		Level:      LevelInfo,
		Colored:    true,
		ShowCaller: false,
		ShowTime:   true,
		Component:  "",
	}
}

// VerboseConfig returns a configuration for verbose troubleshooting.
func VerboseConfig() *Config {
	return &Config{
		Level:      LevelDebug,
		Colored:    true,
		ShowCaller: true,
		ShowTime:   true,
		Component:  "",
	}
}

// New creates a new Logger instance.
func New(cfg *Config) *Logger {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	l := &Logger{
		level:      cfg.Level,
		output:     os.Stderr,
		colored:    cfg.Colored,
		showCaller: cfg.ShowCaller,
		showTime:   cfg.ShowTime,
		component:  cfg.Component,
		fields:     make(map[string]interface{}),
	}

	// Set up file logging if path provided
	if cfg.FilePath != "" {
		if err := l.SetFileOutput(cfg.FilePath); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to open log file: %v\n", err)
		}
	}

	return l
}

// ═══════════════════════════════════════════════════════════════════════════════
// GLOBAL LOGGER
// ═══════════════════════════════════════════════════════════════════════════════

var (
	globalLogger *Logger
	globalMu     sync.RWMutex
)

func init() {
	globalLogger = New(DefaultConfig())
}

// SetGlobal sets the global logger instance.
func SetGlobal(l *Logger) {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalLogger = l
}

// Global returns the global logger instance.
func Global() *Logger {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return globalLogger
}

// EnableVerbose enables verbose logging globally.
func EnableVerbose() {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalLogger.level = LevelDebug
	globalLogger.showCaller = true
}

// SetLevel sets the global log level.
func SetLevel(level Level) {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalLogger.level = level
}

// DisableConsoleOutput disables console output, logging only to file.
// This should be called when running in TUI mode to prevent log messages
// from interfering with the terminal UI.
func DisableConsoleOutput() {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalLogger.output = io.Discard
}

// EnableConsoleOutput re-enables console output.
func EnableConsoleOutput() {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalLogger.output = os.Stderr
}

// ═══════════════════════════════════════════════════════════════════════════════
// LOGGER METHODS
// ═══════════════════════════════════════════════════════════════════════════════

// SetFileOutput sets up file logging.
func (l *Logger) SetFileOutput(path string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create log directory: %w", err)
	}

	// Open file for appending
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}

	// Close previous file if exists
	if l.file != nil {
		l.file.Close()
	}

	l.file = f
	l.fileOutput = f
	return nil
}

// Close closes any open file handles.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file != nil {
		err := l.file.Close()
		l.file = nil
		l.fileOutput = nil
		return err
	}
	return nil
}

// WithComponent returns a new logger with a component prefix.
func (l *Logger) WithComponent(name string) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()

	newLogger := &Logger{
		level:      l.level,
		output:     l.output,
		fileOutput: l.fileOutput,
		file:       l.file,
		colored:    l.colored,
		showCaller: l.showCaller,
		showTime:   l.showTime,
		component:  name,
		fields:     make(map[string]interface{}),
	}

	// Copy fields
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}

	return newLogger
}

// WithField returns a new logger with an additional field.
func (l *Logger) WithField(key string, value interface{}) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()

	newLogger := &Logger{
		level:      l.level,
		output:     l.output,
		fileOutput: l.fileOutput,
		file:       l.file,
		colored:    l.colored,
		showCaller: l.showCaller,
		showTime:   l.showTime,
		component:  l.component,
		fields:     make(map[string]interface{}),
	}

	// Copy existing fields
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}
	newLogger.fields[key] = value

	return newLogger
}

// WithFields returns a new logger with additional fields.
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()

	newLogger := &Logger{
		level:      l.level,
		output:     l.output,
		fileOutput: l.fileOutput,
		file:       l.file,
		colored:    l.colored,
		showCaller: l.showCaller,
		showTime:   l.showTime,
		component:  l.component,
		fields:     make(map[string]interface{}),
	}

	// Copy existing fields
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}
	// Add new fields
	for k, v := range fields {
		newLogger.fields[k] = v
	}

	return newLogger
}

// ═══════════════════════════════════════════════════════════════════════════════
// LOG METHODS
// ═══════════════════════════════════════════════════════════════════════════════

// log is the internal logging method.
func (l *Logger) log(level Level, format string, args ...interface{}) {
	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Build the log message
	var sb strings.Builder

	// Reset color at start
	reset := "\033[0m"

	// Timestamp
	if l.showTime {
		timestamp := time.Now().Format("2006-01-02 15:04:05.000")
		if l.colored {
			sb.WriteString("\033[90m") // Gray
			sb.WriteString(timestamp)
			sb.WriteString(reset)
			sb.WriteString(" ")
		} else {
			sb.WriteString(timestamp)
			sb.WriteString(" ")
		}
	}

	// Level
	if l.colored {
		sb.WriteString(level.Color())
		sb.WriteString(fmt.Sprintf("%-5s", level.String()))
		sb.WriteString(reset)
		sb.WriteString(" ")
	} else {
		sb.WriteString(fmt.Sprintf("%-5s ", level.String()))
	}

	// Component
	if l.component != "" {
		if l.colored {
			sb.WriteString("\033[94m") // Blue
			sb.WriteString("[")
			sb.WriteString(l.component)
			sb.WriteString("]")
			sb.WriteString(reset)
			sb.WriteString(" ")
		} else {
			sb.WriteString("[")
			sb.WriteString(l.component)
			sb.WriteString("] ")
		}
	}

	// Caller info
	if l.showCaller {
		_, file, line, ok := runtime.Caller(2)
		if ok {
			// Shorten file path to just filename
			file = filepath.Base(file)
			if l.colored {
				sb.WriteString("\033[90m") // Gray
				sb.WriteString(fmt.Sprintf("(%s:%d)", file, line))
				sb.WriteString(reset)
				sb.WriteString(" ")
			} else {
				sb.WriteString(fmt.Sprintf("(%s:%d) ", file, line))
			}
		}
	}

	// Message
	message := fmt.Sprintf(format, args...)
	sb.WriteString(message)

	// Fields
	if len(l.fields) > 0 {
		sb.WriteString(" ")
		if l.colored {
			sb.WriteString("\033[90m") // Gray
		}
		sb.WriteString("{")
		first := true
		for k, v := range l.fields {
			if !first {
				sb.WriteString(", ")
			}
			sb.WriteString(fmt.Sprintf("%s=%v", k, v))
			first = false
		}
		sb.WriteString("}")
		if l.colored {
			sb.WriteString(reset)
		}
	}

	sb.WriteString("\n")

	// Write to outputs
	output := sb.String()
	l.output.Write([]byte(output))

	// Write to file (without colors)
	if l.fileOutput != nil {
		// Strip ANSI codes for file output
		plainOutput := stripANSI(output)
		l.fileOutput.Write([]byte(plainOutput))
	}
}

// Debug logs a debug message.
func (l *Logger) Debug(format string, args ...interface{}) {
	l.log(LevelDebug, format, args...)
}

// Info logs an info message.
func (l *Logger) Info(format string, args ...interface{}) {
	l.log(LevelInfo, format, args...)
}

// Warn logs a warning message.
func (l *Logger) Warn(format string, args ...interface{}) {
	l.log(LevelWarn, format, args...)
}

// Error logs an error message.
func (l *Logger) Error(format string, args ...interface{}) {
	l.log(LevelError, format, args...)
}

// Fatal logs a fatal message and exits.
func (l *Logger) Fatal(format string, args ...interface{}) {
	l.log(LevelFatal, format, args...)
	os.Exit(1)
}

// ═══════════════════════════════════════════════════════════════════════════════
// GLOBAL LOG FUNCTIONS
// ═══════════════════════════════════════════════════════════════════════════════

// Debug logs a debug message using the global logger.
func Debug(format string, args ...interface{}) {
	Global().Debug(format, args...)
}

// Info logs an info message using the global logger.
func Info(format string, args ...interface{}) {
	Global().Info(format, args...)
}

// Warn logs a warning message using the global logger.
func Warn(format string, args ...interface{}) {
	Global().Warn(format, args...)
}

// Error logs an error message using the global logger.
func Error(format string, args ...interface{}) {
	Global().Error(format, args...)
}

// Fatal logs a fatal message using the global logger and exits.
func Fatal(format string, args ...interface{}) {
	Global().Fatal(format, args...)
}

// ═══════════════════════════════════════════════════════════════════════════════
// VERBOSE TRACING
// ═══════════════════════════════════════════════════════════════════════════════

// Trace logs entry into a function (for verbose tracing).
func (l *Logger) Trace(funcName string) func() {
	start := time.Now()
	l.Debug("→ ENTER %s", funcName)
	return func() {
		l.Debug("← EXIT  %s (took %v)", funcName, time.Since(start))
	}
}

// TraceWithArgs logs entry with arguments.
func (l *Logger) TraceWithArgs(funcName string, args map[string]interface{}) func() {
	start := time.Now()
	l.WithFields(args).Debug("→ ENTER %s", funcName)
	return func() {
		l.Debug("← EXIT  %s (took %v)", funcName, time.Since(start))
	}
}

// Trace logs entry using global logger.
func Trace(funcName string) func() {
	return Global().Trace(funcName)
}

// ═══════════════════════════════════════════════════════════════════════════════
// SPECIALIZED LOGGING
// ═══════════════════════════════════════════════════════════════════════════════

// Request logs an incoming request.
func (l *Logger) Request(method, path string, fields map[string]interface{}) {
	l.WithFields(fields).Info("▶ %s %s", method, path)
}

// Response logs an outgoing response.
func (l *Logger) Response(method, path string, status int, duration time.Duration) {
	l.WithFields(map[string]interface{}{
		"status":   status,
		"duration": duration,
	}).Info("◀ %s %s", method, path)
}

// SQL logs a SQL query (for debugging).
func (l *Logger) SQL(query string, args ...interface{}) {
	// Truncate long queries
	if len(query) > 200 {
		query = query[:200] + "..."
	}
	// Clean up whitespace
	query = strings.Join(strings.Fields(query), " ")
	l.WithField("args", args).Debug("SQL: %s", query)
}

// ═══════════════════════════════════════════════════════════════════════════════
// HELPERS
// ═══════════════════════════════════════════════════════════════════════════════

// stripANSI removes ANSI escape codes from a string.
func stripANSI(s string) string {
	var result strings.Builder
	inEscape := false

	for i := 0; i < len(s); i++ {
		if s[i] == '\033' {
			inEscape = true
			continue
		}
		if inEscape {
			if s[i] == 'm' {
				inEscape = false
			}
			continue
		}
		result.WriteByte(s[i])
	}

	return result.String()
}

// ParseLevel parses a string into a Level.
func ParseLevel(s string) Level {
	switch strings.ToLower(s) {
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "warn", "warning":
		return LevelWarn
	case "error":
		return LevelError
	case "fatal":
		return LevelFatal
	default:
		return LevelInfo
	}
}
