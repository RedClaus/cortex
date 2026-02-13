// Package agent provides agentic execution capabilities for CortexBrain.
// This file handles cleanup of orphaned processes from previous sessions.
package agent

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/normanking/cortex/internal/logging"
)

// ProcessCleanup handles tracking and cleanup of spawned subprocesses.
type ProcessCleanup struct {
	pidFile string
	log     *logging.Logger
}

// NewProcessCleanup creates a new process cleanup manager.
func NewProcessCleanup(cortexDir string) *ProcessCleanup {
	return &ProcessCleanup{
		pidFile: filepath.Join(cortexDir, "spawned_pids.txt"),
		log:     logging.Global(),
	}
}

// CleanupOrphanedProcesses kills any orphaned processes from previous sessions.
// This should be called at startup before any new processes are spawned.
func (pc *ProcessCleanup) CleanupOrphanedProcesses() (int, error) {
	pc.log.Info("[ProcessCleanup] Checking for orphaned processes from previous sessions...")

	killed := 0

	// Method 1: Check our PID file for tracked processes
	if fileKilled, err := pc.cleanupFromPIDFile(); err == nil {
		killed += fileKilled
	}

	// Method 2: Find and kill common orphaned commands
	if cmdKilled, err := pc.cleanupOrphanedCommands(); err == nil {
		killed += cmdKilled
	}

	if killed > 0 {
		pc.log.Info("[ProcessCleanup] Cleaned up %d orphaned process(es)", killed)
	} else {
		pc.log.Info("[ProcessCleanup] No orphaned processes found")
	}

	// Clear the PID file for fresh session
	pc.clearPIDFile()

	return killed, nil
}

// cleanupFromPIDFile reads saved PIDs and kills any still running.
func (pc *ProcessCleanup) cleanupFromPIDFile() (int, error) {
	file, err := os.Open(pc.pidFile)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil // No PID file, nothing to clean
		}
		return 0, err
	}
	defer file.Close()

	killed := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse PID and optional command info
		parts := strings.SplitN(line, " ", 2)
		pid, err := strconv.Atoi(parts[0])
		if err != nil {
			continue
		}

		// Check if process is still running
		process, err := os.FindProcess(pid)
		if err != nil {
			continue
		}

		// On Unix, FindProcess always succeeds, so we need to check if it's actually running
		err = process.Signal(syscall.Signal(0))
		if err != nil {
			continue // Process not running
		}

		// Kill the orphaned process
		cmdInfo := ""
		if len(parts) > 1 {
			cmdInfo = parts[1]
		}
		pc.log.Info("[ProcessCleanup] Killing orphaned process PID %d: %s", pid, cmdInfo)

		// Kill the process group if possible
		syscall.Kill(-pid, syscall.SIGKILL)
		// Also try killing the process directly
		process.Kill()
		killed++
	}

	return killed, scanner.Err()
}

// cleanupOrphanedCommands finds and kills common orphaned commands from Cortex.
func (pc *ProcessCleanup) cleanupOrphanedCommands() (int, error) {
	// Common commands that Cortex spawns and might leave orphaned
	orphanPatterns := []string{
		"du -sh",      // Disk usage commands (common slow ones)
		"find /Users", // Find commands in home
	}

	killed := 0

	for _, pattern := range orphanPatterns {
		// Use pgrep to find matching processes
		cmd := exec.Command("pgrep", "-f", pattern)
		output, err := cmd.Output()
		if err != nil {
			continue // No matches or error
		}

		pids := strings.Fields(string(output))
		for _, pidStr := range pids {
			pid, err := strconv.Atoi(pidStr)
			if err != nil {
				continue
			}

			// Don't kill our own process or parent
			if pid == os.Getpid() || pid == os.Getppid() {
				continue
			}

			// Check if this process has been running for more than 2 minutes
			// (to avoid killing legitimate user commands)
			if !pc.isOldProcess(pid, 2*time.Minute) {
				continue
			}

			pc.log.Info("[ProcessCleanup] Killing stale '%s' process PID %d", pattern, pid)
			syscall.Kill(-pid, syscall.SIGKILL)
			syscall.Kill(pid, syscall.SIGKILL)
			killed++
		}
	}

	return killed, nil
}

// isOldProcess checks if a process has been running longer than the threshold.
func (pc *ProcessCleanup) isOldProcess(pid int, threshold time.Duration) bool {
	// Use ps to get process start time
	cmd := exec.Command("ps", "-o", "etime=", "-p", strconv.Itoa(pid))
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	etime := strings.TrimSpace(string(output))
	duration := parseEtime(etime)
	return duration > threshold
}

// parseEtime parses the elapsed time format from ps (e.g., "01:23", "1-02:03:04").
func parseEtime(etime string) time.Duration {
	etime = strings.TrimSpace(etime)
	if etime == "" {
		return 0
	}

	var days, hours, minutes, seconds int

	// Handle days format: "1-02:03:04"
	if strings.Contains(etime, "-") {
		parts := strings.SplitN(etime, "-", 2)
		days, _ = strconv.Atoi(parts[0])
		etime = parts[1]
	}

	// Parse HH:MM:SS or MM:SS
	parts := strings.Split(etime, ":")
	switch len(parts) {
	case 3:
		hours, _ = strconv.Atoi(parts[0])
		minutes, _ = strconv.Atoi(parts[1])
		seconds, _ = strconv.Atoi(parts[2])
	case 2:
		minutes, _ = strconv.Atoi(parts[0])
		seconds, _ = strconv.Atoi(parts[1])
	case 1:
		seconds, _ = strconv.Atoi(parts[0])
	}

	return time.Duration(days)*24*time.Hour +
		time.Duration(hours)*time.Hour +
		time.Duration(minutes)*time.Minute +
		time.Duration(seconds)*time.Second
}

// TrackProcess adds a PID to the tracking file.
func (pc *ProcessCleanup) TrackProcess(pid int, command string) error {
	file, err := os.OpenFile(pc.pidFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = fmt.Fprintf(file, "%d %s\n", pid, command)
	return err
}

// UntrackProcess removes a PID from the tracking file.
func (pc *ProcessCleanup) UntrackProcess(pid int) error {
	// Read all lines
	content, err := os.ReadFile(pc.pidFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	// Filter out the PID
	var newLines []string
	pidStr := strconv.Itoa(pid)
	for _, line := range strings.Split(string(content), "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), pidStr+" ") {
			newLines = append(newLines, line)
		}
	}

	// Write back
	return os.WriteFile(pc.pidFile, []byte(strings.Join(newLines, "\n")), 0644)
}

// clearPIDFile removes the PID tracking file.
func (pc *ProcessCleanup) clearPIDFile() error {
	err := os.Remove(pc.pidFile)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// CleanupOnShutdown should be called when Cortex exits to clean up spawned processes.
func (pc *ProcessCleanup) CleanupOnShutdown() {
	pc.log.Info("[ProcessCleanup] Cleaning up spawned processes on shutdown...")
	pc.cleanupFromPIDFile()
	pc.clearPIDFile()
}
