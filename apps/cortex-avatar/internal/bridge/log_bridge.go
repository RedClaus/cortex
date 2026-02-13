// Package bridge provides Wails Go-JS bindings.
package bridge

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"

	"github.com/normanking/cortexavatar/internal/logging"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// LogBridge exposes logging methods to the frontend
type LogBridge struct {
	ctx    context.Context
	logger *logging.Logger
}

// NewLogBridge creates a new log bridge
func NewLogBridge(logger *logging.Logger) *LogBridge {
	return &LogBridge{
		logger: logger,
	}
}

// Bind sets the Wails context
func (b *LogBridge) Bind(ctx context.Context) {
	b.ctx = ctx

	// Set up real-time log streaming to frontend
	b.logger.SetOnLog(func(entry logging.LogEntry) {
		if b.ctx != nil {
			runtime.EventsEmit(b.ctx, "log:entry", entry)
		}
	})
}

// Log logs a message from the frontend
func (b *LogBridge) Log(level, component, message string, data map[string]interface{}) {
	switch level {
	case "debug":
		b.logger.Debug(component, message, data)
	case "info":
		b.logger.Info(component, message, data)
	case "warn":
		b.logger.Warn(component, message, data)
	case "error":
		b.logger.Error(component, message, nil, data)
	default:
		b.logger.Info(component, message, data)
	}
}

// LogDebug logs a debug message from the frontend
func (b *LogBridge) LogDebug(component, message string) {
	b.logger.Debug(component, message, nil)
}

// LogInfo logs an info message from the frontend
func (b *LogBridge) LogInfo(component, message string) {
	b.logger.Info(component, message, nil)
}

// LogWarn logs a warning message from the frontend
func (b *LogBridge) LogWarn(component, message string) {
	b.logger.Warn(component, message, nil)
}

// LogError logs an error message from the frontend
func (b *LogBridge) LogError(component, message string) {
	b.logger.Error(component, message, nil, nil)
}

// GetLogHistory returns recent log entries
func (b *LogBridge) GetLogHistory(limit int) []logging.LogEntry {
	return b.logger.GetHistory(limit)
}

// GetLogPath returns the current log file path
func (b *LogBridge) GetLogPath() string {
	return b.logger.GetLogPath()
}

// OpenLogFile opens the log file in the default text editor
func (b *LogBridge) OpenLogFile() error {
	logPath := b.logger.GetLogPath()

	var cmd *exec.Cmd
	switch goruntime.GOOS {
	case "darwin":
		cmd = exec.Command("open", logPath)
	case "linux":
		cmd = exec.Command("xdg-open", logPath)
	case "windows":
		cmd = exec.Command("notepad", logPath)
	default:
		cmd = exec.Command("open", logPath)
	}

	return cmd.Start()
}

// OpenLogDir opens the log directory in the file manager
func (b *LogBridge) OpenLogDir() error {
	logPath := b.logger.GetLogPath()
	logDir := filepath.Dir(logPath)

	var cmd *exec.Cmd
	switch goruntime.GOOS {
	case "darwin":
		cmd = exec.Command("open", logDir)
	case "linux":
		cmd = exec.Command("xdg-open", logDir)
	case "windows":
		cmd = exec.Command("explorer", logDir)
	default:
		cmd = exec.Command("open", logDir)
	}

	return cmd.Start()
}

// GetSystemInfo returns system information for troubleshooting
func (b *LogBridge) GetSystemInfo() map[string]interface{} {
	info := make(map[string]interface{})

	info["os"] = goruntime.GOOS
	info["arch"] = goruntime.GOARCH
	info["goVersion"] = goruntime.Version()
	info["numCPU"] = goruntime.NumCPU()
	info["numGoroutine"] = goruntime.NumGoroutine()

	// Memory stats
	var m goruntime.MemStats
	goruntime.ReadMemStats(&m)
	info["memAlloc"] = m.Alloc / 1024 / 1024 // MB
	info["memTotalAlloc"] = m.TotalAlloc / 1024 / 1024
	info["memSys"] = m.Sys / 1024 / 1024
	info["numGC"] = m.NumGC

	// Environment
	info["home"] = os.Getenv("HOME")
	info["logPath"] = b.logger.GetLogPath()

	return info
}
