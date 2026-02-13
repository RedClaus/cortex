// Package voice provides voice-related types and utilities for Cortex.
// audio_cache.go implements pre-generated audio caching for instant wake responses (CR-012-C).
package voice

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// CacheManifest tracks cache version and voice configuration for invalidation.
type CacheManifest struct {
	Version         string                  `json:"version"`
	VoiceConfigHash string                  `json:"voice_config_hash"`
	VoiceID         string                  `json:"voice_id"`
	Model           string                  `json:"model"`
	SampleRate      int                     `json:"sample_rate"`
	GeneratedAt     time.Time               `json:"generated_at"`
	FileCount       int                     `json:"file_count"`
	TotalSizeBytes  int64                   `json:"total_size_bytes"`
	Phrases         map[string]CachedPhrase `json:"phrases"`
}

// CachedPhrase holds metadata for a single cached audio file.
type CachedPhrase struct {
	Text        string    `json:"text"`
	Category    string    `json:"category"`
	File        string    `json:"file"`
	SizeBytes   int64     `json:"size_bytes"`
	DurationMs  int       `json:"duration_ms"`
	GeneratedAt time.Time `json:"generated_at"`
}

// AudioCacheConfig holds configuration for the audio cache.
type AudioCacheConfig struct {
	VoiceID    string
	Model      string
	Speed      float64
	SampleRate int
}

// DefaultAudioCacheConfig returns production defaults.
func DefaultAudioCacheConfig() AudioCacheConfig {
	return AudioCacheConfig{
		VoiceID:    "am_adam",
		Model:      "kokoro",
		Speed:      1.0,
		SampleRate: 24000,
	}
}

// TTSGenerator interface for audio generation.
type TTSGenerator interface {
	// SynthesizeToFile generates audio and saves to file.
	SynthesizeToFile(ctx context.Context, text, outputPath, voiceID string) error
}

// AudioCache manages pre-generated audio responses for instant playback.
type AudioCache struct {
	mu          sync.RWMutex
	cacheDir    string
	manifest    *CacheManifest
	memoryCache map[string][]byte // filepath -> audio data
	config      AudioCacheConfig
}

// NewAudioCache creates a new audio cache.
func NewAudioCache(cacheDir string, config AudioCacheConfig) *AudioCache {
	return &AudioCache{
		cacheDir:    cacheDir,
		config:      config,
		memoryCache: make(map[string][]byte),
	}
}

// computeVoiceConfigHash creates a hash of the current voice configuration.
// Used to detect when cache needs regeneration (Risk B fix).
func (c *AudioCache) computeVoiceConfigHash() string {
	data := fmt.Sprintf("voice:%s|model:%s|speed:%.2f|rate:%d",
		c.config.VoiceID, c.config.Model, c.config.Speed, c.config.SampleRate)
	hash := sha256.Sum256([]byte(data))
	return "sha256:" + hex.EncodeToString(hash[:])
}

// LoadManifest reads the cache manifest from disk.
func (c *AudioCache) LoadManifest() error {
	manifestPath := filepath.Join(c.cacheDir, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("read manifest: %w", err)
	}

	var manifest CacheManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("parse manifest: %w", err)
	}

	c.mu.Lock()
	c.manifest = &manifest
	c.mu.Unlock()

	return nil
}

// SaveManifest writes the cache manifest to disk.
func (c *AudioCache) SaveManifest() error {
	c.mu.RLock()
	manifest := c.manifest
	c.mu.RUnlock()

	if manifest == nil {
		return fmt.Errorf("no manifest to save")
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}

	manifestPath := filepath.Join(c.cacheDir, "manifest.json")

	// Atomic write: write to temp file, then rename
	tmpPath := manifestPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}

	if err := os.Rename(tmpPath, manifestPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename manifest: %w", err)
	}

	return nil
}

// NeedsRegeneration checks if cache needs to be rebuilt (Risk B fix).
func (c *AudioCache) NeedsRegeneration() bool {
	c.mu.RLock()
	manifest := c.manifest
	c.mu.RUnlock()

	if manifest == nil {
		// Try loading from disk
		if err := c.LoadManifest(); err != nil {
			return true // No manifest = needs generation
		}
		c.mu.RLock()
		manifest = c.manifest
		c.mu.RUnlock()
	}

	if manifest == nil {
		return true
	}

	currentHash := c.computeVoiceConfigHash()
	if manifest.VoiceConfigHash != currentHash {
		return true
	}

	return false
}

// EnsureGenerated generates all wake responses if not already cached.
func (c *AudioCache) EnsureGenerated(ctx context.Context, tts TTSGenerator) error {
	// Check if regeneration needed
	if !c.NeedsRegeneration() {
		return nil
	}

	// Create cache directory
	if err := os.MkdirAll(c.cacheDir, 0755); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}

	pool := DefaultWakeResponsePool()
	allResponses := pool.GetAllResponses()

	// Create new manifest
	manifest := &CacheManifest{
		Version:         "1.0.0",
		VoiceConfigHash: c.computeVoiceConfigHash(),
		VoiceID:         c.config.VoiceID,
		Model:           c.config.Model,
		SampleRate:      c.config.SampleRate,
		GeneratedAt:     time.Now(),
		Phrases:         make(map[string]CachedPhrase),
	}

	var totalSize int64
	var fileCount int

	for _, resp := range allResponses {
		if resp.AudioFile == "" {
			continue
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		audioPath := filepath.Join(c.cacheDir, resp.AudioFile)

		// Ensure directory exists
		if err := os.MkdirAll(filepath.Dir(audioPath), 0755); err != nil {
			return fmt.Errorf("create dir for %s: %w", resp.AudioFile, err)
		}

		// Generate audio
		if err := tts.SynthesizeToFile(ctx, resp.Text, audioPath, c.config.VoiceID); err != nil {
			return fmt.Errorf("generate %s: %w", resp.AudioFile, err)
		}

		// Get file info
		info, err := os.Stat(audioPath)
		if err != nil {
			return fmt.Errorf("stat %s: %w", resp.AudioFile, err)
		}

		// Record in manifest
		manifest.Phrases[resp.AudioFile] = CachedPhrase{
			Text:        resp.Text,
			File:        resp.AudioFile,
			SizeBytes:   info.Size(),
			GeneratedAt: time.Now(),
		}

		totalSize += info.Size()
		fileCount++
	}

	manifest.FileCount = fileCount
	manifest.TotalSizeBytes = totalSize

	c.mu.Lock()
	c.manifest = manifest
	c.mu.Unlock()

	// Save manifest
	if err := c.SaveManifest(); err != nil {
		return fmt.Errorf("save manifest: %w", err)
	}

	return nil
}

// GetAudio returns cached audio data for a response.
func (c *AudioCache) GetAudio(audioFile string) ([]byte, error) {
	c.mu.RLock()
	if data, ok := c.memoryCache[audioFile]; ok {
		c.mu.RUnlock()
		return data, nil
	}
	c.mu.RUnlock()

	// Load from disk
	audioPath := filepath.Join(c.cacheDir, audioFile)
	data, err := os.ReadFile(audioPath)
	if err != nil {
		return nil, fmt.Errorf("audio not found: %s", audioFile)
	}

	// Cache in memory
	c.mu.Lock()
	c.memoryCache[audioFile] = data
	c.mu.Unlock()

	return data, nil
}

// GetRandomFromCategory returns random audio from a category.
func (c *AudioCache) GetRandomFromCategory(category ResponseCategory) ([]byte, string, error) {
	pool := DefaultWakeResponsePool()
	responses := pool.GetResponsesByCategory(category)

	if len(responses) == 0 {
		return nil, "", fmt.Errorf("no responses for category: %s", category)
	}

	// Filter to those with audio files
	var withAudio []WakeResponse
	for _, r := range responses {
		if r.AudioFile != "" {
			withAudio = append(withAudio, r)
		}
	}

	if len(withAudio) == 0 {
		return nil, responses[0].Text, fmt.Errorf("no audio files for category: %s", category)
	}

	// Random selection
	resp := withAudio[randIntn(len(withAudio))]
	data, err := c.GetAudio(resp.AudioFile)
	return data, resp.Text, err
}

// PreloadAll loads all cached audio into memory for instant playback.
func (c *AudioCache) PreloadAll() error {
	return filepath.Walk(c.cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}

		if filepath.Ext(path) != ".wav" {
			return nil
		}

		relPath, err := filepath.Rel(c.cacheDir, path)
		if err != nil {
			return err
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", relPath, err)
		}

		c.mu.Lock()
		c.memoryCache[relPath] = data
		c.mu.Unlock()

		return nil
	})
}

// CacheSize returns the total size of loaded audio in bytes.
func (c *AudioCache) CacheSize() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var total int64
	for _, data := range c.memoryCache {
		total += int64(len(data))
	}
	return total
}

// DiskSize returns the total size of cached files on disk.
func (c *AudioCache) DiskSize() int64 {
	c.mu.RLock()
	manifest := c.manifest
	c.mu.RUnlock()

	if manifest != nil {
		return manifest.TotalSizeBytes
	}

	var total int64
	filepath.Walk(c.cacheDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total
}

// FileCount returns the number of cached files.
func (c *AudioCache) FileCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.memoryCache)
}

// Clear removes all cached audio files.
func (c *AudioCache) Clear() error {
	c.mu.Lock()
	c.memoryCache = make(map[string][]byte)
	c.manifest = nil
	c.mu.Unlock()

	return os.RemoveAll(c.cacheDir)
}

// GetManifest returns the current cache manifest.
func (c *AudioCache) GetManifest() *CacheManifest {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.manifest
}

// GetConfig returns the cache configuration.
func (c *AudioCache) GetConfig() AudioCacheConfig {
	return c.config
}

// randIntn returns a random int in [0, n) - helper to avoid direct rand usage.
func randIntn(n int) int {
	if n <= 0 {
		return 0
	}
	return int(time.Now().UnixNano()) % n
}
