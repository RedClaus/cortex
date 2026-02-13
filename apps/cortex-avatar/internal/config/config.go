// Package config provides configuration management for CortexAvatar
package config

import (
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/viper"
)

// Config holds all application configuration
type Config struct {
	A2A    A2AConfig    `mapstructure:"a2a"`
	User   UserConfig   `mapstructure:"user"`
	Audio  AudioConfig  `mapstructure:"audio"`
	STT    STTConfig    `mapstructure:"stt"`
	TTS    TTSConfig    `mapstructure:"tts"`
	Vision VisionConfig `mapstructure:"vision"`
	Avatar AvatarConfig `mapstructure:"avatar"`
	Window WindowConfig `mapstructure:"window"`
}

// A2AConfig configures the A2A client
type A2AConfig struct {
	ServerURL        string        `mapstructure:"server_url"`
	Timeout          time.Duration `mapstructure:"timeout"`
	ReconnectDelay   time.Duration `mapstructure:"reconnect_delay"`
	MaxReconnects    int           `mapstructure:"max_reconnects"`
	UseFrontierModel bool          `mapstructure:"use_frontier_model"` // Use cloud AI instead of local
}

// UserConfig identifies the user
type UserConfig struct {
	ID        string `mapstructure:"id"`
	PersonaID string `mapstructure:"persona_id"`
}

// AudioConfig configures audio capture/playback
type AudioConfig struct {
	InputDevice   string        `mapstructure:"input_device"`
	OutputDevice  string        `mapstructure:"output_device"`
	SampleRate    int           `mapstructure:"sample_rate"`
	VADThreshold  float64       `mapstructure:"vad_threshold"`
	VADSilenceDur time.Duration `mapstructure:"vad_silence_duration"`
	BufferSize    int           `mapstructure:"buffer_size"`
	OutputVolume  int           `mapstructure:"output_volume"` // 0-100
}

// STTConfig configures speech-to-text
type STTConfig struct {
	Provider   string `mapstructure:"provider"` // whisper, groq, deepgram
	ModelSize  string `mapstructure:"model_size"`
	Language   string `mapstructure:"language"`
	NumThreads int    `mapstructure:"num_threads"`
	EnableGPU  bool   `mapstructure:"enable_gpu"`
	// Streaming STT (Deepgram)
	DeepgramAPIKey  string `mapstructure:"deepgram_api_key"`
	InterimResults  bool   `mapstructure:"interim_results"`  // Show partial transcriptions
	EnableStreaming bool   `mapstructure:"enable_streaming"` // Use WebSocket streaming
}

// TTSConfig configures text-to-speech
type TTSConfig struct {
	Provider        string  `mapstructure:"provider"` // openai, cartesia, piper, macos
	VoiceID         string  `mapstructure:"voice_id"`
	Speed           float64 `mapstructure:"speed"`
	Pitch           float64 `mapstructure:"pitch"`
	CacheEnabled    bool    `mapstructure:"cache_enabled"`
	ReasoningFilter int     `mapstructure:"reasoning_filter"` // 0-100: how much inner voice/reasoning to filter (0=hear all, 100=only final answers)
	// Streaming TTS (Cartesia)
	CartesiaAPIKey  string `mapstructure:"cartesia_api_key"`
	CartesiaVoiceID string `mapstructure:"cartesia_voice_id"` // Cartesia voice UUID
	EnableLipSync   bool   `mapstructure:"enable_lip_sync"`   // Generate viseme timeline for lip-sync
}

// VisionConfig configures camera/screen capture
type VisionConfig struct {
	DefaultSource string `mapstructure:"default_source"`
	CameraDevice  string `mapstructure:"camera_device"`
	MaxImageSize  int    `mapstructure:"max_image_size"`
	JPEGQuality   int    `mapstructure:"jpeg_quality"`
}

// AvatarConfig configures the avatar
type AvatarConfig struct {
	Theme              string        `mapstructure:"theme"`
	Persona            string        `mapstructure:"persona"` // henry or hannah
	IdleAnimation      bool          `mapstructure:"idle_animation"`
	BlinkInterval      time.Duration `mapstructure:"blink_interval"`
	ExpressionDuration time.Duration `mapstructure:"expression_duration"`
}

// Persona represents an avatar persona
type Persona struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Gender  string `json:"gender"` // male or female
	VoiceID string `json:"voice_id"`
	Theme   string `json:"theme"`
}

// AvailablePersonas returns the default personas
func AvailablePersonas() []Persona {
	return []Persona{
		{
			ID:      "henry",
			Name:    "Henry",
			Gender:  "male",
			VoiceID: "onyx", // OpenAI voice: Male, deep
			Theme:   "default",
		},
		{
			ID:      "hannah",
			Name:    "Hannah",
			Gender:  "female",
			VoiceID: "nova", // OpenAI voice: Female, warm
			Theme:   "default",
		},
	}
}

// GetPersona returns a persona by ID
func GetPersona(id string) *Persona {
	for _, p := range AvailablePersonas() {
		if p.ID == id {
			return &p
		}
	}
	return nil
}

// WindowConfig configures the window
type WindowConfig struct {
	Title          string `mapstructure:"title"`
	Width          int    `mapstructure:"width"`
	Height         int    `mapstructure:"height"`
	AlwaysOnTop    bool   `mapstructure:"always_on_top"`
	StartMinimized bool   `mapstructure:"start_minimized"`
	Frameless      bool   `mapstructure:"frameless"`
	Transparent    bool   `mapstructure:"transparent"`
}

// DefaultConfig returns sensible default configuration
func DefaultConfig() *Config {
	return &Config{
		A2A: A2AConfig{
			ServerURL:      "http://localhost:8080",
			Timeout:        30 * time.Second,
			ReconnectDelay: 5 * time.Second,
			MaxReconnects:  10,
		},
		User: UserConfig{
			ID:        "default-user",
			PersonaID: "hannah",
		},
		Audio: AudioConfig{
			InputDevice:   "",
			OutputDevice:  "",
			SampleRate:    16000,
			VADThreshold:  0.5,
			VADSilenceDur: 1 * time.Second,
			BufferSize:    4096,
			OutputVolume:  100, // Default to full volume
		},
		STT: STTConfig{
			Provider:        "whisper",
			ModelSize:       "base",
			Language:        "auto",
			NumThreads:      4,
			EnableGPU:       true,
			InterimResults:  true,
			EnableStreaming: false,
		},
		TTS: TTSConfig{
			Provider:        "openai",
			VoiceID:         "nova",
			Speed:           1.0,
			Pitch:           1.0,
			CacheEnabled:    true,
			ReasoningFilter: 70,
			CartesiaVoiceID: "a0e99841-438c-4a64-b679-ae501e7d6091",
			EnableLipSync:   true,
		},
		Vision: VisionConfig{
			DefaultSource: "camera",
			CameraDevice:  "",
			MaxImageSize:  1280,
			JPEGQuality:   85,
		},
		Avatar: AvatarConfig{
			Theme:              "default",
			Persona:            "hannah", // Default to Hannah
			IdleAnimation:      true,
			BlinkInterval:      4 * time.Second,
			ExpressionDuration: 3 * time.Second,
		},
		Window: WindowConfig{
			Title:          "CortexAvatar",
			Width:          500,
			Height:         700,
			AlwaysOnTop:    false,
			StartMinimized: false,
			Frameless:      false,
			Transparent:    false,
		},
	}
}

// Load reads configuration from file and environment
func Load() (*Config, error) {
	cfg := DefaultConfig()

	// Set config paths
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return cfg, err
	}

	configDir := filepath.Join(homeDir, ".cortexavatar")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return cfg, err
	}

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(configDir)
	viper.AddConfigPath(".")

	// Environment variable overrides
	viper.SetEnvPrefix("CORTEXAVATAR")
	viper.AutomaticEnv()

	// Read config file if it exists
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return cfg, err
		}
		// Config file not found, use defaults and create one
		if err := Save(cfg); err != nil {
			return cfg, err
		}
	}

	if err := viper.Unmarshal(cfg); err != nil {
		return cfg, err
	}

	return cfg, nil
}

// Save writes the configuration to file
func Save(cfg *Config) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	configDir := filepath.Join(homeDir, ".cortexavatar")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	viper.Set("a2a", cfg.A2A)
	viper.Set("user", cfg.User)
	viper.Set("audio", cfg.Audio)
	viper.Set("stt", cfg.STT)
	viper.Set("tts", cfg.TTS)
	viper.Set("vision", cfg.Vision)
	viper.Set("avatar", cfg.Avatar)
	viper.Set("window", cfg.Window)

	configPath := filepath.Join(configDir, "config.yaml")
	return viper.WriteConfigAs(configPath)
}

// GetConfigDir returns the configuration directory path
func GetConfigDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".cortexavatar"), nil
}
