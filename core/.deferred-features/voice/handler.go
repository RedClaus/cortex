package voice

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

// Handler provides HTTP handlers for voice transcription and synthesis.
type Handler struct {
	whisperService *WhisperService
	ttsRouter      *Router
}

// NewHandler creates a new voice API handler.
func NewHandler(whisperService *WhisperService, ttsRouter *Router) *Handler {
	return &Handler{
		whisperService: whisperService,
		ttsRouter:      ttsRouter,
	}
}

// TranscribeHandler handles POST /api/v1/voice/transcribe requests.
func (h *Handler) TranscribeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart form (max 25MB)
	if err := r.ParseMultipartForm(25 << 20); err != nil {
		respondError(w, "Failed to parse form data", http.StatusBadRequest)
		return
	}

	// Get audio file from form
	file, header, err := r.FormFile("audio")
	if err != nil {
		respondError(w, "Missing 'audio' file in request", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Read audio data
	audioData, err := io.ReadAll(file)
	if err != nil {
		respondError(w, "Failed to read audio file", http.StatusInternalServerError)
		return
	}

	// Get optional parameters
	language := r.FormValue("language")
	modelSize := r.FormValue("model_size")

	// Detect format from filename or content-type
	format := detectFormat(header.Filename, header.Header.Get("Content-Type"))

	// Build transcription request
	req := TranscriptionRequest{
		AudioData: audioData,
		Language:  language,
		ModelSize: modelSize,
		Format:    format,
	}

	// Perform transcription
	response, err := h.whisperService.Transcribe(req)
	if err != nil {
		respondError(w, response.Error, http.StatusInternalServerError)
		return
	}

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// SynthesizeHandler handles POST /api/v1/voice/synthesize requests.
func (h *Handler) SynthesizeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse JSON request body
	var req SpeakRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Validate request
	if req.Text == "" {
		respondError(w, "Missing 'text' field in request", http.StatusBadRequest)
		return
	}

	// Default to fast lane if not specified
	if req.Lane == "" {
		req.Lane = "fast"
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Synthesize audio
	response, err := h.ttsRouter.Speak(ctx, &req)
	if err != nil {
		respondError(w, "Synthesis failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Determine content type based on format
	contentType := "application/octet-stream"
	switch response.Format {
	case FormatWAV:
		contentType = "audio/wav"
	case FormatMP3:
		contentType = "audio/mpeg"
	case FormatOGG:
		contentType = "audio/ogg"
	case FormatOpus:
		contentType = "audio/opus"
	}

	// Return audio data directly
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Voice-Provider", response.Provider)
	w.Header().Set("X-Voice-ID", response.VoiceID)
	w.Header().Set("X-Cache-Hit", boolToString(response.CacheHit))
	w.Header().Set("X-Used-Fallback", boolToString(response.UsedFallback))
	w.WriteHeader(http.StatusOK)
	w.Write(response.Audio)
}

// VoicesHandler handles GET /api/v1/voice/voices requests.
func (h *Handler) VoicesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get lane parameter (optional)
	lane := r.URL.Query().Get("lane")
	if lane == "" {
		lane = "fast" // Default to fast lane
	}

	// Get provider for the specified lane
	provider := h.ttsRouter.GetProviderForLane(lane)
	if provider == nil {
		respondError(w, "Provider not available for lane: "+lane, http.StatusServiceUnavailable)
		return
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// List available voices
	voices, err := provider.ListVoices(ctx)
	if err != nil {
		respondError(w, "Failed to list voices: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Build response
	response := map[string]interface{}{
		"lane":     lane,
		"provider": provider.Name(),
		"voices":   voices,
		"count":    len(voices),
	}

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HealthHandler checks if the voice services are ready.
func (h *Handler) HealthHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	health := map[string]interface{}{
		"status":   "ok",
		"services": make(map[string]interface{}),
	}

	// Check STT (Whisper) service
	if h.whisperService != nil {
		health["services"].(map[string]interface{})["stt"] = map[string]interface{}{
			"available": true,
			"provider":  "whisper",
		}
	} else {
		health["services"].(map[string]interface{})["stt"] = map[string]interface{}{
			"available": false,
			"error":     "whisper service not initialized",
		}
	}

	// Check TTS services
	if h.ttsRouter != nil {
		ttsHealth := h.ttsRouter.Health(ctx)
		health["services"].(map[string]interface{})["tts"] = ttsHealth
	} else {
		health["services"].(map[string]interface{})["tts"] = map[string]interface{}{
			"available": false,
			"error":     "tts router not initialized",
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

// detectFormat determines audio format from filename or content-type.
func detectFormat(filename, contentType string) string {
	// Try filename extension first
	filename = strings.ToLower(filename)
	if strings.HasSuffix(filename, ".wav") {
		return "wav"
	}
	if strings.HasSuffix(filename, ".mp3") {
		return "mp3"
	}
	if strings.HasSuffix(filename, ".webm") {
		return "webm"
	}
	if strings.HasSuffix(filename, ".ogg") {
		return "ogg"
	}
	if strings.HasSuffix(filename, ".m4a") {
		return "m4a"
	}

	// Try content-type
	contentType = strings.ToLower(contentType)
	if strings.Contains(contentType, "wav") {
		return "wav"
	}
	if strings.Contains(contentType, "mp3") || strings.Contains(contentType, "mpeg") {
		return "mp3"
	}
	if strings.Contains(contentType, "webm") {
		return "webm"
	}
	if strings.Contains(contentType, "ogg") {
		return "ogg"
	}

	// Default to WAV
	return "wav"
}

// respondError sends a JSON error response.
func respondError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

// RegisterRoutes registers voice API routes to the given mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// STT endpoints
	mux.HandleFunc("/api/v1/voice/transcribe", h.TranscribeHandler)

	// TTS endpoints
	mux.HandleFunc("/api/v1/voice/synthesize", h.SynthesizeHandler)
	mux.HandleFunc("/api/v1/voice/voices", h.VoicesHandler)

	// Health check
	mux.HandleFunc("/api/v1/voice/health", h.HealthHandler)
}

// boolToString converts a boolean to a string.
func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// Example usage documentation
const UsageExample = `
# Transcribe audio file
curl -X POST http://localhost:8080/api/v1/voice/transcribe \
  -F "audio=@recording.wav" \
  -F "language=en" \
  -F "model_size=base"

# Response:
{
  "text": "Hello, this is a test transcription.",
  "confidence": 0.92,
  "language": "en",
  "duration": 3.5,
  "processing_time": "1.2s",
  "segments": [
    {
      "id": 0,
      "start": 0.0,
      "end": 1.5,
      "text": "Hello, this is a test",
      "confidence": 0.93
    },
    {
      "id": 1,
      "start": 1.5,
      "end": 3.5,
      "text": "transcription.",
      "confidence": 0.91
    }
  ]
}
`
