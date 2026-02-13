package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/normanking/cortex/internal/agent"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// Config holds all application configuration for the Cortex AI assistant.
// It is loaded from ~/.cortex/config.yaml and can be overridden by environment variables.
type Config struct {
	LLM        LLMConfig        `mapstructure:"llm" yaml:"llm"`
	Knowledge  KnowledgeConfig  `mapstructure:"knowledge" yaml:"knowledge"`
	Sync       SyncConfig       `mapstructure:"sync" yaml:"sync"`
	TUI        TUIConfig        `mapstructure:"tui" yaml:"tui"`
	Logging    LoggingConfig    `mapstructure:"logging" yaml:"logging"`
	Cognitive  CognitiveConfig  `mapstructure:"cognitive" yaml:"cognitive"`
	Voice      VoiceConfig      `mapstructure:"voice" yaml:"voice"`
	CortexEyes CortexEyesConfig `mapstructure:"cortex_eyes" yaml:"cortex_eyes"`
	Sleep      SleepConfig      `mapstructure:"sleep" yaml:"sleep"`
	LEANN      LEANNConfig      `mapstructure:"leann" yaml:"leann"`
	Agent      AgentConfig      `mapstructure:"agent" yaml:"agent"`
}

// AgentConfig contains configuration for the agentic execution system.
// Controls supervised mode, step limits, and checkpoint behavior.
type AgentConfig struct {
	// Mode controls how the agent handles checkpoints: "supervised", "autonomous", or "disabled"
	// - supervised: Pauses at checkpoints to ask user what to do (default)
	// - autonomous: Runs to completion without pausing
	// - disabled: No agentic execution
	Mode string `mapstructure:"mode" yaml:"mode"`

	// StepLimit is the maximum number of tool execution steps before triggering a checkpoint
	StepLimit int `mapstructure:"step_limit" yaml:"step_limit"`

	// CheckpointOnLoop pauses when the agent detects a loop (repeated tool calls)
	CheckpointOnLoop bool `mapstructure:"checkpoint_on_loop" yaml:"checkpoint_on_loop"`

	// CheckpointOnError pauses when a tool execution fails
	CheckpointOnError bool `mapstructure:"checkpoint_on_error" yaml:"checkpoint_on_error"`

	// CheckpointOnStepLimit pauses when the step limit is reached
	CheckpointOnStepLimit bool `mapstructure:"checkpoint_on_step_limit" yaml:"checkpoint_on_step_limit"`

	// AutoEscalateOnLoop automatically switches to a more capable model when a loop is detected
	AutoEscalateOnLoop bool `mapstructure:"auto_escalate_on_loop" yaml:"auto_escalate_on_loop"`

	// CostCheckpointTokens triggers a checkpoint after this many tokens are consumed (0 = disabled)
	CostCheckpointTokens int `mapstructure:"cost_checkpoint_tokens" yaml:"cost_checkpoint_tokens"`
}

// ToSupervisedConfig converts AgentConfig to agent.SupervisedConfig for use by the agent package.
func (c AgentConfig) ToSupervisedConfig() agent.SupervisedConfig {
	mode := agent.AgenticModeSupervised
	switch strings.ToLower(c.Mode) {
	case "autonomous":
		mode = agent.AgenticModeAutonomous
	case "disabled":
		mode = agent.AgenticModeDisabled
	}

	return agent.SupervisedConfig{
		Mode:                  mode,
		StepLimit:             c.StepLimit,
		CheckpointOnLoop:      c.CheckpointOnLoop,
		CheckpointOnError:     c.CheckpointOnError,
		CheckpointOnStepLimit: c.CheckpointOnStepLimit,
		AutoEscalateOnLoop:    c.AutoEscalateOnLoop,
		CostCheckpointTokens:  c.CostCheckpointTokens,
	}
}

// LLMConfig contains configuration for Language Model providers.
type LLMConfig struct {
	// DefaultProvider specifies which provider to use by default (e.g., "ollama", "openai", "anthropic")
	DefaultProvider string `mapstructure:"default_provider" yaml:"default_provider"`
	// Providers maps provider names to their specific configuration
	Providers map[string]ProviderConfig `mapstructure:"providers" yaml:"providers"`
}

// ProviderConfig contains configuration for a specific LLM provider.
type ProviderConfig struct {
	// Endpoint is the API endpoint URL (primarily used for local providers like Ollama)
	Endpoint string `mapstructure:"endpoint" yaml:"endpoint,omitempty"`
	// APIKey is the authentication key for the provider
	APIKey string `mapstructure:"api_key" yaml:"api_key,omitempty"`
	// Model is the specific model to use with this provider
	Model string `mapstructure:"model" yaml:"model,omitempty"`
	// Timeouts contains timeout configuration (primarily for Ollama)
	Timeouts *TimeoutConfig `mapstructure:"timeouts" yaml:"timeouts,omitempty"`
}

// TimeoutConfig contains timeout settings for LLM providers.
// These are most relevant for Ollama which may experience cold start delays.
type TimeoutConfig struct {
	// ConnectionTimeoutSec is the time to establish HTTP connection (default: 30s)
	ConnectionTimeoutSec int `mapstructure:"connection_timeout_sec" yaml:"connection_timeout_sec,omitempty"`
	// FirstTokenTimeoutSec is the time to receive first token after connection (default: 120s for local, 300s for remote)
	// This should be long enough to handle model loading (cold start) scenarios
	FirstTokenTimeoutSec int `mapstructure:"first_token_timeout_sec" yaml:"first_token_timeout_sec,omitempty"`
	// StreamIdleTimeoutSec is the max time between tokens during streaming (default: 30s)
	StreamIdleTimeoutSec int `mapstructure:"stream_idle_timeout_sec" yaml:"stream_idle_timeout_sec,omitempty"`
	// WarmupOnStart triggers a model warmup request on provider initialization (default: false)
	WarmupOnStart bool `mapstructure:"warmup_on_start" yaml:"warmup_on_start,omitempty"`
}

// KnowledgeConfig contains configuration for the knowledge management system.
type KnowledgeConfig struct {
	// DBPath is the path to the SQLite knowledge database
	DBPath string `mapstructure:"db_path" yaml:"db_path"`
	// DefaultTier is the default trust tier for new knowledge ("personal", "team", "public")
	DefaultTier string `mapstructure:"default_tier" yaml:"default_tier"`
	// TrustDecayDays is the number of days before trust scores begin to decay
	TrustDecayDays int `mapstructure:"trust_decay_days" yaml:"trust_decay_days"`
}

// SyncConfig contains configuration for cloud synchronization.
type SyncConfig struct {
	// Enabled determines whether sync is active
	Enabled bool `mapstructure:"enabled" yaml:"enabled"`
	// Endpoint is the sync server URL
	Endpoint string `mapstructure:"endpoint" yaml:"endpoint"`
	// Interval is how often to sync (e.g., "5m", "1h")
	Interval time.Duration `mapstructure:"interval" yaml:"interval"`
	// AuthToken is the authentication token for the sync service
	AuthToken string `mapstructure:"auth_token" yaml:"auth_token,omitempty"`
	// TeamID is the team identifier for multi-user sync
	TeamID string `mapstructure:"team_id" yaml:"team_id,omitempty"`
}

// TUIConfig contains configuration for the terminal user interface.
type TUIConfig struct {
	// Theme is the UI theme ("dark" or "light")
	Theme string `mapstructure:"theme" yaml:"theme"`
	// VimMode enables vim-style keybindings
	VimMode bool `mapstructure:"vim_mode" yaml:"vim_mode"`
	// SidebarWidth is the width of the sidebar in characters
	SidebarWidth int `mapstructure:"sidebar_width" yaml:"sidebar_width"`
}

// LoggingConfig contains configuration for application logging.
type LoggingConfig struct {
	// Level is the log level ("debug", "info", "warn", "error")
	Level string `mapstructure:"level" yaml:"level"`
	// File is the path to the log file
	File string `mapstructure:"file" yaml:"file"`
}

// CognitiveConfig contains configuration for the Cognitive Architecture.
// The cognitive system provides template-based responses, semantic routing,
// and runtime distillation from frontier models.
type CognitiveConfig struct {
	// Enabled determines whether the cognitive architecture is active
	Enabled bool `mapstructure:"enabled" yaml:"enabled"`
	// OllamaURL is the URL for the Ollama server (used for embeddings)
	OllamaURL string `mapstructure:"ollama_url" yaml:"ollama_url"`
	// EmbeddingModel is the model to use for generating embeddings (e.g., "nomic-embed-text")
	EmbeddingModel string `mapstructure:"embedding_model" yaml:"embedding_model"`
	// FrontierModel is the model to use for distillation (e.g., "claude-sonnet-4-20250514")
	FrontierModel string `mapstructure:"frontier_model" yaml:"frontier_model"`
	// SimilarityThresholdHigh is the threshold for high-confidence template matches (default 0.85)
	SimilarityThresholdHigh float64 `mapstructure:"similarity_threshold_high" yaml:"similarity_threshold_high"`
	// SimilarityThresholdMedium is the threshold for medium-confidence template matches (default 0.70)
	SimilarityThresholdMedium float64 `mapstructure:"similarity_threshold_medium" yaml:"similarity_threshold_medium"`
	// SimilarityThresholdLow is the threshold for low-confidence template matches (default 0.50)
	SimilarityThresholdLow float64 `mapstructure:"similarity_threshold_low" yaml:"similarity_threshold_low"`
	// ComplexityThreshold is the score above which tasks are considered complex (default 70)
	ComplexityThreshold int `mapstructure:"complexity_threshold" yaml:"complexity_threshold"`
}

// VADConfig holds Voice Activity Detection configuration.
type VADConfig struct {
	// Enabled controls whether VAD is active
	Enabled bool `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	// Mode is the VAD backend ("silero", "webrtc", or "energy")
	Mode string `mapstructure:"mode" yaml:"mode" json:"mode"`
	// Endpoint is the WebSocket URL for the VAD service
	Endpoint string `mapstructure:"endpoint" yaml:"endpoint" json:"endpoint"`
	// Threshold is the speech probability threshold (0.0-1.0)
	Threshold float64 `mapstructure:"threshold" yaml:"threshold" json:"threshold"`
	// MinSpeechMs is the minimum speech duration in milliseconds
	MinSpeechMs int `mapstructure:"min_speech_ms" yaml:"min_speech_ms" json:"min_speech_ms"`
	// MinSilenceMs is the minimum silence duration to end speech in milliseconds
	MinSilenceMs int `mapstructure:"min_silence_ms" yaml:"min_silence_ms" json:"min_silence_ms"`
	// PreBufferMs is the audio pre-buffer duration in milliseconds
	PreBufferMs int `mapstructure:"pre_buffer_ms" yaml:"pre_buffer_ms" json:"pre_buffer_ms"`
}

// DefaultVADConfig returns a VADConfig with sensible default values.
func DefaultVADConfig() VADConfig {
	return VADConfig{
		Enabled:      true,
		Mode:         "silero",
		Endpoint:     "ws://127.0.0.1:8880/v1/vad/stream",
		Threshold:    0.5,
		MinSpeechMs:  250,
		MinSilenceMs: 300,
		PreBufferMs:  300,
	}
}

// WakeWordConfig holds wake word detection configuration (CR-015).
type WakeWordConfig struct {
	// Enabled controls whether pre-STT wake word detection is active
	Enabled bool `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	// WakeWords is the list of wake words to detect
	WakeWords []string `mapstructure:"wake_words" yaml:"wake_words" json:"wake_words"`
	// Threshold is the detection confidence threshold (0.0-1.0)
	Threshold float64 `mapstructure:"threshold" yaml:"threshold" json:"threshold"`
	// ModelDir is the directory containing custom wake word models
	ModelDir string `mapstructure:"model_dir" yaml:"model_dir" json:"model_dir"`
	// UseVADFilter enables Silero VAD pre-filtering to reduce false positives
	UseVADFilter bool `mapstructure:"use_vad_filter" yaml:"use_vad_filter" json:"use_vad_filter"`
	// VADThreshold is the VAD threshold for pre-filtering (0.0-1.0)
	VADThreshold float64 `mapstructure:"vad_threshold" yaml:"vad_threshold" json:"vad_threshold"`
}

// ResembleAgentsConfig holds configuration for Resemble.ai Voice Agents (CR-016).
type ResembleAgentsConfig struct {
	// Enabled controls whether Resemble Agents integration is active
	Enabled bool `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	// WebhookPort is the port for the Cortex webhook server
	WebhookPort int `mapstructure:"webhook_port" yaml:"webhook_port" json:"webhook_port"`
	// WebhookHost is the host for the webhook server (default: localhost)
	WebhookHost string `mapstructure:"webhook_host" yaml:"webhook_host" json:"webhook_host"`
	// WebhookToken is the authentication token for webhook calls
	WebhookToken string `mapstructure:"webhook_token" yaml:"webhook_token,omitempty" json:"webhook_token,omitempty"`
	// DefaultAgent is the UUID of the default agent to use
	DefaultAgent string `mapstructure:"default_agent" yaml:"default_agent,omitempty" json:"default_agent,omitempty"`
	// Agents maps friendly names to agent UUIDs
	Agents map[string]AgentEntry `mapstructure:"agents" yaml:"agents,omitempty" json:"agents,omitempty"`
}

// AgentEntry represents a configured Resemble Agent.
type AgentEntry struct {
	UUID string `mapstructure:"uuid" yaml:"uuid" json:"uuid"`
	Name string `mapstructure:"name" yaml:"name" json:"name"`
}

// DefaultResembleAgentsConfig returns sensible defaults for Resemble Agents.
func DefaultResembleAgentsConfig() ResembleAgentsConfig {
	return ResembleAgentsConfig{
		Enabled:      false,
		WebhookPort:  8766,
		WebhookHost:  "localhost",
		WebhookToken: "",
		DefaultAgent: "",
		Agents:       make(map[string]AgentEntry),
	}
}

// DefaultWakeWordConfig returns a WakeWordConfig with sensible default values.
func DefaultWakeWordConfig() WakeWordConfig {
	homeDir, _ := os.UserHomeDir()
	modelDir := filepath.Join(homeDir, ".cortex", "voicebox", "wake_word_models")

	return WakeWordConfig{
		Enabled:      true,
		WakeWords:    []string{"hey_cortex", "hey_henry", "hey_hannah"},
		Threshold:    0.5,
		ModelDir:     modelDir,
		UseVADFilter: true,
		VADThreshold: 0.3,
	}
}

// CortexEyesConfig holds configuration for CortexEyes screen awareness (CR-023).
type CortexEyesConfig struct {
	// Enabled controls whether CortexEyes is active
	Enabled bool `mapstructure:"enabled" yaml:"enabled" json:"enabled"`

	// Vision settings (MLX vision model)
	Vision CortexEyesVisionConfig `mapstructure:"vision" yaml:"vision" json:"vision"`

	// Capture settings (screen capture)
	Capture CortexEyesCaptureConfig `mapstructure:"capture" yaml:"capture" json:"capture"`

	// Webcam settings (optional, disabled by default)
	Webcam CortexEyesWebcamConfig `mapstructure:"webcam" yaml:"webcam" json:"webcam"`

	// Privacy settings
	Privacy CortexEyesPrivacyConfig `mapstructure:"privacy" yaml:"privacy" json:"privacy"`

	// Learning settings
	Learning CortexEyesLearningConfig `mapstructure:"learning" yaml:"learning" json:"learning"`
}

// CortexEyesVisionConfig controls vision model settings.
type CortexEyesVisionConfig struct {
	// Endpoint is the MLX vision server URL (default: http://127.0.0.1:8082)
	Endpoint string `mapstructure:"endpoint" yaml:"endpoint" json:"endpoint"`
	// Model is the vision model to use (default: mlx-community/Qwen2-VL-2B-Instruct-4bit)
	Model string `mapstructure:"model" yaml:"model" json:"model"`
	// Enabled allows disabling vision while keeping screen capture for activity tracking
	Enabled bool `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
}

// CortexEyesCaptureConfig controls screen capture behavior.
type CortexEyesCaptureConfig struct {
	// FPS is frames per second to analyze (default: 0.2 = 1 every 5 seconds)
	FPS float64 `mapstructure:"fps" yaml:"fps" json:"fps"`
	// ChangeThreshold is 0.0-1.0, how different must frame be to trigger analysis (default: 0.3)
	ChangeThreshold float64 `mapstructure:"change_threshold" yaml:"change_threshold" json:"change_threshold"`
	// MinIntervalSec is minimum seconds between analyses (default: 5)
	MinIntervalSec int `mapstructure:"min_interval_sec" yaml:"min_interval_sec" json:"min_interval_sec"`
}

// CortexEyesPrivacyConfig controls privacy settings.
type CortexEyesPrivacyConfig struct {
	// ExcludedApps is a list of apps to never capture
	ExcludedApps []string `mapstructure:"excluded_apps" yaml:"excluded_apps" json:"excluded_apps"`
	// ExcludedWindows is a list of window title patterns to exclude
	ExcludedWindows []string `mapstructure:"excluded_windows" yaml:"excluded_windows" json:"excluded_windows"`
	// AutoPauseIdleMin pauses after N minutes of no activity (default: 5)
	AutoPauseIdleMin int `mapstructure:"auto_pause_idle_min" yaml:"auto_pause_idle_min" json:"auto_pause_idle_min"`
	// MaxRetentionDays auto-deletes observations after N days (default: 30)
	MaxRetentionDays int `mapstructure:"max_retention_days" yaml:"max_retention_days" json:"max_retention_days"`
	// RequireConsent prompts before first capture (default: true)
	RequireConsent bool `mapstructure:"require_consent" yaml:"require_consent" json:"require_consent"`
	// AllowedHoursStart is the start time for allowed capture hours (e.g., "08:00")
	AllowedHoursStart string `mapstructure:"allowed_hours_start" yaml:"allowed_hours_start" json:"allowed_hours_start"`
	// AllowedHoursEnd is the end time for allowed capture hours (e.g., "22:00")
	AllowedHoursEnd string `mapstructure:"allowed_hours_end" yaml:"allowed_hours_end" json:"allowed_hours_end"`
}

// CortexEyesLearningConfig controls learning behavior.
type CortexEyesLearningConfig struct {
	// EnablePatterns enables pattern detection from observations
	EnablePatterns bool `mapstructure:"enable_patterns" yaml:"enable_patterns" json:"enable_patterns"`
	// EnableInsights enables proactive insight generation (Phase 2)
	EnableInsights bool `mapstructure:"enable_insights" yaml:"enable_insights" json:"enable_insights"`
	// MinObservationsForPattern is minimum observations needed for pattern detection (default: 10)
	MinObservationsForPattern int `mapstructure:"min_observations_for_pattern" yaml:"min_observations_for_pattern" json:"min_observations_for_pattern"`
}

// CortexEyesWebcamConfig controls webcam capture behavior.
type CortexEyesWebcamConfig struct {
	// Enabled controls whether webcam capture is active (default: false for privacy)
	Enabled bool `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	// CameraIndex is the AVFoundation camera index (0 = FaceTime HD Camera on most Macs)
	CameraIndex int `mapstructure:"camera_index" yaml:"camera_index" json:"camera_index"`
	// FPS is frames per second to capture (default: 0.5 = 1 every 2 seconds)
	FPS float64 `mapstructure:"fps" yaml:"fps" json:"fps"`
}

// DefaultCortexEyesConfig returns default CortexEyes configuration.
func DefaultCortexEyesConfig() CortexEyesConfig {
	return CortexEyesConfig{
		Enabled: false, // Disabled by default for privacy
		Vision: CortexEyesVisionConfig{
			Endpoint: "http://127.0.0.1:8082", // Separate port for vision server
			Model:    "mlx-community/Qwen2-VL-2B-Instruct-4bit",
			Enabled:  true, // Vision enabled when CortexEyes is enabled
		},
		Capture: CortexEyesCaptureConfig{
			FPS:             0.2, // 1 frame every 5 seconds
			ChangeThreshold: 0.3,
			MinIntervalSec:  5,
		},
		Webcam: CortexEyesWebcamConfig{
			Enabled:     false, // Disabled by default for privacy
			CameraIndex: 0,     // FaceTime HD Camera
			FPS:         0.5,   // 1 frame every 2 seconds
		},
		Privacy: CortexEyesPrivacyConfig{
			ExcludedApps: []string{
				"1Password",
				"Keychain Access",
				"System Preferences",
				"System Settings",
				"Bitwarden",
				"LastPass",
			},
			ExcludedWindows: []string{
				"*password*",
				"*credential*",
				"*secret*",
				"*private*",
			},
			AutoPauseIdleMin: 5,
			MaxRetentionDays: 30,
			RequireConsent:   true,
			AllowedHoursStart: "08:00",
			AllowedHoursEnd:   "22:00",
		},
		Learning: CortexEyesLearningConfig{
			EnablePatterns:            true,
			EnableInsights:            false, // Phase 2
			MinObservationsForPattern: 10,
		},
	}
}

// SleepConfig holds configuration for the sleep cycle self-improvement system (CR-020).
type SleepConfig struct {
	// Enabled controls whether the sleep cycle is active
	Enabled bool `mapstructure:"enabled" yaml:"enabled"`

	// Mode is the improvement mode: "off", "supervised", or "auto"
	// - off: Sleep cycle disabled
	// - supervised: Proposals require user approval
	// - auto: Safe proposals auto-applied
	Mode string `mapstructure:"mode" yaml:"mode"`

	// IdleTimeoutMin is minutes of idle before auto-sleep triggers (default: 30)
	IdleTimeoutMin int `mapstructure:"idle_timeout_min" yaml:"idle_timeout_min"`

	// MinInteractions is minimum interactions before sleep is allowed (default: 10)
	MinInteractions int `mapstructure:"min_interactions" yaml:"min_interactions"`

	// PersonalityPath is the directory for personality files (default: ~/.cortex/personality)
	PersonalityPath string `mapstructure:"personality_path" yaml:"personality_path"`
}

// DefaultSleepConfig returns sensible defaults for the sleep cycle.
func DefaultSleepConfig() SleepConfig {
	return SleepConfig{
		Enabled:         true, // Enabled by default
		Mode:            "supervised",
		IdleTimeoutMin:  30,
		MinInteractions: 10,
		PersonalityPath: "", // Will use ~/.cortex/personality
	}
}

// LEANNConfig holds configuration for LEANN semantic search sidecar (CR-026).
// LEANN (Low-storage Embedding Approximate Nearest Neighbor) provides 97% storage
// reduction compared to traditional vector databases through graph-based selective
// recomputation of embeddings.
type LEANNConfig struct {
	// Enabled controls whether LEANN is active
	Enabled bool `mapstructure:"enabled" yaml:"enabled"`

	// Port is the LEANN sidecar port (default: 8882)
	Port int `mapstructure:"port" yaml:"port"`

	// FallbackEnabled uses FTS5/LSH when LEANN unavailable
	FallbackEnabled bool `mapstructure:"fallback_enabled" yaml:"fallback_enabled"`

	// ShadowMode writes to both LEANN and SQLite for migration
	ShadowMode bool `mapstructure:"shadow_mode" yaml:"shadow_mode"`

	// FastPathTimeoutMs is the strict timeout for passive retrieval (default: 50)
	FastPathTimeoutMs int `mapstructure:"fast_path_timeout_ms" yaml:"fast_path_timeout_ms"`

	// GraphFile is the path to the LEANN graph file
	GraphFile string `mapstructure:"graph_file" yaml:"graph_file"`

	// EmbeddingModel is the Ollama model for embeddings (default: nomic-embed-text)
	EmbeddingModel string `mapstructure:"embedding_model" yaml:"embedding_model"`

	// MaxConnections is the connection pool size (default: 4)
	MaxConnections int `mapstructure:"max_connections" yaml:"max_connections"`

	// BatchSize is the batch size for bulk indexing (default: 100)
	BatchSize int `mapstructure:"batch_size" yaml:"batch_size"`

	// CacheSize is the embedding cache size (default: 1000)
	CacheSize int `mapstructure:"cache_size" yaml:"cache_size"`
}

// DefaultLEANNConfig returns sensible defaults for LEANN.
func DefaultLEANNConfig() LEANNConfig {
	return LEANNConfig{
		Enabled:           false, // Disabled by default until installed
		Port:              8882,
		FallbackEnabled:   true,
		ShadowMode:        false,
		FastPathTimeoutMs: 50,
		GraphFile:         "", // Will use ~/.cortex/leann/cortex.leann
		EmbeddingModel:    "nomic-embed-text",
		MaxConnections:    4,
		BatchSize:         100,
		CacheSize:         1000,
	}
}

// VoiceConfig holds voice-related settings for speech input/output.
type VoiceConfig struct {
	// Enabled controls whether voice features are available
	Enabled bool `mapstructure:"enabled" yaml:"enabled"`

	// OrchestratorURL is the WebSocket URL for the voice orchestrator
	OrchestratorURL string `mapstructure:"orchestrator_url" yaml:"orchestrator_url"`

	// AutoConnect attempts to connect to voice orchestrator on startup
	AutoConnect bool `mapstructure:"auto_connect" yaml:"auto_connect"`

	// VADThreshold is the voice activity detection sensitivity (0.0-1.0)
	// Deprecated: Use VAD.Threshold instead
	VADThreshold float64 `mapstructure:"vad_threshold" yaml:"vad_threshold"`

	// DefaultVoice is the TTS voice ID to use
	DefaultVoice string `mapstructure:"default_voice" yaml:"default_voice"`

	// ReconnectDelay is the delay between reconnection attempts (in seconds)
	ReconnectDelay int `mapstructure:"reconnect_delay" yaml:"reconnect_delay"`

	// MaxReconnects is the maximum number of reconnection attempts (0 = infinite)
	MaxReconnects int `mapstructure:"max_reconnects" yaml:"max_reconnects"`

	// ResembleAPIKey is the API key for Resemble.ai cloud TTS
	ResembleAPIKey string `mapstructure:"resemble_api_key" yaml:"resemble_api_key,omitempty"`

	// ResembleDefaultVoice is the default Resemble voice UUID
	ResembleDefaultVoice string `mapstructure:"resemble_default_voice" yaml:"resemble_default_voice,omitempty"`

	// TTSProvider is the active TTS provider ("local" for Kokoro, "cloud" for Resemble)
	TTSProvider string `mapstructure:"tts_provider" yaml:"tts_provider"`

	// VAD contains Voice Activity Detection configuration
	VAD VADConfig `mapstructure:"vad" yaml:"vad" json:"vad"`

	// WakeWord contains pre-STT wake word detection configuration (CR-015)
	WakeWord WakeWordConfig `mapstructure:"wake_word" yaml:"wake_word" json:"wake_word"`

	// ResembleAgents contains Resemble Voice Agents configuration (CR-016)
	ResembleAgents ResembleAgentsConfig `mapstructure:"resemble_agents" yaml:"resemble_agents" json:"resemble_agents"`
}

// Default returns a Config with sensible default values.
func Default() *Config {
	homeDir, _ := os.UserHomeDir()
	cortexDir := filepath.Join(homeDir, ".cortex")

	return &Config{
		LLM: LLMConfig{
			DefaultProvider: "ollama",
			Providers: map[string]ProviderConfig{
				"ollama": {
					Endpoint: "http://127.0.0.1:11434",
					Model:    "llama3.2",
				},
				"openai": {
					APIKey: "",
					Model:  "gpt-4o-mini",
				},
				"anthropic": {
					APIKey: "",
					Model:  "claude-3-5-sonnet-20241022",
				},
				"gemini": {
					APIKey: "",
					Model:  "gemini-1.5-flash",
				},
				"grok": {
					Endpoint: "https://api.x.ai/v1",
					APIKey:   "",
					Model:    "grok-3-fast",
				},
			},
		},
		Knowledge: KnowledgeConfig{
			DBPath:         filepath.Join(cortexDir, "knowledge.db"),
			DefaultTier:    "personal",
			TrustDecayDays: 30,
		},
		Sync: SyncConfig{
			Enabled:   false,
			Endpoint:  "https://api.acontext.io",
			Interval:  5 * time.Minute,
			AuthToken: "",
			TeamID:    "",
		},
		TUI: TUIConfig{
			Theme:        "dark",
			VimMode:      false,
			SidebarWidth: 30,
		},
		Logging: LoggingConfig{
			Level: "info",
			File:  filepath.Join(cortexDir, "logs", "cortex.log"),
		},
		Cognitive: CognitiveConfig{
			Enabled:                   true,
			OllamaURL:                 "http://127.0.0.1:11434",
			EmbeddingModel:            "nomic-embed-text",
			FrontierModel:             "claude-sonnet-4-20250514",
			SimilarityThresholdHigh:   0.85,
			SimilarityThresholdMedium: 0.70,
			SimilarityThresholdLow:    0.50,
			ComplexityThreshold:       70,
		},
		Voice: VoiceConfig{
			Enabled:              false,
			OrchestratorURL:      "ws://localhost:8765/ws/voice",
			AutoConnect:          true,
			VADThreshold:         0.5,
			DefaultVoice:         "af_sky",
			ReconnectDelay:       5,
			MaxReconnects:        10,
			ResembleAPIKey:       "",
			ResembleDefaultVoice: "",
			TTSProvider:          "local", // "local" (Kokoro) or "cloud" (Resemble)
			VAD:                  DefaultVADConfig(),
			WakeWord:             DefaultWakeWordConfig(),       // CR-015: Pre-STT wake word detection
			ResembleAgents:       DefaultResembleAgentsConfig(), // CR-016: Resemble Voice Agents
		},
		CortexEyes: DefaultCortexEyesConfig(), // CR-023: Screen awareness
		Agent: AgentConfig{
			Mode:                  "supervised", // Default to supervised mode
			StepLimit:             10,           // Pause after 10 steps
			CheckpointOnLoop:      true,         // Pause on detected loops
			CheckpointOnError:     true,         // Pause on tool errors
			CheckpointOnStepLimit: true,         // Pause when step limit reached
			AutoEscalateOnLoop:    false,        // Don't auto-escalate
			CostCheckpointTokens:  10000,        // Pause after ~10K tokens
		},
	}
}

// Load reads configuration from the default location (~/.cortex/config.yaml)
// and merges with environment variables. If no config file exists, it creates
// one with default values.
func Load() (*Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	configPath := filepath.Join(homeDir, ".cortex", "config.yaml")
	return LoadFromPath(configPath)
}

// LoadFromPath reads configuration from a specific file path and merges with
// environment variables. If the file doesn't exist, it creates one with default values.
func LoadFromPath(path string) (*Config, error) {
	// Expand tilde in path
	path = expandPath(path)

	// Ensure the config directory exists
	configDir := filepath.Dir(path)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	// Check if config file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Create default config
		cfg := Default()
		if err := writeConfigFile(path, cfg); err != nil {
			return nil, fmt.Errorf("failed to write default config: %w", err)
		}
	}

	// Configure Viper
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	// Enable environment variable overrides
	// Example: CORTEX_LLM_PROVIDERS_OPENAI_API_KEY
	v.SetEnvPrefix("CORTEX")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Read the config file
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Unmarshal into Config struct
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Expand paths with tilde
	cfg.Knowledge.DBPath = expandPath(cfg.Knowledge.DBPath)
	cfg.Logging.File = expandPath(cfg.Logging.File)

	// Apply defaults for missing voice config values
	cfg.applyVoiceDefaults()

	return &cfg, nil
}

// applyVoiceDefaults fills in missing voice configuration values with sensible defaults.
// This handles the case where voice.enabled=true but other fields are empty/zero.
func (c *Config) applyVoiceDefaults() {
	if !c.Voice.Enabled {
		return
	}

	defaults := Default().Voice

	if c.Voice.OrchestratorURL == "" {
		c.Voice.OrchestratorURL = defaults.OrchestratorURL
	}
	if c.Voice.VADThreshold == 0 {
		c.Voice.VADThreshold = defaults.VADThreshold
	}
	if c.Voice.DefaultVoice == "" {
		c.Voice.DefaultVoice = defaults.DefaultVoice
	}
	if c.Voice.ReconnectDelay == 0 {
		c.Voice.ReconnectDelay = defaults.ReconnectDelay
	}
	if c.Voice.MaxReconnects == 0 {
		c.Voice.MaxReconnects = defaults.MaxReconnects
	}

	// Apply VAD defaults
	vadDefaults := DefaultVADConfig()
	if c.Voice.VAD.Mode == "" {
		c.Voice.VAD.Mode = vadDefaults.Mode
	}
	if c.Voice.VAD.Endpoint == "" {
		c.Voice.VAD.Endpoint = vadDefaults.Endpoint
	}
	if c.Voice.VAD.Threshold == 0 {
		c.Voice.VAD.Threshold = vadDefaults.Threshold
	}
	if c.Voice.VAD.MinSpeechMs == 0 {
		c.Voice.VAD.MinSpeechMs = vadDefaults.MinSpeechMs
	}
	if c.Voice.VAD.MinSilenceMs == 0 {
		c.Voice.VAD.MinSilenceMs = vadDefaults.MinSilenceMs
	}
	if c.Voice.VAD.PreBufferMs == 0 {
		c.Voice.VAD.PreBufferMs = vadDefaults.PreBufferMs
	}
}

// Save writes the current configuration to the default config file location.
func (c *Config) Save() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	configPath := filepath.Join(homeDir, ".cortex", "config.yaml")
	return c.SaveToPath(configPath)
}

// SaveToPath writes the current configuration to a specific file path.
func (c *Config) SaveToPath(path string) error {
	path = expandPath(path)

	// Ensure the config directory exists
	configDir := filepath.Dir(path)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	return writeConfigFile(path, c)
}

// GetDataDir returns the Cortex data directory path (~/.cortex).
func (c *Config) GetDataDir() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".cortex")
}

// GetConfigPath returns the full path to the config file.
func (c *Config) GetConfigPath() string {
	return filepath.Join(c.GetDataDir(), "config.yaml")
}

// EnsureDirectories creates all necessary directories for Cortex operation.
// This includes the data directory, logs directory, and knowledge database directory.
func (c *Config) EnsureDirectories() error {
	dirs := []string{
		c.GetDataDir(),
		filepath.Dir(c.Logging.File),
		filepath.Dir(c.Knowledge.DBPath),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}

// Validate checks the configuration for common errors and inconsistencies.
func (c *Config) Validate() error {
	// Validate LLM config
	if c.LLM.DefaultProvider == "" {
		return fmt.Errorf("llm.default_provider cannot be empty")
	}

	if _, exists := c.LLM.Providers[c.LLM.DefaultProvider]; !exists {
		return fmt.Errorf("default provider '%s' not found in providers map", c.LLM.DefaultProvider)
	}

	// Validate knowledge config
	validTiers := map[string]bool{"personal": true, "team": true, "public": true}
	if !validTiers[c.Knowledge.DefaultTier] {
		return fmt.Errorf("invalid default_tier '%s', must be one of: personal, team, public", c.Knowledge.DefaultTier)
	}

	if c.Knowledge.TrustDecayDays < 0 {
		return fmt.Errorf("trust_decay_days cannot be negative")
	}

	// Validate TUI config
	if c.TUI.Theme != "dark" && c.TUI.Theme != "light" {
		return fmt.Errorf("invalid theme '%s', must be 'dark' or 'light'", c.TUI.Theme)
	}

	if c.TUI.SidebarWidth < 10 || c.TUI.SidebarWidth > 100 {
		return fmt.Errorf("sidebar_width must be between 10 and 100")
	}

	// Validate logging config
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLevels[c.Logging.Level] {
		return fmt.Errorf("invalid log level '%s', must be one of: debug, info, warn, error", c.Logging.Level)
	}

	return nil
}

// writeConfigFile writes a Config struct to a YAML file.
// Uses gopkg.in/yaml.v3 directly to ensure proper tag-based serialization.
func writeConfigFile(path string, cfg *Config) error {
	// Marshal config to YAML bytes using yaml struct tags
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to file with proper permissions
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// expandPath expands ~ to the user's home directory in a path string.
func expandPath(path string) string {
	if strings.HasPrefix(path, "~") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(homeDir, path[1:])
	}
	return path
}
