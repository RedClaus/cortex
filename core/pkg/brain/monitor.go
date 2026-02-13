package brain

import (
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// SystemMetrics holds current system state.
type SystemMetrics struct {
	CPUUsagePercent    float64   `json:"cpu_usage_percent"`
	MemoryUsedMB       uint64    `json:"memory_used_mb"`
	MemoryTotalMB      uint64    `json:"memory_total_mb"`
	MemoryUsagePercent float64   `json:"memory_usage_percent"`
	GoRoutineCount     int       `json:"goroutine_count"`
	HeapAllocMB        uint64    `json:"heap_alloc_mb"`
	LastUpdated        time.Time `json:"last_updated"`
}

// SystemMonitor tracks system resources for adaptive compute decisions.
type SystemMonitor struct {
	mu           sync.RWMutex
	lastMetrics  SystemMetrics
	pollInterval time.Duration
	stopCh       chan struct{}
	running      bool
}

// NewSystemMonitor creates a monitor with the given poll interval.
func NewSystemMonitor(pollInterval time.Duration) *SystemMonitor {
	return &SystemMonitor{
		pollInterval: pollInterval,
		stopCh:       make(chan struct{}),
	}
}

// Start begins background monitoring.
func (m *SystemMonitor) Start() {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.stopCh = make(chan struct{}) // Re-create in case of restart
	m.mu.Unlock()

	// Initial collection
	m.collectMetrics()

	go func() {
		ticker := time.NewTicker(m.pollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				m.collectMetrics()
			case <-m.stopCh:
				return
			}
		}
	}()
}

// Stop halts background monitoring.
func (m *SystemMonitor) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return
	}
	close(m.stopCh)
	m.running = false
}

// GetMetrics returns the latest system metrics.
func (m *SystemMonitor) GetMetrics() SystemMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastMetrics
}

// collectMetrics gathers current system stats.
func (m *SystemMonitor) collectMetrics() SystemMetrics {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	totalMemMB := getTotalMemoryMB()
	usedMemMB := bToMB(memStats.Alloc) // Using Heap Alloc as primary metric per requirements

	// If we can get actual system memory usage, that would be better,
	// but sticking to runtime.MemStats as requested for "Go memory metrics".
	// For "MemoryTotalMB" we try to get system total.

	percent := 0.0
	if totalMemMB > 0 {
		percent = float64(usedMemMB) / float64(totalMemMB) * 100
	}

	goroutines := runtime.NumGoroutine()
	// CPU usage is not easily available via stdlib; using 0.0 placeholder.
	// Logic relies on GoRoutineCount as a proxy.
	cpuPercent := 0.0

	metrics := SystemMetrics{
		CPUUsagePercent:    cpuPercent,
		MemoryUsedMB:       usedMemMB,
		MemoryTotalMB:      totalMemMB,
		MemoryUsagePercent: percent,
		GoRoutineCount:     goroutines,
		HeapAllocMB:        bToMB(memStats.HeapAlloc),
		LastUpdated:        time.Now(),
	}

	m.mu.Lock()
	m.lastMetrics = metrics
	m.mu.Unlock()

	return metrics
}

// SuggestComputeTier recommends a compute tier based on current load.
func (m *SystemMonitor) SuggestComputeTier() ComputeTier {
	metrics := m.GetMetrics()

	// CPU High proxy: > 1000 goroutines per CPU core (rough heuristic)
	numCPU := runtime.NumCPU()
	cpuHigh := metrics.GoRoutineCount > (numCPU * 1000)

	// If memory > 80% or CPU high → suggest ComputeFast
	if metrics.MemoryUsagePercent > 80.0 || cpuHigh {
		return ComputeFast
	}

	// If memory < 50% and CPU low → suggest ComputeDeep or ComputeMax
	// "CPU low" is !cpuHigh and maybe stricter check
	cpuLow := metrics.GoRoutineCount < (numCPU * 100)
	if metrics.MemoryUsagePercent < 50.0 && cpuLow {
		// Prefer Deep (Local Large) as default powerful option
		return ComputeDeep
	}

	// Otherwise → suggest ComputeHybrid
	return ComputeHybrid
}

// IsSystemConstrained returns true if resources are limited.
func (m *SystemMonitor) IsSystemConstrained() bool {
	metrics := m.GetMetrics()

	if metrics.MemoryUsagePercent > 75.0 {
		return true
	}

	numCPU := runtime.NumCPU()
	if metrics.GoRoutineCount > (numCPU * 2000) {
		return true
	}

	return false
}

func bToMB(b uint64) uint64 {
	return b / 1024 / 1024
}

func getTotalMemoryMB() uint64 {
	switch runtime.GOOS {
	case "darwin":
		out, err := exec.Command("sysctl", "-n", "hw.memsize").Output()
		if err == nil {
			s := strings.TrimSpace(string(out))
			if v, err := strconv.ParseUint(s, 10, 64); err == nil {
				return bToMB(v)
			}
		}
	case "linux":
		if data, err := os.ReadFile("/proc/meminfo"); err == nil {
			lines := strings.Split(string(data), "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "MemTotal:") {
					parts := strings.Fields(line)
					if len(parts) >= 2 {
						v, err := strconv.ParseUint(parts[1], 10, 64)
						if err == nil {
							// MemTotal is in kB
							return v / 1024
						}
					}
				}
			}
		}
	}
	// Default fallback if detection fails (e.g. 8GB)
	return 8192
}
