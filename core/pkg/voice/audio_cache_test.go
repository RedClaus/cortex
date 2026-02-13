// Package voice provides voice-related types and utilities for Cortex.
// audio_cache_test.go contains tests for the audio cache (CR-012-C).
package voice

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAudioCache_ComputeVoiceConfigHash_Deterministic(t *testing.T) {
	config := AudioCacheConfig{
		VoiceID:    "am_adam",
		Model:      "kokoro",
		Speed:      1.0,
		SampleRate: 24000,
	}

	cache1 := NewAudioCache(t.TempDir(), config)
	cache2 := NewAudioCache(t.TempDir(), config)

	hash1 := cache1.computeVoiceConfigHash()
	hash2 := cache2.computeVoiceConfigHash()

	assert.Equal(t, hash1, hash2, "same config should produce same hash")
	assert.Contains(t, hash1, "sha256:", "hash should have sha256 prefix")
}

func TestAudioCache_ComputeVoiceConfigHash_ChangesOnVoiceID(t *testing.T) {
	config1 := AudioCacheConfig{VoiceID: "am_adam", Model: "kokoro", Speed: 1.0}
	config2 := AudioCacheConfig{VoiceID: "af_heart", Model: "kokoro", Speed: 1.0}

	cache1 := NewAudioCache(t.TempDir(), config1)
	cache2 := NewAudioCache(t.TempDir(), config2)

	assert.NotEqual(t, cache1.computeVoiceConfigHash(), cache2.computeVoiceConfigHash())
}

func TestAudioCache_ComputeVoiceConfigHash_ChangesOnSpeed(t *testing.T) {
	config1 := AudioCacheConfig{VoiceID: "am_adam", Model: "kokoro", Speed: 1.0}
	config2 := AudioCacheConfig{VoiceID: "am_adam", Model: "kokoro", Speed: 1.2}

	cache1 := NewAudioCache(t.TempDir(), config1)
	cache2 := NewAudioCache(t.TempDir(), config2)

	assert.NotEqual(t, cache1.computeVoiceConfigHash(), cache2.computeVoiceConfigHash())
}

func TestAudioCache_NeedsRegeneration_EmptyCache(t *testing.T) {
	config := DefaultAudioCacheConfig()
	cache := NewAudioCache(t.TempDir(), config)

	assert.True(t, cache.NeedsRegeneration())
}

func TestAudioCache_NeedsRegeneration_ValidCache(t *testing.T) {
	dir := t.TempDir()
	config := AudioCacheConfig{VoiceID: "am_adam", Model: "kokoro", Speed: 1.0, SampleRate: 24000}

	// Create cache and save manifest
	cache := NewAudioCache(dir, config)
	cache.manifest = &CacheManifest{
		Version:         "1.0.0",
		VoiceConfigHash: cache.computeVoiceConfigHash(),
		VoiceID:         "am_adam",
	}
	require.NoError(t, cache.SaveManifest())

	// New cache with same config should not need regeneration
	cache2 := NewAudioCache(dir, config)
	assert.False(t, cache2.NeedsRegeneration())
}

func TestAudioCache_NeedsRegeneration_HashMismatch(t *testing.T) {
	dir := t.TempDir()
	config := AudioCacheConfig{VoiceID: "am_adam", Model: "kokoro", Speed: 1.0}

	// Create cache with manifest
	cache := NewAudioCache(dir, config)
	cache.manifest = &CacheManifest{
		Version:         "1.0.0",
		VoiceConfigHash: "sha256:oldhash",
		VoiceID:         "am_adam",
	}
	require.NoError(t, cache.SaveManifest())

	// New cache with different config should need regeneration
	config2 := AudioCacheConfig{VoiceID: "af_heart", Model: "kokoro", Speed: 1.0}
	cache2 := NewAudioCache(dir, config2)
	assert.True(t, cache2.NeedsRegeneration())
}

func TestAudioCache_ManifestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	config := DefaultAudioCacheConfig()

	cache := NewAudioCache(dir, config)
	cache.manifest = &CacheManifest{
		Version:         "1.0.0",
		VoiceConfigHash: cache.computeVoiceConfigHash(),
		VoiceID:         "am_adam",
		FileCount:       5,
		Phrases: map[string]CachedPhrase{
			"wake/cold/hi.wav": {
				Text:      "Hi!",
				File:      "wake/cold/hi.wav",
				SizeBytes: 1024,
			},
		},
	}

	require.NoError(t, cache.SaveManifest())

	// Load into new cache instance
	cache2 := NewAudioCache(dir, config)
	require.NoError(t, cache2.LoadManifest())

	assert.Equal(t, cache.manifest.VoiceID, cache2.manifest.VoiceID)
	assert.Equal(t, cache.manifest.FileCount, cache2.manifest.FileCount)
	assert.Equal(t, len(cache.manifest.Phrases), len(cache2.manifest.Phrases))
}

func TestAudioCache_GetAudio_NotFound(t *testing.T) {
	config := DefaultAudioCacheConfig()
	cache := NewAudioCache(t.TempDir(), config)

	_, err := cache.GetAudio("nonexistent.wav")
	assert.Error(t, err)
}

func TestAudioCache_GetAudio_FromDisk(t *testing.T) {
	dir := t.TempDir()
	config := DefaultAudioCacheConfig()
	cache := NewAudioCache(dir, config)

	// Create test audio file
	audioPath := filepath.Join(dir, "test.wav")
	testData := []byte("fake audio data")
	require.NoError(t, os.WriteFile(audioPath, testData, 0644))

	// Get audio should load from disk
	data, err := cache.GetAudio("test.wav")
	require.NoError(t, err)
	assert.Equal(t, testData, data)

	// Should be cached in memory now
	assert.Equal(t, 1, cache.FileCount())
}

func TestAudioCache_GetAudio_FromMemory(t *testing.T) {
	dir := t.TempDir()
	config := DefaultAudioCacheConfig()
	cache := NewAudioCache(dir, config)

	// Pre-populate memory cache
	testData := []byte("fake audio data")
	cache.memoryCache["test.wav"] = testData

	// Should get from memory, not disk
	data, err := cache.GetAudio("test.wav")
	require.NoError(t, err)
	assert.Equal(t, testData, data)
}

func TestAudioCache_PreloadAll(t *testing.T) {
	dir := t.TempDir()
	config := DefaultAudioCacheConfig()
	cache := NewAudioCache(dir, config)

	// Create test audio files
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "wake", "cold"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "wake", "cold", "hi.wav"), []byte("hi"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "wake", "cold", "hello.wav"), []byte("hello"), 0644))

	// Preload all
	require.NoError(t, cache.PreloadAll())

	assert.Equal(t, 2, cache.FileCount())
}

func TestAudioCache_CacheSize(t *testing.T) {
	config := DefaultAudioCacheConfig()
	cache := NewAudioCache(t.TempDir(), config)

	cache.memoryCache["a.wav"] = []byte("12345")
	cache.memoryCache["b.wav"] = []byte("123")

	assert.Equal(t, int64(8), cache.CacheSize())
}

func TestAudioCache_Clear(t *testing.T) {
	dir := t.TempDir()
	config := DefaultAudioCacheConfig()
	cache := NewAudioCache(dir, config)

	// Create some files
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.wav"), []byte("test"), 0644))
	cache.memoryCache["test.wav"] = []byte("test")
	cache.manifest = &CacheManifest{Version: "1.0.0"}

	// Clear
	require.NoError(t, cache.Clear())

	assert.Equal(t, 0, cache.FileCount())
	assert.Nil(t, cache.manifest)
	assert.NoDirExists(t, dir)
}

func TestAudioCache_GetRandomFromCategory(t *testing.T) {
	dir := t.TempDir()
	config := DefaultAudioCacheConfig()
	cache := NewAudioCache(dir, config)

	// Create test audio for confused category
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "confused"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "confused", "say_again.wav"), []byte("say again audio"), 0644))

	// Get random from category
	data, text, err := cache.GetRandomFromCategory(CategoryConfused)
	require.NoError(t, err)
	assert.NotEmpty(t, data)
	assert.NotEmpty(t, text)
}

func TestAudioCache_GetConfig(t *testing.T) {
	config := AudioCacheConfig{
		VoiceID:    "test_voice",
		Model:      "test_model",
		Speed:      1.5,
		SampleRate: 22050,
	}
	cache := NewAudioCache(t.TempDir(), config)

	retrieved := cache.GetConfig()
	assert.Equal(t, config.VoiceID, retrieved.VoiceID)
	assert.Equal(t, config.Model, retrieved.Model)
	assert.Equal(t, config.Speed, retrieved.Speed)
	assert.Equal(t, config.SampleRate, retrieved.SampleRate)
}

// MockTTSGenerator implements TTSGenerator for testing.
type MockTTSGenerator struct {
	GeneratedFiles map[string]string // path -> text
	ShouldFail     bool
}

func (m *MockTTSGenerator) SynthesizeToFile(ctx context.Context, text, outputPath, voiceID string) error {
	if m.ShouldFail {
		return assert.AnError
	}
	if m.GeneratedFiles == nil {
		m.GeneratedFiles = make(map[string]string)
	}
	m.GeneratedFiles[outputPath] = text
	// Create empty file
	return os.WriteFile(outputPath, []byte("fake audio for: "+text), 0644)
}

func TestAudioCache_EnsureGenerated(t *testing.T) {
	dir := t.TempDir()
	config := DefaultAudioCacheConfig()
	cache := NewAudioCache(dir, config)

	mockTTS := &MockTTSGenerator{}

	err := cache.EnsureGenerated(context.Background(), mockTTS)
	require.NoError(t, err)

	// Should have generated files
	assert.Greater(t, len(mockTTS.GeneratedFiles), 0)

	// Manifest should be saved
	assert.NotNil(t, cache.manifest)
	assert.Greater(t, cache.manifest.FileCount, 0)
}

func TestAudioCache_EnsureGenerated_SkipsIfValid(t *testing.T) {
	dir := t.TempDir()
	config := DefaultAudioCacheConfig()
	cache := NewAudioCache(dir, config)

	// Create valid manifest
	cache.manifest = &CacheManifest{
		Version:         "1.0.0",
		VoiceConfigHash: cache.computeVoiceConfigHash(),
		VoiceID:         config.VoiceID,
		FileCount:       10,
	}
	require.NoError(t, cache.SaveManifest())

	// Reload cache
	cache2 := NewAudioCache(dir, config)

	mockTTS := &MockTTSGenerator{}
	err := cache2.EnsureGenerated(context.Background(), mockTTS)
	require.NoError(t, err)

	// Should NOT have generated any files
	assert.Equal(t, 0, len(mockTTS.GeneratedFiles))
}
