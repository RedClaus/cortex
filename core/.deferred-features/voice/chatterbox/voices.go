// Package chatterbox - Voice Library Management
// CR-004-A: Voice Library API
//
// KEY DIFFERENCE FROM XTTS:
// - XTTS: Send WAV file with EVERY synthesis request (slow, wasteful)
// - Chatterbox: Upload voice ONCE to library, reference by name forever (efficient)
//
// Audio Requirements for Voice Upload:
// - Format: MP3, WAV, FLAC, M4A, OGG
// - Duration: 5-30 seconds recommended
// - Quality: 16-48kHz sample rate, single speaker, no background noise
package chatterbox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
)

// ═══════════════════════════════════════════════════════════════════════════
// VOICE LIBRARY TYPES
// ═══════════════════════════════════════════════════════════════════════════

// VoiceInfo represents a voice in the library.
type VoiceInfo struct {
	Name             string `json:"name"`
	Filename         string `json:"filename"`
	OriginalFilename string `json:"original_filename"`
	FileExtension    string `json:"file_extension"`
	FileSize         int64  `json:"file_size"`
	UploadDate       string `json:"upload_date"`
	Path             string `json:"path"`
}

// VoiceListResponse from GET /v1/voices.
type VoiceListResponse struct {
	Voices []VoiceInfo `json:"voices"`
	Count  int         `json:"count"`
}

// VoiceUploadResponse from POST /v1/voices.
type VoiceUploadResponse struct {
	Status  string    `json:"status"`
	Voice   VoiceInfo `json:"voice"`
	Message string    `json:"message,omitempty"`
}

// ═══════════════════════════════════════════════════════════════════════════
// VOICE LIBRARY OPERATIONS
// ═══════════════════════════════════════════════════════════════════════════

// GetVoiceLibrary returns all voices in the library.
// Endpoint: GET /v1/voices
func (p *Provider) GetVoiceLibrary(ctx context.Context) (*VoiceListResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", p.config.BaseURL+"/v1/voices", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list voices: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list voices error %d: %s", resp.StatusCode, string(body))
	}

	var result VoiceListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// CreateVoice uploads a new voice to the library.
// Endpoint: POST /v1/voices
//
// Unlike XTTS which required WAV per request, Chatterbox stores voices permanently.
// Upload once, reference by name forever.
//
// Parameters:
//   - name: Voice name (no slashes, colons, question marks)
//   - audioData: Raw audio bytes
//   - filename: Original filename for extension detection
func (p *Provider) CreateVoice(ctx context.Context, name string, audioData []byte, filename string) error {
	// Validate name
	if name == "" {
		return fmt.Errorf("voice name is required")
	}
	for _, c := range name {
		if c == '/' || c == '\\' || c == ':' || c == '?' || c == '"' || c == '\'' || c == '*' || c == '<' || c == '>' || c == '|' {
			return fmt.Errorf("voice name contains invalid character: %c", c)
		}
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add voice name
	if err := writer.WriteField("voice_name", name); err != nil {
		return fmt.Errorf("failed to write voice_name: %w", err)
	}

	// Add audio file
	part, err := writer.CreateFormFile("voice_file", filename)
	if err != nil {
		return fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := part.Write(audioData); err != nil {
		return fmt.Errorf("failed to write audio data: %w", err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to close writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.config.BaseURL+"/v1/voices", body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to upload voice: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)

		// Parse error for better message
		var errResp ErrorResponse
		if json.Unmarshal(bodyBytes, &errResp) == nil && errResp.Error.Message != "" {
			return fmt.Errorf("create voice failed: %s", errResp.Error.Message)
		}

		return fmt.Errorf("create voice error %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// DeleteVoice removes a voice from the library.
// Endpoint: DELETE /v1/voices/{voice_name}
func (p *Provider) DeleteVoice(ctx context.Context, name string) error {
	if name == "" {
		return fmt.Errorf("voice name is required")
	}

	req, err := http.NewRequestWithContext(ctx, "DELETE",
		fmt.Sprintf("%s/v1/voices/%s", p.config.BaseURL, name), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete voice: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete voice error %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// GetVoice retrieves info about a specific voice.
// Endpoint: GET /v1/voices/{voice_name}
func (p *Provider) GetVoice(ctx context.Context, name string) (*VoiceInfo, error) {
	if name == "" {
		return nil, fmt.Errorf("voice name is required")
	}

	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s/v1/voices/%s", p.config.BaseURL, name), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get voice: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("voice '%s' not found in library", name)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get voice error %d: %s", resp.StatusCode, string(body))
	}

	var voice VoiceInfo
	if err := json.NewDecoder(resp.Body).Decode(&voice); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &voice, nil
}

// VoiceExists checks if a voice exists in the library.
func (p *Provider) VoiceExists(ctx context.Context, name string) bool {
	_, err := p.GetVoice(ctx, name)
	return err == nil
}

// EnsureVoiceExists creates a voice if it doesn't already exist.
// Returns true if the voice was created, false if it already existed.
func (p *Provider) EnsureVoiceExists(ctx context.Context, name string, audioData []byte, filename string) (bool, error) {
	if p.VoiceExists(ctx, name) {
		return false, nil
	}

	if err := p.CreateVoice(ctx, name, audioData, filename); err != nil {
		return false, err
	}

	return true, nil
}

// ═══════════════════════════════════════════════════════════════════════════
// VOICE MIGRATION HELPERS
// ═══════════════════════════════════════════════════════════════════════════

// MigrateFromXTTS helps migrate cloned voices from XTTS format to Chatterbox library.
// XTTS stored voices as local files that were sent per-request.
// This uploads them to Chatterbox's permanent voice library.
func (p *Provider) MigrateFromXTTS(ctx context.Context, name string, wavData []byte) error {
	// Check if already migrated
	if p.VoiceExists(ctx, name) {
		return nil // Already in library
	}

	// Upload to Chatterbox library
	return p.CreateVoice(ctx, name, wavData, name+".wav")
}
