// CortexAvatar - The Face, Eyes, and Ears of CortexBrain
package main

import (
	"bufio"
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/normanking/cortexavatar/internal/a2a"
	"github.com/normanking/cortexavatar/internal/avatar"
	"github.com/normanking/cortexavatar/internal/bridge"
	"github.com/normanking/cortexavatar/internal/bus"
	"github.com/normanking/cortexavatar/internal/config"
	"github.com/normanking/cortexavatar/internal/discovery"
	"github.com/normanking/cortexavatar/internal/logging"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

//go:embed all:frontend/dist
var assets embed.FS

// Global logger instance
var syslog *logging.Logger

// getAssets returns the frontend assets with the correct path
func getAssets() fs.FS {
	syslog.Debug("assets", "Getting assets from embedded filesystem", nil)
	fsys, err := fs.Sub(assets, "frontend/dist")
	if err != nil {
		syslog.Error("assets", "Failed to get assets", err, nil)
		panic(err)
	}

	// List files to verify
	entries, _ := fs.ReadDir(fsys, ".")
	syslog.Debug("assets", "Assets loaded", map[string]interface{}{
		"fileCount": len(entries),
	})

	return fsys
}

// loadEnvFile loads API keys from .env files into process environment.
// Checks both ~/.cortex/.env (shared with CortexBrain) and ~/.cortexavatar/.env
func loadEnvFile() {
	home, err := os.UserHomeDir()
	if err != nil {
		syslog.Warn("env", "Could not get home directory", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	// Load from ~/.cortex/.env first (shared with CortexBrain)
	envPaths := []string{
		filepath.Join(home, ".cortex", ".env"),
		filepath.Join(home, ".cortexavatar", ".env"),
	}

	for _, envPath := range envPaths {
		file, err := os.Open(envPath)
		if err != nil {
			continue // File doesn't exist, skip
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		loadedCount := 0
		loadedKeys := []string{}
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			// Remove quotes if present
			value = strings.Trim(value, "\"'")

			// Only set if not already in environment
			if os.Getenv(key) == "" {
				os.Setenv(key, value)
				loadedCount++
				loadedKeys = append(loadedKeys, key)
			}
		}
		if loadedCount > 0 {
			syslog.Info("env", "Loaded environment variables", map[string]interface{}{
				"source": filepath.Base(envPath),
				"count":  loadedCount,
				"keys":   strings.Join(loadedKeys, ", "),
			})
		}
	}
}

func main() {
	// Initialize structured logger FIRST
	var err error
	syslog, err = logging.New(nil) // Uses default config
	if err != nil {
		// Fallback to standard log if logger fails
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer syslog.Close()

	syslog.Info("main", "========================================", nil)
	syslog.Info("main", "CortexAvatar starting...", nil)
	syslog.Info("main", "========================================", nil)

	// Load API keys from .env files
	loadEnvFile()

	// Get zerolog instance for components that need it
	zlogger := syslog.Zerolog()

	// Load configuration
	syslog.Debug("config", "Loading configuration", nil)
	cfg, err := config.Load()
	if err != nil {
		syslog.Warn("config", "Failed to load config, using defaults", map[string]interface{}{
			"error": err.Error(),
		})
		cfg = config.DefaultConfig()
	}
	syslog.Info("config", "Configuration loaded", map[string]interface{}{
		"windowSize": fmt.Sprintf("%dx%d", cfg.Window.Width, cfg.Window.Height),
		"a2aServer":  cfg.A2A.ServerURL,
	})

	// Create event bus
	syslog.Debug("bus", "Creating event bus", nil)
	eventBus := bus.NewEventBus()

	// Create A2A client
	syslog.Debug("a2a", "Creating A2A client", nil)
	a2aConfig := &a2a.ClientConfig{
		ServerURL:      cfg.A2A.ServerURL,
		Timeout:        cfg.A2A.Timeout,
		ReconnectDelay: cfg.A2A.ReconnectDelay,
		MaxReconnects:  cfg.A2A.MaxReconnects,
		UserID:         cfg.User.ID,
		PersonaID:      cfg.User.PersonaID,
	}
	a2aClient := a2a.NewClient(a2aConfig, zlogger)

	// Create brain discovery service
	syslog.Debug("discovery", "Creating brain discovery service", nil)
	discoveryService := discovery.NewService(nil) // Uses default config

	// Create avatar controller
	syslog.Debug("avatar", "Creating avatar controller", nil)
	avatarController := avatar.NewController()
	avatarController.Start()

	// Create bridges
	syslog.Debug("bridge", "Creating Wails bridges", nil)
	avatarBridge := bridge.NewAvatarBridge(avatarController, eventBus)
	audioBridge := bridge.NewAudioBridge(a2aClient, avatarController, eventBus, cfg, zlogger)
	connectionBridge := bridge.NewConnectionBridge(a2aClient, eventBus, cfg, avatarController, zlogger)
	settingsBridge := bridge.NewSettingsBridge(cfg, zlogger)
	logBridge := bridge.NewLogBridge(syslog)
	brainBridge := bridge.NewBrainBridge(discoveryService, a2aClient, cfg, syslog)

	// Create application
	syslog.Debug("main", "Creating App struct", nil)
	app := &App{
		cfg:              cfg,
		syslog:           syslog,
		eventBus:         eventBus,
		a2aClient:        a2aClient,
		discoveryService: discoveryService,
		avatarController: avatarController,
		avatarBridge:     avatarBridge,
		audioBridge:      audioBridge,
		connectionBridge: connectionBridge,
		settingsBridge:   settingsBridge,
		logBridge:        logBridge,
		brainBridge:      brainBridge,
	}

	// Get assets
	syslog.Debug("assets", "Preparing assets", nil)
	assetFS := getAssets()

	// Create Wails application options
	syslog.Debug("wails", "Configuring Wails options", nil)
	appOptions := &options.App{
		Title:     cfg.Window.Title,
		Width:     cfg.Window.Width,
		Height:    cfg.Window.Height,
		MinWidth:  300,
		MinHeight: 400,
		AssetServer: &assetserver.Options{
			Assets: assetFS,
		},
		BackgroundColour: &options.RGBA{R: 26, G: 26, B: 46, A: 255},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
			avatarBridge,
			audioBridge,
			connectionBridge,
			settingsBridge,
			logBridge,
			brainBridge,
		},
		Mac: &mac.Options{
			TitleBar: &mac.TitleBar{
				TitlebarAppearsTransparent: true,
				HideTitle:                  false,
				HideTitleBar:               false,
				FullSizeContent:            true,
				UseToolbar:                 false,
			},
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
			About: &mac.AboutInfo{
				Title:   "CortexAvatar",
				Message: "The Face, Eyes, and Ears of CortexBrain\nVersion 1.0.0",
			},
			// Enable media capture in WebView for microphone/camera access
			Preferences: &mac.Preferences{
				TabFocusesLinks:        mac.Enabled,
				TextInteractionEnabled: mac.Enabled,
				FullscreenEnabled:      mac.Enabled,
			},
		},
	}

	// Run Wails
	syslog.Info("wails", "========================================", nil)
	syslog.Info("wails", "Starting Wails application...", nil)
	syslog.Info("wails", "========================================", nil)

	err = wails.Run(appOptions)
	if err != nil {
		syslog.Error("wails", "Wails.Run failed", err, nil)
		os.Exit(1)
	}

	syslog.Info("main", "Application exited normally", nil)
}

// App struct holds the main application state
type App struct {
	ctx              context.Context
	cfg              *config.Config
	syslog           *logging.Logger
	eventBus         *bus.EventBus
	a2aClient        *a2a.Client
	discoveryService *discovery.Service
	avatarController *avatar.Controller
	avatarBridge     *bridge.AvatarBridge
	audioBridge      *bridge.AudioBridge
	connectionBridge *bridge.ConnectionBridge
	settingsBridge   *bridge.SettingsBridge
	logBridge        *bridge.LogBridge
	brainBridge      *bridge.BrainBridge
}

// startup is called when the app starts
func (a *App) startup(ctx context.Context) {
	a.syslog.Debug("lifecycle", "App.startup() called", nil)
	a.ctx = ctx

	// Bind bridges to context
	a.syslog.Debug("lifecycle", "Binding bridges to context", nil)
	a.avatarBridge.Bind(ctx)
	a.audioBridge.Bind(ctx)
	a.connectionBridge.Bind(ctx)
	a.settingsBridge.Bind(ctx)
	a.logBridge.Bind(ctx)
	a.brainBridge.Bind(ctx)

	// Connect to CortexBrain
	a.syslog.Info("a2a", "Starting A2A connection in background", nil)
	go func() {
		if err := a.a2aClient.Connect(context.Background()); err != nil {
			a.syslog.Error("a2a", "Failed to connect to CortexBrain", err, nil)
		} else {
			a.syslog.Info("a2a", "Connected to CortexBrain", nil)
		}
	}()

	a.syslog.Info("lifecycle", "App.startup() complete", nil)
}

// shutdown is called when the app is closing
func (a *App) shutdown(ctx context.Context) {
	a.syslog.Info("lifecycle", "App.shutdown() called", nil)
	a.avatarController.Stop()
	a.brainBridge.Shutdown()
	a.a2aClient.Close()
	a.syslog.Info("lifecycle", "CortexAvatar shutdown complete", nil)
}

// GetVersion returns the application version
func (a *App) GetVersion() string {
	return "1.0.0"
}

// GetConfig returns the current configuration
func (a *App) GetConfig() *config.Config {
	return a.cfg
}

// Greet returns a greeting message (for testing)
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, I am CortexAvatar!", name)
}
