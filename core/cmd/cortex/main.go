// Package main is the entry point for the Cortex CLI application.
// Cortex is a local-first AI assistant that combines intelligent context gathering,
// trust-based knowledge management, and team collaboration.
package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/lipgloss"
	_ "github.com/mattn/go-sqlite3" // SQLite driver for database/sql
	"github.com/muesli/termenv"
	"github.com/rs/zerolog"
	zlog "github.com/rs/zerolog/log"

	"github.com/normanking/cortex/internal/acontext"
	"github.com/normanking/cortex/internal/agent"
	"github.com/normanking/cortex/internal/autollm"
	internalbrain "github.com/normanking/cortex/internal/brain"
	"github.com/normanking/cortex/internal/bus"
	"github.com/normanking/cortex/internal/cognitive"
	"github.com/normanking/cortex/internal/cognitive/decomposer"
	"github.com/normanking/cortex/internal/cognitive/distillation"
	"github.com/normanking/cortex/internal/cognitive/feedback"
	"github.com/normanking/cortex/internal/cognitive/introspection"
	cogRouter "github.com/normanking/cortex/internal/cognitive/router"
	"github.com/normanking/cortex/internal/cognitive/templates"
	"github.com/normanking/cortex/internal/config"
	"github.com/normanking/cortex/internal/data"
	"github.com/normanking/cortex/internal/eval"
	"github.com/normanking/cortex/internal/facets"
	"github.com/normanking/cortex/internal/knowledge"
	"github.com/normanking/cortex/internal/llm"
	"github.com/normanking/cortex/internal/logging"
	"github.com/normanking/cortex/internal/memory"
	"github.com/normanking/cortex/internal/orchestrator"
	"github.com/normanking/cortex/internal/router"
	"github.com/normanking/cortex/internal/server"
	"github.com/normanking/cortex/internal/tools"
	"github.com/normanking/cortex/internal/tui"
	"github.com/normanking/cortex/internal/vision"
	visionmlx "github.com/normanking/cortex/internal/vision/mlx"
	visionollama "github.com/normanking/cortex/internal/vision/ollama"
	"github.com/normanking/cortex/internal/voice"
	"github.com/normanking/cortex/internal/voice/kokoro"
	"github.com/normanking/cortex/internal/voice/resemble"
	"github.com/normanking/cortex/pkg/brain"
	"github.com/normanking/cortex/pkg/brain/sleep"
	"github.com/normanking/cortex/pkg/theme"
	"github.com/normanking/cortex/pkg/types"
	pkgvoice "github.com/normanking/cortex/pkg/voice"
	"github.com/spf13/cobra"
)

var (
	version              = "0.1.0"
	cfgPath              string
	dbPath               string
	verbose              bool
	voiceEnabled         bool
	voiceOrchestratorURL string
	log                  *logging.Logger
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "cortex",
		Short: "Cortex - Local-first AI assistant with team collaboration",
		Long: `Cortex is an intelligent AI assistant that combines:
  • Smart routing with fast/slow classification
  • Persistent knowledge with three-tier scoping (personal/team/global)
  • Secure tool execution with risk assessment
  • Background sync with trust-weighted merging
  • Beautiful BubbleTea terminal interface

Start interactive mode:  cortex
Search knowledge:        cortex knowledge search <query>
Configuration:           cortex config show`,
		PersistentPreRunE: initLogging,
		RunE:              runTUI,
	}

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgPath, "config", "", "config file path (default ~/.cortex/config.yaml)")
	rootCmd.PersistentFlags().StringVar(&dbPath, "db", "", "database path (default ~/.cortex/knowledge.db)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolVar(&voiceEnabled, "voice", false, "enable voice features")
	rootCmd.PersistentFlags().StringVar(&voiceOrchestratorURL, "voice-url", "ws://localhost:8765/ws/voice", "voice orchestrator WebSocket URL")

	// Version command
	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Cortex v%s\n", version)
		},
	})

	// Knowledge command group
	rootCmd.AddCommand(knowledgeCmd())

	// Config command group
	rootCmd.AddCommand(configCmd())

	// UI command (Prism control plane)
	rootCmd.AddCommand(uiCmd())

	// TUI command (Charmbracelet terminal interface)
	rootCmd.AddCommand(tuiCmd())

	// Quick memory command (alias for knowledge add)
	rootCmd.AddCommand(rememberCmd())

	// One-shot ask command
	rootCmd.AddCommand(askCmd())

	// Debug commands
	rootCmd.AddCommand(debugCmd())

	// Sync commands
	rootCmd.AddCommand(syncCmd())

	// Voice commands (CR-012)
	rootCmd.AddCommand(voiceCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// LOGGING INITIALIZATION
// ═══════════════════════════════════════════════════════════════════════════════

func initLogging(cmd *cobra.Command, args []string) error {
	// Determine log file path
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	logDir := filepath.Join(home, ".cortex", "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to create log directory: %v\n", err)
	}

	// Create timestamped log file for this session
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	logFile := filepath.Join(logDir, fmt.Sprintf("cortex_%s.log", timestamp))

	var cfg *logging.Config
	if verbose {
		cfg = logging.VerboseConfig()
	} else {
		cfg = logging.DefaultConfig()
	}

	// Always enable file logging
	cfg.FilePath = logFile

	log = logging.New(cfg)
	logging.SetGlobal(log)

	log.Info("Cortex session started - logging to %s", logFile)

	if verbose {
		log.Debug("Verbose logging enabled")
		log.Debug("Config path: %s", getConfigPath())
		log.Debug("DB path override: %s", dbPath)
	}

	return nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// TUI COMMAND (ROOT)
// ═══════════════════════════════════════════════════════════════════════════════

// loadEnvFile loads API keys from ~/.cortex/.env into process environment.
// This makes keys available to os.Getenv() calls throughout the application.
func loadEnvFile() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}

	envPath := filepath.Join(home, ".cortex", ".env")
	data, err := os.ReadFile(envPath)
	if err != nil {
		return // File doesn't exist or can't be read
	}

	// Parse each line
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)

		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse KEY=VALUE
		if idx := strings.Index(line, "="); idx > 0 {
			key := strings.TrimSpace(line[:idx])
			value := strings.TrimSpace(line[idx+1:])
			value = strings.Trim(value, `"'`)

			// Only set if not already in environment
			if os.Getenv(key) == "" && value != "" {
				os.Setenv(key, value)
				if log != nil {
					log.Debug("[Env] Loaded %s from .env file", key)
				}
			}
		}
	}
}

func runTUI(cmd *cobra.Command, args []string) error {
	if log != nil {
		defer log.Trace("runTUI")()
	}

	// Load .env file into process environment BEFORE initializing orchestrator
	// This makes API keys available to os.Getenv() calls
	loadEnvFile()

	// Get configuration
	cfg, _ := loadConfig()

	// Apply CLI flags to voice config
	if voiceEnabled {
		cfg.Voice.Enabled = true
	}
	if voiceOrchestratorURL != "" {
		cfg.Voice.OrchestratorURL = voiceOrchestratorURL
	}

	// Initialize components
	log.Info("Initializing Cortex AI...")
	orch, eventBus, modelSel, llmProv, _, db, cleanup, err := initializeOrchestrator()
	if err != nil {
		log.Error("Failed to initialize orchestrator: %v", err)
		return fmt.Errorf("failed to initialize: %w", err)
	}
	defer cleanup()

	// ═══════════════════════════════════════════════════════════════════════════════
	// BRAIN EXECUTIVE INITIALIZATION
	// ═══════════════════════════════════════════════════════════════════════════════

	// Create Brain Executive if LLM is available
	var brainExec *brain.Executive
	if llmProv != nil && llmProv.Available() {
		log.Info("Initializing Brain Executive...")
		brainExec = internalbrain.NewExecutive(internalbrain.FactoryConfig{
			LLMProvider:  llmProv,
			MemorySystem: nil,    // CoreMemoryStore doesn't implement brain.MemorySystem
			UserID:       "user", // Default user ID for CLI
		})
		brainExec.Start()
		log.Info("Brain Executive started")

		// CR-027: Register lobes with capability registrar
		if orch != nil && orch.Registrar() != nil {
			if err := internalbrain.RegisterLobesWithRegistrar(brainExec, orch.Registrar()); err != nil {
				log.Warn("Failed to register lobes with registrar: %v", err)
			} else {
				log.Info("Lobes registered with capability registrar")
			}
		}

		defer func() {
			brainExec.Stop()
			log.Info("Brain Executive stopped")
		}()
	} else {
		log.Warn("Brain Executive disabled (LLM not available)")
	}

	// ═══════════════════════════════════════════════════════════════════════════════
	// CORTEXEYES INITIALIZATION (CR-023: Screen Awareness)
	// ═══════════════════════════════════════════════════════════════════════════════

	var cortexEyes *vision.CortexEyes
	var visionLauncher *vision.MLXVisionLauncher
	if cfg.CortexEyes.Enabled {
		log.Info("Initializing CortexEyes (Screen Awareness)...")

		// Detect available vision backend (MLX preferred for Apple Silicon)
		var fastProvider, smartProvider vision.Provider
		var visionURL string

		// Configure and start vision server launcher
		visionModel := cfg.CortexEyes.Vision.Model
		if visionModel == "" {
			visionModel = "mlx-community/Qwen2-VL-2B-Instruct-4bit"
		}

		visionPort := 8082
		if cfg.CortexEyes.Vision.Endpoint != "" {
			// Extract port from endpoint if configured
			if _, err := fmt.Sscanf(cfg.CortexEyes.Vision.Endpoint, "http://127.0.0.1:%d", &visionPort); err != nil {
				visionPort = 8082
			}
		}

		// Create and start vision launcher
		visionLauncher = vision.NewMLXVisionLauncher(vision.MLXVisionConfig{
			Model:          visionModel,
			Host:           "127.0.0.1",
			Port:           visionPort,
			StartupTimeout: 120 * time.Second,
			HealthTimeout:  5 * time.Second,
		})

		// Try to start the vision server (this will use existing if already running)
		if err := visionLauncher.EnsureRunning(context.Background()); err != nil {
			log.Warn("[CortexEyes] Could not start vision server: %v", err)
			log.Warn("[CortexEyes] Vision context extraction will be disabled (activity tracking still works)")
		} else {
			visionURL = visionLauncher.Endpoint()
			log.Info("[CortexEyes] Vision server ready @ %s", visionURL)
		}

		// Set up providers based on what's available
		if visionURL != "" {
			// Vision server is running
			fastProvider = visionmlx.NewQwen2VLFastProvider(visionURL)
			smartProvider = visionmlx.NewQwen2VLSmartProvider(visionURL)
		} else {
			// Fall back to Ollama if available
			ollamaURL := cfg.LLM.Providers["ollama"].Endpoint
			if ollamaURL == "" {
				ollamaURL = "http://127.0.0.1:11434"
			}
			// Check if Ollama has vision models
			if _, err := llm.FetchMLXModels(ollamaURL); err == nil {
				log.Info("[CortexEyes] Using Ollama vision backend @ %s", ollamaURL)
				fastProvider = visionollama.NewMoondreamProvider(ollamaURL)
				smartProvider = visionollama.NewMiniCPMProvider(ollamaURL)
			} else {
				log.Warn("[CortexEyes] No vision backend available - using nil providers")
				// Use nil providers - CortexEyes will gracefully degrade
			}
		}

		// Create vision router
		visionRouter := vision.NewRouter(fastProvider, smartProvider, vision.DefaultConfig())

		// Create CortexEyes config from app config
		eyesConfig := &vision.CortexEyesConfig{
			CaptureFPS:         cfg.CortexEyes.Capture.FPS,
			ChangeThreshold:    cfg.CortexEyes.Capture.ChangeThreshold,
			MinInterval:        time.Duration(cfg.CortexEyes.Capture.MinIntervalSec) * time.Second,
			MaxRetentionDays:   cfg.CortexEyes.Privacy.MaxRetentionDays,
			Enabled:            true,
			EnablePatterns:     cfg.CortexEyes.Learning.EnablePatterns,
			EnableInsights:     cfg.CortexEyes.Learning.EnableInsights,
			Webcam: &vision.WebcamConfig{
				Enabled:     cfg.CortexEyes.Webcam.Enabled,
				CameraIndex: cfg.CortexEyes.Webcam.CameraIndex,
				FPS:         cfg.CortexEyes.Webcam.FPS,
			},
			Privacy: &vision.PrivacyConfig{
				Enabled:          true,
				ExcludedApps:     cfg.CortexEyes.Privacy.ExcludedApps,
				ExcludedWindows:  cfg.CortexEyes.Privacy.ExcludedWindows,
				AutoPauseOnIdle:  time.Duration(cfg.CortexEyes.Privacy.AutoPauseIdleMin) * time.Minute,
				MaxRetentionDays: cfg.CortexEyes.Privacy.MaxRetentionDays,
				RequireConsent:   cfg.CortexEyes.Privacy.RequireConsent,
				AllowedHours: &vision.TimeRange{
					Start: cfg.CortexEyes.Privacy.AllowedHoursStart,
					End:   cfg.CortexEyes.Privacy.AllowedHoursEnd,
				},
			},
		}

		// Create CortexEyes
		var err error
		cortexEyes, err = vision.NewCortexEyes(visionRouter, db, eventBus, eyesConfig)
		if err != nil {
			log.Warn("Failed to initialize CortexEyes: %v", err)
		} else {
			// Start CortexEyes watching
			if err := cortexEyes.Start(context.Background()); err != nil {
				log.Warn("Failed to start CortexEyes: %v", err)
			} else {
				log.Info("CortexEyes started (watching screen for contextual learning)")
			}

			defer func() {
				cortexEyes.Stop()
				log.Info("CortexEyes stopped")
			}()
		}

		// Stop vision launcher on exit (keeps running between Cortex sessions for faster restart)
		// Note: We don't stop it by default to allow faster restarts. Use scripts/stop-vision-server.sh to fully stop.
		_ = visionLauncher // Suppress unused warning
	} else {
		log.Info("CortexEyes disabled (enable in config.yaml)")
	}

	// Suppress unused variable warning
	_ = cortexEyes

	// ═══════════════════════════════════════════════════════════════════════════════
	// VOICE BRIDGE INITIALIZATION
	// ═══════════════════════════════════════════════════════════════════════════════

	var voiceBridge *voice.VoiceBridge

	if cfg.Voice.Enabled {
		log.Info("Initializing voice bridge...")

		voiceBridgeConfig := voice.BridgeConfig{
			OrchestratorURL:       cfg.Voice.OrchestratorURL,
			InitialReconnectDelay: time.Duration(cfg.Voice.ReconnectDelay) * time.Second,
			MaxReconnectDelay:     30 * time.Second,
			MaxReconnects:         cfg.Voice.MaxReconnects,
			PingInterval:          30 * time.Second,
			PongTimeout:           60 * time.Second,
			WriteTimeout:          10 * time.Second,
			ReadTimeout:           120 * time.Second,
		}

		voiceBridge = voice.NewVoiceBridge(voiceBridgeConfig)
		voiceBridge.SetEventBus(eventBus) // Set event bus for publishing voice events

		// CR-021: Initialize voice emotion bridge for Blackboard integration
		voice.InitBlackboardBridge(eventBus)
		log.Debug("Voice emotion bridge initialized for Blackboard")

		// Attempt to connect to voice orchestrator
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := voiceBridge.Connect(ctx); err != nil {
			log.Warn("Voice orchestrator not available, voice features disabled: %v", err)
		} else {
			log.Info("Connected to voice orchestrator at %s", cfg.Voice.OrchestratorURL)

			// Wire up interrupt handler
			voiceBridge.OnInterrupt(func(reason string) {
				log.Info("[Voice] Interrupt received: reason=%s", reason)
				if err := orch.Interrupt(reason); err != nil {
					log.Error("[Voice] Failed to interrupt orchestrator: %v", err)
				}
			})

			// Wire up transcript handler for logging (event publishing is done by VoiceBridge)
			voiceBridge.OnTranscript(func(text string, isFinal bool) {
				if isFinal && text != "" {
					log.Info("[Voice] STT Final: %q", text)
				} else if text != "" {
					log.Debug("[Voice] STT Interim: %q", text)
				}
			})

			// Wire up status handler for connection monitoring
			voiceBridge.OnStatus(func(state string) {
				log.Info("[Voice] Status change: %s", state)
			})
		}

		defer func() {
			if voiceBridge != nil {
				voiceBridge.Close()
			}
		}()
	} else {
		log.Debug("Voice features disabled in config")
	}

	// ═══════════════════════════════════════════════════════════════════════════════
	// RESEMBLE AGENTS WEBHOOK SERVER (CR-016)
	// ═══════════════════════════════════════════════════════════════════════════════

	var webhookServer *resemble.WebhookServer

	if cfg.Voice.ResembleAgents.Enabled {
		log.Info("Initializing Resemble Agents webhook server...")

		webhookCfg := resemble.WebhookConfig{
			Port:      cfg.Voice.ResembleAgents.WebhookPort,
			Host:      cfg.Voice.ResembleAgents.WebhookHost,
			AuthToken: cfg.Voice.ResembleAgents.WebhookToken,
		}
		webhookServer = resemble.NewWebhookServer(webhookCfg)

		if err := webhookServer.Start(); err != nil {
			log.Error("Failed to start webhook server: %v", err)
		} else {
			log.Info("Webhook server started on http://%s:%d", webhookCfg.Host, webhookCfg.Port)
			defer webhookServer.Stop()
		}
	}

	// ═══════════════════════════════════════════════════════════════════════════════
	// BACKGROUND SYNC WORKER (ACONTEXT CLOUD SYNC)
	// ═══════════════════════════════════════════════════════════════════════════════

	var syncWorker acontext.BackgroundWorker
	if cfg != nil && cfg.Sync.Enabled {
		log.Info("Initializing Acontext cloud sync...")

		// Get store for sync operations
		store, _, _, storeErr := initializeStore()
		if storeErr != nil {
			log.Warn("Failed to initialize store for sync: %v", storeErr)
		} else {
			// Create acontext components
			acontextCfg := acontext.Config{
				Endpoint:     cfg.Sync.Endpoint,
				Token:        cfg.Sync.AuthToken,
				TeamID:       cfg.Sync.TeamID,
				SyncInterval: cfg.Sync.Interval,
				Enabled:      true,
			}

			// Check if sync is properly configured
			if acontextCfg.Token == "" {
				log.Warn("Sync enabled but no auth token configured")
			} else {
				// Create sync components
				client := acontext.NewClient(acontextCfg)
				localStore := acontext.NewStoreAdapter(store)
				resolver := acontext.NewTrustWeightedResolver()
				syncer := acontext.NewSyncService(client, localStore, resolver, acontextCfg)

				// Create event bus for sync status updates
				eventBus := bus.New()
				defer eventBus.Close()

				// Create and start background worker
				syncWorker = acontext.NewWorker(syncer, acontextCfg, eventBus)
				ctx := context.Background()
				if err := syncWorker.Start(ctx); err != nil {
					log.Warn("Failed to start sync worker: %v", err)
				} else {
					log.Info("Background sync worker started (interval: %v)", acontextCfg.SyncInterval)
					defer func() {
						if syncWorker != nil {
							log.Debug("Stopping sync worker...")
							syncWorker.Stop()
						}
					}()
				}
			}
		}
	} else {
		log.Debug("Cloud sync disabled in config")
	}

	log.Debug("Creating TUI with config: Title=Cortex AI, MaxHistory=100")
	tuiOpts := []tui.Option{
		tui.WithConfig(&tui.Config{
			Title:         "Cortex AI",
			ShowTimestamp: false,
			ShowDuration:  true,
			MaxHistory:    100,
		}),
		tui.WithAppConfig(cfg),
	}
	// Add Brain Executive if available
	if brainExec != nil {
		tuiOpts = append(tuiOpts, tui.WithBrainExecutive(brainExec))
	}
	// Add CortexEyes if available (CR-023)
	if cortexEyes != nil {
		tuiOpts = append(tuiOpts, tui.WithCortexEyes(cortexEyes))
	}
	// Apply theme from config if present
	if cfg.TUI.Theme != "" {
		tuiOpts = append(tuiOpts, tui.WithTheme(cfg.TUI.Theme))
	}
	// Set default model from auto-selection
	if modelSel != nil && modelSel.Model != "" {
		tuiOpts = append(tuiOpts, tui.WithDefaultModel(modelSel.Provider, modelSel.Model))
		log.Info("TUI default model set to: %s/%s", modelSel.Provider, modelSel.Model)
	}
	// Wire up voice bridge to TUI if enabled (CR-010 Track 4)
	if voiceBridge != nil && voiceBridge.IsConnected() {
		adapter := tui.NewTUIVoiceBridgeAdapter(voiceBridge, eventBus, nil)

		// CR-012: Initialize TTS router for hybrid voice routing (local Kokoro + cloud Resemble)
		// Create Kokoro provider for local voices (fast lane)
		kokoroProvider := kokoro.NewProvider(kokoro.Config{
			BaseURL:       "http://localhost:8880",
			DefaultVoice:  "af_bella",
			MaxTextLength: 2000,
		})

		// Create TTS router with Kokoro as fast provider
		routerConfig := voice.DefaultRouterConfig()
		routerConfig.FastLaneDefaultVoice = "af_bella"
		routerConfig.EnableCache = true
		routerConfig.Enabled = true
		ttsRouter := voice.NewRouter(kokoroProvider, nil, routerConfig)

		// Add Resemble cloud provider if API key is configured
		resembleAPIKey := os.Getenv("RESEMBLE_API_KEY")
		if resembleAPIKey == "" {
			// Try loading from .cortex/.env
			if envData, err := os.ReadFile(filepath.Join(os.Getenv("HOME"), ".cortex", ".env")); err == nil {
				for _, line := range strings.Split(string(envData), "\n") {
					if strings.HasPrefix(line, "RESEMBLE_API_KEY=") {
						resembleAPIKey = strings.TrimPrefix(line, "RESEMBLE_API_KEY=")
						break
					}
				}
			}
		}
		if resembleAPIKey != "" {
			resembleCfg := resemble.Config{
				APIKey:     resembleAPIKey,
				SampleRate: 48000,
			}
			resembleProvider, err := resemble.NewProvider(resembleCfg)
			if err != nil {
				log.Warn("Failed to initialize Resemble provider: %v", err)
			} else {
				ttsRouter.SetCloudProvider(resembleProvider)
				log.Info("CR-012: Resemble cloud TTS provider initialized for hybrid voice routing")
			}
		}

		// Connect TTS router to adapter for hybrid routing
		adapter.SetTTSRouter(ttsRouter)

		tuiOpts = append(tuiOpts, tui.WithVoiceBridge(adapter))
		log.Info("Voice bridge connected to TUI (hybrid routing enabled)")

		// CR-011: Initialize Voice Intelligence components
		// Create mode detector for voice/text mode detection
		modeDetector := voice.NewModeDetector()
		modeDetector.SetTTSEnabled(true) // TTS available via voice bridge

		// Create VoiceHandler with mode detector (TTSEngine and VoiceLLM can be nil,
		// the handler will use the voice bridge directly for TTS)
		voiceHandler := tui.NewVoiceHandler(nil, nil, modeDetector)
		tuiOpts = append(tuiOpts, tui.WithVoiceHandler(voiceHandler))
		log.Info("CR-011 VoiceHandler initialized")

		// CR-012-C: Initialize HenryBrain for conversational state machine
		henryConfig := pkgvoice.DefaultHenryBrainConfig()

		// Create TTS engine and HenryAdapter for audio playback
		ttsConfig := voice.DefaultTTSConfig()
		ttsEngine := voice.NewTTSEngine(ttsConfig)
		henryAdapter := voice.NewHenryAdapter(ttsEngine)

		// Connect VoiceBridge to HenryAdapter for pre-cached audio playback
		henryAdapter.SetVoiceBridge(voiceBridge)

		// Create HenryBrain with adapter for TTS and audio playback
		henryBrain, err := pkgvoice.NewHenryBrain(henryConfig, henryAdapter, henryAdapter)
		if err != nil {
			log.Warn("Failed to initialize HenryBrain: %v", err)
		} else {
			tuiOpts = append(tuiOpts, tui.WithHenryBrain(henryBrain))
			log.Info("CR-012-C HenryBrain initialized with TTSEngine + VoiceBridge (state machine enabled)")
		}
	}
	app := tui.New(orch, tuiOpts...)

	app.AddSystemMessage(tui.WelcomeScreen(theme.Get(theme.DefaultTheme)))

	log.Info("Starting interactive TUI...")

	// ═══════════════════════════════════════════════════════════════════════════════
	// CRITICAL: Redirect ALL logging away from stdout/stderr before TUI starts
	// ═══════════════════════════════════════════════════════════════════════════════

	// 1. Disable console output for custom logger
	logging.DisableConsoleOutput()
	defer logging.EnableConsoleOutput()

	// 2. Redirect zerolog to log file (used by vision/voice/memory packages)
	home, _ := os.UserHomeDir()
	logDir := filepath.Join(home, ".cortex", "logs")
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	zerologPath := filepath.Join(logDir, fmt.Sprintf("cortex_zerolog_%s.log", timestamp))

	zerologFile, err := os.OpenFile(zerologPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Warn("Failed to redirect zerolog: %v", err)
	} else {
		defer zerologFile.Close()
		// Redirect ALL zerolog output to file (including global log.Info(), log.Debug(), etc.)
		zerologWriter := zerolog.ConsoleWriter{Out: zerologFile, NoColor: true}
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
		fileLogger := zerolog.New(zerologWriter).With().Timestamp().Logger()
		zerolog.DefaultContextLogger = &fileLogger
		// CRITICAL: Also set the global logger used by log.Info(), log.Debug(), etc.
		zlog.Logger = fileLogger
		log.Debug("Zerolog redirected to: %s", zerologPath)
	}

	return app.Run()
}

// ═══════════════════════════════════════════════════════════════════════════════
// KNOWLEDGE COMMANDS
// ═══════════════════════════════════════════════════════════════════════════════

func knowledgeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "knowledge",
		Aliases: []string{"k"},
		Short:   "Manage knowledge items",
	}

	// List command
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List knowledge items",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, _, cleanup, err := initializeStore()
			if err != nil {
				return err
			}
			defer cleanup()

			// Get items by scope (using the store directly)
			items, err := store.GetByScope(context.Background(), types.ScopePersonal)
			if err != nil {
				return fmt.Errorf("failed to list: %w", err)
			}

			if len(items) == 0 {
				fmt.Println("No knowledge items found.")
				return nil
			}

			fmt.Printf("Found %d items:\n\n", len(items))
			for _, item := range items {
				fmt.Printf("  [%s] %s\n", item.Scope, truncate(item.Content, 60))
				fmt.Printf("       Tags: %s | Trust: %.1f\n\n",
					strings.Join(item.Tags, ", "), item.TrustScore)
			}
			return nil
		},
	})

	// Search command
	cmd.AddCommand(&cobra.Command{
		Use:   "search [query]",
		Short: "Search knowledge items using FTS5",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.Join(args, " ")
			log.Debug("Searching knowledge for: %q", query)

			store, _, cleanup, err := initializeStore()
			if err != nil {
				return err
			}
			defer cleanup()

			// Create fabric with searcher
			log.Debug("Creating search components (FTS5 + TrustWeightedMerge)...")
			searcher := knowledge.NewFTS5Searcher(store.DB())
			merger := knowledge.NewTrustWeightedMerge()
			fabric := knowledge.NewFabric(store, searcher, merger)

			log.Debug("Executing FTS5 search...")
			start := time.Now()
			result, err := fabric.Search(context.Background(), query, types.SearchOptions{
				Limit: 10,
			})
			duration := time.Since(start)

			if err != nil {
				log.Error("Search failed: %v", err)
				return fmt.Errorf("search failed: %w", err)
			}

			log.Debug("Search completed in %v", duration)

			if result == nil || len(result.Items) == 0 {
				log.Debug("No results found")
				fmt.Printf("No results found for: %s\n", query)
				return nil
			}

			log.Debug("Found %d results from tier: %s", len(result.Items), result.Tier)
			fmt.Printf("Found %d results in %v (Tier: %s):\n\n", len(result.Items), duration, result.Tier)
			for i, item := range result.Items {
				fmt.Printf("%d. [%s] %s\n", i+1, item.Scope, truncate(item.Content, 60))
				fmt.Printf("   Trust: %.1f | Tags: %s\n\n", item.TrustScore, strings.Join(item.Tags, ", "))
			}
			return nil
		},
	})

	// Add command
	cmd.AddCommand(&cobra.Command{
		Use:   "add [content]",
		Short: "Add a new knowledge item",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			content := strings.Join(args, " ")

			store, _, cleanup, err := initializeStore()
			if err != nil {
				return err
			}
			defer cleanup()

			// Create fabric
			searcher := knowledge.NewFTS5Searcher(store.DB())
			merger := knowledge.NewTrustWeightedMerge()
			fabric := knowledge.NewFabric(store, searcher, merger)

			item := &types.KnowledgeItem{
				ID:         fmt.Sprintf("cli-%d", time.Now().UnixNano()),
				Type:       types.TypeDocument, // Default type for CLI-added items
				Content:    content,
				Scope:      types.ScopePersonal,
				Tags:       []string{"manual", "cli"},
				TrustScore: 0.5,
				AuthorID:   "cli-user",
				AuthorName: "CLI User",
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
			}

			if err := fabric.Create(context.Background(), item); err != nil {
				return fmt.Errorf("failed to add: %w", err)
			}

			fmt.Printf("✅ Added knowledge item: %s\n", item.ID)
			return nil
		},
	})

	return cmd
}

// ═══════════════════════════════════════════════════════════════════════════════
// CONFIG COMMANDS
// ═══════════════════════════════════════════════════════════════════════════════

func configCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration",
	}

	// Show command
	cmd.AddCommand(&cobra.Command{
		Use:   "show",
		Short: "Show current configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			fmt.Println("Cortex Configuration:")
			fmt.Println("─────────────────────")
			fmt.Printf("Database Path: %s\n", cfg.Knowledge.DBPath)
			fmt.Printf("Sync Enabled:  %t\n", cfg.Sync.Enabled)
			fmt.Printf("Sync Interval: %s\n", cfg.Sync.Interval)
			fmt.Printf("Log Level:     %s\n", cfg.Logging.Level)
			return nil
		},
	})

	// Path command
	cmd.AddCommand(&cobra.Command{
		Use:   "path",
		Short: "Show configuration file path",
		Run: func(cmd *cobra.Command, args []string) {
			path := getConfigPath()
			fmt.Println(path)
		},
	})

	return cmd
}

// ═══════════════════════════════════════════════════════════════════════════════
// REMEMBER COMMAND (Quick memory add)
// ═══════════════════════════════════════════════════════════════════════════════

func rememberCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remember [content]",
		Short: "Remember information (adds to knowledge fabric)",
		Long: `Remember information for later recall.

Examples:
  cortex remember "My favorite color is blue"
  cortex remember "Project deadline is March 15th"
  cortex remember "API key for service X is stored in ~/.config/x"`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			content := strings.Join(args, " ")

			store, _, cleanup, err := initializeStore()
			if err != nil {
				return err
			}
			defer cleanup()

			// Create fabric
			searcher := knowledge.NewFTS5Searcher(store.DB())
			merger := knowledge.NewTrustWeightedMerge()
			fabric := knowledge.NewFabric(store, searcher, merger)

			item := &types.KnowledgeItem{
				ID:         fmt.Sprintf("remember-%d", time.Now().UnixNano()),
				Type:       types.TypeDocument,
				Content:    content,
				Scope:      types.ScopePersonal,
				Tags:       []string{"memory", "user-defined"},
				TrustScore: 0.8, // Higher trust for explicit user memories
				AuthorID:   "user",
				AuthorName: "User",
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
			}

			if err := fabric.Create(context.Background(), item); err != nil {
				return fmt.Errorf("failed to remember: %w", err)
			}

			fmt.Printf("✅ Remembered: %s\n", content)
			return nil
		},
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// ASK COMMAND (One-shot query)
// ═══════════════════════════════════════════════════════════════════════════════

func askCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ask [question]",
		Short: "Ask Cortex a question (one-shot query)",
		Long: `Ask a question and get an AI-powered response.

Examples:
  cortex ask "What's my favorite color?"
  cortex ask "Summarize my project notes"
  cortex ask "What commands did I run yesterday?"`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			question := strings.Join(args, " ")

			// Initialize orchestrator
			orch, _, _, _, _, _, cleanup, err := initializeOrchestrator()
			if err != nil {
				return fmt.Errorf("failed to initialize: %w", err)
			}
			defer cleanup()

			// Get working directory
			cwd, _ := os.Getwd()

			// Create request
			req := &orchestrator.Request{
				ID:        fmt.Sprintf("ask-%d", time.Now().UnixNano()),
				Type:      orchestrator.RequestChat,
				Input:     question,
				Timestamp: time.Now(),
				Context: &orchestrator.RequestContext{
					WorkingDir: cwd,
				},
			}

			// Process request
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()

			resp, err := orch.Process(ctx, req)
			if err != nil {
				return fmt.Errorf("failed to process: %w", err)
			}

			if !resp.Success {
				return fmt.Errorf("query failed: %s", resp.Error)
			}

			fmt.Println(resp.Content)
			return nil
		},
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// DEBUG COMMANDS
// ═══════════════════════════════════════════════════════════════════════════════

func debugCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "debug",
		Short: "Debug and diagnostic commands",
	}

	// Prompt tier command
	cmd.AddCommand(&cobra.Command{
		Use:   "prompt-tier [model]",
		Short: "Show what prompt tier would be used for a model",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			modelName := args[0]

			// Classify the model tier
			tier := eval.ClassifyModelTier("", modelName)

			// Determine prompt tier based on model tier
			var promptTier string
			switch tier {
			case eval.TierSmall, eval.TierMedium:
				promptTier = "small"
			case eval.TierLarge, eval.TierXL, eval.TierFrontier:
				promptTier = "large"
			default:
				promptTier = "small" // Default to small for unknown
			}

			fmt.Printf("Model: %s\n", modelName)
			fmt.Printf("Model Tier: %s\n", tier)
			fmt.Printf("Prompt Tier: %s\n", promptTier)

			return nil
		},
	})

	// Model info command
	cmd.AddCommand(&cobra.Command{
		Use:   "model-info [model]",
		Short: "Show detailed model information and scoring",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			modelName := args[0]

			// Get model registry
			registry := eval.DefaultRegistry()

			// Try to get model info
			var info *eval.ModelCapability
			var found bool
			info, found = registry.Get("", modelName)
			if !found {
				// Try with common providers
				for _, provider := range []string{"ollama", "anthropic", "openai", "gemini", "grok"} {
					info, found = registry.Get(provider, modelName)
					if found {
						break
					}
				}
			}

			if found && info != nil {
				fmt.Printf("Model: %s\n", info.DisplayName)
				fmt.Printf("ID: %s\n", info.ID)
				fmt.Printf("Provider: %s\n", info.Provider)
				fmt.Printf("Tier: %s\n", info.Tier)
				fmt.Printf("Context Window: %d tokens\n", info.ContextWindow)
				fmt.Printf("Score: %d/100\n", info.Score.Overall)
				fmt.Printf("  Coding: %d\n", info.Score.Coding)
				fmt.Printf("  Reasoning: %d\n", info.Score.Reasoning)
				fmt.Printf("  Instruction: %d\n", info.Score.Instruction)
				fmt.Printf("  Speed: %d\n", info.Score.Speed)
			} else {
				// Fallback to tier classification
				tier := eval.ClassifyModelTier("", modelName)
				fmt.Printf("Model: %s\n", modelName)
				fmt.Printf("Tier: %s (heuristic)\n", tier)
				fmt.Println("Note: Model not in registry, using heuristic classification")
			}

			return nil
		},
	})

	// Fallback chain command
	cmd.AddCommand(&cobra.Command{
		Use:   "fallback-chain",
		Short: "Show the configured fallback provider chain",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Fallback Chain Priority:")
			fmt.Println("─────────────────────────")
			fmt.Println("1. Local (Ollama) - Most capable available model")

			// Check which cloud providers are configured
			providers := []struct {
				name   string
				envVar string
				model  string
			}{
				{"Grok", "XAI_API_KEY", "grok-3"},
				{"Anthropic", "ANTHROPIC_API_KEY", "claude-sonnet-4-20250514"},
				{"OpenAI", "OPENAI_API_KEY", "gpt-4o"},
				{"Gemini", "GEMINI_API_KEY", "gemini-2.0-flash"},
			}

			priority := 2
			for _, p := range providers {
				status := "❌ Not configured"
				if os.Getenv(p.envVar) != "" {
					status = fmt.Sprintf("✅ Configured (%s)", p.model)
				}
				fmt.Printf("%d. %s - %s\n", priority, p.name, status)
				priority++
			}

			return nil
		},
	})

	return cmd
}

// ═══════════════════════════════════════════════════════════════════════════════
// SYNC COMMANDS (ACONTEXT CLOUD SYNC)
// ═══════════════════════════════════════════════════════════════════════════════

func syncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Manage knowledge synchronization",
		Long: `Manage knowledge synchronization (local or cloud).

Local Sync Server (CR-013):
  cortex sync install       # Install local sync server
  cortex sync server start  # Start server
  cortex sync server stop   # Stop server
  cortex sync server status # Check status

Cloud Sync (Acontext):
  cortex sync status    # Show sync status
  cortex sync enable    # Enable cloud sync
  cortex sync disable   # Disable cloud sync

Export/Import:
  cortex sync export    # Export knowledge to JSON
  cortex sync import    # Import knowledge from JSON

Configure sync in ~/.cortex/config.yaml:
  sync:
    mode: local         # local | cloud
    enabled: true
    endpoint: https://api.acontext.io  # for cloud mode
    token: your-token
    team_id: your-team-id
    interval: 5m`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Default action: force sync now
			return runSyncNow()
		},
	}

	// Local server management commands (CR-013)
	cmd.AddCommand(syncInstallCmd())
	cmd.AddCommand(syncServerCmd())
	cmd.AddCommand(syncExportCmd())
	cmd.AddCommand(syncImportCmd())

	// Cloud sync commands
	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show current sync status",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSyncStatus()
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "enable",
		Short: "Enable automatic background sync",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSyncToggle(true)
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "disable",
		Short: "Disable automatic background sync",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSyncToggle(false)
		},
	})

	return cmd
}

// ═══════════════════════════════════════════════════════════════════════════════
// SYNC SERVER COMMANDS (CR-013: Native Sync Server)
// ═══════════════════════════════════════════════════════════════════════════════

func syncInstallCmd() *cobra.Command {
	var upgrade bool

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install local sync server",
		Long: `Install the local sync server to ~/.cortex/syncserver/

This will:
  1. Create a Python virtual environment
  2. Install fastapi, uvicorn, aiosqlite dependencies
  3. Create management scripts (start.sh, stop.sh, status.sh)

Requirements:
  - Python 3.10 or higher
  - pip`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Installing Sync Server...")

			// Build install script path
			scriptPath := ""

			// Check common locations
			locations := []string{
				"scripts/install-syncserver.sh",
				"./scripts/install-syncserver.sh",
			}

			for _, loc := range locations {
				if _, err := os.Stat(loc); err == nil {
					scriptPath = loc
					break
				}
			}

			// If not found, try to use embedded install logic
			if scriptPath == "" {
				return runEmbeddedSyncInstall(upgrade)
			}

			// Build arguments
			installArgs := []string{scriptPath}
			if upgrade {
				installArgs = append(installArgs, "--upgrade")
			}

			c := exec.Command("bash", installArgs...)
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr

			return c.Run()
		},
	}

	cmd.Flags().BoolVar(&upgrade, "upgrade", false, "Upgrade existing installation")

	return cmd
}

func runEmbeddedSyncInstall(upgrade bool) error {
	launcher := acontext.GetSyncServerLauncher()
	config := launcher.Config()

	fmt.Printf("Installing to: %s\n", config.InstallDir)

	// Check Python
	pythonCmd := "python3"
	if _, err := exec.LookPath(pythonCmd); err != nil {
		pythonCmd = "python"
		if _, err := exec.LookPath(pythonCmd); err != nil {
			return fmt.Errorf("Python not found. Please install Python 3.10 or higher")
		}
	}

	// Check Python version
	versionOut, err := exec.Command(pythonCmd, "-c", "import sys; print(f'{sys.version_info.major}.{sys.version_info.minor}')").Output()
	if err != nil {
		return fmt.Errorf("failed to check Python version: %w", err)
	}
	version := strings.TrimSpace(string(versionOut))
	fmt.Printf("Found Python %s\n", version)

	// Create virtual environment
	if !upgrade || !launcher.IsInstalled() {
		fmt.Println("Creating virtual environment...")
		if err := os.MkdirAll(config.InstallDir, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
		if err := os.MkdirAll(filepath.Join(config.InstallDir, "data"), 0755); err != nil {
			return fmt.Errorf("failed to create data directory: %w", err)
		}

		c := exec.Command(pythonCmd, "-m", "venv", config.InstallDir)
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		if err := c.Run(); err != nil {
			return fmt.Errorf("failed to create venv: %w", err)
		}
	}

	// Install dependencies
	fmt.Println("Installing dependencies...")
	pipPath := filepath.Join(config.InstallDir, "bin", "pip")
	c := exec.Command(pipPath, "install", "--upgrade", "pip")
	c.Run() // Ignore errors for pip upgrade

	c = exec.Command(pipPath, "install", "fastapi", "uvicorn[standard]", "pydantic", "aiosqlite")
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("failed to install dependencies: %w", err)
	}

	fmt.Println("\n✅ Sync Server installed successfully!")
	fmt.Printf("Directory: %s\n", config.InstallDir)
	fmt.Println("\nNote: Run 'cortex sync server start' to start the server")

	return nil
}

func syncServerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Manage local sync server",
		Long: `Manage the local sync server (Acontext-compatible API).

Commands:
  start   - Start the sync server
  stop    - Stop the sync server
  status  - Check server status`,
	}

	cmd.AddCommand(syncServerStartCmd())
	cmd.AddCommand(syncServerStopCmd())
	cmd.AddCommand(syncServerStatusCmd())

	return cmd
}

func syncServerStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start local sync server",
		RunE: func(cmd *cobra.Command, args []string) error {
			launcher := acontext.GetSyncServerLauncher()

			if !launcher.IsInstalled() {
				return fmt.Errorf("Sync Server not installed. Run: cortex sync install")
			}

			fmt.Println("Starting Sync Server...")

			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			if err := launcher.Start(ctx); err != nil {
				return err
			}

			fmt.Println("✅ Sync Server started successfully")
			fmt.Printf("Endpoint: %s\n", launcher.Endpoint())
			return nil
		},
	}
}

func syncServerStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop local sync server",
		RunE: func(cmd *cobra.Command, args []string) error {
			launcher := acontext.GetSyncServerLauncher()

			if err := launcher.Stop(); err != nil {
				return err
			}

			fmt.Println("✅ Sync Server stopped")
			return nil
		},
	}
}

func syncServerStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check local sync server status",
		RunE: func(cmd *cobra.Command, args []string) error {
			launcher := acontext.GetSyncServerLauncher()
			config := launcher.Config()

			fmt.Println("Sync Server Status")
			fmt.Println("==================")
			fmt.Printf("Install Dir: %s\n", config.InstallDir)

			if launcher.IsInstalled() {
				fmt.Println("Installed:   Yes")
			} else {
				fmt.Println("Installed:   No")
				fmt.Println("\nRun 'cortex sync install' to install Sync Server")
				return nil
			}

			if launcher.IsHealthy() {
				fmt.Println("Status:      Running")
				fmt.Printf("Endpoint:    %s\n", launcher.Endpoint())

				// Get detailed health info
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()

				if health, err := launcher.GetHealth(ctx); err == nil {
					fmt.Printf("Service:     %s\n", health.Service)
					fmt.Printf("Version:     %s\n", health.Version)
				}

				// Check database
				dbPath := filepath.Join(config.InstallDir, "data", "sync.db")
				if info, err := os.Stat(dbPath); err == nil {
					fmt.Printf("Database:    %s (%.1f KB)\n", dbPath, float64(info.Size())/1024)
				}
			} else {
				fmt.Println("Status:      Not running")
				fmt.Println("\nRun 'cortex sync server start' to start Sync Server")
			}

			return nil
		},
	}
}

func syncExportCmd() *cobra.Command {
	var outputFile string
	var teamID string

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export knowledge to JSON file",
		Long: `Export all knowledge from the local sync server to a JSON file.

This requires the local sync server to be running.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			launcher := acontext.GetSyncServerLauncher()

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			if err := launcher.EnsureRunning(ctx); err != nil {
				return fmt.Errorf("Sync Server not running: %w", err)
			}

			// Fetch export from server
			url := fmt.Sprintf("%s/v1/export?team_id=%s", launcher.Endpoint(), teamID)
			resp, err := http.Get(url)
			if err != nil {
				return fmt.Errorf("failed to export: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("export failed: %s", string(body))
			}

			// Determine output file
			if outputFile == "" {
				outputFile = fmt.Sprintf("cortex-knowledge-%s.json", time.Now().Format("2006-01-02"))
			}

			// Write to file
			out, err := os.Create(outputFile)
			if err != nil {
				return fmt.Errorf("failed to create output file: %w", err)
			}
			defer out.Close()

			n, err := io.Copy(out, resp.Body)
			if err != nil {
				return fmt.Errorf("failed to write output: %w", err)
			}

			fmt.Printf("✅ Exported to %s (%.1f KB)\n", outputFile, float64(n)/1024)
			return nil
		},
	}

	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file path (default: cortex-knowledge-YYYY-MM-DD.json)")
	cmd.Flags().StringVar(&teamID, "team", "local", "Team ID to export")

	return cmd
}

func syncImportCmd() *cobra.Command {
	var teamID string

	cmd := &cobra.Command{
		Use:   "import [file]",
		Short: "Import knowledge from JSON file",
		Long: `Import knowledge from a JSON file into the local sync server.

This requires the local sync server to be running.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			inputFile := args[0]

			launcher := acontext.GetSyncServerLauncher()

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			if err := launcher.EnsureRunning(ctx); err != nil {
				return fmt.Errorf("Sync Server not running: %w", err)
			}

			// Read input file
			data, err := os.ReadFile(inputFile)
			if err != nil {
				return fmt.Errorf("failed to read input file: %w", err)
			}

			// Parse to extract items
			var exportData struct {
				Items []json.RawMessage `json:"items"`
			}
			if err := json.Unmarshal(data, &exportData); err != nil {
				return fmt.Errorf("invalid JSON format: %w", err)
			}

			// Prepare import request
			importReq := map[string]interface{}{
				"items":   exportData.Items,
				"team_id": teamID,
			}
			importData, _ := json.Marshal(importReq)

			// Send to server
			url := fmt.Sprintf("%s/v1/import", launcher.Endpoint())
			resp, err := http.Post(url, "application/json", bytes.NewReader(importData))
			if err != nil {
				return fmt.Errorf("failed to import: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("import failed: %s", string(body))
			}

			var result struct {
				Imported  int      `json:"imported"`
				Rejected  int      `json:"rejected"`
				Conflicts []string `json:"conflicts"`
			}
			json.NewDecoder(resp.Body).Decode(&result)

			fmt.Printf("✅ Imported %d items from %s\n", result.Imported, inputFile)
			if result.Rejected > 0 {
				fmt.Printf("   Rejected: %d\n", result.Rejected)
			}
			if len(result.Conflicts) > 0 {
				fmt.Printf("   Conflicts: %d\n", len(result.Conflicts))
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&teamID, "team", "local", "Team ID to import into")

	return cmd
}

func runSyncNow() error {
	log.Info("Forcing immediate sync...")

	// Load config
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if !cfg.Sync.Enabled {
		log.Warn("Sync is disabled in config. Enable it first with: cortex sync enable")
		return fmt.Errorf("sync disabled in config")
	}

	// Initialize store and components
	store, _, cleanup, err := initializeStore()
	if err != nil {
		return err
	}
	defer cleanup()

	// Create acontext components
	acontextCfg := acontext.Config{
		Endpoint:     cfg.Sync.Endpoint,
		Token:        cfg.Sync.AuthToken,
		TeamID:       cfg.Sync.TeamID,
		SyncInterval: cfg.Sync.Interval,
		Enabled:      cfg.Sync.Enabled,
	}

	client := acontext.NewClient(acontextCfg)
	localStore := acontext.NewStoreAdapter(store)
	resolver := acontext.NewTrustWeightedResolver()
	syncer := acontext.NewSyncService(client, localStore, resolver, acontextCfg)

	// Perform sync
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	fmt.Println("Syncing knowledge with Acontext...")
	result, err := syncer.Sync(ctx)
	if err != nil {
		log.Error("Sync failed: %v", err)
		return fmt.Errorf("sync failed: %w", err)
	}

	// Print results
	fmt.Printf("✅ Sync completed in %v\n", result.Duration)
	fmt.Printf("   Pulled: %d items\n", result.ItemsPulled)
	fmt.Printf("   Pushed: %d items\n", result.ItemsPushed)
	if result.Conflicts > 0 {
		fmt.Printf("   Conflicts: %d (resolved: %d)\n", result.Conflicts, result.Resolved)
	}
	if len(result.ManualReview) > 0 {
		fmt.Printf("   ⚠️  Manual review needed: %d items\n", len(result.ManualReview))
		for _, id := range result.ManualReview {
			fmt.Printf("      - %s\n", id)
		}
	}

	return nil
}

func runSyncStatus() error {
	// Load config
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	fmt.Println("Acontext Sync Status")
	fmt.Println("────────────────────")
	fmt.Printf("Enabled:       %t\n", cfg.Sync.Enabled)
	fmt.Printf("Endpoint:      %s\n", cfg.Sync.Endpoint)
	fmt.Printf("Sync Interval: %v\n", cfg.Sync.Interval)

	if !cfg.Sync.Enabled {
		fmt.Println("\nSync is disabled. Enable with: cortex sync enable")
		return nil
	}

	// Check if token is configured
	if cfg.Sync.AuthToken == "" {
		fmt.Println("Status:        ⚠️  No auth token configured")
		fmt.Println("\nConfigure token in ~/.cortex/config.yaml:")
		fmt.Println("  sync:")
		fmt.Println("    auth_token: your-token-here")
		return nil
	}

	// Initialize store to check sync cursor
	store, _, cleanup, err := initializeStore()
	if err != nil {
		return err
	}
	defer cleanup()

	localStore := acontext.NewStoreAdapter(store)
	cursor, err := localStore.GetCursor(context.Background())
	if err != nil {
		log.Warn("Failed to get sync cursor: %v", err)
	}

	// Get pending items
	pending, err := localStore.GetPendingPush(context.Background())
	if err != nil {
		log.Warn("Failed to get pending items: %v", err)
	}

	if cursor != nil {
		fmt.Printf("Last Pull:     %s\n", formatTime(cursor.LastPullAt))
		fmt.Printf("Last Push:     %s\n", formatTime(cursor.LastPushAt))
	} else {
		fmt.Println("Last Sync:     Never")
	}

	fmt.Printf("Pending Push:  %d items\n", len(pending))

	return nil
}

func runSyncToggle(enable bool) error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	cfg.Sync.Enabled = enable

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	if enable {
		fmt.Println("✅ Sync enabled")
		fmt.Println("Background sync will start automatically when using the TUI")
	} else {
		fmt.Println("✅ Sync disabled")
	}

	return nil
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return "Never"
	}
	return t.Format("2006-01-02 15:04:05")
}

// ═══════════════════════════════════════════════════════════════════════════════
// VOICE COMMANDS (CR-012: Native Voice Box)
// ═══════════════════════════════════════════════════════════════════════════════

func voiceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "voice",
		Short: "Manage Voice Box (native TTS engine)",
		Long: `Manage the Kokoro TTS sidecar for voice synthesis.

Voice Box is a native Python TTS server that runs locally without Docker.
It provides fast, high-quality text-to-speech synthesis.

Commands:
  install  - Install Voice Box to ~/.cortex/voicebox/
  start    - Start the Voice Box server
  stop     - Stop the Voice Box server
  status   - Check Voice Box status
  test     - Test TTS synthesis with sample text`,
	}

	cmd.AddCommand(voiceInstallCmd())
	cmd.AddCommand(voiceStartCmd())
	cmd.AddCommand(voiceStopCmd())
	cmd.AddCommand(voiceStatusCmd())
	cmd.AddCommand(voiceTestCmd())
	cmd.AddCommand(voiceTranscribeCmd())
	cmd.AddCommand(voiceEnhanceCmd())
	cmd.AddCommand(voiceCacheCmd()) // CR-012-C: Audio cache management

	return cmd
}

// ═══════════════════════════════════════════════════════════════════════════════
// VOICE CACHE COMMANDS (CR-012-C: Conversational State Machine)
// ═══════════════════════════════════════════════════════════════════════════════

func voiceCacheCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Manage pre-generated audio cache (CR-012-C)",
		Long: `Manage the pre-generated audio cache for instant wake responses.

The audio cache stores pre-synthesized wake word responses, backchannels,
and other conversational audio for sub-200ms latency playback.

Commands:
  generate - Generate all cached audio files
  stats    - Show cache statistics
  clear    - Clear the audio cache`,
	}

	cmd.AddCommand(voiceCacheGenerateCmd())
	cmd.AddCommand(voiceCacheStatsCmd())
	cmd.AddCommand(voiceCacheClearCmd())

	return cmd
}

func voiceCacheGenerateCmd() *cobra.Command {
	var voiceID string
	var force bool

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate pre-cached wake responses",
		Long: `Generate pre-cached audio files for instant wake word responses.

This will synthesize all conversational audio (wake responses, backchannels,
farewells, etc.) using the configured TTS voice and store them in the cache.

Cache location: ~/.cortex/voicebox/audio_cache/

If the cache is already valid (matching voice config), generation is skipped
unless --force is specified.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Generating pre-cached wake responses (voice: %s)...\n", voiceID)

			// Get cache directory
			home, _ := os.UserHomeDir()
			cacheDir := filepath.Join(home, ".cortex", "voicebox", "audio_cache")

			// Create audio cache config
			cacheConfig := pkgvoice.AudioCacheConfig{
				VoiceID:    voiceID,
				Model:      "kokoro",
				Speed:      1.0,
				SampleRate: 24000,
			}

			cache := pkgvoice.NewAudioCache(cacheDir, cacheConfig)

			// Check if regeneration needed
			if !force && !cache.NeedsRegeneration() {
				fmt.Println("Cache is up to date. Use --force to regenerate.")
				return nil
			}

			// Ensure Voice Box is running for TTS
			launcher := voice.GetVoiceBoxLauncher()
			if !launcher.IsInstalled() {
				return fmt.Errorf("Voice Box not installed. Run: cortex voice install")
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()

			if err := launcher.EnsureRunning(ctx); err != nil {
				return fmt.Errorf("failed to start Voice Box: %w", err)
			}

			// Create TTS generator adapter
			generator := &ttsGeneratorAdapter{
				endpoint: launcher.SpeechEndpoint(),
				voiceID:  voiceID,
			}
			if err := cache.EnsureGenerated(ctx, generator); err != nil {
				return fmt.Errorf("cache generation failed: %w", err)
			}

			// Show stats
			manifest := cache.GetManifest()
			if manifest != nil {
				fmt.Printf("\n✅ Cache generation complete!\n")
				fmt.Printf("   Voice: %s\n", manifest.VoiceID)
				fmt.Printf("   Files: %d\n", manifest.FileCount)
				fmt.Printf("   Size:  %.2f KB\n", float64(manifest.TotalSizeBytes)/1024)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&voiceID, "voice", "am_adam", "Voice ID for TTS")
	cmd.Flags().BoolVar(&force, "force", false, "Force regeneration even if cache is valid")

	return cmd
}

func voiceCacheStatsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stats",
		Short: "Show audio cache statistics",
		RunE: func(cmd *cobra.Command, args []string) error {
			home, _ := os.UserHomeDir()
			cacheDir := filepath.Join(home, ".cortex", "voicebox", "audio_cache")

			// Load cache
			cache := pkgvoice.NewAudioCache(cacheDir, pkgvoice.DefaultAudioCacheConfig())

			// Try to load manifest
			if err := cache.LoadManifest(); err != nil {
				fmt.Println("No cache manifest found.")
				fmt.Println("Run 'cortex voice cache generate' to create the cache.")
				return nil
			}

			manifest := cache.GetManifest()
			if manifest == nil {
				fmt.Println("No cache manifest found.")
				return nil
			}

			// Preload to get memory stats
			_ = cache.PreloadAll()

			fmt.Println("Audio Cache Statistics")
			fmt.Println("======================")
			fmt.Printf("Location:     %s\n", cacheDir)
			fmt.Printf("Voice ID:     %s\n", manifest.VoiceID)
			fmt.Printf("Model:        %s\n", manifest.Model)
			fmt.Printf("Sample Rate:  %d Hz\n", manifest.SampleRate)
			fmt.Printf("Config Hash:  %s\n", manifest.VoiceConfigHash[:16]+"...")
			fmt.Printf("Generated:    %s\n", manifest.GeneratedAt.Format("2006-01-02 15:04:05"))
			fmt.Printf("\nFile Count:   %d\n", manifest.FileCount)
			fmt.Printf("Disk Size:    %.2f KB\n", float64(manifest.TotalSizeBytes)/1024)
			fmt.Printf("Memory Size:  %.2f KB\n", float64(cache.CacheSize())/1024)

			// Check if regeneration needed
			if cache.NeedsRegeneration() {
				fmt.Println("\n⚠️  Cache needs regeneration (voice config changed)")
			} else {
				fmt.Println("\n✓ Cache is up to date")
			}

			// Show categories
			pool := pkgvoice.DefaultWakeResponsePool()
			fmt.Printf("\nCategories:\n")
			fmt.Printf("  Wake (Cold):    %d responses\n", len(pool.Cold))
			fmt.Printf("  Wake (Warm):    %d responses\n", len(pool.Warm))
			fmt.Printf("  Wake (Active):  %d responses\n", len(pool.Active))
			fmt.Printf("  Confused:       %d responses\n", len(pool.Confused))
			fmt.Printf("  Backchannel:    %d responses\n", len(pool.Backchannel))
			fmt.Printf("  Farewell:       %d responses\n", len(pool.Farewell))
			fmt.Printf("  Acknowledge:    %d responses\n", len(pool.Acknowledge))

			return nil
		},
	}
}

func voiceCacheClearCmd() *cobra.Command {
	var confirm bool

	cmd := &cobra.Command{
		Use:   "clear",
		Short: "Clear the audio cache",
		RunE: func(cmd *cobra.Command, args []string) error {
			home, _ := os.UserHomeDir()
			cacheDir := filepath.Join(home, ".cortex", "voicebox", "audio_cache")

			// Check if cache exists
			if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
				fmt.Println("No cache to clear.")
				return nil
			}

			if !confirm {
				fmt.Println("This will delete all cached audio files.")
				fmt.Println("Use --confirm to proceed.")
				return nil
			}

			cache := pkgvoice.NewAudioCache(cacheDir, pkgvoice.DefaultAudioCacheConfig())
			if err := cache.Clear(); err != nil {
				return fmt.Errorf("failed to clear cache: %w", err)
			}

			fmt.Println("✅ Audio cache cleared.")
			return nil
		},
	}

	cmd.Flags().BoolVar(&confirm, "confirm", false, "Confirm cache deletion")

	return cmd
}

// ttsGeneratorAdapter adapts HTTP-based TTS to the TTSGenerator interface
type ttsGeneratorAdapter struct {
	endpoint string
	voiceID  string
}

func (a *ttsGeneratorAdapter) SynthesizeToFile(ctx context.Context, text, outputPath, voiceID string) error {
	if voiceID == "" {
		voiceID = a.voiceID
	}

	// Build TTS request
	reqBody := map[string]interface{}{
		"model":           "kokoro",
		"input":           text,
		"voice":           voiceID,
		"response_format": "wav",
		"speed":           1.0,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Make HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", a.endpoint, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("TTS request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("TTS failed (%d): %s", resp.StatusCode, string(body))
	}

	// Save to file
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	if _, err := io.Copy(outFile, resp.Body); err != nil {
		return fmt.Errorf("failed to write audio: %w", err)
	}

	return nil
}

func voiceInstallCmd() *cobra.Command {
	var noPrewarm bool
	var upgrade bool

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install Voice Box",
		Long: `Install Kokoro TTS engine to ~/.cortex/voicebox/

This will:
  1. Create a Python virtual environment
  2. Install kokoro, fastapi, uvicorn, and dependencies
  3. Download the Kokoro-82M model (~80MB)
  4. Create management scripts (start.sh, stop.sh, status.sh)

Requirements:
  - Python 3.10 or higher
  - pip`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Installing Voice Box...")

			// Build install script path
			// First try to find it relative to the executable
			scriptPath := ""

			// Check common locations
			locations := []string{
				"scripts/install-voicebox.sh",
				"./scripts/install-voicebox.sh",
			}

			for _, loc := range locations {
				if _, err := os.Stat(loc); err == nil {
					scriptPath = loc
					break
				}
			}

			// If not found, try to use embedded install logic
			if scriptPath == "" {
				return runEmbeddedInstall(noPrewarm, upgrade)
			}

			// Build arguments
			installArgs := []string{scriptPath}
			if noPrewarm {
				installArgs = append(installArgs, "--no-prewarm")
			}
			if upgrade {
				installArgs = append(installArgs, "--upgrade")
			}

			c := exec.Command("bash", installArgs...)
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr

			return c.Run()
		},
	}

	cmd.Flags().BoolVar(&noPrewarm, "no-prewarm", false, "Skip model pre-download")
	cmd.Flags().BoolVar(&upgrade, "upgrade", false, "Upgrade existing installation")

	return cmd
}

func runEmbeddedInstall(noPrewarm, upgrade bool) error {
	launcher := voice.GetVoiceBoxLauncher()
	config := launcher.Config()

	fmt.Printf("Installing to: %s\n", config.InstallDir)

	// Check Python
	pythonCmd := "python3"
	if _, err := exec.LookPath(pythonCmd); err != nil {
		pythonCmd = "python"
		if _, err := exec.LookPath(pythonCmd); err != nil {
			return fmt.Errorf("Python not found. Please install Python 3.10 or higher")
		}
	}

	// Check Python version
	versionOut, err := exec.Command(pythonCmd, "-c", "import sys; print(f'{sys.version_info.major}.{sys.version_info.minor}')").Output()
	if err != nil {
		return fmt.Errorf("failed to check Python version: %w", err)
	}
	version := strings.TrimSpace(string(versionOut))
	fmt.Printf("Found Python %s\n", version)

	// Create virtual environment
	if !upgrade || !launcher.IsInstalled() {
		fmt.Println("Creating virtual environment...")
		if err := os.MkdirAll(config.InstallDir, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}

		c := exec.Command(pythonCmd, "-m", "venv", config.InstallDir)
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		if err := c.Run(); err != nil {
			return fmt.Errorf("failed to create venv: %w", err)
		}
	}

	// Install dependencies
	fmt.Println("Installing dependencies (this may take a few minutes)...")
	pipPath := filepath.Join(config.InstallDir, "bin", "pip")
	c := exec.Command(pipPath, "install", "--upgrade", "pip")
	c.Run() // Ignore errors for pip upgrade

	c = exec.Command(pipPath, "install", "kokoro>=0.9.2", "soundfile", "fastapi", "uvicorn[standard]", "numpy")
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("failed to install dependencies: %w", err)
	}

	fmt.Println("\n✅ Voice Box installed successfully!")
	fmt.Printf("Directory: %s\n", config.InstallDir)
	fmt.Println("\nNote: The server script and management scripts will be created")
	fmt.Println("when you first run 'cortex voice start'")

	return nil
}

func voiceStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start Voice Box server",
		RunE: func(cmd *cobra.Command, args []string) error {
			launcher := voice.GetVoiceBoxLauncher()

			if !launcher.IsInstalled() {
				return fmt.Errorf("Voice Box not installed. Run: cortex voice install")
			}

			fmt.Println("Starting Voice Box...")

			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			if err := launcher.Start(ctx); err != nil {
				return err
			}

			fmt.Println("✅ Voice Box started successfully")
			fmt.Printf("Endpoint: %s\n", launcher.Endpoint())
			return nil
		},
	}
}

func voiceStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop Voice Box server",
		RunE: func(cmd *cobra.Command, args []string) error {
			launcher := voice.GetVoiceBoxLauncher()

			if err := launcher.Stop(); err != nil {
				return err
			}

			fmt.Println("✅ Voice Box stopped")
			return nil
		},
	}
}

func voiceStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check Voice Box status",
		RunE: func(cmd *cobra.Command, args []string) error {
			launcher := voice.GetVoiceBoxLauncher()
			config := launcher.Config()

			fmt.Println("Voice Box Status")
			fmt.Println("================")
			fmt.Printf("Install Dir: %s\n", config.InstallDir)

			if launcher.IsInstalled() {
				fmt.Println("Installed:   Yes")
			} else {
				fmt.Println("Installed:   No")
				fmt.Println("\nRun 'cortex voice install' to install Voice Box")
				return nil
			}

			if launcher.IsHealthy() {
				fmt.Println("Status:      Running")
				fmt.Printf("Endpoint:    %s\n", launcher.Endpoint())

				// Get detailed health info
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()

				if health, err := launcher.GetHealth(ctx); err == nil {
					fmt.Printf("Model:       %s\n", health.Model)
					fmt.Printf("Version:     %s\n", health.Version)
				}

				// List voices
				if voices, err := launcher.GetVoices(ctx); err == nil && len(voices) > 0 {
					fmt.Println("\nAvailable Voices:")
					for _, v := range voices {
						fmt.Printf("  %s - %s (%s, %s)\n", v.ID, v.Name, v.Gender, v.Accent)
					}
				}
			} else {
				fmt.Println("Status:      Not running")
				fmt.Println("\nRun 'cortex voice start' to start Voice Box")
			}

			return nil
		},
	}
}

func voiceTestCmd() *cobra.Command {
	var voiceID string
	var speed float64

	cmd := &cobra.Command{
		Use:   "test [text]",
		Short: "Test Voice Box with sample text",
		Long: `Test TTS synthesis with sample text.

If no text is provided, a default greeting will be used.
Audio will be played through the default audio player (mpv, afplay, or aplay).`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			text := "Hello, I am Cortex, your AI assistant. Voice Box is working correctly."
			if len(args) > 0 {
				text = args[0]
			}

			launcher := voice.GetVoiceBoxLauncher()

			if !launcher.IsInstalled() {
				return fmt.Errorf("Voice Box not installed. Run: cortex voice install")
			}

			fmt.Printf("Speaking: %q\n", text)
			fmt.Printf("Voice: %s (speed: %.1f)\n", voiceID, speed)

			// Ensure Voice Box is running
			// Use 2 minute timeout for first run (model download may take time)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()

			if err := launcher.EnsureRunning(ctx); err != nil {
				return fmt.Errorf("failed to start Voice Box: %w", err)
			}

			// Create TTS engine
			config := voice.DefaultTTSConfig()
			config.VoiceID = voiceID
			config.Speed = speed

			engine := voice.NewTTSEngine(config)
			defer engine.Stop()

			// Set up playback via external player
			engine.SetPlaybackFunc(func(audio []byte, format voice.AudioFormat) error {
				return playAudio(audio)
			})

			return engine.SpeakSync(ctx, text)
		},
	}

	cmd.Flags().StringVar(&voiceID, "voice", "am_adam", "Voice ID to use")
	cmd.Flags().Float64VarP(&speed, "speed", "s", 1.0, "Speech speed (0.5-2.0)")

	return cmd
}

func voiceTranscribeCmd() *cobra.Command {
	var language string

	cmd := &cobra.Command{
		Use:   "transcribe <audio-file>",
		Short: "Transcribe audio file to text (STT)",
		Long: `Transcribe audio file to text using Whisper STT.

Supports: wav, mp3, m4a, webm, ogg, flac
Uses MLX-Whisper on Apple Silicon (10x faster) or faster-whisper elsewhere.

Examples:
  cortex voice transcribe recording.wav
  cortex voice transcribe meeting.mp3 --language en`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			audioPath := args[0]

			// Check file exists
			if _, err := os.Stat(audioPath); err != nil {
				return fmt.Errorf("file not found: %s", audioPath)
			}

			// Get STT engine
			sttEngine, err := voice.GetSTTEngine()
			if err != nil {
				return fmt.Errorf("failed to create STT engine: %w", err)
			}

			// Check if Voice Box is installed
			launcher := voice.GetVoiceBoxLauncher()
			if !launcher.IsInstalled() {
				return fmt.Errorf("Voice Box not installed. Run: cortex voice install")
			}

			fmt.Printf("Transcribing: %s\n", audioPath)
			if language != "" {
				fmt.Printf("Language: %s\n", language)
			}
			fmt.Println()

			// Use longer timeout for transcription (models may need to load)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()

			// Perform transcription
			result, err := sttEngine.TranscribeFile(ctx, audioPath)
			if err != nil {
				return fmt.Errorf("transcription failed: %w", err)
			}

			// Display result
			fmt.Printf("Text: %s\n", result.Text)
			if result.Language != "" {
				fmt.Printf("Language: %s\n", result.Language)
			}
			if result.Backend != "" {
				fmt.Printf("Backend: %s\n", result.Backend)
			}
			fmt.Printf("Latency: %dms\n", result.Latency)

			return nil
		},
	}

	cmd.Flags().StringVarP(&language, "language", "l", "", "Language code (e.g., 'en', 'es', 'fr')")

	return cmd
}

// voiceEnhanceCmd creates the voice enhance command (CR-012-B)
func voiceEnhanceCmd() *cobra.Command {
	var mode string
	var output string

	cmd := &cobra.Command{
		Use:   "enhance <audio-file>",
		Short: "Enhance audio quality (denoise/restore)",
		Long: `Enhance audio quality using resemble-enhance (CR-012-B).

Modes:
  denoise - Fast background noise removal (~100ms/s)
  full    - Complete enhancement with fidelity restoration (~300ms/s)

The enhanced audio is saved to the same directory with '.enhanced.wav' suffix,
or to the specified output path.

Examples:
  cortex voice enhance noisy.wav                  # Denoise (default)
  cortex voice enhance noisy.wav --mode full      # Full enhancement
  cortex voice enhance noisy.wav -o clean.wav     # Specify output`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			audioPath := args[0]

			// Check file exists
			if _, err := os.Stat(audioPath); err != nil {
				return fmt.Errorf("file not found: %s", audioPath)
			}

			// Validate mode
			if mode != "denoise" && mode != "full" {
				return fmt.Errorf("invalid mode '%s'. Use 'denoise' or 'full'", mode)
			}

			// Check if Voice Box is installed
			launcher := voice.GetVoiceBoxLauncher()
			if !launcher.IsInstalled() {
				return fmt.Errorf("Voice Box not installed. Run: cortex voice install")
			}

			fmt.Printf("Enhancing: %s\n", audioPath)
			fmt.Printf("Mode: %s\n", mode)
			fmt.Println()

			// Use longer timeout for enhancement
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()

			// Ensure Voice Box is running
			if err := launcher.EnsureRunning(ctx); err != nil {
				return fmt.Errorf("failed to start Voice Box: %w", err)
			}

			// Make HTTP request to enhance endpoint
			file, err := os.Open(audioPath)
			if err != nil {
				return fmt.Errorf("failed to open file: %w", err)
			}
			defer file.Close()

			// Create multipart form
			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)

			part, err := writer.CreateFormFile("file", filepath.Base(audioPath))
			if err != nil {
				return fmt.Errorf("failed to create form file: %w", err)
			}
			if _, err := io.Copy(part, file); err != nil {
				return fmt.Errorf("failed to copy file data: %w", err)
			}

			if err := writer.WriteField("mode", mode); err != nil {
				return fmt.Errorf("failed to write mode field: %w", err)
			}

			if err := writer.Close(); err != nil {
				return fmt.Errorf("failed to close writer: %w", err)
			}

			// Create request
			endpoint := launcher.Endpoint() + "/v1/audio/enhance"
			req, err := http.NewRequestWithContext(ctx, "POST", endpoint, body)
			if err != nil {
				return fmt.Errorf("failed to create request: %w", err)
			}
			req.Header.Set("Content-Type", writer.FormDataContentType())

			// Send request
			client := &http.Client{Timeout: 2 * time.Minute}
			resp, err := client.Do(req)
			if err != nil {
				return fmt.Errorf("enhancement request failed: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				respBody, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("enhancement failed (%d): %s", resp.StatusCode, string(respBody))
			}

			// Determine output path
			outputPath := output
			if outputPath == "" {
				ext := filepath.Ext(audioPath)
				baseName := strings.TrimSuffix(audioPath, ext)
				outputPath = baseName + ".enhanced.wav"
			}

			// Save response to file
			outFile, err := os.Create(outputPath)
			if err != nil {
				return fmt.Errorf("failed to create output file: %w", err)
			}
			defer outFile.Close()

			if _, err := io.Copy(outFile, resp.Body); err != nil {
				return fmt.Errorf("failed to write output file: %w", err)
			}

			// Display result
			processingTime := resp.Header.Get("X-Processing-Time-Ms")
			originalDuration := resp.Header.Get("X-Original-Duration-Ms")

			fmt.Printf("Enhanced audio saved to: %s\n", outputPath)
			if processingTime != "" {
				fmt.Printf("Processing time: %sms\n", processingTime)
			}
			if originalDuration != "" {
				fmt.Printf("Original duration: %sms\n", originalDuration)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&mode, "mode", "m", "denoise", "Enhancement mode: denoise | full")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Output file path (default: <input>.enhanced.wav)")

	return cmd
}

// playAudio plays audio data using available system players
func playAudio(audio []byte) error {
	// Write to temp file first (most compatible)
	tmpFile, err := os.CreateTemp("", "cortex-tts-*.wav")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(audio); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write audio to temp file: %w", err)
	}
	tmpFile.Close()

	// Try available audio players
	players := []struct {
		cmd  string
		args []string
	}{
		{"mpv", []string{"--no-terminal", tmpFile.Name()}},
		{"afplay", []string{tmpFile.Name()}}, // macOS
		{"aplay", []string{tmpFile.Name()}},  // Linux ALSA
		{"paplay", []string{tmpFile.Name()}}, // PulseAudio
	}

	for _, player := range players {
		if _, err := exec.LookPath(player.cmd); err == nil {
			cmd := exec.Command(player.cmd, player.args...)
			return cmd.Run()
		}
	}

	return fmt.Errorf("no audio player found. Install mpv (recommended), afplay (macOS), or aplay (Linux)")
}

// ═══════════════════════════════════════════════════════════════════════════════
// INITIALIZATION HELPERS
// ═══════════════════════════════════════════════════════════════════════════════

// ModelSelection contains the auto-selected model info for the TUI.
type ModelSelection struct {
	Provider string
	Model    string
}

func initializeOrchestrator() (*orchestrator.Orchestrator, *bus.EventBus, *ModelSelection, llm.Provider, *memory.CoreMemoryStore, *sql.DB, func(), error) {
	if log != nil {
		defer log.Trace("initializeOrchestrator")()
	}

	loadEnvFile()

	// Cleanup orphaned processes from previous sessions (CR-024: process management)
	cortexDir := filepath.Join(os.Getenv("HOME"), ".cortex")
	processCleanup := agent.NewProcessCleanup(cortexDir)
	if killed, err := processCleanup.CleanupOrphanedProcesses(); err != nil {
		log.Warn("[Startup] Process cleanup failed: %v", err)
	} else if killed > 0 {
		log.Info("[Startup] Cleaned up %d orphaned process(es) from previous session", killed)
	}

	store, cfg, storeCleanup, err := initializeStore()
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, err
	}

	// Initialize Event Bus (CR-010)
	log.Debug("Initializing Event Bus...")
	eventBus := bus.New()
	log.Info("Event Bus initialized")

	// Initialize MemGPT-style memory system
	log.Debug("Initializing CoreMemoryStore...")
	memoryStore, err := memory.NewCoreMemoryStore(store.DB(), memory.DefaultCoreMemoryConfig())
	if err != nil {
		log.Warn("Failed to initialize memory store: %v", err)
		memoryStore = nil // Continue without memory store
	} else {
		log.Info("CoreMemoryStore initialized")
	}

	// Initialize PersonaStore for persona management
	log.Debug("Initializing PersonaStore...")
	personaStore := facets.NewPersonaStore(store.DB())
	if err := personaStore.InitBuiltIns(context.Background()); err != nil {
		log.Debug("Persona initialization: %v", err) // May fail if already seeded or no schema
	}
	log.Info("PersonaStore initialized")

	// Create components
	log.Debug("Creating FTS5 searcher...")
	searcher := knowledge.NewFTS5Searcher(store.DB())

	log.Debug("Creating trust-weighted merger...")
	merger := knowledge.NewTrustWeightedMerge()

	log.Debug("Creating knowledge fabric...")
	fabric := knowledge.NewFabric(store, searcher, merger).(*knowledge.Fabric)

	log.Debug("Creating smart router (fast/slow pattern)...")
	smartRouter := router.NewSmartRouter(nil) // No semantic classifier for now

	log.Debug("Creating secure tool executor...")
	toolExecutor := tools.NewExecutor() // Uses default options

	// ─────────────────────────────────────────────────────────────────────────────
	// DYNAMIC BACKEND DETECTION & AUTO-START
	// Priority: MLX (fastest on Apple Silicon) > Ollama > dnet
	// If no backend is running, automatically start the best available one
	// ─────────────────────────────────────────────────────────────────────────────
	mlxEndpoint := cfg.LLM.Providers["mlx"].Endpoint
	if mlxEndpoint == "" {
		mlxEndpoint = "http://127.0.0.1:8081" // Default mlx-lm port
	}
	ollamaEndpoint := cfg.LLM.Providers["ollama"].Endpoint
	if ollamaEndpoint == "" {
		ollamaEndpoint = "http://127.0.0.1:11434"
	}
	dnetEndpoint := cfg.LLM.Providers["dnet"].Endpoint
	if dnetEndpoint == "" {
		dnetEndpoint = "http://127.0.0.1:9080"
	}

	// Get default MLX model from config or use fallback
	mlxDefaultModel := cfg.LLM.Providers["mlx"].Model
	if mlxDefaultModel == "" {
		mlxDefaultModel = "mlx-community/Llama-3.2-3B-Instruct-4bit"
	}

	// Create backend launcher with auto-start capability
	backendLauncherConfig := autollm.BackendLauncherConfig{
		MLXEndpoint:     mlxEndpoint,
		OllamaEndpoint:  ollamaEndpoint,
		DnetEndpoint:    dnetEndpoint,
		MLXDefaultModel: mlxDefaultModel,
		StartupTimeout:  60 * time.Second,
		HealthTimeout:   3 * time.Second,
	}
	backendLauncher := autollm.NewBackendLauncher(backendLauncherConfig)

	// Try to ensure a backend is running (will auto-start if needed)
	log.Info("[Backend] Checking for available LLM backends...")
	backendInfo, backendErr := backendLauncher.EnsureBackendRunning(context.Background())

	// Also run standard detection for fallback selection
	backendDetector := autollm.NewBackendDetectorWithMLX(mlxEndpoint, ollamaEndpoint, dnetEndpoint)
	backendSelection := backendDetector.DetectBackends(context.Background())

	// Use auto-started backend if available, otherwise use detection result
	if backendInfo != nil && backendInfo.Available {
		detectedProvider := backendInfo.Type.ProviderName()
		log.Info("[Backend] Using: %s @ %s", backendInfo.Type, backendInfo.Endpoint)

		// Dynamically add provider config if not present
		if _, exists := cfg.LLM.Providers[detectedProvider]; !exists {
			if cfg.LLM.Providers == nil {
				cfg.LLM.Providers = make(map[string]config.ProviderConfig)
			}
			cfg.LLM.Providers[detectedProvider] = config.ProviderConfig{
				Endpoint: backendInfo.Endpoint,
			}
			log.Debug("[Backend] Added %s provider to config dynamically", detectedProvider)
		}

		// Override the config default provider
		cfg.LLM.DefaultProvider = detectedProvider
	} else if backendSelection.Primary != nil && backendSelection.Primary.Available {
		// Fallback to detection result
		detectedProvider := backendSelection.Primary.Type.ProviderName()
		log.Info("[Backend] Detected: %s (%s)", backendSelection.Primary, backendSelection.Reason)
		if backendSelection.Secondary != nil && backendSelection.Secondary.Available {
			log.Info("[Backend] Fallback: %s", backendSelection.Secondary)
		}

		if _, exists := cfg.LLM.Providers[detectedProvider]; !exists {
			if cfg.LLM.Providers == nil {
				cfg.LLM.Providers = make(map[string]config.ProviderConfig)
			}
			cfg.LLM.Providers[detectedProvider] = config.ProviderConfig{
				Endpoint: backendSelection.Primary.Endpoint,
			}
			log.Debug("[Backend] Added %s provider to config dynamically", detectedProvider)
		}

		cfg.LLM.DefaultProvider = detectedProvider
	} else {
		log.Warn("[Backend] No local backends available (mlx/ollama/dnet offline)")
		if backendErr != nil {
			log.Warn("[Backend] Auto-start failed: %v", backendErr)
		}
		log.Warn("[Backend] Install ollama, mlx-lm, or dnet for local inference")
	}

	// Create LLM provider with detected backend
	log.Debug("Creating LLM provider (detected: %s)...", cfg.LLM.DefaultProvider)
	log.Info("[LLM] Warming up model in background...")
	llmProvider, err := llm.NewProvider(cfg)
	var llmAdapter *llm.TypesAdapter
	var multiAdapter *llm.MultiProviderAdapter // Use MultiProviderAdapter for model routing

	// ─────────────────────────────────────────────────────────────────────────────
	// AUTO-SELECT BEST MODELS ON STARTUP
	// ─────────────────────────────────────────────────────────────────────────────
	log.Debug("Auto-selecting best models based on availability and scoring...")

	// Use ModelSelector to pick the best available model based on cognitive weighting
	modelSelector := autollm.NewModelSelectorWithMLX(mlxEndpoint, ollamaEndpoint, 120)
	modelSelection, _ := modelSelector.Select(context.Background())

	// Use the selected model if available, otherwise fall back to config
	var selectedLocalModel string
	if modelSelection != nil && modelSelection.LocalModel != "" {
		selectedLocalModel = modelSelection.LocalModel
		log.Info("Starting with selected model: %s (%s)", selectedLocalModel, modelSelection.LocalReason)
	} else {
		// Fall back to config model if selector finds nothing
		selectedLocalModel = cfg.LLM.Providers["ollama"].Model
		if selectedLocalModel == "" {
			log.Warn("No model selected - Ollama may not be available")
		} else {
			log.Info("Starting with config model: %s", selectedLocalModel)
		}
	}

	if modelSelection != nil && len(modelSelection.Candidates) > 0 {
		log.Debug("Available models for upgrade:")
		for i, c := range modelSelection.Candidates {
			if i >= 5 {
				break
			}
			log.Debug("  %s (score=%d)", c.Name, c.Score)
		}
	}

	// Store selection for use in fallback configuration
	var selectedFallbacks []autollm.FallbackSelection
	if modelSelection != nil {
		selectedFallbacks = modelSelection.Fallbacks
	}

	if err != nil {
		log.Warn("LLM provider not available: %v", err)
		log.Warn("Natural language queries will not work. Configure an API key to enable.")
	} else {
		if llmProvider.Available() {
			llmAdapter = llm.NewTypesAdapter(llmProvider)
			providerCfg := cfg.LLM.Providers[cfg.LLM.DefaultProvider]

			var model string
			if cfg.LLM.DefaultProvider == "ollama" || cfg.LLM.DefaultProvider == "mlx" {
				// MLX and Ollama both use auto-selected local models
				model = selectedLocalModel
				if model == "" {
					model = providerCfg.Model
				}
				if model == "" && cfg.LLM.DefaultProvider == "ollama" {
					model = autollm.GetSafeMinimalModel()
				}
			} else {
				model = providerCfg.Model
				if model == "" {
					model = llm.DefaultConfig(cfg.LLM.DefaultProvider).Model
				}
			}

			// Create MultiProviderAdapter for proper model routing
			// This ensures MLX models route to MLX provider, Ollama models to Ollama, etc.
			multiAdapter = llm.NewMultiProviderAdapter(cfg.LLM.DefaultProvider, model)

			// Register the primary detected provider
			multiAdapter.AddProvider(cfg.LLM.DefaultProvider, llmProvider)

			// Also register secondary local providers if available
			// This allows TUI model switching between MLX and Ollama models
			if backendSelection.Secondary != nil && backendSelection.Secondary.Available {
				secondaryName := backendSelection.Secondary.Type.ProviderName()
				if secondaryName != cfg.LLM.DefaultProvider {
					secondaryCfg := &llm.ProviderConfig{
						Name:     secondaryName,
						Endpoint: backendSelection.Secondary.Endpoint,
					}
					secondaryProvider, secErr := llm.NewProviderByName(secondaryName, secondaryCfg)
					if secErr == nil && secondaryProvider != nil && secondaryProvider.Available() {
						multiAdapter.AddProvider(secondaryName, secondaryProvider)
						log.Debug("[MultiProvider] Added secondary: %s @ %s", secondaryName, backendSelection.Secondary.Endpoint)
					}
				}
			}

			log.Info("LLM provider ready: %s/%s (agentic mode enabled)", llmProvider.Name(), model)
		} else {
			log.Warn("LLM provider '%s' configured but not available (missing API key?)", llmProvider.Name())
		}
	}

	// Create orchestrator with all components
	log.Debug("Wiring orchestrator with all components...")
	orchOpts := []orchestrator.Option{
		orchestrator.WithRouter(smartRouter),
		orchestrator.WithKnowledgeFabric(fabric),
		orchestrator.WithToolExecutor(toolExecutor),
		orchestrator.WithTaskManager(store.DB()), // CR-007: Task management with TaskWing algorithms
	}
	log.Debug("Enabled task management with 5 LLM tools (task_create, task_list, task_update, task_dependency, task_next)")

	// Add memory store if initialized successfully
	if memoryStore != nil {
		orchOpts = append(orchOpts, orchestrator.WithMemoryStore(memoryStore))
		log.Debug("Memory store wired into orchestrator")
	}

	// CRITICAL FIX: Add PassiveRetriever for automatic memory injection
	// This enables memory to be retrieved and injected into LLM context without bypassing
	if fabric != nil {
		passiveRetriever := memory.NewPassiveRetriever(fabric, memory.DefaultPassiveRetrievalConfig())
		orchOpts = append(orchOpts, orchestrator.WithPassiveRetriever(passiveRetriever))
		log.Info("PassiveRetriever initialized (automatic memory injection enabled)")
	}

	// Add persona store for persona management
	if personaStore != nil {
		orchOpts = append(orchOpts, orchestrator.WithFacetStore(personaStore))
		log.Debug("Persona store wired into orchestrator")
	}

	// Add LLM if available
	if llmAdapter != nil {
		orchOpts = append(orchOpts, orchestrator.WithLLMProvider(llmAdapter))
	}

	// Add agent LLM for agentic mode (using MultiProviderAdapter for proper routing)
	if multiAdapter != nil {
		orchOpts = append(orchOpts, orchestrator.WithAgentLLM(multiAdapter))
		log.Debug("Agentic mode enabled with tool use capabilities")

		// Configure primary endpoint based on default provider (mlx, ollama, or dnet)
		switch cfg.LLM.DefaultProvider {
		case "mlx":
			// MLX-LM: 5-10x faster than Ollama on Apple Silicon
			primaryEndpoint := cfg.LLM.Providers["mlx"].Endpoint
			if primaryEndpoint == "" {
				primaryEndpoint = "http://127.0.0.1:8081"
			}
			primaryModel := selectedLocalModel
			if primaryModel == "" {
				primaryModel = "mlx-community/Llama-3.2-3B-Instruct-4bit"
			}
			orchOpts = append(orchOpts, orchestrator.WithPrimaryEndpoint(primaryEndpoint, "mlx", primaryModel))
		case "ollama":
			primaryEndpoint := cfg.LLM.Providers["ollama"].Endpoint
			if primaryEndpoint == "" {
				primaryEndpoint = "http://127.0.0.1:11434"
			}
			primaryModel := selectedLocalModel
			if primaryModel == "" {
				primaryModel = autollm.GetSafeMinimalModel()
			}
			orchOpts = append(orchOpts, orchestrator.WithPrimaryEndpoint(primaryEndpoint, "ollama", primaryModel))
		case "dnet":
			primaryEndpoint := cfg.LLM.Providers["dnet"].Endpoint
			if primaryEndpoint == "" {
				primaryEndpoint = "http://127.0.0.1:9080"
			}
			// For dnet, use the first available model or a sensible default
			primaryModel := selectedLocalModel
			if primaryModel == "" {
				primaryModel = "mlx-community/Llama-3.2-3B-Instruct-4bit"
			}
			orchOpts = append(orchOpts, orchestrator.WithPrimaryEndpoint(primaryEndpoint, "dnet", primaryModel))
		}

		// Add fallback providers for timeout recovery (using auto-selected models)
		fallbackCount := 0

		// Helper to get auto-selected model for a provider
		getFallbackModel := func(provider, defaultModel string) string {
			for _, fb := range selectedFallbacks {
				if fb.Provider == provider {
					return fb.Model
				}
			}
			return defaultModel
		}

		// Helper to get API key from config.yaml OR environment variable
		// Priority: config.yaml api_key > environment variable
		getAPIKey := func(provider, envVar string) string {
			// First check config.yaml
			if provCfg, ok := cfg.LLM.Providers[provider]; ok && provCfg.APIKey != "" {
				return provCfg.APIKey
			}
			// Fall back to environment variable
			return os.Getenv(envVar)
		}

		// ═══════════════════════════════════════════════════════════════════════════════
		// CLOUD FALLBACK CHAIN: Grok → Anthropic → OpenAI
		// Priority order for timeout recovery and quality-based escalation
		// ═══════════════════════════════════════════════════════════════════════════════

		// Grok fallback (PRIMARY cloud - xAI, excellent reasoning)
		if apiKey := getAPIKey("grok", "XAI_API_KEY"); apiKey != "" {
			fallbackModel := getFallbackModel("grok", "grok-3")
			grokCfg := &llm.ProviderConfig{
				Name:   "grok",
				APIKey: apiKey,
				Model:  fallbackModel,
			}
			grokProvider := llm.NewGrokProvider(grokCfg)
			fallbackAdapter := llm.NewAgentAdapter(grokProvider, grokCfg.Model)
			orchOpts = append(orchOpts, orchestrator.WithFallbackLLM("grok", fallbackAdapter))
			fallbackCount++
			log.Debug("Added Grok fallback (priority 1): %s", fallbackModel)
		}

		// Anthropic fallback (SECONDARY - excellent tool use)
		if apiKey := getAPIKey("anthropic", "ANTHROPIC_API_KEY"); apiKey != "" {
			fallbackModel := getFallbackModel("anthropic", "claude-sonnet-4-20250514")
			anthropicCfg := &llm.ProviderConfig{
				Name:   "anthropic",
				APIKey: apiKey,
				Model:  fallbackModel,
			}
			anthropicProvider := llm.NewAnthropicProvider(anthropicCfg)
			fallbackAdapter := llm.NewAgentAdapter(anthropicProvider, anthropicCfg.Model)
			orchOpts = append(orchOpts, orchestrator.WithFallbackLLM("anthropic", fallbackAdapter))
			fallbackCount++
			log.Debug("Added Anthropic fallback (priority 2): %s", fallbackModel)
		}

		// OpenAI fallback (TERTIARY - strong all-around)
		if apiKey := getAPIKey("openai", "OPENAI_API_KEY"); apiKey != "" {
			fallbackModel := getFallbackModel("openai", "gpt-4o")
			openaiCfg := &llm.ProviderConfig{
				Name:   "openai",
				APIKey: apiKey,
				Model:  fallbackModel,
			}
			openaiProvider := llm.NewOpenAIProvider(openaiCfg)
			fallbackAdapter := llm.NewAgentAdapter(openaiProvider, openaiCfg.Model)
			orchOpts = append(orchOpts, orchestrator.WithFallbackLLM("openai", fallbackAdapter))
			fallbackCount++
			log.Debug("Added OpenAI fallback (priority 3): %s", fallbackModel)
		}

		// Gemini fallback (QUATERNARY - kept as optional extra)
		if apiKey := getAPIKey("gemini", "GEMINI_API_KEY"); apiKey != "" {
			fallbackModel := getFallbackModel("gemini", "gemini-2.0-flash")
			geminiCfg := &llm.ProviderConfig{
				Name:   "gemini",
				APIKey: apiKey,
				Model:  fallbackModel,
			}
			geminiProvider := llm.NewGeminiProvider(geminiCfg)
			fallbackAdapter := llm.NewAgentAdapter(geminiProvider, geminiCfg.Model)
			orchOpts = append(orchOpts, orchestrator.WithFallbackLLM("gemini", fallbackAdapter))
			fallbackCount++
			log.Debug("Added Gemini fallback (priority 4): %s", fallbackModel)
		}

		// Ollama fallback - required when default is cloud/MLX but user selects Ollama in TUI
		// CRITICAL: Use an Ollama-specific model, NOT the selectedLocalModel which could be MLX
		if cfg.LLM.DefaultProvider != "ollama" {
			ollamaEndpoint := cfg.LLM.Providers["ollama"].Endpoint
			if ollamaEndpoint == "" {
				ollamaEndpoint = "http://127.0.0.1:11434"
			}
			// Get the first available Ollama model, or use a sensible default
			ollamaModel := autollm.GetSafeMinimalModel() // "qwen3:4b" or similar
			if cfgModel := cfg.LLM.Providers["ollama"].Model; cfgModel != "" {
				ollamaModel = cfgModel // Prefer user-configured Ollama model
			}
			ollamaCfg := &llm.ProviderConfig{
				Name:     "ollama",
				Endpoint: ollamaEndpoint,
				Model:    ollamaModel,
			}
			ollamaProvider := llm.NewOllamaProvider(ollamaCfg)
			if ollamaProvider.Available() {
				fallbackAdapter := llm.NewAgentAdapter(ollamaProvider, ollamaCfg.Model)
				orchOpts = append(orchOpts, orchestrator.WithFallbackLLM("ollama", fallbackAdapter))
				fallbackCount++
				log.Debug("Added Ollama fallback: %s", ollamaModel)
			}
		}

		// Log configured vs missing providers (checks both config.yaml and .env)
		type providerStatus struct {
			name   string
			envVar string
		}
		allProviders := []providerStatus{
			{"grok", "XAI_API_KEY"},
			{"anthropic", "ANTHROPIC_API_KEY"},
			{"openai", "OPENAI_API_KEY"},
			{"gemini", "GEMINI_API_KEY"},
		}
		var configured, missing []string
		for _, p := range allProviders {
			if getAPIKey(p.name, p.envVar) != "" {
				configured = append(configured, p.name)
			} else {
				missing = append(missing, p.name)
			}
		}
		if len(configured) > 0 {
			log.Info("[Providers] Cloud fallbacks configured: %s", strings.Join(configured, ", "))
		}
		if len(missing) > 0 {
			log.Debug("[Providers] Not configured (add api_key to config.yaml or ~/.cortex/.env): %s", strings.Join(missing, ", "))
		}

		if fallbackCount > 0 {
			log.Info("Timeout recovery enabled with %d fallback provider(s)", fallbackCount)

			// Add learning callback to record timeout events
			orchOpts = append(orchOpts, orchestrator.WithTimeoutLearning(func(learning *agent.TimeoutLearning) {
				log.Info("[TimeoutLearning] Task: %s", truncateString(learning.Task, 50))
				log.Info("[TimeoutLearning] Primary model: %s, Complexity: %s", learning.PrimaryModel, learning.Complexity)
				log.Info("[TimeoutLearning] Timeout after: %v, Steps completed: %d", learning.TimeoutAfter, learning.StepsCompleted)
				log.Info("[TimeoutLearning] Action: %s, Fallback: %s, Success: %v",
					learning.RecoveryAction, learning.FallbackUsed, learning.FallbackSuccess)
				if learning.LearningNote != "" {
					log.Info("[TimeoutLearning] Note: %s", learning.LearningNote)
				}
			}))
		}
	}

	// ─────────────────────────────────────────────────────────────────────────────
	// CONVERSATION LOGGING & MODEL CAPABILITY ASSESSMENT
	// ─────────────────────────────────────────────────────────────────────────────
	log.Info("Initializing Conversation Logging & Model Assessment...")

	// Get data directory for JSON exports
	home, _ := os.UserHomeDir()
	dataDir := filepath.Join(home, ".cortex")

	// Create the SQLite logger using the existing store
	convLogger := eval.NewSQLiteLogger(store, dataDir)
	log.Debug("  Created SQLite conversation logger")

	// Create outcome logger for learned routing (Phase 2.5)
	outcomeLogger := eval.NewSQLiteOutcomeLogger(store.DB())
	log.Debug("  Created SQLite outcome logger for learned routing")

	// Create capability assessor with default thresholds
	assessor := eval.NewCapabilityAssessor()
	log.Debug("  Created capability assessor (timeout: 30s, repetition: 3x)")

	recommender := eval.NewModelRecommender()
	if modelSelection != nil && len(modelSelection.Candidates) > 0 {
		availableModels := make([]eval.ModelInfo, 0, len(modelSelection.Candidates))
		for _, c := range modelSelection.Candidates {
			if c.Available {
				availableModels = append(availableModels, eval.ModelInfo{
					Provider:  c.Provider,
					Name:      c.Name,
					Tier:      c.Tier,
					SizeBytes: int64(c.SizeGB * 1024 * 1024 * 1024),
				})
			}
		}
		recommender.SetAvailableModels(availableModels)
		log.Debug("  Model recommender initialized with %d available models", len(availableModels))
	} else {
		log.Debug("  Model recommender created (no available models to validate)")
	}

	// Add eval components to orchestrator
	orchOpts = append(orchOpts,
		orchestrator.WithConversationLogger(convLogger),
		orchestrator.WithCapabilityAssessor(assessor),
		orchestrator.WithModelRecommender(recommender),
		orchestrator.WithOutcomeLogger(outcomeLogger), // Phase 2.5: Learned routing outcomes
		orchestrator.EnableEval(),
		orchestrator.WithEventBus(eventBus), // CR-010: Event Bus integration
	)
	log.Info("Conversation Logging & Model Assessment initialized")
	log.Info("Outcome learning enabled for adaptive routing (Phase 2.5)")

	// ─────────────────────────────────────────────────────────────────────────────
	// COGNITIVE ARCHITECTURE INITIALIZATION
	// ─────────────────────────────────────────────────────────────────────────────
	var feedbackLoop *feedback.Loop
	cognitiveEnabled := cfg.Cognitive.Enabled

	if cognitiveEnabled {
		log.Info("Initializing Cognitive Architecture...")
		ctx := context.Background()

		// 1. Create cognitive registry using the existing store's database
		cogRegistry := cognitive.NewSQLiteRegistry(store.DB())
		log.Debug("  Created SQLite-backed template registry")

		// 2. Create embedder with fallback chain: Ollama -> OpenAI -> Null
		// This allows graceful degradation when local embedders aren't available
		var embedder cogRouter.Embedder

		// Try Ollama first (local, fast, free)
		ollamaEmbedder := cogRouter.NewOllamaEmbedder(&cogRouter.OllamaEmbedderConfig{
			Host:     cfg.Cognitive.OllamaURL,
			Model:    cfg.Cognitive.EmbeddingModel,
			AutoPull: true,
		})

		// OpenAI as fallback (cloud, requires API key)
		openaiEmbedder := cogRouter.NewOpenAIEmbedder(&cogRouter.OpenAIEmbedderConfig{
			// Uses OPENAI_API_KEY from environment
		})

		// Create multi-embedder with fallback chain
		embedder = cogRouter.NewMultiEmbedder(ollamaEmbedder, openaiEmbedder)

		if embedder.Available() {
			log.Info("[Embedder] Using: %s (dim=%d)", embedder.ModelName(), embedder.Dimension())
			// Warm up if using Ollama
			if ollamaEmbedder.Available() {
				log.Debug("[Embedder] Warming up Ollama embedding model in background...")
				ollamaEmbedder.WarmupAsync(context.Background())
			}
		} else {
			log.Warn("[Embedder] No embedding backend available - semantic features disabled")
		}

		// 3. Create semantic router
		semRouter := cogRouter.NewRouter(&cogRouter.RouterConfig{
			Embedder:      embedder,
			Registry:      cogRegistry,
			RefreshPeriod: 5 * time.Minute,
		})

		// Initialize router index
		if err := semRouter.Initialize(ctx); err != nil {
			log.Warn("  Failed to initialize router index: %v", err)
		} else {
			log.Debug("  Initialized semantic router with %d templates", semRouter.IndexSize())
		}

		// 4. Create template engine
		templateEngine := templates.NewEngine()
		log.Debug("  Created template rendering engine")

		// 5. Create distillation engine (for novel request learning)
		var distiller *distillation.Engine
		if llmProvider != nil && llmProvider.Available() {
			// Create LLM adapter for distillation
			distillLLM := &simpleChatLLMAdapter{provider: llmProvider, model: cfg.Cognitive.FrontierModel}
			distiller = distillation.NewEngine(&distillation.EngineConfig{
				LLM:           distillLLM,
				Registry:      cogRegistry,
				Embedder:      embedder,
				FrontierModel: cfg.Cognitive.FrontierModel,
				GraderModel:   cfg.Cognitive.FrontierModel,
			})
			log.Debug("  Created distillation engine with frontier model: %s", cfg.Cognitive.FrontierModel)
		} else {
			log.Warn("  Distillation disabled (no LLM provider available)")
		}

		// 6. Create complexity scorer and decomposer (nil LLM means no auto-decomposition)
		decomp := decomposer.NewDecomposer(nil)
		log.Debug("  Created task decomposer with complexity scoring")

		// 7. Create feedback loop (with distiller as grader if available)
		var grader feedback.Grader
		if distiller != nil {
			grader = distiller
		}
		feedbackLoop = feedback.NewLoop(cogRegistry, grader, &feedback.LoopConfig{
			GradingBatchSize:  10,
			PromotionInterval: 15 * time.Minute,
		})
		log.Debug("  Created feedback loop (promotion: 15m)")

		// Wire cognitive components to orchestrator
		orchOpts = append(orchOpts,
			orchestrator.WithCognitiveRouter(semRouter),
			orchestrator.WithCognitiveRegistry(cogRegistry),
			orchestrator.WithCognitiveTemplateEngine(templateEngine),
			orchestrator.WithCognitiveDecomposer(decomp),
			orchestrator.WithCognitiveFeedback(feedbackLoop),
			orchestrator.EnableCognitive(),
		)

		// Add distiller if available
		if distiller != nil {
			orchOpts = append(orchOpts, orchestrator.WithCognitiveDistiller(distiller))
		}

		// Wire CR-015 Enhanced Memory (depends on cognitive embedder)
		// Create stores first so they can be shared with CR-018 introspection
		var enhancedStores *orchestrator.EnhancedMemoryStores
		if embedder != nil && llmProvider != nil && llmProvider.Available() {
			memEmbedder := &memoryEmbedderAdapter{embedder: embedder}
			memLLM := &memoryLLMAdapter{provider: llmProvider, model: cfg.Cognitive.FrontierModel}
			enhancedMemConfig := orchestrator.DefaultEnhancedMemoryConfig()

			// Create stores independently so we can share them with introspection
			enhancedStores = orchestrator.CreateEnhancedMemoryStores(
				store.DB(),
				memEmbedder,
				memLLM,
				enhancedMemConfig,
			)

			if enhancedStores != nil {
				orchOpts = append(orchOpts, orchestrator.WithEnhancedMemoryStores(enhancedStores, enhancedMemConfig))
				log.Info("Enhanced Memory (CR-015) wired: Strategic, Topics, Links, Orientation")
			}
		}

		// CR-025: SkillLibrary and NextScenePredictor for evolutionary reasoning
		if store.DB() != nil && embedder != nil {
			memEmbedderForCR025 := &memoryEmbedderAdapter{embedder: embedder}

			// Create SkillLibrary for learning from successful executions
			skillLibrary := memory.NewSkillLibrary(store.DB(), memEmbedderForCR025)
			orchOpts = append(orchOpts, orchestrator.WithSkillLibrary(skillLibrary))
			log.Info("[CR-025] SkillLibrary initialized for execution pattern learning")

			// Create MemCubeStore and NextScenePredictor for predictive memory loading
			vectorIndex := memory.NewVectorIndex(store.DB())
			memCubeStore := memory.NewMemCubeStore(store.DB(), memEmbedderForCR025, vectorIndex)
			nextScenePredictor := memory.NewNextScenePredictor(memCubeStore)
			orchOpts = append(orchOpts, orchestrator.WithNextScenePredictor(nextScenePredictor))
			log.Info("[CR-025] NextScenePredictor initialized for predictive memory loading")
		}

		// CR-018: Introspection Coordinator (Metacognitive Self-Awareness)
		if llmProvider != nil && llmProvider.Available() {
			log.Info("Initializing Introspection Coordinator (CR-018)...")

			introLLM := &introspectionLLMAdapter{provider: llmProvider, model: cfg.Cognitive.FrontierModel}
			var introEmbedder introspection.Embedder
			if embedder != nil {
				introEmbedder = &introspectionEmbedderAdapter{embedder: embedder}
			}

			classifier := introspection.NewClassifier(introLLM, introEmbedder)
			analyzer := introspection.NewGapAnalyzer(introLLM)
			responder := introspection.NewMetacognitiveResponder()

			// Create web search tool early for introspection acquisition
			var webSearchTool *tools.WebSearchTool
			var introWebSearcher introspection.WebSearcher
			if tavilyKey := os.Getenv("TAVILY_API_KEY"); tavilyKey != "" {
				webSearchTool = tools.NewWebSearchTool(tools.WithAPIKey(tavilyKey))
				introWebSearcher = introspection.NewWebSearchAdapter(webSearchTool)
				orchOpts = append(orchOpts, orchestrator.WithWebSearchTool(webSearchTool))
			}

			// Create acquisition engine if web search is available
			// CR-018: Wire TopicStore from enhanced memory stores
			var acquisitionEngine *introspection.AcquisitionEngine
			if introWebSearcher != nil {
				introEventBus := &introspectionEventAdapter{eventBus: eventBus}

				// Wire TopicStore adapter if enhanced stores are available
				var topicStoreAdapter introspection.TopicStoreCreator
				if enhancedStores != nil && enhancedStores.Topics != nil {
					topicStoreAdapter = &topicStoreCreatorAdapter{store: enhancedStores.Topics}
				}

				acquisitionEngine = introspection.NewAcquisitionEngine(
					nil, // KnowledgeFabric - TODO: wire when available
					topicStoreAdapter,
					introWebSearcher,
					introEventBus,
					llmProvider,
				)
			}

			// CR-018: Create KnowledgeInventory if enhanced stores are available
			var knowledgeInventory *memory.KnowledgeInventory
			if enhancedStores != nil {
				knowledgeInventory = memory.NewKnowledgeInventory(
					nil, // KnowledgeSearcher - TODO: wire knowledge fabric when available
					enhancedStores.Strategic,
					enhancedStores.Topics,
					memoryStore, // CoreMemoryStore created earlier in initializeOrchestrator
					nil,         // ArchivalSearcher - TODO: wire archival memory when available
					nil,         // Embedder already used in stores
				)
				log.Debug("CR-018: KnowledgeInventory created with Strategic, Topics, CoreMemory")
			}

			introspectionCoord := orchestrator.NewIntrospectionCoordinator(&orchestrator.IntrospectionCoordinatorConfig{
				Classifier:  classifier,
				Inventory:   knowledgeInventory,
				Analyzer:    analyzer,
				Responder:   responder,
				Acquisition: acquisitionEngine,
				Learning:    nil, // TODO: wire LearningConfirmation when ready
				EventBus:    eventBus,
				Enabled:     true,
			})

			orchOpts = append(orchOpts, orchestrator.WithIntrospectionCoordinator(introspectionCoord))
			if acquisitionEngine != nil && knowledgeInventory != nil {
				log.Info("Introspection Coordinator (CR-018) initialized: Classifier, GapAnalyzer, Responder, Acquisition, Inventory")
			} else if acquisitionEngine != nil {
				log.Info("Introspection Coordinator (CR-018) initialized: Classifier, GapAnalyzer, Responder, Acquisition")
			} else {
				log.Info("Introspection Coordinator (CR-018) initialized: Classifier, GapAnalyzer, Responder")
			}
		}

		log.Info("Cognitive Architecture initialized successfully")
	} else {
		log.Debug("Cognitive Architecture disabled in config")
	}

	// ═══════════════════════════════════════════════════════════════════════════════
	// CR-020: SLEEP CYCLE SELF-IMPROVEMENT
	// ═══════════════════════════════════════════════════════════════════════════════
	if cfg.Sleep.Enabled {
		// Determine personality path
		personalityPath := cfg.Sleep.PersonalityPath
		if personalityPath == "" {
			personalityPath = filepath.Join(filepath.Dir(cfg.Knowledge.DBPath), "personality")
		}

		// Map config mode string to sleep.ImprovementMode
		var sleepMode sleep.ImprovementMode
		switch cfg.Sleep.Mode {
		case "auto":
			sleepMode = sleep.ImprovementAuto
		case "off":
			sleepMode = sleep.ImprovementOff
		default:
			sleepMode = sleep.ImprovementSupervised
		}

		// Calculate idle timeout
		idleTimeout := time.Duration(cfg.Sleep.IdleTimeoutMin) * time.Minute
		if idleTimeout == 0 {
			idleTimeout = 30 * time.Minute
		}

		// Get min interactions
		minInteractions := cfg.Sleep.MinInteractions
		if minInteractions == 0 {
			minInteractions = 10
		}

		sleepCoord := orchestrator.NewSleepCoordinator(&orchestrator.SleepCoordinatorConfig{
			PersonalityPath: personalityPath,
			MemoryStore:     nil, // TODO: Create adapter for CoreMemoryStore -> sleep.MemoryStore
			EventBus:        eventBus,
			Config: sleep.SleepConfig{
				Mode:            sleepMode,
				IdleTimeout:     idleTimeout,
				MinInteractions: minInteractions,
			},
			Enabled: true,
		})

		orchOpts = append(orchOpts, orchestrator.WithSleepCoordinator(sleepCoord))
		log.Info("Sleep Coordinator (CR-020) initialized: mode=%s, idle=%v, min_interactions=%d",
			cfg.Sleep.Mode, idleTimeout, minInteractions)
	} else {
		log.Debug("Sleep Coordinator (CR-020) disabled in config")
	}

	orch := orchestrator.New(orchOpts...)

	// Start feedback loop background worker if enabled
	if feedbackLoop != nil {
		if err := feedbackLoop.Start(context.Background()); err != nil {
			log.Warn("Failed to start feedback loop: %v", err)
		} else {
			log.Debug("Started feedback loop background worker")
		}
	}

	// Start CR-015 enhanced memory background jobs
	orch.StartEnhancedMemoryJobs()

	log.Info("Orchestrator initialized successfully")
	log.Debug("  Database: %s", cfg.Knowledge.DBPath)
	log.Debug("  LLM: %s", cfg.LLM.DefaultProvider)
	log.Debug("  Cognitive: %t", cognitiveEnabled)
	log.Debug("  Sync enabled: %t", cfg.Sync.Enabled)

	cleanup := func() {
		log.Debug("Cleaning up orchestrator resources...")
		orch.StopEnhancedMemoryJobs()
		if feedbackLoop != nil {
			feedbackLoop.Stop()
			log.Debug("Stopped feedback loop")
		}
		eventBus.Close()
		log.Debug("Event bus closed")
		storeCleanup()
	}

	// Return the auto-selected model for TUI default
	modelSel := &ModelSelection{
		Provider: "ollama",
		Model:    selectedLocalModel,
	}

	return orch, eventBus, modelSel, llmProvider, memoryStore, store.DB(), cleanup, nil
}

func initializeStore() (*data.Store, *config.Config, func(), error) {
	if log != nil {
		defer log.Trace("initializeStore")()
	}

	cfg, err := loadConfig()
	if err != nil {
		log.Error("Failed to load config: %v", err)
		return nil, nil, nil, fmt.Errorf("failed to load config: %w", err)
	}

	// data.NewDB expects a directory, extract from DBPath
	dbDir := filepath.Dir(cfg.Knowledge.DBPath)
	log.Debug("Database directory: %s", dbDir)

	if err := os.MkdirAll(dbDir, 0755); err != nil {
		log.Error("Failed to create db directory: %v", err)
		return nil, nil, nil, fmt.Errorf("failed to create db directory: %w", err)
	}

	// Open store - NewDB takes a directory path
	log.Debug("Opening SQLite database...")
	store, err := data.NewDB(dbDir)
	if err != nil {
		log.Error("Failed to open database: %v", err)
		return nil, nil, nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Set global store for cross-package access (e.g., LEANN migration)
	data.SetGlobalStore(store)

	log.Info("Database opened successfully: %s", cfg.Knowledge.DBPath)

	cleanup := func() {
		log.Debug("Closing database connection...")
		store.Close()
	}

	return store, cfg, cleanup, nil
}

func loadConfig() (*config.Config, error) {
	path := getConfigPath()
	log.Debug("Loading config from: %s", path)

	// Try to load existing config
	cfg, err := config.LoadFromPath(path)
	if err != nil {
		log.Debug("Config file not found, using defaults")
		// Create default config
		cfg = config.Default()

		// Override with CLI flags
		if dbPath != "" {
			log.Debug("Overriding db path from CLI flag: %s", dbPath)
			cfg.Knowledge.DBPath = dbPath
		}
	} else {
		log.Debug("Config loaded successfully")
	}

	return cfg, nil
}

func getConfigPath() string {
	if cfgPath != "" {
		return cfgPath
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ".cortex/config.yaml"
	}
	return filepath.Join(home, ".cortex", "config.yaml")
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// ═══════════════════════════════════════════════════════════════════════════════
// SIMPLE CHAT LLM ADAPTER
// ═══════════════════════════════════════════════════════════════════════════════

// simpleChatLLMAdapter adapts llm.Provider to cognitive.SimpleChatProvider.
type simpleChatLLMAdapter struct {
	provider llm.Provider
	model    string
}

// Chat implements cognitive.SimpleChatProvider.
func (a *simpleChatLLMAdapter) Chat(ctx context.Context, messages []cognitive.ChatMessage, systemPrompt string) (string, error) {
	// Convert messages to llm.Message
	llmMessages := make([]llm.Message, len(messages))
	for i, m := range messages {
		llmMessages[i] = llm.Message{
			Role:    m.Role,
			Content: m.Content,
		}
	}

	// Create chat request
	req := &llm.ChatRequest{
		Model:        a.model,
		SystemPrompt: systemPrompt,
		Messages:     llmMessages,
		MaxTokens:    4096, // Reasonable default for distillation
		Temperature:  0.3,  // Lower temperature for more consistent results
	}

	// Call the LLM
	resp, err := a.provider.Chat(ctx, req)
	if err != nil {
		return "", err
	}

	return resp.Content, nil
}

// truncateString truncates a string to maxLen characters.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// memoryEmbedderAdapter adapts cogRouter.OllamaEmbedder to memory.Embedder.
type memoryEmbedderAdapter struct {
	embedder cogRouter.Embedder
}

func (a *memoryEmbedderAdapter) Embed(ctx context.Context, text string) ([]float32, error) {
	embedding, err := a.embedder.Embed(ctx, text)
	if err != nil {
		return nil, err
	}
	return []float32(embedding), nil
}

func (a *memoryEmbedderAdapter) EmbedFast(ctx context.Context, text string) ([]float32, error) {
	embedding, err := a.embedder.EmbedFast(ctx, text)
	if err != nil {
		return nil, err
	}
	return []float32(embedding), nil
}

func (a *memoryEmbedderAdapter) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	embeddings, err := a.embedder.EmbedBatch(ctx, texts)
	if err != nil {
		return nil, err
	}
	result := make([][]float32, len(embeddings))
	for i, emb := range embeddings {
		result[i] = []float32(emb)
	}
	return result, nil
}

func (a *memoryEmbedderAdapter) Dimension() int {
	return a.embedder.Dimension()
}

func (a *memoryEmbedderAdapter) ModelName() string {
	return a.embedder.ModelName()
}

// memoryLLMAdapter adapts llm.Provider to memory.LLMProvider.
type memoryLLMAdapter struct {
	provider llm.Provider
	model    string
}

func (a *memoryLLMAdapter) Complete(ctx context.Context, prompt string) (string, error) {
	req := &llm.ChatRequest{
		Model:       a.model,
		Messages:    []llm.Message{{Role: "user", Content: prompt}},
		MaxTokens:   2048,
		Temperature: 0.3,
	}
	resp, err := a.provider.Chat(ctx, req)
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

type introspectionLLMAdapter struct {
	provider llm.Provider
	model    string
}

func (a *introspectionLLMAdapter) Complete(ctx context.Context, prompt string) (string, error) {
	req := &llm.ChatRequest{
		Model:       a.model,
		Messages:    []llm.Message{{Role: "user", Content: prompt}},
		MaxTokens:   2048,
		Temperature: 0.3,
	}
	resp, err := a.provider.Chat(ctx, req)
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

type introspectionEmbedderAdapter struct {
	embedder cogRouter.Embedder
}

func (a *introspectionEmbedderAdapter) Embed(ctx context.Context, text string) ([]float32, error) {
	embedding, err := a.embedder.Embed(ctx, text)
	if err != nil {
		return nil, err
	}
	return []float32(embedding), nil
}

func (a *introspectionEmbedderAdapter) Dimension() int {
	return a.embedder.Dimension()
}

// introspectionEventAdapter adapts bus.EventBus to introspection.EventPublisher.
type introspectionEventAdapter struct {
	eventBus *bus.EventBus
}

func (a *introspectionEventAdapter) Publish(event interface{}) {
	if a.eventBus == nil {
		return
	}
	// Convert to bus.Event if possible, otherwise wrap
	if e, ok := event.(bus.Event); ok {
		a.eventBus.Publish(e)
	}
}

// topicStoreCreatorAdapter adapts memory.TopicStore to introspection.TopicStoreCreator.
// CR-018: Enables introspection acquisition to create topics in enhanced memory.
type topicStoreCreatorAdapter struct {
	store *memory.TopicStore
}

func (a *topicStoreCreatorAdapter) CreateTopic(ctx context.Context, topic *memory.Topic) error {
	if a.store == nil {
		return nil
	}
	return a.store.CreateTopic(ctx, topic)
}

// ═══════════════════════════════════════════════════════════════════════════════
// UI COMMAND (PRISM CONTROL PLANE)
// ═══════════════════════════════════════════════════════════════════════════════

func uiCmd() *cobra.Command {
	var port int
	var devMode bool
	var noBrowser bool

	cmd := &cobra.Command{
		Use:   "ui",
		Short: "Launch Prism web UI (control plane)",
		Long: `Launch the Prism web UI - a graphical control plane for Cortex.

Prism provides:
  • Dashboard with system status and active facet
  • Facet browser and one-shot builder
  • Knowledge source management
  • Settings configuration
  • Voice input (when configured)

The UI runs on localhost only for security. Opens browser automatically.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUI(port, devMode, !noBrowser)
		},
	}

	cmd.Flags().IntVarP(&port, "port", "p", 7890, "Port to run server on")
	cmd.Flags().BoolVarP(&devMode, "dev", "d", false, "Enable dev mode (CORS for Vite)")
	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "Don't auto-open browser")

	return cmd
}

func runUI(port int, devMode bool, openBrowser bool) error {
	// Load .env file for API key detection in status endpoint
	loadEnvFile()

	log.Info("Starting Prism web UI...")

	// Get configuration
	appCfg, _ := loadConfig()

	// Apply CLI flags to voice config
	if voiceEnabled {
		appCfg.Voice.Enabled = true
	}
	if voiceOrchestratorURL != "" {
		appCfg.Voice.OrchestratorURL = voiceOrchestratorURL
	}

	// Create server config
	cfg := server.DefaultConfig()
	cfg.Port = port
	cfg.DevMode = devMode
	cfg.OpenBrowser = openBrowser

	if devMode {
		log.Info("Dev mode enabled - CORS configured for http://localhost:5173")
	}

	// Create server
	prism := server.New(cfg)

	// Initialize Event Bus (CR-010)
	eventBus := bus.New()
	defer eventBus.Close()
	prism.SetEventBus(eventBus)
	log.Debug("Event bus initialized for Prism server")

	// ═══════════════════════════════════════════════════════════════════════════════
	// CR-023: CORTEXEYES INITIALIZATION (Screen Awareness)
	// ═══════════════════════════════════════════════════════════════════════════════

	var cortexEyesServer *vision.CortexEyes
	if appCfg.CortexEyes.Enabled {
		log.Info("Initializing CortexEyes for Prism server (Screen Awareness)...")

		// Get Ollama endpoint for vision providers
		ollamaURL := appCfg.LLM.Providers["ollama"].Endpoint
		if ollamaURL == "" {
			ollamaURL = "http://127.0.0.1:11434"
		}

		// Create vision providers
		fastProvider := visionollama.NewMoondreamProvider(ollamaURL)
		smartProvider := visionollama.NewMiniCPMProvider(ollamaURL)

		// Create vision router
		visionRouter := vision.NewRouter(fastProvider, smartProvider, vision.DefaultConfig())

		// Initialize database for CortexEyes observations
		home, _ := os.UserHomeDir()
		eyesDbPath := filepath.Join(home, ".cortex", "knowledge.db")
		eyesDb, err := sql.Open("sqlite3", eyesDbPath+"?_journal_mode=WAL&_timeout=5000")
		if err != nil {
			log.Warn("Failed to open database for CortexEyes: %v", err)
		} else {
			defer eyesDb.Close()
			// Create CortexEyes config from app config
			eyesConfig := &vision.CortexEyesConfig{
				CaptureFPS:         appCfg.CortexEyes.Capture.FPS,
				ChangeThreshold:    appCfg.CortexEyes.Capture.ChangeThreshold,
				MinInterval:        time.Duration(appCfg.CortexEyes.Capture.MinIntervalSec) * time.Second,
				MaxRetentionDays:   appCfg.CortexEyes.Privacy.MaxRetentionDays,
				Enabled:            true,
				EnablePatterns:     appCfg.CortexEyes.Learning.EnablePatterns,
				EnableInsights:     appCfg.CortexEyes.Learning.EnableInsights,
				Webcam: &vision.WebcamConfig{
					Enabled:     appCfg.CortexEyes.Webcam.Enabled,
					CameraIndex: appCfg.CortexEyes.Webcam.CameraIndex,
					FPS:         appCfg.CortexEyes.Webcam.FPS,
				},
				Privacy: &vision.PrivacyConfig{
					Enabled:          true,
					ExcludedApps:     appCfg.CortexEyes.Privacy.ExcludedApps,
					ExcludedWindows:  appCfg.CortexEyes.Privacy.ExcludedWindows,
					AutoPauseOnIdle:  time.Duration(appCfg.CortexEyes.Privacy.AutoPauseIdleMin) * time.Minute,
					MaxRetentionDays: appCfg.CortexEyes.Privacy.MaxRetentionDays,
					RequireConsent:   appCfg.CortexEyes.Privacy.RequireConsent,
					AllowedHours: &vision.TimeRange{
						Start: appCfg.CortexEyes.Privacy.AllowedHoursStart,
						End:   appCfg.CortexEyes.Privacy.AllowedHoursEnd,
					},
				},
			}

			// Create CortexEyes
			cortexEyesServer, err = vision.NewCortexEyes(visionRouter, eyesDb, eventBus, eyesConfig)
			if err != nil {
				log.Warn("Failed to initialize CortexEyes: %v", err)
			} else {
				// Start CortexEyes watching
				if err := cortexEyesServer.Start(context.Background()); err != nil {
					log.Warn("Failed to start CortexEyes: %v", err)
				} else {
					log.Info("CortexEyes started for Prism server (screen awareness active)")

					// Wire CortexEyes to receive frames from VisionStreamHandler
					prism.SetCortexEyesCallback(func(frame *vision.Frame, appName, windowTitle string) {
						if cortexEyesServer != nil {
							_ = cortexEyesServer.ProcessFrame(context.Background(), frame, appName, windowTitle)
						}
					})
					log.Debug("CortexEyes frame callback wired to Prism VisionStreamHandler")
				}

				defer func() {
					cortexEyesServer.Stop()
					log.Info("CortexEyes stopped")
				}()
			}
		}
	} else {
		log.Debug("CortexEyes disabled in config")
	}

	// Suppress unused variable warning
	_ = cortexEyesServer

	// Initialize Voice Services (STT/TTS endpoints)
	if err := prism.InitializeVoice(); err != nil {
		log.Warn("Failed to initialize voice services: %v", err)
	}

	// ═══════════════════════════════════════════════════════════════════════════════
	// VOICE BRIDGE INITIALIZATION (for Prism web UI)
	// ═══════════════════════════════════════════════════════════════════════════════

	var voiceBridge *voice.VoiceBridge

	if appCfg.Voice.Enabled {
		log.Info("Initializing voice bridge for Prism...")

		voiceBridgeConfig := voice.BridgeConfig{
			OrchestratorURL:       appCfg.Voice.OrchestratorURL,
			InitialReconnectDelay: time.Duration(appCfg.Voice.ReconnectDelay) * time.Second,
			MaxReconnectDelay:     30 * time.Second,
			MaxReconnects:         appCfg.Voice.MaxReconnects,
			PingInterval:          30 * time.Second,
			PongTimeout:           60 * time.Second,
			WriteTimeout:          10 * time.Second,
			ReadTimeout:           120 * time.Second,
		}

		voiceBridge = voice.NewVoiceBridge(voiceBridgeConfig)
		voiceBridge.SetEventBus(eventBus) // Set event bus for publishing voice events

		// CR-021: Initialize voice emotion bridge for Blackboard integration
		voice.InitBlackboardBridge(eventBus)
		log.Debug("Voice emotion bridge initialized for Blackboard")

		// Attempt to connect to voice orchestrator
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := voiceBridge.Connect(ctx); err != nil {
			log.Warn("Voice orchestrator not available, voice features disabled: %v", err)
		} else {
			log.Info("Connected to voice orchestrator at %s", appCfg.Voice.OrchestratorURL)

			// Wire up transcript handler with event bus
			voiceBridge.OnTranscript(func(text string, isFinal bool) {
				log.Debug("Voice transcript: text=%q final=%v", text, isFinal)

				if isFinal && text != "" {
					// Publish transcript event to event bus
					if eventBus != nil {
						eventBus.Publish(bus.NewVoiceTranscriptEvent(text, isFinal))
					}
					log.Info("Final voice transcript: %s", text)
				}
			})

			// Set voice bridge on Prism server
			prism.SetVoiceBridge(voiceBridge)
		}

		defer func() {
			if voiceBridge != nil {
				voiceBridge.Close()
			}
		}()
	} else {
		log.Debug("Voice features disabled in config")
	}

	// Initialize database for persona management
	home, _ := os.UserHomeDir()
	dataDir := filepath.Join(home, ".cortex")
	if err := prism.SetDatabase(dataDir); err != nil {
		log.Error("Failed to initialize database: %v", err)
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	log.Debug("Database initialized for Prism server")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	actualPort, err := prism.Start(ctx)
	if err != nil {
		log.Error("Failed to start Prism: %v", err)
		return fmt.Errorf("failed to start server: %w", err)
	}

	log.Info("Prism running at http://127.0.0.1:%d", actualPort)
	if actualPort != port {
		log.Info("Note: Requested port %d was unavailable, using %d", port, actualPort)
	}

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	fmt.Printf("\n⬡ Prism Control Plane\n")
	fmt.Printf("  URL: http://127.0.0.1:%d\n", actualPort)
	fmt.Printf("  Mode: %s\n", func() string {
		if devMode {
			return "Development"
		}
		return "Production"
	}())
	fmt.Printf("\nPress Ctrl+C to stop...\n")

	<-sigChan
	fmt.Println("\nShutting down...")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := prism.Stop(shutdownCtx); err != nil {
		log.Error("Shutdown error: %v", err)
		return err
	}

	log.Info("Prism stopped gracefully")
	return nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// TUI COMMAND (CHARMBRACELET TERMINAL INTERFACE)
// ═══════════════════════════════════════════════════════════════════════════════

func tuiCmd() *cobra.Command {
	var themeFlag string

	cmd := &cobra.Command{
		Use:   "tui",
		Short: "Launch interactive Charmbracelet TUI",
		Long: `Launch the Charmbracelet terminal user interface.

The TUI provides an interactive chat interface with:
  • Real-time AI conversation
  • Syntax-highlighted code blocks
  • Theme customization
  • Tool execution visualization
  • Knowledge fabric integration

Available themes:
  • default   - Tokyo Night (blue, modern)
  • dracula   - Dracula (pink, purple)
  • nord      - Nord (frost, minimal)
  • gruvbox   - Gruvbox Dark (warm, retro)

Example:
  cortex tui
  cortex tui --theme dracula
  cortex tui --theme nord`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTUIWithTheme(themeFlag)
		},
	}

	cmd.Flags().StringVarP(&themeFlag, "theme", "t", "default", "UI theme (default, dracula, nord, gruvbox)")

	return cmd
}

func runTUIWithTheme(themeName string) error {
	// Map user-friendly theme names to theme IDs
	themeMap := map[string]string{
		"default":    "tokyo_night",
		"dracula":    "dracula",
		"nord":       "nord",
		"gruvbox":    "gruvbox_dark",
		"monokai":    "monokai",
		"catppuccin": "catppuccin_mocha",
	}

	themeID := themeMap[themeName]
	if themeID == "" {
		// Fallback to default if theme not recognized
		log.Warn("Unknown theme '%s', using default (tokyo_night)", themeName)
		themeID = theme.DefaultTheme
	}

	log.Info("Starting TUI with theme: %s (%s)", themeName, themeID)

	// Load .env file into process environment
	loadEnvFile()

	// Initialize components
	log.Info("Initializing Cortex AI...")

	// CRITICAL: Force TrueColor to ensure backgrounds are rendered correctly
	// This fixes issues where light themes render with black backgrounds on some terminals
	lipgloss.SetColorProfile(termenv.TrueColor)

	orch, _, modelSel, llmProv, _, _, cleanup, err := initializeOrchestrator()
	if err != nil {
		log.Error("Failed to initialize orchestrator: %v", err)
		return fmt.Errorf("failed to initialize: %w", err)
	}
	defer cleanup()

	// ═══════════════════════════════════════════════════════════════════════════════
	// BRAIN EXECUTIVE INITIALIZATION
	// ═══════════════════════════════════════════════════════════════════════════════

	// Create Brain Executive if LLM is available
	var brainExec *brain.Executive
	if llmProv != nil && llmProv.Available() {
		log.Info("Initializing Brain Executive...")
		brainExec = internalbrain.NewExecutive(internalbrain.FactoryConfig{
			LLMProvider:  llmProv,
			MemorySystem: nil,    // CoreMemoryStore doesn't implement brain.MemorySystem
			UserID:       "user", // Default user ID for CLI
		})
		brainExec.Start()
		log.Info("Brain Executive started")

		// CR-027: Register lobes with capability registrar
		if orch != nil && orch.Registrar() != nil {
			if err := internalbrain.RegisterLobesWithRegistrar(brainExec, orch.Registrar()); err != nil {
				log.Warn("Failed to register lobes with registrar: %v", err)
			} else {
				log.Info("Lobes registered with capability registrar")
			}
		}

		defer func() {
			brainExec.Stop()
			log.Info("Brain Executive stopped")
		}()
	} else {
		log.Warn("Brain Executive disabled (LLM not available)")
	}

	// ═══════════════════════════════════════════════════════════════════════════════
	// VOICE BRIDGE INITIALIZATION (CR-012-A)
	// ═══════════════════════════════════════════════════════════════════════════════

	var voiceBridge *voice.VoiceBridge

	// Load config to check voice settings
	cfg, _ := loadConfig()

	log.Debug("Voice config loaded: enabled=%v, url=%s", cfg.Voice.Enabled, cfg.Voice.OrchestratorURL)
	log.Debug("Voice CLI flag: --voice=%v", voiceEnabled)

	// Apply --voice CLI flag
	if voiceEnabled {
		cfg.Voice.Enabled = true
	}
	if voiceOrchestratorURL != "" {
		cfg.Voice.OrchestratorURL = voiceOrchestratorURL
	}

	// Apply voice defaults if enabled but URL is empty
	if cfg.Voice.Enabled && cfg.Voice.OrchestratorURL == "" {
		cfg.Voice.OrchestratorURL = "ws://localhost:8765/ws/voice"
	}

	eventBus := orch.EventBus()

	if cfg.Voice.Enabled {
		log.Info("Initializing voice bridge...")

		voiceBridgeConfig := voice.BridgeConfig{
			OrchestratorURL:       cfg.Voice.OrchestratorURL,
			InitialReconnectDelay: time.Duration(cfg.Voice.ReconnectDelay) * time.Second,
			MaxReconnectDelay:     30 * time.Second,
			MaxReconnects:         cfg.Voice.MaxReconnects,
			PingInterval:          30 * time.Second,
			PongTimeout:           60 * time.Second,
			WriteTimeout:          10 * time.Second,
			ReadTimeout:           120 * time.Second,
		}

		voiceBridge = voice.NewVoiceBridge(voiceBridgeConfig)
		voiceBridge.SetEventBus(eventBus)

		// CR-021: Initialize voice emotion bridge for Blackboard integration
		voice.InitBlackboardBridge(eventBus)
		log.Debug("Voice emotion bridge initialized for Blackboard")

		// Attempt to connect to voice orchestrator
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := voiceBridge.Connect(ctx); err != nil {
			log.Warn("Voice orchestrator not available, voice features disabled: %v", err)
		} else {
			log.Info("Connected to voice orchestrator at %s", cfg.Voice.OrchestratorURL)

			// Wire up interrupt handler
			voiceBridge.OnInterrupt(func(reason string) {
				log.Info("[Voice] Interrupt received: reason=%s", reason)
				if err := orch.Interrupt(reason); err != nil {
					log.Error("[Voice] Failed to interrupt orchestrator: %v", err)
				}
			})

			// Wire up transcript handler
			voiceBridge.OnTranscript(func(text string, isFinal bool) {
				if isFinal && text != "" {
					log.Info("[Voice] STT Final: %q", text)
				}
			})

			// Wire up status handler
			voiceBridge.OnStatus(func(state string) {
				log.Info("[Voice] Status change: %s", state)
			})
		}
	}

	// Create TUI with custom theme
	log.Debug("Creating TUI with theme: %s", themeID)
	tuiOpts := []tui.Option{
		tui.WithConfig(&tui.Config{
			Title:         "Cortex AI",
			ShowTimestamp: false,
			ShowDuration:  true,
			MaxHistory:    100,
		}),
		tui.WithTheme(themeID),
	}
	// Add Brain Executive if available
	if brainExec != nil {
		tuiOpts = append(tuiOpts, tui.WithBrainExecutive(brainExec))
	}
	// Note: CortexEyes not initialized in server mode (only TUI mode)

	// Set default model from auto-selection
	if modelSel != nil && modelSel.Model != "" {
		tuiOpts = append(tuiOpts, tui.WithDefaultModel(modelSel.Provider, modelSel.Model))
		log.Info("TUI default model set to: %s/%s", modelSel.Provider, modelSel.Model)
	}

	// Wire up voice bridge to TUI if enabled and connected
	if voiceBridge != nil && voiceBridge.IsConnected() {
		adapter := tui.NewTUIVoiceBridgeAdapter(voiceBridge, eventBus, nil)

		// CR-012: Initialize TTS router for hybrid voice routing (local Kokoro + cloud Resemble)
		kokoroProvider := kokoro.NewProvider(kokoro.Config{
			BaseURL:       "http://localhost:8880",
			DefaultVoice:  "af_bella",
			MaxTextLength: 2000,
		})

		routerConfig := voice.DefaultRouterConfig()
		routerConfig.FastLaneDefaultVoice = "af_bella"
		routerConfig.EnableCache = true
		routerConfig.Enabled = true
		ttsRouter := voice.NewRouter(kokoroProvider, nil, routerConfig)

		// Add Resemble cloud provider if API key is configured
		resembleAPIKey := os.Getenv("RESEMBLE_API_KEY")
		if resembleAPIKey == "" {
			if envData, err := os.ReadFile(filepath.Join(os.Getenv("HOME"), ".cortex", ".env")); err == nil {
				for _, line := range strings.Split(string(envData), "\n") {
					if strings.HasPrefix(line, "RESEMBLE_API_KEY=") {
						resembleAPIKey = strings.TrimPrefix(line, "RESEMBLE_API_KEY=")
						break
					}
				}
			}
		}
		if resembleAPIKey != "" {
			resembleCfg := resemble.Config{
				APIKey:     resembleAPIKey,
				SampleRate: 48000,
			}
			if resembleProvider, err := resemble.NewProvider(resembleCfg); err == nil {
				ttsRouter.SetCloudProvider(resembleProvider)
				log.Info("CR-012: Resemble cloud TTS provider initialized for hybrid voice routing")
			}
		}

		adapter.SetTTSRouter(ttsRouter)
		tuiOpts = append(tuiOpts, tui.WithVoiceBridge(adapter))
		log.Info("Voice bridge connected to TUI (hybrid routing enabled)")

		// Initialize Voice Intelligence components
		modeDetector := voice.NewModeDetector()
		modeDetector.SetTTSEnabled(true)

		voiceHandler := tui.NewVoiceHandler(nil, nil, modeDetector)
		tuiOpts = append(tuiOpts, tui.WithVoiceHandler(voiceHandler))
		log.Info("Voice handler initialized")

		// CR-012-C: Initialize HenryBrain for conversational state machine
		henryConfig := pkgvoice.DefaultHenryBrainConfig()

		// Create TTS engine and HenryAdapter for audio playback
		ttsConfig := voice.DefaultTTSConfig()
		ttsEngine := voice.NewTTSEngine(ttsConfig)
		henryAdapter := voice.NewHenryAdapter(ttsEngine)

		// Connect VoiceBridge to HenryAdapter for pre-cached audio playback
		henryAdapter.SetVoiceBridge(voiceBridge)

		// Create HenryBrain with adapter for TTS and audio playback
		henryBrain, err := pkgvoice.NewHenryBrain(henryConfig, henryAdapter, henryAdapter)
		if err != nil {
			log.Warn("Failed to initialize HenryBrain: %v", err)
		} else {
			tuiOpts = append(tuiOpts, tui.WithHenryBrain(henryBrain))
			log.Info("CR-012-C HenryBrain initialized with TTSEngine + VoiceBridge (state machine enabled)")
		}
	}

	app := tui.New(orch, tuiOpts...)
	app.AddSystemMessage(tui.WelcomeScreen(theme.Get(themeID)))

	log.Info("Starting interactive TUI...")

	// ═══════════════════════════════════════════════════════════════════════════════
	// CRITICAL: Redirect ALL logging away from stdout/stderr before TUI starts
	// ═══════════════════════════════════════════════════════════════════════════════

	// 1. Disable console output for custom logger
	logging.DisableConsoleOutput()
	defer logging.EnableConsoleOutput()

	// 2. Redirect zerolog to log file
	home, _ := os.UserHomeDir()
	logDir := filepath.Join(home, ".cortex", "logs")
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	zerologPath := filepath.Join(logDir, fmt.Sprintf("cortex_zerolog_%s.log", timestamp))

	zerologFile, err := os.OpenFile(zerologPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Warn("Failed to redirect zerolog: %v", err)
	} else {
		defer zerologFile.Close()
		zerologWriter := zerolog.ConsoleWriter{Out: zerologFile, NoColor: true}
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
		fileLogger := zerolog.New(zerologWriter).With().Timestamp().Logger()
		zerolog.DefaultContextLogger = &fileLogger
		// CRITICAL: Also set the global logger used by log.Info(), log.Debug(), etc.
		zlog.Logger = fileLogger
		log.Debug("Zerolog redirected to: %s", zerologPath)
	}

	return app.Run()
}
