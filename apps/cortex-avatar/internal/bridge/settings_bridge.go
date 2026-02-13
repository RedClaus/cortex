// Package bridge provides settings management for CortexAvatar
package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/normanking/cortexavatar/internal/config"
	"github.com/normanking/cortexavatar/internal/devices"
	"github.com/rs/zerolog"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// DeviceInfo represents an audio/video device
type DeviceInfo struct {
	DeviceID string `json:"deviceId"`
	Label    string `json:"label"`
	Kind     string `json:"kind"` // audioinput, audiooutput, videoinput
	GroupID  string `json:"groupId,omitempty"`
}

// SettingsData represents all configurable settings
type SettingsData struct {
	// Audio settings
	MicrophoneID string  `json:"microphoneId"`
	SpeakerID    string  `json:"speakerId"`
	VADThreshold float64 `json:"vadThreshold"`
	OutputVolume int     `json:"outputVolume"` // 0-100

	// Video settings
	CameraID    string `json:"cameraId"`
	MaxFPS      int    `json:"maxFps"`
	JPEGQuality int    `json:"jpegQuality"`

	// Connection settings
	ServerURL        string `json:"serverUrl"`
	UseFrontierModel bool   `json:"useFrontierModel"` // Use cloud AI instead of local Ollama

	// Avatar settings
	Persona         string `json:"persona"`
	VoiceID         string `json:"voiceId"`         // TTS voice ID
	ReasoningFilter int    `json:"reasoningFilter"` // 0-100: how much reasoning/thinking to filter from TTS
}

// VoiceInfo represents an available TTS voice
type VoiceInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Provider string `json:"provider"` // openai, kokoro, browser
	Gender   string `json:"gender"`   // male, female, neutral
}

// SettingsBridge exposes settings methods to the frontend
type SettingsBridge struct {
	ctx    context.Context
	cfg    *config.Config
	logger zerolog.Logger
}

// NewSettingsBridge creates a new settings bridge
func NewSettingsBridge(cfg *config.Config, logger zerolog.Logger) *SettingsBridge {
	return &SettingsBridge{
		cfg:    cfg,
		logger: logger.With().Str("component", "settings").Logger(),
	}
}

// Bind sets the Wails runtime context
func (b *SettingsBridge) Bind(ctx context.Context) {
	b.ctx = ctx
}

// GetSettings returns current settings
func (b *SettingsBridge) GetSettings() SettingsData {
	return SettingsData{
		MicrophoneID:     b.cfg.Audio.InputDevice,
		SpeakerID:        b.cfg.Audio.OutputDevice,
		VADThreshold:     b.cfg.Audio.VADThreshold,
		OutputVolume:     b.cfg.Audio.OutputVolume,
		CameraID:         b.cfg.Vision.CameraDevice,
		MaxFPS:           1, // Default 1 FPS for vision
		JPEGQuality:      b.cfg.Vision.JPEGQuality,
		ServerURL:        b.cfg.A2A.ServerURL,
		UseFrontierModel: b.cfg.A2A.UseFrontierModel,
		Persona:          b.cfg.Avatar.Persona,
		VoiceID:          b.cfg.TTS.VoiceID,
		ReasoningFilter:  b.cfg.TTS.ReasoningFilter,
	}
}

// SaveSettings saves all settings
func (b *SettingsBridge) SaveSettings(settings SettingsData) error {
	b.cfg.Audio.InputDevice = settings.MicrophoneID
	b.cfg.Audio.OutputDevice = settings.SpeakerID
	b.cfg.Audio.VADThreshold = settings.VADThreshold
	b.cfg.Audio.OutputVolume = settings.OutputVolume
	b.cfg.Vision.CameraDevice = settings.CameraID
	b.cfg.Vision.JPEGQuality = settings.JPEGQuality
	b.cfg.A2A.ServerURL = settings.ServerURL
	b.cfg.A2A.UseFrontierModel = settings.UseFrontierModel
	b.cfg.Avatar.Persona = settings.Persona
	b.cfg.TTS.VoiceID = settings.VoiceID
	b.cfg.TTS.ReasoningFilter = settings.ReasoningFilter

	if err := config.Save(b.cfg); err != nil {
		b.logger.Error().Err(err).Msg("Failed to save settings")
		return err
	}

	// If frontier model setting changed, update CortexBrain
	if settings.UseFrontierModel {
		go b.setCortexBrainProvider("anthropic")
	} else {
		go b.setCortexBrainProvider("ollama")
	}

	b.logger.Info().Bool("useFrontier", settings.UseFrontierModel).Msg("Settings saved")
	runtime.EventsEmit(b.ctx, "settings:saved", settings)
	return nil
}

// SetVolume sets the output volume
func (b *SettingsBridge) SetVolume(volume int) error {
	if volume < 0 {
		volume = 0
	}
	if volume > 100 {
		volume = 100
	}
	b.cfg.Audio.OutputVolume = volume
	if err := config.Save(b.cfg); err != nil {
		return err
	}
	b.logger.Info().Int("volume", volume).Msg("Volume set")
	runtime.EventsEmit(b.ctx, "settings:volume_changed", volume)
	return nil
}

// SetMicrophone sets the microphone device
func (b *SettingsBridge) SetMicrophone(deviceID string) error {
	b.cfg.Audio.InputDevice = deviceID
	if err := config.Save(b.cfg); err != nil {
		return err
	}
	b.logger.Info().Str("device", deviceID).Msg("Microphone set")
	runtime.EventsEmit(b.ctx, "settings:microphone_changed", deviceID)
	return nil
}

// SetSpeaker sets the speaker device
func (b *SettingsBridge) SetSpeaker(deviceID string) error {
	b.cfg.Audio.OutputDevice = deviceID
	if err := config.Save(b.cfg); err != nil {
		return err
	}
	b.logger.Info().Str("device", deviceID).Msg("Speaker set")
	runtime.EventsEmit(b.ctx, "settings:speaker_changed", deviceID)
	return nil
}

// SetCamera sets the camera device
func (b *SettingsBridge) SetCamera(deviceID string) error {
	b.cfg.Vision.CameraDevice = deviceID
	if err := config.Save(b.cfg); err != nil {
		return err
	}
	b.logger.Info().Str("device", deviceID).Msg("Camera set")
	runtime.EventsEmit(b.ctx, "settings:camera_changed", deviceID)
	return nil
}

// SetVADThreshold sets the voice activity detection threshold
func (b *SettingsBridge) SetVADThreshold(threshold float64) error {
	b.cfg.Audio.VADThreshold = threshold
	if err := config.Save(b.cfg); err != nil {
		return err
	}
	b.logger.Info().Float64("threshold", threshold).Msg("VAD threshold set")
	runtime.EventsEmit(b.ctx, "settings:vad_threshold_changed", threshold)
	return nil
}

// SetServerURL sets the CortexBrain server URL
func (b *SettingsBridge) SetServerURL(url string) error {
	b.cfg.A2A.ServerURL = url
	if err := config.Save(b.cfg); err != nil {
		return err
	}
	b.logger.Info().Str("url", url).Msg("Server URL set")
	runtime.EventsEmit(b.ctx, "settings:server_url_changed", url)
	return nil
}

// SetPersona sets the avatar persona and updates voice to match
func (b *SettingsBridge) SetPersona(personaID string) error {
	b.cfg.Avatar.Persona = personaID
	b.cfg.User.PersonaID = personaID // Also update for A2A messages

	// Update voice to match persona
	persona := config.GetPersona(personaID)
	if persona != nil {
		b.cfg.TTS.VoiceID = persona.VoiceID
		b.logger.Info().
			Str("persona", personaID).
			Str("voice", persona.VoiceID).
			Msg("Persona and voice set")
	}

	if err := config.Save(b.cfg); err != nil {
		return err
	}
	runtime.EventsEmit(b.ctx, "settings:persona_changed", personaID)
	return nil
}

// GetPersonas returns available personas
func (b *SettingsBridge) GetPersonas() []config.Persona {
	return config.AvailablePersonas()
}

// GetVoices returns available TTS voices
func (b *SettingsBridge) GetVoices() []VoiceInfo {
	voices := []VoiceInfo{
		// ElevenLabs voices (high-quality, free tier)
		{ID: "21m00Tcm4TlvDq8ikWAM", Name: "Rachel (Female, Calm)", Provider: "elevenlabs", Gender: "female"},
		{ID: "EXAVITQu4vr4xnSDxMaL", Name: "Bella (Female, Soft)", Provider: "elevenlabs", Gender: "female"},
		{ID: "MF3mGyEYCl7XYWbV9V6O", Name: "Emily (Female, Calm)", Provider: "elevenlabs", Gender: "female"},
		{ID: "ErXwobaYiN019PkySvjV", Name: "Antoni (Male, Well-rounded)", Provider: "elevenlabs", Gender: "male"},
		{ID: "VR6AewLTigWG4xSOukaG", Name: "Arnold (Male, Crisp)", Provider: "elevenlabs", Gender: "male"},
		{ID: "TxGEqnHWrfWFTfGW9XjX", Name: "Josh (Male, Deep)", Provider: "elevenlabs", Gender: "male"},
		// OpenAI voices (natural, high-quality)
		{ID: "nova", Name: "Nova (Female, Warm)", Provider: "openai", Gender: "female"},
		{ID: "shimmer", Name: "Shimmer (Female, Clear)", Provider: "openai", Gender: "female"},
		{ID: "alloy", Name: "Alloy (Neutral)", Provider: "openai", Gender: "neutral"},
		{ID: "echo", Name: "Echo (Male, Warm)", Provider: "openai", Gender: "male"},
		{ID: "onyx", Name: "Onyx (Male, Deep)", Provider: "openai", Gender: "male"},
		{ID: "fable", Name: "Fable (British)", Provider: "openai", Gender: "neutral"},
	}
	b.logger.Info().Int("count", len(voices)).Msg("Listed available voices")
	return voices
}

// SetVoice sets the TTS voice
func (b *SettingsBridge) SetVoice(voiceID string) error {
	b.cfg.TTS.VoiceID = voiceID
	if err := config.Save(b.cfg); err != nil {
		return err
	}
	b.logger.Info().Str("voice", voiceID).Msg("Voice set")
	runtime.EventsEmit(b.ctx, "settings:voice_changed", voiceID)
	return nil
}

// SetReasoningFilter sets how much reasoning/thinking to filter from TTS
// 0 = hear everything (no filter), 100 = only hear final answers (max filter)
func (b *SettingsBridge) SetReasoningFilter(level int) error {
	if level < 0 {
		level = 0
	}
	if level > 100 {
		level = 100
	}
	b.cfg.TTS.ReasoningFilter = level
	if err := config.Save(b.cfg); err != nil {
		return err
	}
	b.logger.Info().Int("level", level).Msg("Reasoning filter set")
	runtime.EventsEmit(b.ctx, "settings:reasoning_filter_changed", level)
	return nil
}

// GetReasoningFilter returns the current reasoning filter level
func (b *SettingsBridge) GetReasoningFilter() int {
	return b.cfg.TTS.ReasoningFilter
}

// TestVoice plays a sample of the specified voice with the given text
func (b *SettingsBridge) TestVoice(voiceID string, text string) bool {
	b.logger.Info().Str("voice", voiceID).Str("text", text).Msg("Testing voice")
	runtime.EventsEmit(b.ctx, "settings:test_voice", map[string]string{
		"voiceId": voiceID,
		"text":    text,
	})
	return true
}

// GetConfig returns the full configuration (for debugging)
func (b *SettingsBridge) GetConfig() *config.Config {
	return b.cfg
}

// TestMicrophone tests the microphone (frontend will handle actual test)
func (b *SettingsBridge) TestMicrophone() bool {
	runtime.EventsEmit(b.ctx, "settings:test_microphone", nil)
	return true
}

// TestSpeaker tests the speaker (frontend will handle actual test)
func (b *SettingsBridge) TestSpeaker() bool {
	runtime.EventsEmit(b.ctx, "settings:test_speaker", nil)
	return true
}

// TestCamera tests the camera (frontend will handle actual test)
func (b *SettingsBridge) TestCamera() bool {
	runtime.EventsEmit(b.ctx, "settings:test_camera", nil)
	return true
}

// GetMicrophones returns all available microphone devices
func (b *SettingsBridge) GetMicrophones() []DeviceInfo {
	mics := devices.ListMicrophones()
	result := make([]DeviceInfo, len(mics))
	for i, m := range mics {
		result[i] = DeviceInfo{
			DeviceID: m.DeviceID,
			Label:    m.Name,
			Kind:     m.Kind,
		}
	}
	b.logger.Info().Int("count", len(result)).Msg("Listed microphones")
	return result
}

// GetSpeakers returns all available speaker devices
func (b *SettingsBridge) GetSpeakers() []DeviceInfo {
	speakers := devices.ListSpeakers()
	result := make([]DeviceInfo, len(speakers))
	for i, s := range speakers {
		result[i] = DeviceInfo{
			DeviceID: s.DeviceID,
			Label:    s.Name,
			Kind:     s.Kind,
		}
	}
	b.logger.Info().Int("count", len(result)).Msg("Listed speakers")
	return result
}

// GetCameras returns all available camera devices
func (b *SettingsBridge) GetCameras() []DeviceInfo {
	cameras := devices.ListCameras()
	result := make([]DeviceInfo, len(cameras))
	for i, c := range cameras {
		result[i] = DeviceInfo{
			DeviceID: c.DeviceID,
			Label:    c.Name,
			Kind:     c.Kind,
		}
	}
	b.logger.Info().Int("count", len(result)).Msg("Listed cameras")
	return result
}

// GetAllDevices returns all available audio and video devices
func (b *SettingsBridge) GetAllDevices() []DeviceInfo {
	all := devices.ListAllDevices()
	result := make([]DeviceInfo, len(all))
	for i, d := range all {
		result[i] = DeviceInfo{
			DeviceID: d.DeviceID,
			Label:    d.Name,
			Kind:     d.Kind,
		}
	}
	b.logger.Info().Int("count", len(result)).Msg("Listed all devices")
	return result
}

// SetUseFrontierModel toggles between frontier and local models
func (b *SettingsBridge) SetUseFrontierModel(useFrontier bool) error {
	b.cfg.A2A.UseFrontierModel = useFrontier
	if err := config.Save(b.cfg); err != nil {
		return err
	}

	// Update CortexBrain provider
	provider := "ollama"
	if useFrontier {
		provider = "anthropic"
	}

	if err := b.setCortexBrainProvider(provider); err != nil {
		b.logger.Warn().Err(err).Str("provider", provider).Msg("Failed to update CortexBrain provider")
		// Don't fail - the setting is saved locally
	}

	b.logger.Info().Bool("useFrontier", useFrontier).Msg("Frontier model setting changed")
	runtime.EventsEmit(b.ctx, "settings:frontier_changed", useFrontier)
	return nil
}

// setCortexBrainProvider calls CortexBrain API to switch the default provider
func (b *SettingsBridge) setCortexBrainProvider(provider string) error {
	url := fmt.Sprintf("%s/api/config/providers", b.cfg.A2A.ServerURL)

	payload := map[string]any{
		"default_provider": provider,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("PUT", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		b.logger.Warn().Err(err).Str("url", url).Msg("Failed to call CortexBrain config API")
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("CortexBrain API returned %d", resp.StatusCode)
	}

	b.logger.Info().Str("provider", provider).Msg("Updated CortexBrain default provider")
	return nil
}

// CommandResult represents the result of running a shell command
type CommandResult struct {
	Success bool   `json:"success"`
	Output  string `json:"output"`
	Error   string `json:"error"`
}

// RunCommand runs a shell command (e.g., for installing Ollama models)
// Only allows safe commands like "ollama pull"
func (b *SettingsBridge) RunCommand(command string) CommandResult {
	b.logger.Info().Str("command", command).Msg("Running command")

	// Security: Only allow specific safe commands
	allowedPrefixes := []string{
		"ollama pull ",
		"ollama list",
	}

	allowed := false
	for _, prefix := range allowedPrefixes {
		if strings.HasPrefix(command, prefix) {
			allowed = true
			break
		}
	}

	if !allowed {
		b.logger.Warn().Str("command", command).Msg("Command not allowed")
		return CommandResult{
			Success: false,
			Error:   "Command not allowed. Only ollama commands are permitted.",
		}
	}

	// Parse command
	parts := strings.Fields(command)
	if len(parts) < 1 {
		return CommandResult{
			Success: false,
			Error:   "Invalid command",
		}
	}

	cmd := exec.Command(parts[0], parts[1:]...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		b.logger.Error().Err(err).Str("output", string(output)).Msg("Command failed")
		return CommandResult{
			Success: false,
			Output:  string(output),
			Error:   err.Error(),
		}
	}

	b.logger.Info().Str("output", string(output)).Msg("Command succeeded")
	return CommandResult{
		Success: true,
		Output:  string(output),
	}
}
