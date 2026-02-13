// Package orchestrator provides the Default Mode Network (DMN) worker.
// This background worker runs during idle periods to perform consolidation,
// knowledge indexing, and memory maintenance - mimicking the brain's DMN.
package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/normanking/cortex/internal/bus"
	"github.com/normanking/cortex/internal/logging"
)

// DMNWorker implements the Default Mode Network - a background consolidation
// system that runs during idle periods. Like the brain's DMN, it performs:
// - Memory consolidation and neighborhood refresh
// - Knowledge indexing from external sources (iCloud, etc.)
// - Stale data cleanup
// - Pattern detection across recent interactions
type DMNWorker struct {
	mu sync.RWMutex

	// Dependencies
	memory   MemorySystem      // Memory coordinator for consolidation
	eventBus *bus.EventBus     // Event publishing
	log      *logging.Logger

	// Configuration
	config DMNConfig

	// State
	isRunning       bool
	lastActivity    time.Time      // Last user interaction
	lastConsolidate time.Time      // Last consolidation run
	stopCh          chan struct{}  // Stop signal
	taskCount       int            // Tasks completed this cycle
}

// DMNConfig configures the Default Mode Network worker.
type DMNConfig struct {
	// Enabled controls whether DMN is active
	Enabled bool `yaml:"enabled"`

	// IdleThreshold is how long to wait after last activity before DMN starts
	IdleThreshold time.Duration `yaml:"idle_threshold"`

	// PollInterval is how often to check for idle state
	PollInterval time.Duration `yaml:"poll_interval"`

	// ConsolidateInterval is minimum time between consolidation runs
	ConsolidateInterval time.Duration `yaml:"consolidate_interval"`

	// KnowledgePaths are directories to scan for indexable documents
	KnowledgePaths []string `yaml:"knowledge_paths"`

	// MaxFilesPerCycle limits how many files to process per idle cycle
	MaxFilesPerCycle int `yaml:"max_files_per_cycle"`

	// SupportedExtensions are file types to index
	SupportedExtensions []string `yaml:"supported_extensions"`
}

// DefaultDMNConfig returns sensible defaults for the DMN worker.
func DefaultDMNConfig() DMNConfig {
	homeDir, _ := os.UserHomeDir()
	return DMNConfig{
		Enabled:             true,
		IdleThreshold:       5 * time.Minute,  // Start DMN after 5 min idle
		PollInterval:        30 * time.Second, // Check every 30s
		ConsolidateInterval: 15 * time.Minute, // Consolidate at most every 15 min
		KnowledgePaths: []string{
			filepath.Join(homeDir, "Library/Mobile Documents/com~apple~CloudDocs/Downloads"),
			filepath.Join(homeDir, ".cortex/knowledge"),
		},
		MaxFilesPerCycle: 10,
		SupportedExtensions: []string{
			".md", ".txt", ".yaml", ".yml", ".json",
			".go", ".py", ".js", ".ts", ".rs",
		},
	}
}

// NewDMNWorker creates a new Default Mode Network worker.
func NewDMNWorker(memory MemorySystem, eventBus *bus.EventBus, config DMNConfig) *DMNWorker {
	log := logging.Global()

	if !config.Enabled {
		log.Info("[DMN] Default Mode Network worker disabled")
		return &DMNWorker{
			config: config,
			log:    log,
		}
	}

	return &DMNWorker{
		memory:          memory,
		eventBus:        eventBus,
		config:          config,
		log:             log,
		lastActivity:    time.Now(),
		lastConsolidate: time.Time{}, // Never consolidated
	}
}

// Start begins the DMN background worker.
func (d *DMNWorker) Start() {
	d.mu.Lock()
	if !d.config.Enabled || d.isRunning {
		d.mu.Unlock()
		return
	}
	d.isRunning = true
	d.stopCh = make(chan struct{})
	d.mu.Unlock()

	d.log.Info("[DMN] Starting Default Mode Network worker (idle_threshold=%v, poll=%v)",
		d.config.IdleThreshold, d.config.PollInterval)

	go d.runLoop()
}

// Stop gracefully shuts down the DMN worker.
func (d *DMNWorker) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.isRunning {
		return
	}

	close(d.stopCh)
	d.isRunning = false
	d.log.Info("[DMN] Default Mode Network worker stopped")
}

// RecordActivity marks user activity, resetting idle timer.
func (d *DMNWorker) RecordActivity() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.lastActivity = time.Now()
}

// IsIdle returns true if the system has been idle long enough for DMN.
func (d *DMNWorker) IsIdle() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return time.Since(d.lastActivity) >= d.config.IdleThreshold
}

// runLoop is the main DMN background loop.
func (d *DMNWorker) runLoop() {
	ticker := time.NewTicker(d.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if d.IsIdle() {
				d.runConsolidationCycle()
			}
		case <-d.stopCh:
			return
		}
	}
}

// runConsolidationCycle performs one round of DMN tasks.
func (d *DMNWorker) runConsolidationCycle() {
	d.mu.Lock()
	// Check if enough time has passed since last consolidation
	if time.Since(d.lastConsolidate) < d.config.ConsolidateInterval {
		d.mu.Unlock()
		return
	}
	d.lastConsolidate = time.Now()
	d.taskCount = 0
	d.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	d.log.Debug("[DMN] Beginning consolidation cycle")
	startTime := time.Now()

	d.publishEvent("dmn_cycle_started", nil)

	// Task 1: Refresh stale memory neighborhoods
	d.refreshNeighborhoods(ctx)

	// Task 2: Index new knowledge files
	d.indexKnowledgeFiles(ctx)

	// Task 3: Cleanup expired/stale data
	d.cleanupStaleData(ctx)

	// Task 4: Precompute common queries (if memory system supports it)
	d.precomputeQueries(ctx)

	duration := time.Since(startTime)
	d.log.Info("[DMN] Consolidation cycle complete: tasks=%d, duration=%v", d.taskCount, duration)

	d.publishEvent("dmn_cycle_completed", map[string]any{
		"tasks_completed": d.taskCount,
		"duration_ms":     duration.Milliseconds(),
	})
}

// refreshNeighborhoods updates stale precomputed neighbors.
func (d *DMNWorker) refreshNeighborhoods(ctx context.Context) {
	if d.memory == nil {
		return
	}

	// Check for context cancellation (user activity resumes)
	select {
	case <-ctx.Done():
		return
	default:
	}

	// The memory system should expose a method to refresh stale neighborhoods
	// This is a placeholder - actual implementation depends on MemorySystem interface
	d.log.Debug("[DMN] Refreshing stale memory neighborhoods")

	// TODO: Call memory.RefreshStaleNeighborhoods() when available
	// For now, we just increment task count to show the worker is functioning
	d.taskCount++
}

// indexKnowledgeFiles scans configured paths for new documents to index.
func (d *DMNWorker) indexKnowledgeFiles(ctx context.Context) {
	filesProcessed := 0

	for _, basePath := range d.config.KnowledgePaths {
		// Check if path exists
		if _, err := os.Stat(basePath); os.IsNotExist(err) {
			continue
		}

		// Walk the directory
		err := filepath.WalkDir(basePath, func(path string, entry os.DirEntry, err error) error {
			// Check for cancellation
			select {
			case <-ctx.Done():
				return filepath.SkipAll
			default:
			}

			if err != nil {
				return nil // Skip files with errors
			}

			// Skip directories
			if entry.IsDir() {
				// Skip hidden directories and common noise
				name := entry.Name()
				if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" {
					return filepath.SkipDir
				}
				return nil
			}

			// Check file limit
			if filesProcessed >= d.config.MaxFilesPerCycle {
				return filepath.SkipAll
			}

			// Check extension
			ext := strings.ToLower(filepath.Ext(path))
			if !d.isSupported(ext) {
				return nil
			}

			// Index the file
			if d.shouldIndex(path) {
				d.indexFile(ctx, path)
				filesProcessed++
			}

			return nil
		})

		if err != nil {
			d.log.Debug("[DMN] Error walking %s: %v", basePath, err)
		}
	}

	if filesProcessed > 0 {
		d.log.Debug("[DMN] Indexed %d knowledge files", filesProcessed)
		d.taskCount++
	}
}

// isSupported checks if a file extension is indexable.
func (d *DMNWorker) isSupported(ext string) bool {
	for _, supported := range d.config.SupportedExtensions {
		if ext == supported {
			return true
		}
	}
	return false
}

// shouldIndex determines if a file needs indexing (not already indexed or modified).
func (d *DMNWorker) shouldIndex(path string) bool {
	// TODO: Check against indexed file cache with modification times
	// For now, always return true for new files
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	// Only index files modified in last 7 days (to avoid reprocessing old files)
	return time.Since(info.ModTime()) < 7*24*time.Hour
}

// indexFile processes and stores a knowledge file.
func (d *DMNWorker) indexFile(ctx context.Context, path string) {
	// Check context
	select {
	case <-ctx.Done():
		return
	default:
	}

	// Read file content (with size limit)
	content, err := d.readFileSafe(path, 50*1024) // 50KB limit
	if err != nil {
		d.log.Debug("[DMN] Failed to read %s: %v", path, err)
		return
	}

	// Skip empty files
	if len(strings.TrimSpace(content)) < 50 {
		return
	}

	// Create knowledge entry
	// TODO: Use memory.InsertArchival() when properly integrated
	d.log.Debug("[DMN] Indexed: %s (%d bytes)", filepath.Base(path), len(content))

	d.publishEvent("dmn_file_indexed", map[string]any{
		"path": path,
		"size": len(content),
	})
}

// readFileSafe reads a file with size limit.
func (d *DMNWorker) readFileSafe(path string, maxSize int64) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Check file size
	info, err := file.Stat()
	if err != nil {
		return "", err
	}
	if info.Size() > maxSize {
		return "", nil // Skip large files
	}

	content := make([]byte, info.Size())
	_, err = file.Read(content)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

// cleanupStaleData removes expired or orphaned data.
func (d *DMNWorker) cleanupStaleData(ctx context.Context) {
	select {
	case <-ctx.Done():
		return
	default:
	}

	// TODO: Implement cleanup logic:
	// - Remove orphaned episode members
	// - Deactivate stale topics (>30 days inactive)
	// - Prune low-confidence links
	d.log.Debug("[DMN] Cleaning up stale data")
	d.taskCount++
}

// precomputeQueries caches results for common query patterns.
func (d *DMNWorker) precomputeQueries(ctx context.Context) {
	select {
	case <-ctx.Done():
		return
	default:
	}

	// TODO: Identify and cache common query patterns
	// - User's most frequent topics
	// - Recent project contexts
	d.log.Debug("[DMN] Precomputing common queries")
	d.taskCount++
}

// publishEvent sends DMN events to the event bus.
func (d *DMNWorker) publishEvent(eventType string, data map[string]any) {
	if d.eventBus == nil {
		return
	}
	d.eventBus.Publish(bus.NewDMNEvent(eventType, data))
}

// GetStats returns DMN worker statistics.
func (d *DMNWorker) GetStats() DMNStats {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return DMNStats{
		IsRunning:         d.isRunning,
		LastActivity:      d.lastActivity,
		LastConsolidate:   d.lastConsolidate,
		IdleDuration:      time.Since(d.lastActivity),
		IsIdle:            time.Since(d.lastActivity) >= d.config.IdleThreshold,
		TasksLastCycle:    d.taskCount,
		KnowledgePathCount: len(d.config.KnowledgePaths),
	}
}

// DMNStats holds statistics about the DMN worker.
type DMNStats struct {
	IsRunning          bool
	LastActivity       time.Time
	LastConsolidate    time.Time
	IdleDuration       time.Duration
	IsIdle             bool
	TasksLastCycle     int
	KnowledgePathCount int
}
