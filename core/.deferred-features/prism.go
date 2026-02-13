package server

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/normanking/cortex/internal/avatar"
	"github.com/normanking/cortex/internal/bus"
	"github.com/normanking/cortex/internal/cognitive"
	"github.com/normanking/cortex/internal/data"
	"github.com/normanking/cortex/internal/facets"
	"github.com/normanking/cortex/internal/ingestion"
	"github.com/normanking/cortex/internal/logging"
	"github.com/normanking/cortex/internal/orchestrator"
	"github.com/normanking/cortex/internal/vision"
	"github.com/normanking/cortex/internal/vision/ollama"
	"github.com/normanking/cortex/internal/voice"
	"github.com/normanking/cortex/internal/voice/kokoro"
	"github.com/normanking/cortex/internal/voice/resemble"
)

// ═══════════════════════════════════════════════════════════════════════════════
// EMBEDDED ASSETS
// ═══════════════════════════════════════════════════════════════════════════════

// Assets will hold the embedded React SPA assets.
// This variable is populated by embed.go using //go:embed directive.
// In dev mode, this can be nil and requests are proxied to Vite.
var Assets embed.FS

// HasEmbeddedAssets returns true if embedded assets are available.
func HasEmbeddedAssets() bool {
	entries, err := fs.ReadDir(Assets, ".")
	return err == nil && len(entries) > 0
}

// ═══════════════════════════════════════════════════════════════════════════════
// PRISM SERVER
// ═══════════════════════════════════════════════════════════════════════════════

// Prism is the HTTP server for the Cortex control plane.
type Prism struct {
	config    *Config
	server    *http.Server
	port      int
	startedAt time.Time
	log       *logging.Logger

	// SSE clients
	sseClients map[chan SSEEvent]struct{}
	sseMu      sync.RWMutex

	// State
	tuiActive   bool
	activeFacet string

	// Facets storage (in-memory for now, will persist to config in future)
	facets   []Facet
	facetsMu sync.RWMutex

	// Knowledge storage (in-memory for now, will use KnowledgeFabric in future)
	documents   []KnowledgeDocument
	documentsMu sync.RWMutex

	// Configuration (in-memory, mirrors ~/.cortex/config.yaml)
	appConfig   ConfigResponse
	appConfigMu sync.RWMutex

	// Provider API keys (stored separately for security)
	providerKeys   map[string]string
	providerKeysMu sync.RWMutex

	// Conversations storage (in-memory for now)
	conversations   map[string]*Conversation
	conversationsMu sync.RWMutex

	// Mode tracking (behavioral modes per conversation)
	modeTracker *cognitive.ModeTracker

	// Cognitive pipeline (lane-gated architecture)
	usePipeline     bool
	pipeline        *cognitive.Pipeline
	pipelineMetrics *cognitive.PipelineMetrics

	// Database and stores
	dataStore    *data.Store
	personaStore *facets.PersonaStore

	// Vision router (for image analysis)
	visionRouter *vision.Router

	// Voice handler (for STT/TTS)
	voiceHandler *voice.Handler

	// Voice bridge (for voice orchestrator connection)
	voiceBridge *voice.VoiceBridge

	// Avatar state manager and handler
	avatarManager *avatar.StateManager
	avatarHandler *AvatarHandler

	// Vision stream handler (WebSocket video streaming)
	visionStreamHandler *VisionStreamHandler

	// Event Bus (CR-010)
	eventBus *bus.EventBus

	// Orchestrator (CR-017: Interface for decoupling)
	orch orchestrator.Interface

	// Ingestion pipeline (CR-012)
	ingestionPipeline  *ingestion.Pipeline
	ingestionRetriever *ingestion.Retriever
	ingestionStore     *ingestion.Store
}

// New creates a new Prism server.
func New(cfg *Config) *Prism {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	now := time.Now()
	defaultFacets := []Facet{
		{
			ID:           "default",
			Name:         "Default",
			Description:  "General-purpose AI assistant with balanced capabilities",
			SystemPrompt: "You are a helpful, accurate, and concise AI assistant. Provide clear explanations and practical solutions.",
			Icon:         "🤖",
			Color:        "cyan",
			Active:       true,
			IsBuiltIn:    true,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		{
			ID:           "coder",
			Name:         "Coder",
			Description:  "Expert software engineer focused on code quality and best practices",
			SystemPrompt: "You are an expert software engineer. Write clean, well-documented code. Explain your reasoning and suggest improvements. Follow best practices for the language and framework being used.",
			Icon:         "💻",
			Color:        "purple",
			Active:       false,
			IsBuiltIn:    true,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		{
			ID:           "analyst",
			Name:         "Analyst",
			Description:  "Data analyst skilled in insights and visualization",
			SystemPrompt: "You are a data analyst. Analyze data thoroughly, identify patterns, and present insights clearly. Use appropriate statistical methods and suggest visualizations.",
			Icon:         "📊",
			Color:        "green",
			Active:       false,
			IsBuiltIn:    true,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		{
			ID:           "writer",
			Name:         "Writer",
			Description:  "Creative writer for documentation, content, and communication",
			SystemPrompt: "You are a skilled writer. Create clear, engaging, and well-structured content. Adapt your tone to the audience and purpose.",
			Icon:         "✍️",
			Color:        "yellow",
			Active:       false,
			IsBuiltIn:    true,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
	}

	// Sample documents to demonstrate the Knowledge API
	defaultDocuments := []KnowledgeDocument{
		{
			ID:         "doc-001",
			Title:      "Cortex Architecture Guide",
			Type:       "file",
			SourcePath: "/docs/architecture.md",
			ChunkCount: 12,
			SizeBytes:  45678,
			CreatedAt:  now,
			UpdatedAt:  now,
			Metadata:   map[string]interface{}{"category": "documentation", "author": "system"},
		},
		{
			ID:         "doc-002",
			Title:      "Go Best Practices",
			Type:       "file",
			SourcePath: "/docs/go-best-practices.md",
			ChunkCount: 8,
			SizeBytes:  23456,
			CreatedAt:  now,
			UpdatedAt:  now,
			Metadata:   map[string]interface{}{"category": "guidelines", "language": "go"},
		},
		{
			ID:         "doc-003",
			Title:      "React Component Patterns",
			Type:       "file",
			SourcePath: "/docs/react-patterns.md",
			ChunkCount: 15,
			SizeBytes:  67890,
			CreatedAt:  now,
			UpdatedAt:  now,
			Metadata:   map[string]interface{}{"category": "guidelines", "language": "typescript"},
		},
		{
			ID:         "doc-004",
			Title:      "Team Standup Notes 2024-01",
			Type:       "conversation",
			ChunkCount: 5,
			SizeBytes:  12345,
			CreatedAt:  now.Add(-72 * time.Hour),
			UpdatedAt:  now.Add(-72 * time.Hour),
			Metadata:   map[string]interface{}{"category": "meetings", "participants": 4},
		},
	}

	// Default application configuration (mirrors ~/.cortex/config.yaml)
	defaultAppConfig := ConfigResponse{
		LLM: LLMConfig{
			DefaultProvider: "ollama",
			Providers: map[string]LLMProviderConfig{
				"ollama": {
					Endpoint: "http://127.0.0.1:11434",
					Model:    "llama3.2",
				},
				"openai": {
					Model: "gpt-4o-mini",
				},
				"anthropic": {
					Model: "claude-3-5-sonnet-20241022",
				},
				"gemini": {
					Model: "gemini-1.5-flash",
				},
			},
		},
		Knowledge: KnowledgeConfigAPI{
			DBPath:         "~/.cortex/knowledge.db",
			DefaultTier:    "personal",
			TrustDecayDays: 30,
		},
		Sync: SyncConfigAPI{
			Enabled:  false,
			Endpoint: "https://api.acontext.io",
			Interval: "5m",
		},
		TUI: TUIConfigAPI{
			Theme:        "dark",
			VimMode:      false,
			SidebarWidth: 30,
		},
		Logging: LoggingConfigAPI{
			Level: "info",
			File:  "~/.cortex/logs/cortex.log",
		},
		Cognitive: CognitiveConfigAPI{
			Enabled:                   true,
			OllamaURL:                 "http://127.0.0.1:11434",
			EmbeddingModel:            "nomic-embed-text",
			FrontierModel:             "claude-sonnet-4-20250514",
			SimilarityThresholdHigh:   0.85,
			SimilarityThresholdMedium: 0.70,
			SimilarityThresholdLow:    0.50,
			ComplexityThreshold:       70,
		},
		Voice: &VoiceConfigAPI{
			Enabled:    false,
			STTEnabled: false,
			TTSEnabled: false,
			TTSRate:    1.0,
			TTSPitch:   1.0,
			Language:   "en-US",
		},
	}

	// Initialize mode tracker and register built-in personas
	modeTracker := cognitive.NewModeTracker()
	facets.InitializeBuiltInPersonas()
	for i := range facets.BuiltInPersonas {
		modeTracker.RegisterPersona(&facets.BuiltInPersonas[i])
	}

	return &Prism{
		config:        cfg,
		log:           logging.Global(),
		sseClients:    make(map[chan SSEEvent]struct{}),
		facets:        defaultFacets,
		activeFacet:   "default",
		documents:     defaultDocuments,
		appConfig:     defaultAppConfig,
		providerKeys:  make(map[string]string),
		conversations: make(map[string]*Conversation),
		modeTracker:   modeTracker,
		dataStore:     nil,
		personaStore:  nil,
	}
}

// SetDatabase initializes the database and persona store.
// This should be called after New() and before Start().
func (p *Prism) SetDatabase(dataDir string) error {
	// Create data store
	store, err := data.NewDB(dataDir)
	if err != nil {
		return fmt.Errorf("initialize database: %w", err)
	}

	p.dataStore = store
	p.personaStore = facets.NewPersonaStore(store.DB())

	// Initialize built-in personas in the database
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := p.personaStore.InitBuiltIns(ctx); err != nil {
		return fmt.Errorf("initialize built-in personas: %w", err)
	}

	p.log.Info("[Prism] Database initialized with built-in personas")
	return nil
}

// SetEventBus sets the event bus for publishing server events.
// This should be called after New() and before Start().
func (p *Prism) SetEventBus(eventBus *bus.EventBus) {
	p.eventBus = eventBus
	p.log.Info("[Prism] Event bus configured")
}

// SetOrchestrator sets the orchestrator for project fingerprinting and planning.
// This should be called after New() and before Start().
// CR-017: Accepts interface for decoupling.
func (p *Prism) SetOrchestrator(orch orchestrator.Interface) {
	p.orch = orch
	p.log.Info("[Prism] Orchestrator configured")
}

// SetVoiceBridge sets the voice bridge for voice orchestrator connection.
// This should be called after New() and before Start().
func (p *Prism) SetVoiceBridge(voiceBridge *voice.VoiceBridge) {
	p.voiceBridge = voiceBridge
	p.log.Info("[Prism] Voice bridge configured")
}

// SetCortexEyesCallback sets the callback function for CortexEyes to receive frames.
// CR-023: This enables screen awareness by forwarding frames from the vision stream.
func (p *Prism) SetCortexEyesCallback(cb func(frame *vision.Frame, appName, windowTitle string)) {
	if p.visionStreamHandler != nil {
		p.visionStreamHandler.SetFrameCallback(cb)
	} else {
		p.log.Warn("[Prism] Vision stream handler not initialized, CortexEyes callback not set")
	}
}

// InitializeVision initializes the vision router with Ollama providers.
// This should be called after New() and before Start().
// If ollamaURL is empty, it defaults to http://127.0.0.1:11434.
func (p *Prism) InitializeVision(ollamaURL string) error {
	// Use default Ollama URL if not provided
	if ollamaURL == "" {
		ollamaURL = "http://127.0.0.1:11434"
	}

	// Create vision config
	visionConfig := vision.DefaultConfig()
	visionConfig.OllamaURL = ollamaURL

	// Create Fast Lane provider (Moondream)
	fastProvider := ollama.NewMoondreamProvider(ollamaURL)

	// Create Smart Lane provider (MiniCPM-V)
	// Note: This may be nil if GPU is not available or model is not loaded
	smartProvider := ollama.NewMiniCPMProvider(ollamaURL)

	// Create router with both providers
	p.visionRouter = vision.NewRouter(fastProvider, smartProvider, visionConfig)

	// Check if smart provider is available
	if !smartProvider.IsHealthy() {
		p.log.Warn("[Prism] Smart vision provider (MiniCPM-V) unavailable - will use fast lane only")
		p.log.Warn("[Prism] To enable smart vision, run: ollama pull minicpm-v")
	}

	// Check if fast provider is available
	if !fastProvider.IsHealthy() {
		p.log.Warn("[Prism] Fast vision provider (Moondream) unavailable")
		p.log.Warn("[Prism] To enable vision, run: ollama pull moondream")
	}

	p.log.Info("[Prism] Vision router initialized")
	return nil
}

// InitializeVoice initializes the voice services (STT and TTS).
// This is optional and can be skipped if voice features are not enabled.
func (p *Prism) InitializeVoice() error {
	// Check if voice is enabled in config
	/*
		p.appConfigMu.RLock()
		voiceEnabled := p.appConfig.Voice != nil && p.appConfig.Voice.Enabled
		p.appConfigMu.RUnlock()

		if !voiceEnabled {
			p.log.Info("[Prism] Voice services disabled in config, skipping initialization")
			return nil
		}
	*/

	// Initialize Whisper service for STT (optional, may fail if whisper.cpp not installed)
	var whisperService *voice.WhisperService
	whisperConfig := voice.WhisperConfig{
		DefaultModelSize: "base",
		MaxAudioSize:     25 * 1024 * 1024, // 25MB
		EnableGPU:        false,
		NumThreads:       4,
	}
	whisperService, err := voice.NewWhisperService(whisperConfig)
	if err != nil {
		p.log.Warn("[Prism] Whisper service initialization failed: %v (STT will be unavailable)", err)
		whisperService = nil // Continue without STT
	} else {
		p.log.Info("[Prism] Whisper service initialized successfully")
	}

	// Initialize TTS providers
	// Fast Lane: Kokoro (CPU-only, always available)
	kokoroProvider := kokoro.NewProvider(kokoro.Config{
		BaseURL:       "http://localhost:8880",
		DefaultVoice:  "af_bella",
		MaxTextLength: 2000,
	})

	// Smart Lane: XTTS (GPU-required, optional)
	// Note: XTTS initialization is omitted for now since it requires GPU
	var smartProvider voice.Provider = nil

	// Cloud Lane: Resemble.ai (optional, requires API key)
	var cloudProvider voice.Provider = nil
	p.providerKeysMu.RLock()
	resembleAPIKey := p.providerKeys["resemble"]
	p.providerKeysMu.RUnlock()

	// Fallback to environment variable if not set in provider keys
	if resembleAPIKey == "" {
		resembleAPIKey = os.Getenv("RESEMBLE_API_KEY")
	}

	if resembleAPIKey != "" {
		resembleCfg := resemble.Config{
			APIKey:     resembleAPIKey,
			SampleRate: 48000,
		}
		resembleProvider, err := resemble.NewProvider(resembleCfg)
		if err != nil {
			p.log.Warn("[Prism] Resemble.ai provider initialization failed: %v", err)
		} else {
			cloudProvider = resembleProvider
			p.log.Info("[Prism] Resemble.ai cloud TTS provider initialized")
		}
	}

	// Create TTS router with lane-based routing
	routerConfig := voice.DefaultRouterConfig()
	routerConfig.FastLaneDefaultVoice = "af_bella"
	routerConfig.EnableCache = true
	routerConfig.Enabled = true

	ttsRouter := voice.NewRouter(kokoroProvider, smartProvider, routerConfig)

	// Register cloud provider for provider switching
	if cloudProvider != nil {
		ttsRouter.SetCloudProvider(cloudProvider)
		p.log.Info("[Prism] Cloud TTS (Resemble.ai) available for provider switching")
	}

	// Prewarm cache with common phrases (async, don't block startup)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := ttsRouter.Prewarm(ctx); err != nil {
			p.log.Warn("[Prism] Failed to prewarm TTS cache: %v", err)
			// Continue anyway, cache will warm up on demand
		} else {
			p.log.Info("[Prism] TTS cache prewarmed successfully")
		}
	}()

	// Create voice handler
	p.voiceHandler = voice.NewHandler(whisperService, ttsRouter)
	p.log.Info("[Prism] Voice handler initialized (STT: %v, TTS: enabled)", whisperService != nil)

	return nil
}

// InitializeIngestion initializes the knowledge ingestion pipeline.
// This should be called after New() and before Start().
// It requires a database connection from the data store.
func (p *Prism) InitializeIngestion(pipeline *ingestion.Pipeline, retriever *ingestion.Retriever, store *ingestion.Store) {
	if pipeline == nil || retriever == nil || store == nil {
		p.log.Warn("[Prism] Ingestion initialization skipped - missing required components")
		return
	}

	p.ingestionPipeline = pipeline
	p.ingestionRetriever = retriever
	p.ingestionStore = store

	p.log.Info("[Prism] Knowledge ingestion pipeline initialized")
}

// InitializeAvatar initializes the avatar state manager and handlers.
// This enables real-time avatar animation state streaming (lip sync, emotions, gaze).
// NOTE: Call this after InitializeVision if you want vision stream support.
func (p *Prism) InitializeAvatar() {
	// Create avatar state manager with default config
	avatarCfg := avatar.DefaultStateManagerConfig()
	p.avatarManager = avatar.NewStateManager(avatarCfg, nil, nil, nil)

	// Create avatar handler
	p.avatarHandler = NewAvatarHandler(p.avatarManager, p.log)

	// Create vision stream handler (if vision router is available)
	if p.visionRouter != nil {
		streamCfg := vision.DefaultStreamConfig()
		streamHandler := vision.NewStreamHandler(p.visionRouter, streamCfg)
		p.visionStreamHandler = NewVisionStreamHandler(streamHandler, p.log)
		p.log.Info("[Prism] Vision stream handler initialized")
	}

	p.log.Info("[Prism] Avatar system initialized (SSE streaming enabled)")
}

// GetAvatarManager returns the avatar state manager for external integration.
// This allows other components (like voice/TTS) to update avatar state.
func (p *Prism) GetAvatarManager() *avatar.StateManager {
	return p.avatarManager
}

// Start starts the Prism server.
// It returns the actual port and any startup error.
func (p *Prism) Start(ctx context.Context) (int, error) {
	// Initialize cognitive pipeline if enabled
	if err := p.initializePipeline(); err != nil {
		p.log.Warn("[Prism] Failed to initialize cognitive pipeline: %v", err)
		// Continue without pipeline - will use mock responses
	}

	// Find available port
	port, err := p.findAvailablePort(p.config.Port)
	if err != nil {
		return 0, fmt.Errorf("no available port: %w", err)
	}
	p.port = port

	// Create router
	mux := p.createRouter()

	// Create server with localhost-only binding (security)
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	p.server = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server in goroutine
	serverErr := make(chan error, 1)
	go func() {
		p.log.Info("[Prism] Starting server at http://%s", addr)
		if err := p.server.ListenAndServe(); err != http.ErrServerClosed {
			serverErr <- err
		}
		close(serverErr)
	}()

	// Wait briefly for startup errors
	select {
	case err := <-serverErr:
		return 0, fmt.Errorf("server start failed: %w", err)
	case <-time.After(100 * time.Millisecond):
		// Server started successfully
	}

	p.startedAt = time.Now()
	p.log.Info("[Prism] Server ready at http://127.0.0.1:%d", port)

	// Open browser if configured
	if p.config.OpenBrowser {
		go p.openBrowser(fmt.Sprintf("http://127.0.0.1:%d", port))
	}

	return port, nil
}

// Stop gracefully stops the Prism server.
func (p *Prism) Stop(ctx context.Context) error {
	if p.server == nil {
		return nil
	}

	p.log.Info("[Prism] Shutting down server...")

	// Create shutdown context with timeout
	shutdownCtx, cancel := context.WithTimeout(ctx, p.config.ShutdownTimeout)
	defer cancel()

	// Close all SSE clients
	p.sseMu.Lock()
	for client := range p.sseClients {
		close(client)
		delete(p.sseClients, client)
	}
	p.sseMu.Unlock()

	// Graceful shutdown
	if err := p.server.Shutdown(shutdownCtx); err != nil {
		p.log.Warn("[Prism] Graceful shutdown failed: %v", err)
		p.server.Close()
	}

	// Close database if initialized
	if p.dataStore != nil {
		if err := p.dataStore.Close(); err != nil {
			p.log.Warn("[Prism] Database close failed: %v", err)
		}
	}

	p.log.Info("[Prism] Server stopped")
	return nil
}

// Port returns the actual port the server is running on.
func (p *Prism) Port() int {
	return p.port
}

// URL returns the server URL.
func (p *Prism) URL() string {
	return fmt.Sprintf("http://127.0.0.1:%d", p.port)
}

// SetTUIActive updates the TUI active status.
func (p *Prism) SetTUIActive(active bool) {
	p.tuiActive = active
	p.broadcast(SSEEvent{
		Type:      EventStatus,
		Timestamp: time.Now(),
		Data:      map[string]bool{"tui_active": active},
	})
}

// SetActiveFacet updates the active facet.
func (p *Prism) SetActiveFacet(facetID string) {
	p.activeFacet = facetID
	p.broadcast(SSEEvent{
		Type:      EventFacet,
		Timestamp: time.Now(),
		Data:      map[string]string{"active_facet": facetID},
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// PORT HUNTING
// ═══════════════════════════════════════════════════════════════════════════════

// findAvailablePort finds an available port starting from preferredPort.
// It tries up to 100 consecutive ports.
func (p *Prism) findAvailablePort(preferredPort int) (int, error) {
	for port := preferredPort; port < preferredPort+100; port++ {
		addr := fmt.Sprintf("127.0.0.1:%d", port)
		listener, err := net.Listen("tcp", addr)
		if err == nil {
			listener.Close()
			return port, nil
		}
	}
	return 0, fmt.Errorf("no available port in range %d-%d", preferredPort, preferredPort+99)
}

// ═══════════════════════════════════════════════════════════════════════════════
// BROWSER LAUNCH
// ═══════════════════════════════════════════════════════════════════════════════

// openBrowser opens the default browser to the given URL.
func (p *Prism) openBrowser(url string) {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		p.log.Warn("[Prism] Cannot open browser on %s", runtime.GOOS)
		return
	}

	if err := cmd.Start(); err != nil {
		p.log.Warn("[Prism] Failed to open browser: %v", err)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// ROUTER
// ═══════════════════════════════════════════════════════════════════════════════

// createRouter creates the HTTP router with all endpoints.
func (p *Prism) createRouter() http.Handler {
	mux := http.NewServeMux()

	// API v1 routes
	mux.HandleFunc("/api/v1/status", p.handleStatus)
	mux.HandleFunc("/api/v1/events", p.handleSSE)

	// Facets API
	mux.HandleFunc("/api/v1/facets", p.handleFacets)
	mux.HandleFunc("/api/v1/facets/", p.handleFacetByID)

	// Personas API
	mux.HandleFunc("/api/v1/personas", p.handlePersonas)
	mux.HandleFunc("/api/v1/personas/", p.handlePersonaByID)

	// Knowledge API (legacy mock endpoints - deprecated)
	mux.HandleFunc("/api/v1/knowledge/documents", p.handleDocuments)
	mux.HandleFunc("/api/v1/knowledge/documents/", p.handleDocumentByID)

	// Knowledge Ingestion API (CR-012 - real ingestion pipeline)
	if p.ingestionPipeline != nil && p.ingestionStore != nil {
		mux.HandleFunc("/api/v1/knowledge/ingest", p.handleIngestionIngest)
		mux.HandleFunc("/api/v1/knowledge/search", p.handleIngestionSearch)
		mux.HandleFunc("/api/v1/knowledge/sources", p.handleIngestionSources)
		mux.HandleFunc("/api/v1/knowledge/source/", p.handleIngestionSourceByID)
		mux.HandleFunc("/api/v1/knowledge/stats", p.handleIngestionStats)
	} else {
		// Fallback to legacy mock endpoints
		mux.HandleFunc("/api/v1/knowledge/search", p.handleSearch)
		mux.HandleFunc("/api/v1/knowledge/ingest", p.handleIngest)
		mux.HandleFunc("/api/v1/knowledge/stats", p.handleKnowledgeStats)
	}

	// Config API
	mux.HandleFunc("/api/v1/config", p.handleConfig)
	mux.HandleFunc("/api/v1/providers/", p.handleProviders)

	// Chat API
	mux.HandleFunc("/api/v1/chat", p.handleChat)
	mux.HandleFunc("/api/v1/chat/conversations", p.handleConversations)
	mux.HandleFunc("/api/v1/chat/conversations/", p.handleConversationByID)
	// mux.HandleFunc("/api/v1/interrupt", p.handleInterrupt) // Removed duplicate

	// Mode API (behavioral modes)
	// Note: These are handled via path parsing in handleConversationByID

	// Mode API (global modes - CR-011)
	mux.HandleFunc("/api/v1/modes", p.handleListModes)
	mux.HandleFunc("/api/v1/mode/current", p.handleGetCurrentMode)
	mux.HandleFunc("/api/v1/mode/set", p.handleSetMode)

	// Vision API
	mux.HandleFunc("/api/v1/vision/analyze", p.handleVisionAnalyze)
	mux.HandleFunc("/api/v1/vision/health", p.handleVisionHealth)

	// Voice API (if enabled)
	if p.voiceHandler != nil {
		p.voiceHandler.RegisterRoutes(mux)
	}

	// Voice bridge endpoints
	mux.HandleFunc("/api/v1/interrupt", p.handleInterrupt)
	if p.voiceBridge != nil {
		mux.HandleFunc("/api/v1/voice/ws", p.handleVoiceWebSocket)
	}

	// Avatar State API (SSE streaming for lip sync, emotions, gaze)
	if p.avatarHandler != nil {
		p.avatarHandler.RegisterRoutes(mux)
	}

	// Vision Stream API (WebSocket video streaming)
	if p.visionStreamHandler != nil {
		p.visionStreamHandler.RegisterRoutes(mux)
	}

	// Project & Planning API (CR-011 - if orchestrator is configured)
	if p.orch != nil {
		projectHandler := NewProjectHandler(p.orch)
		mux.HandleFunc("/api/v1/project/detect", projectHandler.DetectProject)
		mux.HandleFunc("/api/v1/project/context", projectHandler.GetProjectContext)
		mux.HandleFunc("/api/v1/planning/decompose", projectHandler.DecomposeTask)
		mux.HandleFunc("/api/v1/planning/execute", projectHandler.ExecutePlan)
		mux.HandleFunc("/api/v1/planning/suggest-next", projectHandler.SuggestNext)
	}

	// Health check
	mux.HandleFunc("/health", p.handleHealth)

	// Static assets (React SPA) or dev mode proxy
	mux.HandleFunc("/", p.handleStatic)

	// Wrap with middleware
	return p.middleware(mux)
}

// middleware applies common middleware (CORS, logging).
func (p *Prism) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// CORS headers for dev mode
		if p.config.DevMode {
			w.Header().Set("Access-Control-Allow-Origin", p.config.DevOrigin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Allow-Credentials", "true")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}

		// Security headers
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")

		// Log request
		start := time.Now()
		next.ServeHTTP(w, r)
		p.log.Debug("[Prism] %s %s %v", r.Method, r.URL.Path, time.Since(start))
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// API HANDLERS
// ═══════════════════════════════════════════════════════════════════════════════

// handleStatus handles GET /api/v1/status.
func (p *Prism) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		p.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	uptime := time.Since(p.startedAt).Round(time.Second)

	// Get voice status
	voiceStatus := VoiceStatus{
		Enabled:   false,
		Connected: false,
	}
	if p.voiceBridge != nil {
		voiceStatus.Enabled = true
		voiceStatus.Connected = p.voiceBridge.IsConnected()
		voiceStatus.OrchestratorURL = p.voiceBridge.URL()
	}

	status := StatusResponse{
		Version:     "0.1.0",
		Uptime:      uptime.String(),
		StartedAt:   p.startedAt,
		Port:        p.port,
		DevMode:     p.config.DevMode,
		TUIActive:   p.tuiActive,
		ActiveFacet: p.activeFacet,
		Providers:   p.getProviderStatus(),
		Voice:       voiceStatus,
		Metrics:     p.getSystemMetrics(),
	}

	p.writeJSON(w, http.StatusOK, status)
}

// handleHealth handles GET /health - comprehensive health check.
// Returns JSON with service status including voice bridge connectivity.
func (p *Prism) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		p.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Get voice bridge health
	voiceHealth := map[string]interface{}{
		"enabled":   false,
		"connected": false,
	}
	if p.voiceBridge != nil {
		voiceHealth["enabled"] = true
		voiceHealth["connected"] = p.voiceBridge.IsConnected()
		voiceHealth["orchestrator_url"] = p.voiceBridge.URL()
	}

	// Get orchestrator health
	orchHealth := map[string]interface{}{
		"available": p.orch != nil,
	}

	// Get event bus health
	eventBusHealth := map[string]interface{}{
		"available": p.eventBus != nil,
	}

	// Build comprehensive health response
	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now(),
		"uptime":    time.Since(p.startedAt).Round(time.Second).String(),
		"services": map[string]interface{}{
			"voice":        voiceHealth,
			"orchestrator": orchHealth,
			"event_bus":    eventBusHealth,
		},
	}

	p.writeJSON(w, http.StatusOK, health)
}

// handleSSE handles GET /api/v1/events (Server-Sent Events).
func (p *Prism) handleSSE(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		p.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Create client channel
	client := make(chan SSEEvent, 10)
	p.sseMu.Lock()
	p.sseClients[client] = struct{}{}
	p.sseMu.Unlock()

	// Clean up on disconnect
	defer func() {
		p.sseMu.Lock()
		delete(p.sseClients, client)
		p.sseMu.Unlock()
	}()

	// Get flusher
	flusher, ok := w.(http.Flusher)
	if !ok {
		p.writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	// Send initial status
	initialStatus := SSEEvent{
		Type:      EventStatus,
		Timestamp: time.Now(),
		Data:      map[string]string{"status": "connected"},
	}
	p.writeSSEEvent(w, flusher, initialStatus)

	// Stream events
	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-client:
			if !ok {
				return
			}
			p.writeSSEEvent(w, flusher, event)
		}
	}
}

// writeSSEEvent writes a single SSE event.
func (p *Prism) writeSSEEvent(w http.ResponseWriter, flusher http.Flusher, event SSEEvent) {
	data, _ := json.Marshal(event)
	fmt.Fprintf(w, "event: %s\n", event.Type)
	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()
}

// broadcast sends an event to all connected SSE clients.
func (p *Prism) broadcast(event SSEEvent) {
	p.sseMu.RLock()
	defer p.sseMu.RUnlock()

	for client := range p.sseClients {
		select {
		case client <- event:
		default:
			// Client buffer full, skip
		}
	}
}

// handleStatic serves static assets or a dev mode placeholder.
func (p *Prism) handleStatic(w http.ResponseWriter, r *http.Request) {
	// In dev mode, serve a redirect page
	if p.config.DevMode {
		if r.URL.Path == "/" {
			p.writeDevModePage(w)
			return
		}
		// For other paths, return 404 (dev should use Vite directly)
		http.NotFound(w, r)
		return
	}

	// Serve embedded assets
	if HasEmbeddedAssets() {
		// Create sub filesystem rooted at "prism/dist"
		sub, err := fs.Sub(Assets, "prism/dist")
		if err != nil {
			p.log.Warn("[Prism] Failed to access embedded assets: %v", err)
			p.writeDevModePage(w)
			return
		}

		// Serve with SPA fallback (serve index.html for unknown routes)
		p.serveSPA(w, r, http.FS(sub))
		return
	}

	// No assets, show dev mode page
	p.writeDevModePage(w)
}

// serveSPA serves static files with SPA fallback.
func (p *Prism) serveSPA(w http.ResponseWriter, r *http.Request, fsys http.FileSystem) {
	path := r.URL.Path

	// Try to serve the exact file
	f, err := fsys.Open(path)
	if err == nil {
		defer f.Close()
		stat, _ := f.Stat()
		if !stat.IsDir() {
			http.ServeContent(w, r, path, stat.ModTime(), f.(http.File))
			return
		}
	}

	// Fallback to index.html for SPA routing
	f, err = fsys.Open("/index.html")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	stat, _ := f.Stat()
	http.ServeContent(w, r, "index.html", stat.ModTime(), f.(http.File))
}

// writeDevModePage writes a placeholder page for dev mode.
func (p *Prism) writeDevModePage(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	html := `<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <title>Prism - Cortex Control Plane</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: linear-gradient(135deg, #0a0a0f 0%, #1a1a2e 100%);
            color: #e0e0e0;
            min-height: 100vh;
            margin: 0;
            display: flex;
            align-items: center;
            justify-content: center;
        }
        .container {
            text-align: center;
            padding: 2rem;
        }
        h1 {
            font-size: 3rem;
            margin-bottom: 0.5rem;
            background: linear-gradient(90deg, #00d4ff, #7b68ee);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
        }
        .subtitle {
            color: #888;
            font-size: 1.2rem;
            margin-bottom: 2rem;
        }
        .status {
            background: rgba(255,255,255,0.05);
            border-radius: 12px;
            padding: 1.5rem;
            margin: 1rem 0;
        }
        .api-link {
            color: #00d4ff;
            text-decoration: none;
        }
        .api-link:hover {
            text-decoration: underline;
        }
        code {
            background: rgba(0,212,255,0.1);
            padding: 0.2rem 0.5rem;
            border-radius: 4px;
            font-family: 'SF Mono', Monaco, monospace;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>⬡ PRISM</h1>
        <p class="subtitle">Cortex Control Plane</p>
        <div class="status">
            <p><strong>Server Status:</strong> Running</p>
            <p><strong>Port:</strong> ` + fmt.Sprintf("%d", p.port) + `</p>
            <p><strong>Mode:</strong> ` + func() string {
		if p.config.DevMode {
			return "Development"
		} else {
			return "Production"
		}
	}() + `</p>
        </div>
        <p>
            <a class="api-link" href="/api/v1/status">View API Status</a>
        </p>
        <p style="margin-top: 2rem; color: #666;">
            ` + func() string {
		if p.config.DevMode {
			return `<code>npm run dev</code> in <code>prism/</code> to start the React app`
		} else {
			return `UI assets not embedded. Run <code>go generate</code> after building React app.`
		}
	}() + `
        </p>
    </div>
</body>
</html>`

	w.Write([]byte(html))
}

// ═══════════════════════════════════════════════════════════════════════════════
// FACETS API HANDLERS
// ═══════════════════════════════════════════════════════════════════════════════

// handleFacets handles /api/v1/facets routes.
// Supports:
//   - GET /api/v1/facets - list all facets
//   - POST /api/v1/facets - create new facet
func (p *Prism) handleFacets(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		p.handleListFacets(w, r)
	case http.MethodPost:
		p.handleCreateFacet(w, r)
	default:
		p.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleListFacets handles GET /api/v1/facets - list all facets.
func (p *Prism) handleListFacets(w http.ResponseWriter, r *http.Request) {
	p.facetsMu.RLock()
	defer p.facetsMu.RUnlock()

	response := FacetsResponse{
		Facets:      p.facets,
		ActiveFacet: p.activeFacet,
	}

	p.writeJSON(w, http.StatusOK, response)
}

// handleCreateFacet handles POST /api/v1/facets - create new facet.
func (p *Prism) handleCreateFacet(w http.ResponseWriter, r *http.Request) {
	var req CreateFacetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		p.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate required fields
	if req.Name == "" {
		p.writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	// Generate UUID for facet ID
	newID := fmt.Sprintf("facet-%d", time.Now().UnixNano())

	now := time.Now()
	newFacet := Facet{
		ID:           newID,
		Name:         req.Name,
		Description:  req.Description,
		SystemPrompt: req.SystemPrompt,
		Icon:         req.Icon,
		Color:        req.Color,
		Active:       false,
		IsBuiltIn:    false,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	// Add to facets slice with mutex lock
	p.facetsMu.Lock()
	p.facets = append(p.facets, newFacet)
	p.facetsMu.Unlock()

	// Broadcast SSE event
	p.broadcast(SSEEvent{
		Type:      EventFacet,
		Timestamp: time.Now(),
		Data:      map[string]interface{}{"action": "created", "facet": newFacet},
	})

	p.log.Info("[Prism] Created facet: %s (%s)", newFacet.Name, newFacet.ID)

	// Return 201 Created with the new facet
	p.writeJSON(w, http.StatusCreated, newFacet)
}

// handleFacetByID handles /api/v1/facets/:id routes.
// Supports:
//   - GET /api/v1/facets/:id - get single facet
//   - PUT /api/v1/facets/:id - update facet
//   - DELETE /api/v1/facets/:id - delete facet
//   - POST /api/v1/facets/:id/activate - activate facet
func (p *Prism) handleFacetByID(w http.ResponseWriter, r *http.Request) {
	// Parse facet ID from path: /api/v1/facets/{id} or /api/v1/facets/{id}/activate
	path := r.URL.Path
	prefix := "/api/v1/facets/"

	if !strings.HasPrefix(path, prefix) {
		p.writeError(w, http.StatusNotFound, "not found")
		return
	}

	remainder := strings.TrimPrefix(path, prefix)
	parts := strings.Split(remainder, "/")

	if len(parts) == 0 || parts[0] == "" {
		p.writeError(w, http.StatusNotFound, "facet ID required")
		return
	}

	facetID := parts[0]

	// Check if this is an activation request
	if len(parts) == 2 && parts[1] == "activate" {
		p.handleActivateFacet(w, r, facetID)
		return
	}

	// Handle single facet operations
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			p.handleGetFacet(w, r, facetID)
		case http.MethodPut:
			p.handleUpdateFacet(w, r, facetID)
		case http.MethodDelete:
			p.handleDeleteFacet(w, r, facetID)
		default:
			p.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	p.writeError(w, http.StatusNotFound, "not found")
}

// handleGetFacet handles GET /api/v1/facets/:id - get a single facet.
func (p *Prism) handleGetFacet(w http.ResponseWriter, r *http.Request, facetID string) {
	p.facetsMu.RLock()
	defer p.facetsMu.RUnlock()

	for _, facet := range p.facets {
		if facet.ID == facetID {
			p.writeJSON(w, http.StatusOK, facet)
			return
		}
	}

	p.writeError(w, http.StatusNotFound, "facet not found")
}

// handleActivateFacet handles POST /api/v1/facets/:id/activate - activate a facet.
func (p *Prism) handleActivateFacet(w http.ResponseWriter, r *http.Request, facetID string) {
	if r.Method != http.MethodPost {
		p.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	p.facetsMu.Lock()
	defer p.facetsMu.Unlock()

	// Find and activate the facet
	found := false
	for i := range p.facets {
		if p.facets[i].ID == facetID {
			p.facets[i].Active = true
			p.facets[i].UpdatedAt = time.Now()
			found = true
		} else {
			p.facets[i].Active = false
		}
	}

	if !found {
		p.writeError(w, http.StatusNotFound, "facet not found")
		return
	}

	// Update active facet
	p.activeFacet = facetID

	// Broadcast facet change via SSE
	p.broadcast(SSEEvent{
		Type:      EventFacet,
		Timestamp: time.Now(),
		Data:      map[string]string{"active_facet": facetID},
	})

	p.log.Info("[Prism] Activated facet: %s", facetID)
	w.WriteHeader(http.StatusNoContent)
}

// handleUpdateFacet handles PUT /api/v1/facets/:id - update a facet.
func (p *Prism) handleUpdateFacet(w http.ResponseWriter, r *http.Request, facetID string) {
	var req UpdateFacetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		p.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	p.facetsMu.Lock()
	defer p.facetsMu.Unlock()

	// Find the facet to update
	var updatedFacet *Facet
	for i := range p.facets {
		if p.facets[i].ID == facetID {
			// Check if it's a built-in facet
			if p.facets[i].IsBuiltIn {
				p.writeError(w, http.StatusForbidden, "cannot update built-in facets")
				return
			}

			// Update allowed fields
			if req.Name != "" {
				p.facets[i].Name = req.Name
			}
			if req.Description != "" {
				p.facets[i].Description = req.Description
			}
			if req.SystemPrompt != "" {
				p.facets[i].SystemPrompt = req.SystemPrompt
			}
			if req.Icon != "" {
				p.facets[i].Icon = req.Icon
			}
			if req.Color != "" {
				p.facets[i].Color = req.Color
			}

			// Update timestamp
			p.facets[i].UpdatedAt = time.Now()
			updatedFacet = &p.facets[i]
			break
		}
	}

	if updatedFacet == nil {
		p.writeError(w, http.StatusNotFound, "facet not found")
		return
	}

	// Broadcast SSE event
	p.broadcast(SSEEvent{
		Type:      EventFacet,
		Timestamp: time.Now(),
		Data:      map[string]interface{}{"action": "updated", "facet": updatedFacet},
	})

	p.log.Info("[Prism] Updated facet: %s (%s)", updatedFacet.Name, updatedFacet.ID)

	// Return 200 OK with updated facet
	p.writeJSON(w, http.StatusOK, updatedFacet)
}

// handleDeleteFacet handles DELETE /api/v1/facets/:id - delete a facet.
func (p *Prism) handleDeleteFacet(w http.ResponseWriter, r *http.Request, facetID string) {
	p.facetsMu.Lock()
	defer p.facetsMu.Unlock()

	// Find the facet to delete
	var deletedIndex = -1
	for i := range p.facets {
		if p.facets[i].ID == facetID {
			// Check if it's a built-in facet
			if p.facets[i].IsBuiltIn {
				p.writeError(w, http.StatusForbidden, "cannot delete built-in facets")
				return
			}

			// Check if it's the currently active facet
			if p.facets[i].Active {
				p.writeError(w, http.StatusConflict, "cannot delete active facet - switch to another facet first")
				return
			}

			deletedIndex = i
			break
		}
	}

	if deletedIndex == -1 {
		p.writeError(w, http.StatusNotFound, "facet not found")
		return
	}

	// Remove from slice
	p.facets = append(p.facets[:deletedIndex], p.facets[deletedIndex+1:]...)

	// Broadcast SSE event
	p.broadcast(SSEEvent{
		Type:      EventFacet,
		Timestamp: time.Now(),
		Data:      map[string]interface{}{"action": "deleted", "facet_id": facetID},
	})

	p.log.Info("[Prism] Deleted facet: %s", facetID)

	// Return 204 No Content
	w.WriteHeader(http.StatusNoContent)
}

// ═══════════════════════════════════════════════════════════════════════════════
// HELPERS
// ═══════════════════════════════════════════════════════════════════════════════

// writeJSON writes a JSON response.
func (p *Prism) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeError writes an error response.
func (p *Prism) writeError(w http.ResponseWriter, status int, message string) {
	p.writeJSON(w, status, APIError{
		Code:    status,
		Message: message,
	})
}

// getProviderStatus returns the current provider configuration status.
func (p *Prism) getProviderStatus() ProviderStatus {
	status := ProviderStatus{}

	// Check environment for API keys
	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		status.Anthropic = true
	}
	if os.Getenv("OPENAI_API_KEY") != "" {
		status.OpenAI = true
	}
	if os.Getenv("GEMINI_API_KEY") != "" {
		status.Gemini = true
	}

	// Check Ollama (try localhost by default)
	ollamaURL := os.Getenv("OLLAMA_ENDPOINT")
	if ollamaURL == "" {
		ollamaURL = "http://127.0.0.1:11434"
	}
	status.OllamaURL = ollamaURL

	// Quick Ollama check (don't block)
	go func() {
		client := http.Client{Timeout: 2 * time.Second}
		resp, err := client.Get(ollamaURL + "/api/tags")
		if err == nil {
			resp.Body.Close()
			// Could update status via SSE here
		}
	}()
	status.Ollama = true // Assume available, SSE will update

	return status
}

// getSystemMetrics returns placeholder system metrics.
// This will be populated from the cognitive metrics in Sprint 3.
func (p *Prism) getSystemMetrics() SystemMetrics {
	return SystemMetrics{
		TemplateCount: 0,
		TemplateHits:  0,
		LocalRate:     0,
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// KNOWLEDGE API HANDLERS
// ═══════════════════════════════════════════════════════════════════════════════

// handleDocuments handles GET /api/v1/knowledge/documents - list all documents.
func (p *Prism) handleDocuments(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		p.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	p.documentsMu.RLock()
	defer p.documentsMu.RUnlock()

	response := DocumentsResponse{
		Documents:  p.documents,
		TotalCount: len(p.documents),
	}

	p.writeJSON(w, http.StatusOK, response)
}

// handleDocumentByID handles /api/v1/knowledge/documents/:id routes.
// Supports:
//   - GET /api/v1/knowledge/documents/:id - get single document
//   - DELETE /api/v1/knowledge/documents/:id - delete document
func (p *Prism) handleDocumentByID(w http.ResponseWriter, r *http.Request) {
	// Parse document ID from path: /api/v1/knowledge/documents/{id}
	path := r.URL.Path
	prefix := "/api/v1/knowledge/documents/"

	if !strings.HasPrefix(path, prefix) {
		p.writeError(w, http.StatusNotFound, "not found")
		return
	}

	docID := strings.TrimPrefix(path, prefix)
	if docID == "" {
		p.writeError(w, http.StatusNotFound, "document ID required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		p.handleGetDocument(w, r, docID)
	case http.MethodDelete:
		p.handleDeleteDocument(w, r, docID)
	default:
		p.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleGetDocument handles GET /api/v1/knowledge/documents/:id - get a single document.
func (p *Prism) handleGetDocument(w http.ResponseWriter, r *http.Request, docID string) {
	p.documentsMu.RLock()
	defer p.documentsMu.RUnlock()

	for _, doc := range p.documents {
		if doc.ID == docID {
			p.writeJSON(w, http.StatusOK, doc)
			return
		}
	}

	p.writeError(w, http.StatusNotFound, "document not found")
}

// handleDeleteDocument handles DELETE /api/v1/knowledge/documents/:id - delete a document.
func (p *Prism) handleDeleteDocument(w http.ResponseWriter, r *http.Request, docID string) {
	p.documentsMu.Lock()
	defer p.documentsMu.Unlock()

	for i, doc := range p.documents {
		if doc.ID == docID {
			// Remove document
			p.documents = append(p.documents[:i], p.documents[i+1:]...)
			p.log.Info("[Prism] Deleted document: %s", docID)
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}

	p.writeError(w, http.StatusNotFound, "document not found")
}

// handleSearch handles POST /api/v1/knowledge/search - search knowledge base.
func (p *Prism) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		p.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		p.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Query == "" {
		p.writeError(w, http.StatusBadRequest, "query is required")
		return
	}

	// Set defaults and validate inputs
	if req.Limit <= 0 {
		req.Limit = 10
	}
	// Input validation: max limit enforcement
	if req.Limit > 1000 {
		req.Limit = 1000
	}
	// Input validation: min score bounds
	if req.MinScore < 0 || req.MinScore > 1 {
		req.MinScore = 0.7
	} else if req.MinScore == 0 {
		req.MinScore = 0.7
	}

	start := time.Now()

	// Improved search implementation: match query against titles and metadata
	// In production, this would use the KnowledgeFabric with FTS5 and embeddings
	p.documentsMu.RLock()
	defer p.documentsMu.RUnlock()

	var results []SearchResult
	queryLower := strings.ToLower(req.Query)
	queryWords := strings.Fields(queryLower)

	for _, doc := range p.documents {
		titleLower := strings.ToLower(doc.Title)

		// Convert metadata to searchable text
		var metadataText string
		if doc.Metadata != nil {
			for key, value := range doc.Metadata {
				metadataText += fmt.Sprintf(" %s:%v", key, value)
			}
		}
		metadataTextLower := strings.ToLower(metadataText)

		// Calculate relevance score based on matched words
		titleMatches := countMatchedWords(titleLower, queryWords)
		metadataMatches := countMatchedWords(metadataTextLower, queryWords)

		// Skip if no matches found
		if titleMatches == 0 && metadataMatches == 0 {
			continue
		}

		// Calculate score based on match percentage and location
		// Title matches are weighted more heavily (0.7) than metadata matches (0.3)
		titleScore := float64(titleMatches) / float64(len(queryWords)) * 0.7
		metadataScore := float64(metadataMatches) / float64(len(queryWords)) * 0.3
		score := titleScore + metadataScore

		// Bonus for exact phrase match in title
		if strings.Contains(titleLower, queryLower) {
			score += 0.2
		}
		// Bonus for exact title match
		if titleLower == queryLower {
			score = 0.99
		}
		// Cap score at 1.0
		if score > 1.0 {
			score = 1.0
		}

		if score >= req.MinScore {
			// Create meaningful chunk text showing what matched
			matchContext := buildMatchContext(doc.Title, titleMatches, metadataMatches, metadataText, req.Query)

			results = append(results, SearchResult{
				DocumentID:    doc.ID,
				DocumentTitle: doc.Title,
				ChunkText:     matchContext,
				Score:         score,
				ChunkIndex:    0,
				Metadata:      doc.Metadata,
			})
		}

		if len(results) >= req.Limit {
			break
		}
	}

	response := SearchResponse{
		Query:        req.Query,
		Results:      results,
		TotalCount:   len(results),
		SearchTimeMs: time.Since(start).Milliseconds(),
	}

	p.writeJSON(w, http.StatusOK, response)
}

// handleIngest handles POST /api/v1/knowledge/ingest - ingest new document.
func (p *Prism) handleIngest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		p.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req IngestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		p.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate request
	if req.Type == "" {
		p.writeError(w, http.StatusBadRequest, "type is required")
		return
	}
	if req.Type == "text" && req.Content == "" {
		p.writeError(w, http.StatusBadRequest, "content is required for text type")
		return
	}
	if (req.Type == "file" || req.Type == "directory") && req.Path == "" {
		p.writeError(w, http.StatusBadRequest, "path is required for file/directory type")
		return
	}

	// Generate document ID
	docID := fmt.Sprintf("doc-%s", uuid.New().String())

	// Determine title
	title := req.Title
	if title == "" {
		if req.Path != "" {
			// Extract filename from path
			parts := strings.Split(req.Path, "/")
			title = parts[len(parts)-1]
		} else {
			title = "Untitled Document"
		}
	}

	// Create document
	now := time.Now()
	doc := KnowledgeDocument{
		ID:         docID,
		Title:      title,
		Type:       req.Type,
		SourcePath: req.Path,
		ChunkCount: 1, // Mock: real implementation would chunk the content
		SizeBytes:  int64(len(req.Content)),
		CreatedAt:  now,
		UpdatedAt:  now,
		Metadata:   req.Metadata,
	}

	// Add to storage
	p.documentsMu.Lock()
	p.documents = append(p.documents, doc)
	p.documentsMu.Unlock()

	p.log.Info("[Prism] Ingested document: %s (%s)", doc.Title, docID)

	response := IngestResponse{
		DocumentID: docID,
		Title:      title,
		ChunkCount: doc.ChunkCount,
		Message:    "Document ingested successfully",
	}

	p.writeJSON(w, http.StatusCreated, response)
}

// handleKnowledgeStats handles GET /api/v1/knowledge/stats - get statistics.
func (p *Prism) handleKnowledgeStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		p.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	p.documentsMu.RLock()
	defer p.documentsMu.RUnlock()

	var totalChunks int
	var totalSize int64
	var lastUpdated time.Time

	for _, doc := range p.documents {
		totalChunks += doc.ChunkCount
		totalSize += doc.SizeBytes
		if doc.UpdatedAt.After(lastUpdated) {
			lastUpdated = doc.UpdatedAt
		}
	}

	stats := KnowledgeStats{
		TotalDocuments: len(p.documents),
		TotalChunks:    totalChunks,
		TotalSizeBytes: totalSize,
		LastUpdated:    lastUpdated,
	}

	p.writeJSON(w, http.StatusOK, stats)
}

// ═══════════════════════════════════════════════════════════════════════════════
// CONFIG API HANDLERS
// ═══════════════════════════════════════════════════════════════════════════════

// handleConfig handles /api/v1/config routes.
// Supports:
//   - GET /api/v1/config - get current configuration
//   - POST /api/v1/config - update configuration
func (p *Prism) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		p.handleGetConfig(w, r)
	case http.MethodPost:
		p.handleUpdateConfig(w, r)
	default:
		p.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleGetConfig handles GET /api/v1/config - get current configuration.
func (p *Prism) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	p.appConfigMu.RLock()
	defer p.appConfigMu.RUnlock()

	// Return config with API keys masked for security
	config := p.appConfig

	// Mask API keys in response (don't expose actual keys)
	for name, provider := range config.LLM.Providers {
		if provider.APIKey != "" {
			provider.APIKey = "••••••••"
			config.LLM.Providers[name] = provider
		}
	}

	p.writeJSON(w, http.StatusOK, config)
}

// handleUpdateConfig handles POST /api/v1/config - update configuration.
func (p *Prism) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var updates ConfigResponse
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		p.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	p.appConfigMu.Lock()

	// Apply updates (merge with existing config)
	// LLM configuration
	if updates.LLM.DefaultProvider != "" {
		p.appConfig.LLM.DefaultProvider = updates.LLM.DefaultProvider
	}
	if updates.LLM.Providers != nil {
		for name, provider := range updates.LLM.Providers {
			existing := p.appConfig.LLM.Providers[name]
			if provider.Endpoint != "" {
				existing.Endpoint = provider.Endpoint
			}
			if provider.Model != "" {
				existing.Model = provider.Model
			}
			// Don't update APIKey here - use the dedicated endpoint
			p.appConfig.LLM.Providers[name] = existing
		}
	}

	// Knowledge configuration
	if updates.Knowledge.DBPath != "" {
		p.appConfig.Knowledge.DBPath = updates.Knowledge.DBPath
	}
	if updates.Knowledge.DefaultTier != "" {
		p.appConfig.Knowledge.DefaultTier = updates.Knowledge.DefaultTier
	}
	if updates.Knowledge.TrustDecayDays > 0 {
		p.appConfig.Knowledge.TrustDecayDays = updates.Knowledge.TrustDecayDays
	}

	// Sync configuration
	p.appConfig.Sync.Enabled = updates.Sync.Enabled
	if updates.Sync.Endpoint != "" {
		p.appConfig.Sync.Endpoint = updates.Sync.Endpoint
	}
	if updates.Sync.Interval != "" {
		p.appConfig.Sync.Interval = updates.Sync.Interval
	}

	// TUI configuration
	if updates.TUI.Theme != "" {
		p.appConfig.TUI.Theme = updates.TUI.Theme
	}
	p.appConfig.TUI.VimMode = updates.TUI.VimMode
	if updates.TUI.SidebarWidth > 0 {
		p.appConfig.TUI.SidebarWidth = updates.TUI.SidebarWidth
	}

	// Logging configuration
	if updates.Logging.Level != "" {
		p.appConfig.Logging.Level = updates.Logging.Level
	}
	if updates.Logging.File != "" {
		p.appConfig.Logging.File = updates.Logging.File
	}

	// Cognitive configuration
	p.appConfig.Cognitive.Enabled = updates.Cognitive.Enabled
	if updates.Cognitive.OllamaURL != "" {
		p.appConfig.Cognitive.OllamaURL = updates.Cognitive.OllamaURL
	}
	if updates.Cognitive.EmbeddingModel != "" {
		p.appConfig.Cognitive.EmbeddingModel = updates.Cognitive.EmbeddingModel
	}
	if updates.Cognitive.FrontierModel != "" {
		p.appConfig.Cognitive.FrontierModel = updates.Cognitive.FrontierModel
	}
	if updates.Cognitive.SimilarityThresholdHigh > 0 {
		p.appConfig.Cognitive.SimilarityThresholdHigh = updates.Cognitive.SimilarityThresholdHigh
	}
	if updates.Cognitive.SimilarityThresholdMedium > 0 {
		p.appConfig.Cognitive.SimilarityThresholdMedium = updates.Cognitive.SimilarityThresholdMedium
	}
	if updates.Cognitive.SimilarityThresholdLow > 0 {
		p.appConfig.Cognitive.SimilarityThresholdLow = updates.Cognitive.SimilarityThresholdLow
	}
	if updates.Cognitive.ComplexityThreshold > 0 {
		p.appConfig.Cognitive.ComplexityThreshold = updates.Cognitive.ComplexityThreshold
	}

	// Voice configuration
	if updates.Voice != nil {
		if p.appConfig.Voice == nil {
			p.appConfig.Voice = &VoiceConfigAPI{}
		}
		p.appConfig.Voice.Enabled = updates.Voice.Enabled
		p.appConfig.Voice.STTEnabled = updates.Voice.STTEnabled
		p.appConfig.Voice.TTSEnabled = updates.Voice.TTSEnabled
		if updates.Voice.TTSVoice != "" {
			p.appConfig.Voice.TTSVoice = updates.Voice.TTSVoice
		}
		if updates.Voice.TTSRate > 0 {
			p.appConfig.Voice.TTSRate = updates.Voice.TTSRate
		}
		if updates.Voice.TTSPitch > 0 {
			p.appConfig.Voice.TTSPitch = updates.Voice.TTSPitch
		}
		if updates.Voice.Language != "" {
			p.appConfig.Voice.Language = updates.Voice.Language
		}
	}

	p.log.Info("[Prism] Configuration updated")
	p.appConfigMu.Unlock()

	// Return updated config (with masked keys)
	p.handleGetConfig(w, r)
}

// handleProviders handles /api/v1/providers/:id/* routes.
// Supports:
//   - POST /api/v1/providers/:id/key - set API key for provider
//   - GET /api/v1/providers/:id/status - check provider status
func (p *Prism) handleProviders(w http.ResponseWriter, r *http.Request) {
	// Parse path: /api/v1/providers/{provider}/{action}
	path := r.URL.Path
	prefix := "/api/v1/providers/"

	if !strings.HasPrefix(path, prefix) {
		p.writeError(w, http.StatusNotFound, "not found")
		return
	}

	remainder := strings.TrimPrefix(path, prefix)
	parts := strings.Split(remainder, "/")

	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		p.writeError(w, http.StatusNotFound, "provider and action required")
		return
	}

	providerID := parts[0]
	action := parts[1]

	// Validate provider ID
	validProviders := map[string]bool{
		"ollama":    true,
		"openai":    true,
		"anthropic": true,
		"gemini":    true,
		"grok":      true,
	}
	if !validProviders[providerID] {
		p.writeError(w, http.StatusNotFound, "unknown provider")
		return
	}

	switch action {
	case "key":
		p.handleSetProviderKey(w, r, providerID)
	case "status":
		p.handleGetProviderStatus(w, r, providerID)
	default:
		p.writeError(w, http.StatusNotFound, "unknown action")
	}
}

// handleSetProviderKey handles POST /api/v1/providers/:id/key - set API key.
func (p *Prism) handleSetProviderKey(w http.ResponseWriter, r *http.Request, providerID string) {
	if r.Method != http.MethodPost {
		p.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req ProviderKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		p.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.APIKey == "" {
		p.writeError(w, http.StatusBadRequest, "api_key is required")
		return
	}

	// Store the API key
	p.providerKeysMu.Lock()
	p.providerKeys[providerID] = req.APIKey
	p.providerKeysMu.Unlock()

	// Also update the config (with masked key for storage)
	p.appConfigMu.Lock()
	if provider, exists := p.appConfig.LLM.Providers[providerID]; exists {
		provider.APIKey = req.APIKey
		p.appConfig.LLM.Providers[providerID] = provider
	}
	p.appConfigMu.Unlock()

	p.log.Info("[Prism] API key set for provider: %s", providerID)

	w.WriteHeader(http.StatusNoContent)
}

// handleGetProviderStatus handles GET /api/v1/providers/:id/status - get provider status.
func (p *Prism) handleGetProviderStatus(w http.ResponseWriter, r *http.Request, providerID string) {
	if r.Method != http.MethodGet {
		p.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	response := ProviderStatusResponseAPI{
		Online: false,
	}

	// Check if we have an API key for this provider
	p.providerKeysMu.RLock()
	hasKey := p.providerKeys[providerID] != ""
	p.providerKeysMu.RUnlock()

	switch providerID {
	case "ollama":
		// Check Ollama connectivity
		p.appConfigMu.RLock()
		ollamaURL := p.appConfig.Cognitive.OllamaURL
		p.appConfigMu.RUnlock()

		if ollamaURL == "" {
			ollamaURL = "http://127.0.0.1:11434"
		}

		client := http.Client{Timeout: 3 * time.Second}
		resp, err := client.Get(ollamaURL + "/api/tags")
		if err != nil {
			response.Error = "Cannot connect to Ollama"
		} else {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				response.Online = true
				// Parse models from response
				var tagsResp struct {
					Models []struct {
						Name string `json:"name"`
					} `json:"models"`
				}
				if err := json.NewDecoder(resp.Body).Decode(&tagsResp); err == nil {
					for _, m := range tagsResp.Models {
						response.Models = append(response.Models, m.Name)
					}
				}
			} else {
				response.Error = fmt.Sprintf("Ollama returned status %d", resp.StatusCode)
			}
		}

	case "openai":
		if !hasKey {
			response.Error = "API key not configured"
		} else {
			// For cloud providers, assume online if key is set
			// (real connectivity check would hit their API)
			response.Online = true
			response.Models = []string{"gpt-4o", "gpt-4o-mini", "gpt-4-turbo", "gpt-3.5-turbo"}
		}

	case "anthropic":
		if !hasKey {
			response.Error = "API key not configured"
		} else {
			response.Online = true
			response.Models = []string{"claude-3-5-sonnet-20241022", "claude-3-opus-20240229", "claude-3-haiku-20240307"}
		}

	case "gemini":
		if !hasKey {
			response.Error = "API key not configured"
		} else {
			response.Online = true
			response.Models = []string{"gemini-1.5-pro", "gemini-1.5-flash", "gemini-1.0-pro"}
		}

	case "grok":
		if !hasKey {
			response.Error = "API key not configured"
		} else {
			response.Online = true
			response.Models = []string{"grok-3", "grok-3-fast", "grok-2"}
		}
	}

	p.writeJSON(w, http.StatusOK, response)
}

// ═══════════════════════════════════════════════════════════════════════════════
// CHAT API HANDLERS
// ═══════════════════════════════════════════════════════════════════════════════

// handleChat handles POST /api/v1/chat - send a message and get AI response.
func (p *Prism) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		p.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		p.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Message == "" {
		p.writeError(w, http.StatusBadRequest, "message is required")
		return
	}

	// Determine lane (default to "auto")
	lane := req.Lane
	if lane == "" {
		lane = "auto"
	}

	// Determine routing based on lane and message complexity
	routing := p.routeMessage(req.Message, lane)

	now := time.Now()

	// Get or create conversation
	var conv *Conversation
	if req.ConversationID != "" {
		p.conversationsMu.RLock()
		conv = p.conversations[req.ConversationID]
		p.conversationsMu.RUnlock()
	}

	if conv == nil {
		// Create new conversation
		convID := fmt.Sprintf("conv-%s", uuid.New().String())
		conv = &Conversation{
			ID:        convID,
			Title:     truncateTitle(req.Message, 50),
			Messages:  []ChatMessage{},
			CreatedAt: now,
			UpdatedAt: now,
			PersonaID: req.PersonaID,
		}
		p.conversationsMu.Lock()
		p.conversations[conv.ID] = conv
		p.conversationsMu.Unlock()
	}

	// Update behavioral mode based on user message
	var modeInfo *ModeInfo
	var currentMode *facets.BehavioralMode
	var modeTransition *cognitive.ModeTransition

	if req.PersonaID != "" {
		// Use mode tracker to check for transitions
		currentMode, modeTransition = p.modeTracker.UpdateMode(conv.ID, req.PersonaID, req.Message)

		if currentMode != nil {
			modeInfo = &ModeInfo{
				CurrentMode:     currentMode.ID,
				ModeName:        currentMode.Name,
				ModeDescription: currentMode.Description,
				PromptAugment:   currentMode.PromptAugment,
			}

			if modeTransition != nil {
				modeInfo.Transition = &ModeTransitionInfo{
					From:        modeTransition.From,
					To:          modeTransition.To,
					Trigger:     modeTransition.Trigger,
					TriggerType: modeTransition.TriggerType,
				}
			}
		}
	}

	// Create user message
	userMsgID := fmt.Sprintf("msg-%s-user", uuid.New().String())
	userMsg := ChatMessage{
		ID:        userMsgID,
		Role:      "user",
		Content:   req.Message,
		Timestamp: now,
		PersonaID: req.PersonaID,
	}

	// Add user message to conversation first
	p.conversationsMu.Lock()
	conv.Messages = append(conv.Messages, userMsg)
	conv.UpdatedAt = time.Now()
	p.conversationsMu.Unlock()

	// Use cognitive pipeline if enabled, otherwise fall back to mock
	var responseContent string
	var assistantMsg ChatMessage
	assistantMsgID := fmt.Sprintf("msg-%s-assistant", uuid.New().String())

	if p.usePipeline {
		// Build system prompt from active facet/persona
		systemPrompt := p.buildSystemPrompt(currentMode)

		// Process through cognitive pipeline (real LLM)
		pipelineResp, err := p.processChatWithPipeline(r.Context(), &req, conv, systemPrompt)
		if err != nil {
			p.log.Error("[Prism] Pipeline error: %v, falling back to mock", err)
			responseContent = p.generateMockResponse(req.Message, routing, currentMode)
		} else {
			responseContent = pipelineResp.Message.Content
			routing = pipelineResp.Routing
			if pipelineResp.ModeInfo != nil {
				modeInfo = pipelineResp.ModeInfo
			}
		}
	} else {
		// Fall back to mock response
		responseContent = p.generateMockResponse(req.Message, routing, currentMode)
	}

	assistantMsg = ChatMessage{
		ID:        assistantMsgID,
		Role:      "assistant",
		Content:   responseContent,
		Timestamp: time.Now(),
		Model:     routing.Model,
		PersonaID: req.PersonaID,
		Routing:   &routing,
	}

	// Add assistant message to conversation
	p.conversationsMu.Lock()
	conv.Messages = append(conv.Messages, assistantMsg)
	conv.UpdatedAt = time.Now()
	p.conversationsMu.Unlock()

	// Calculate latency
	routing.LatencyMs = time.Since(now).Milliseconds()

	// Check if streaming is requested
	if req.Stream {
		p.streamChatResponse(w, conv.ID, assistantMsg, routing)
		return
	}

	// Non-streaming response
	response := ChatResponse{
		Message:        assistantMsg,
		ConversationID: conv.ID,
		Routing:        routing,
		ModeInfo:       modeInfo,
	}

	p.writeJSON(w, http.StatusOK, response)
}

// streamChatResponse streams the chat response as SSE.
func (p *Prism) streamChatResponse(w http.ResponseWriter, convID string, msg ChatMessage, routing ChatRouting) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		p.writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	// Send start event
	startChunk := ChatStreamChunk{
		Type:           "start",
		MessageID:      msg.ID,
		ConversationID: convID,
		Routing:        &routing,
	}
	startData, _ := json.Marshal(startChunk)
	fmt.Fprintf(w, "data: %s\n\n", startData)
	flusher.Flush()

	// Simulate streaming by sending content in chunks
	words := strings.Fields(msg.Content)
	for i, word := range words {
		chunk := ChatStreamChunk{
			Type:    "delta",
			Content: word,
		}
		if i < len(words)-1 {
			chunk.Content += " "
		}
		chunkData, _ := json.Marshal(chunk)
		fmt.Fprintf(w, "data: %s\n\n", chunkData)
		flusher.Flush()

		// Publish StreamChunkEvent (CR-010)
		if p.eventBus != nil {
			evt := bus.NewStreamChunkEvent(convID, chunk.Content, false)
			evt.ConversationID = convID
			evt.MessageID = msg.ID
			evt.ChunkIndex = i
			p.eventBus.Publish(evt)
		}

		time.Sleep(30 * time.Millisecond) // Simulate typing delay
	}

	// Send end event
	endChunk := ChatStreamChunk{
		Type: "end",
		Usage: &TokenUsage{
			PromptTokens:     len(strings.Fields(msg.Content)) * 2,
			CompletionTokens: len(strings.Fields(msg.Content)),
			TotalTokens:      len(strings.Fields(msg.Content)) * 3,
		},
	}
	endData, _ := json.Marshal(endChunk)
	fmt.Fprintf(w, "data: %s\n\n", endData)
	flusher.Flush()
}

// routeMessage determines which model to use based on the message and lane.
func (p *Prism) routeMessage(message, lane string) ChatRouting {
	// Simple routing logic based on message complexity
	wordCount := len(strings.Fields(message))

	routing := ChatRouting{
		Lane: lane,
	}

	switch lane {
	case "fast":
		routing.Model = "llama3.2:1b"
		routing.Reason = "Fast lane requested - using local small model for quick response"

	case "smart":
		routing.Model = "claude-sonnet-4-20250514"
		routing.Reason = "Smart lane requested - using frontier model for complex reasoning"

	default: // "auto"
		msgLower := strings.ToLower(message)

		// Check for complexity keywords FIRST (before word count)
		hasComplexityKeyword := strings.Contains(msgLower, "explain") ||
			strings.Contains(msgLower, "analyze") ||
			strings.Contains(msgLower, "debug") ||
			strings.Contains(msgLower, "why") ||
			strings.Contains(msgLower, "how should") ||
			strings.Contains(msgLower, "compare") ||
			strings.Contains(msgLower, "tradeoff") ||
			strings.Contains(msgLower, "architecture")

		if hasComplexityKeyword || wordCount > 50 {
			routing.Lane = "smart"
			routing.Model = "claude-sonnet-4-20250514"
			routing.Reason = "Complex query detected - routing to smart lane for detailed analysis"
		} else if wordCount < 10 {
			routing.Lane = "fast"
			routing.Model = "llama3.2:1b"
			routing.Reason = "Short query detected - routing to fast lane for quick response"
		} else {
			routing.Lane = "fast"
			routing.Model = "llama3:8b"
			routing.Reason = "Medium complexity - using balanced local model"
		}
	}

	return routing
}

// buildSystemPrompt builds a system prompt from the active facet and behavioral mode.
func (p *Prism) buildSystemPrompt(mode *facets.BehavioralMode) string {
	var parts []string

	// Get active facet for persona context
	p.facetsMu.RLock()
	var activeFacet *Facet
	for _, f := range p.facets {
		if f.ID == p.activeFacet {
			activeFacet = &f
			break
		}
	}
	p.facetsMu.RUnlock()

	// Base system prompt
	parts = append(parts, "You are Cortex, an intelligent AI assistant.")

	// Add facet/persona context
	if activeFacet != nil && activeFacet.SystemPrompt != "" {
		parts = append(parts, activeFacet.SystemPrompt)
	}

	// Add behavioral mode augment
	if mode != nil && mode.PromptAugment != "" {
		parts = append(parts, mode.PromptAugment)
	}

	return strings.Join(parts, "\n\n")
}

// generateMockResponse generates a mock AI response.
// In production, this would call the actual LLM orchestrator.
func (p *Prism) generateMockResponse(message string, routing ChatRouting, mode *facets.BehavioralMode) string {
	// Get active facet for persona context
	p.facetsMu.RLock()
	var activeFacet *Facet
	for _, f := range p.facets {
		if f.ID == p.activeFacet {
			activeFacet = &f
			break
		}
	}
	p.facetsMu.RUnlock()

	personaContext := ""
	if activeFacet != nil {
		personaContext = fmt.Sprintf(" (as %s)", activeFacet.Name)
	}

	// Add mode context if available
	modeContext := ""
	if mode != nil {
		modeContext = fmt.Sprintf(" in **%s mode**", mode.Name)
	}

	// Generate contextual mock response
	response := fmt.Sprintf("I'm responding%s%s using the **%s** model via the **%s lane**.\n\n", personaContext, modeContext, routing.Model, routing.Lane)

	if strings.Contains(strings.ToLower(message), "hello") || strings.Contains(strings.ToLower(message), "hi") {
		response += "Hello! How can I assist you today?"
	} else if strings.Contains(strings.ToLower(message), "help") {
		response += "I'm here to help! I can assist with:\n\n- **Code generation** and debugging\n- **Data analysis** and visualization\n- **Writing** and documentation\n- **General questions** and research\n\nWhat would you like to explore?"
	} else if strings.Contains(strings.ToLower(message), "code") {
		response += "```go\n// Here's an example function\nfunc example() {\n    fmt.Println(\"Hello from Cortex!\")\n}\n```\n\nI can help you write, debug, or explain code in any language."
	} else {
		response += fmt.Sprintf("Thank you for your message. I've processed your query about: *\"%s\"*\n\n", truncateTitle(message, 100))
		response += "This is a mock response from the Cortex Prism server. In production, I would provide a thoughtful, detailed response using the selected AI model."
	}

	return response
}

// truncateTitle truncates a string to maxLen characters.
func truncateTitle(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// handleConversations handles GET /api/v1/chat/conversations - list conversations.
func (p *Prism) handleConversations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		p.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	p.conversationsMu.RLock()
	defer p.conversationsMu.RUnlock()

	conversations := make([]Conversation, 0, len(p.conversations))
	for _, conv := range p.conversations {
		conversations = append(conversations, *conv)
	}

	// Sort by updated_at descending (most recent first)
	sort.Slice(conversations, func(i, j int) bool {
		return conversations[i].UpdatedAt.After(conversations[j].UpdatedAt)
	})

	response := ConversationsResponse{
		Conversations: conversations,
		Total:         len(conversations),
	}

	p.writeJSON(w, http.StatusOK, response)
}

// handleConversationByID handles /api/v1/chat/conversations/:id routes.
// Supports:
//   - GET /api/v1/chat/conversations/:id - get conversation
//   - DELETE /api/v1/chat/conversations/:id - delete conversation
//   - POST /api/v1/chat/conversations/:id/mode/reset - reset mode to default
//   - POST /api/v1/chat/conversations/:id/mode - set specific mode
func (p *Prism) handleConversationByID(w http.ResponseWriter, r *http.Request) {
	// Parse conversation ID from path
	path := r.URL.Path
	prefix := "/api/v1/chat/conversations/"

	if !strings.HasPrefix(path, prefix) {
		p.writeError(w, http.StatusNotFound, "not found")
		return
	}

	remainder := strings.TrimPrefix(path, prefix)
	parts := strings.Split(remainder, "/")

	if len(parts) == 0 || parts[0] == "" {
		p.writeError(w, http.StatusNotFound, "conversation ID required")
		return
	}

	convID := parts[0]

	// Check for mode-related endpoints
	if len(parts) >= 2 && parts[1] == "mode" {
		if len(parts) == 3 && parts[2] == "reset" {
			// POST /api/v1/chat/conversations/:id/mode/reset
			p.handleModeReset(w, r, convID)
			return
		} else if len(parts) == 2 {
			// POST /api/v1/chat/conversations/:id/mode
			p.handleModeSet(w, r, convID)
			return
		}
		p.writeError(w, http.StatusNotFound, "not found")
		return
	}

	// Handle single conversation operations
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			p.handleGetConversation(w, r, convID)
		case http.MethodDelete:
			p.handleDeleteConversation(w, r, convID)
		default:
			p.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	p.writeError(w, http.StatusNotFound, "not found")
}

// handleGetConversation handles GET /api/v1/chat/conversations/:id.
func (p *Prism) handleGetConversation(w http.ResponseWriter, r *http.Request, convID string) {
	p.conversationsMu.RLock()
	conv, exists := p.conversations[convID]
	p.conversationsMu.RUnlock()

	if !exists {
		p.writeError(w, http.StatusNotFound, "conversation not found")
		return
	}

	p.writeJSON(w, http.StatusOK, conv)
}

// handleDeleteConversation handles DELETE /api/v1/chat/conversations/:id.
func (p *Prism) handleDeleteConversation(w http.ResponseWriter, r *http.Request, convID string) {
	p.conversationsMu.Lock()
	defer p.conversationsMu.Unlock()

	if _, exists := p.conversations[convID]; !exists {
		p.writeError(w, http.StatusNotFound, "conversation not found")
		return
	}

	delete(p.conversations, convID)

	// Also clear mode state
	p.modeTracker.ClearConversation(convID)

	p.log.Info("[Prism] Deleted conversation: %s", convID)
	w.WriteHeader(http.StatusNoContent)
}

// handleModeReset handles POST /api/v1/chat/conversations/:id/mode/reset.
func (p *Prism) handleModeReset(w http.ResponseWriter, r *http.Request, convID string) {
	if r.Method != http.MethodPost {
		p.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Check if conversation exists
	p.conversationsMu.RLock()
	_, exists := p.conversations[convID]
	p.conversationsMu.RUnlock()

	if !exists {
		p.writeError(w, http.StatusNotFound, "conversation not found")
		return
	}

	// Reset mode to default
	transition := p.modeTracker.ResetMode(convID)
	if transition == nil {
		p.writeError(w, http.StatusBadRequest, "mode already at default or conversation not tracked")
		return
	}

	// Get the new mode info
	currentMode := p.modeTracker.GetCurrentMode(convID)
	if currentMode == nil {
		p.writeError(w, http.StatusInternalServerError, "failed to get current mode")
		return
	}

	// Build response
	modeInfo := ModeInfo{
		CurrentMode:     currentMode.ID,
		ModeName:        currentMode.Name,
		ModeDescription: currentMode.Description,
		PromptAugment:   currentMode.PromptAugment,
		Transition: &ModeTransitionInfo{
			From:        transition.From,
			To:          transition.To,
			Trigger:     transition.Trigger,
			TriggerType: transition.TriggerType,
		},
	}

	p.log.Info("[Prism] Reset mode for conversation %s: %s -> %s", convID, transition.From, transition.To)
	p.writeJSON(w, http.StatusOK, modeInfo)
}

// handleModeSet handles POST /api/v1/chat/conversations/:id/mode.
func (p *Prism) handleModeSet(w http.ResponseWriter, r *http.Request, convID string) {
	if r.Method != http.MethodPost {
		p.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req SetModeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		p.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ModeID == "" {
		p.writeError(w, http.StatusBadRequest, "mode_id is required")
		return
	}

	// Check if conversation exists
	p.conversationsMu.RLock()
	_, exists := p.conversations[convID]
	p.conversationsMu.RUnlock()

	if !exists {
		p.writeError(w, http.StatusNotFound, "conversation not found")
		return
	}

	// Set the mode
	transition := p.modeTracker.SetMode(convID, req.ModeID)
	if transition == nil {
		p.writeError(w, http.StatusBadRequest, "invalid mode ID or mode already active")
		return
	}

	// Get the new mode info
	currentMode := p.modeTracker.GetCurrentMode(convID)
	if currentMode == nil {
		p.writeError(w, http.StatusInternalServerError, "failed to get current mode")
		return
	}

	// Build response
	modeInfo := ModeInfo{
		CurrentMode:     currentMode.ID,
		ModeName:        currentMode.Name,
		ModeDescription: currentMode.Description,
		PromptAugment:   currentMode.PromptAugment,
		Transition: &ModeTransitionInfo{
			From:        transition.From,
			To:          transition.To,
			Trigger:     transition.Trigger,
			TriggerType: transition.TriggerType,
		},
	}

	p.log.Info("[Prism] Set mode for conversation %s: %s -> %s", convID, transition.From, transition.To)
	p.writeJSON(w, http.StatusOK, modeInfo)
}

// handleInterrupt handles POST /api/v1/interrupt - trigger cognitive interrupt.
// CR-010 Track 3: Cognitive Interrupt Chain
// This cancels the current LLM generation when the user interrupts (e.g., speaks during AI response).
func (p *Prism) handleInterrupt(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		p.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		Reason    string `json:"reason"`
		SessionID string `json:"session_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		p.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Default reason if not provided
	if req.Reason == "" {
		req.Reason = "manual"
	}

	// Check if orchestrator is available
	if p.orch == nil {
		p.writeError(w, http.StatusServiceUnavailable, "orchestrator not available")
		return
	}

	// Call orchestrator's Interrupt method
	if err := p.orch.Interrupt(req.Reason); err != nil {
		p.log.Error("[Prism] Interrupt failed: %v", err)
		p.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	p.log.Info("[Prism] Interrupt triggered (reason: %s, session: %s)", req.Reason, req.SessionID)

	// Return success response
	p.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "interrupted",
		"reason":  req.Reason,
		"session": req.SessionID,
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// SEARCH HELPER FUNCTIONS
// ═══════════════════════════════════════════════════════════════════════════════

// countMatchedWords counts how many words from queryWords appear in the text.
func countMatchedWords(text string, queryWords []string) int {
	count := 0
	for _, word := range queryWords {
		if strings.Contains(text, word) {
			count++
		}
	}
	return count
}

// buildMatchContext creates a meaningful context snippet showing what matched.
func buildMatchContext(title string, titleMatches, metadataMatches int, metadataText, query string) string {
	if titleMatches > 0 && metadataMatches > 0 {
		return fmt.Sprintf("Found '%s' in both title and metadata: %s", query, title)
	} else if titleMatches > 0 {
		return fmt.Sprintf("Title match: %s", title)
	} else if metadataMatches > 0 {
		return fmt.Sprintf("Metadata match for '%s': %s", query, metadataText)
	}
	return fmt.Sprintf("Content from '%s' matching query '%s'", title, query)
}
