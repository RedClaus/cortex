// Package voice provides voice-related types and utilities for Cortex.
// audio_enhancer_test.go provides tests for the audio enhancement client (CR-012-B).
package voice

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultAudioEnhancerConfig(t *testing.T) {
	config := DefaultAudioEnhancerConfig()

	if config.Endpoint == "" {
		t.Error("expected non-empty endpoint")
	}
	if config.AnalyzeEndpoint == "" {
		t.Error("expected non-empty analyze endpoint")
	}
	if config.HealthEndpoint == "" {
		t.Error("expected non-empty health endpoint")
	}
	if config.Timeout == 0 {
		t.Error("expected non-zero timeout")
	}

	// Check default values
	if config.Endpoint != "http://127.0.0.1:8880/v1/audio/enhance" {
		t.Errorf("unexpected endpoint: %s", config.Endpoint)
	}
	if config.Timeout != 30*time.Second {
		t.Errorf("unexpected timeout: %v", config.Timeout)
	}
}

func TestNewAudioEnhancer(t *testing.T) {
	t.Run("default config", func(t *testing.T) {
		enhancer, err := NewAudioEnhancer(AudioEnhancerConfig{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if enhancer == nil {
			t.Fatal("expected non-nil enhancer")
		}
		if enhancer.config.Endpoint == "" {
			t.Error("expected config to have defaults applied")
		}
	})

	t.Run("custom config", func(t *testing.T) {
		config := AudioEnhancerConfig{
			Endpoint:        "http://custom:9999/v1/audio/enhance",
			AnalyzeEndpoint: "http://custom:9999/v1/audio/analyze",
			HealthEndpoint:  "http://custom:9999/health",
			Timeout:         60 * time.Second,
		}
		enhancer, err := NewAudioEnhancer(config)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if enhancer.config.Endpoint != config.Endpoint {
			t.Errorf("expected endpoint %s, got %s", config.Endpoint, enhancer.config.Endpoint)
		}
	})
}

func TestAudioEnhancer_EnhanceModes(t *testing.T) {
	// Test enhance mode constants
	modes := []EnhanceMode{
		EnhanceModeNone,
		EnhanceModeDenoise,
		EnhanceModeFull,
		EnhanceModeAuto,
	}

	expected := []string{"none", "denoise", "full", "auto"}

	for i, mode := range modes {
		if string(mode) != expected[i] {
			t.Errorf("mode %d: expected %s, got %s", i, expected[i], string(mode))
		}
	}
}

func TestAudioEnhancer_IsAvailable(t *testing.T) {
	t.Run("unavailable when server not running", func(t *testing.T) {
		config := AudioEnhancerConfig{
			Endpoint:        "http://localhost:19999/v1/audio/enhance",
			AnalyzeEndpoint: "http://localhost:19999/v1/audio/analyze",
			HealthEndpoint:  "http://localhost:19999/health",
			Timeout:         1 * time.Second,
		}
		enhancer, err := NewAudioEnhancer(config)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if enhancer.IsAvailable() {
			t.Error("expected IsAvailable to return false for unreachable server")
		}
	})

	t.Run("available when server responds healthy", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/health" {
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"status":  "healthy",
					"version": "2.1.0",
				})
			}
		}))
		defer server.Close()

		config := AudioEnhancerConfig{
			Endpoint:        server.URL + "/v1/audio/enhance",
			AnalyzeEndpoint: server.URL + "/v1/audio/analyze",
			HealthEndpoint:  server.URL + "/health",
			Timeout:         5 * time.Second,
		}
		enhancer, err := NewAudioEnhancer(config)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !enhancer.IsAvailable() {
			t.Error("expected IsAvailable to return true for healthy server")
		}
	})
}

func TestAudioEnhancer_AnalyzeFile(t *testing.T) {
	// Create test audio file
	tmpDir := t.TempDir()
	audioPath := filepath.Join(tmpDir, "test.wav")
	if err := os.WriteFile(audioPath, []byte("fake audio data"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	t.Run("successful analysis", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/health":
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]interface{}{"status": "healthy"})
			case "/v1/audio/analyze":
				// Parse multipart form
				if err := r.ParseMultipartForm(10 << 20); err != nil {
					t.Errorf("failed to parse form: %v", err)
				}
				file, _, err := r.FormFile("file")
				if err != nil {
					t.Errorf("failed to get file: %v", err)
				}
				file.Close()

				json.NewEncoder(w).Encode(AnalyzeResult{
					NoiseLevel:     0.35,
					Recommendation: EnhanceModeDenoise,
					Reason:         "Moderate noise detected",
					Duration:       5.0,
					SampleRate:     44100,
				})
			}
		}))
		defer server.Close()

		config := AudioEnhancerConfig{
			Endpoint:        server.URL + "/v1/audio/enhance",
			AnalyzeEndpoint: server.URL + "/v1/audio/analyze",
			HealthEndpoint:  server.URL + "/health",
			Timeout:         5 * time.Second,
		}
		enhancer, err := NewAudioEnhancer(config)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		result, err := enhancer.AnalyzeFile(context.Background(), audioPath)
		if err != nil {
			t.Fatalf("analyze failed: %v", err)
		}

		if result.NoiseLevel != 0.35 {
			t.Errorf("expected noise level 0.35, got %f", result.NoiseLevel)
		}
		if result.Recommendation != EnhanceModeDenoise {
			t.Errorf("expected recommendation denoise, got %s", result.Recommendation)
		}
	})

	t.Run("file not found", func(t *testing.T) {
		enhancer, _ := NewAudioEnhancer(AudioEnhancerConfig{})
		_, err := enhancer.AnalyzeFile(context.Background(), "/nonexistent/file.wav")
		if err == nil {
			t.Error("expected error for nonexistent file")
		}
	})
}

func TestAudioEnhancer_EnhanceFile(t *testing.T) {
	// Create test audio file
	tmpDir := t.TempDir()
	audioPath := filepath.Join(tmpDir, "test.wav")
	audioData := []byte("fake audio data for testing")
	if err := os.WriteFile(audioPath, audioData, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	t.Run("successful enhancement", func(t *testing.T) {
		enhancedData := []byte("enhanced audio data")

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/health":
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]interface{}{"status": "healthy"})
			case "/v1/audio/enhance":
				// Parse multipart form
				if err := r.ParseMultipartForm(10 << 20); err != nil {
					t.Errorf("failed to parse form: %v", err)
				}

				// Check mode field
				mode := r.FormValue("mode")
				if mode == "" {
					mode = "auto"
				}

				file, _, err := r.FormFile("file")
				if err != nil {
					t.Errorf("failed to get file: %v", err)
				}
				file.Close()

				// Set response headers
				w.Header().Set("X-Audio-Duration", "5.0")
				w.Header().Set("X-Enhance-Mode", mode)
				w.Header().Set("Content-Type", "audio/wav")

				// Write enhanced audio
				w.Write(enhancedData)
			}
		}))
		defer server.Close()

		config := AudioEnhancerConfig{
			Endpoint:        server.URL + "/v1/audio/enhance",
			AnalyzeEndpoint: server.URL + "/v1/audio/analyze",
			HealthEndpoint:  server.URL + "/health",
			Timeout:         5 * time.Second,
		}
		enhancer, err := NewAudioEnhancer(config)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		result, err := enhancer.EnhanceFile(context.Background(), audioPath, EnhanceModeDenoise)
		if err != nil {
			t.Fatalf("enhance failed: %v", err)
		}

		// Check result
		if result.AudioPath == "" {
			t.Error("expected non-empty audio path")
		}
		if result.ProcessingTimeMs < 0 {
			t.Error("expected non-negative processing time")
		}

		// Verify output file was created
		if _, err := os.Stat(result.AudioPath); err != nil {
			t.Errorf("output file not found: %v", err)
		}

		// Verify content
		content, err := os.ReadFile(result.AudioPath)
		if err != nil {
			t.Fatalf("failed to read output: %v", err)
		}
		if string(content) != string(enhancedData) {
			t.Error("output content mismatch")
		}

		// Cleanup
		os.Remove(result.AudioPath)
	})

	t.Run("server error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/health":
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]interface{}{"status": "healthy"})
			case "/v1/audio/enhance":
				w.WriteHeader(http.StatusInternalServerError)
				io.WriteString(w, "enhancement failed: model not loaded")
			}
		}))
		defer server.Close()

		config := AudioEnhancerConfig{
			Endpoint:        server.URL + "/v1/audio/enhance",
			AnalyzeEndpoint: server.URL + "/v1/audio/analyze",
			HealthEndpoint:  server.URL + "/health",
			Timeout:         5 * time.Second,
		}
		enhancer, err := NewAudioEnhancer(config)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		_, err = enhancer.EnhanceFile(context.Background(), audioPath, EnhanceModeFull)
		if err == nil {
			t.Error("expected error for server failure")
		}
	})
}

func TestGetAudioEnhancer(t *testing.T) {
	enhancer1, err := GetAudioEnhancer()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if enhancer1 == nil {
		t.Fatal("expected non-nil enhancer")
	}

	// Should return same instance (singleton)
	enhancer2, err := GetAudioEnhancer()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if enhancer1 != enhancer2 {
		t.Error("expected same instance from GetAudioEnhancer")
	}
}
