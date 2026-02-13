package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"log/slog"

	"github.com/cortexhub/cortex-gateway/internal/brain"
	"github.com/cortexhub/cortex-gateway/internal/bridge"
	"github.com/cortexhub/cortex-gateway/internal/bus"
	"github.com/cortexhub/cortex-gateway/internal/channel"
	"github.com/cortexhub/cortex-gateway/internal/channel/discord"
	"github.com/cortexhub/cortex-gateway/internal/channel/telegram"
	"github.com/cortexhub/cortex-gateway/internal/channel/webchat"
	"github.com/cortexhub/cortex-gateway/internal/config"
	"github.com/cortexhub/cortex-gateway/internal/discovery"
	"github.com/cortexhub/cortex-gateway/internal/healthring"
	"github.com/cortexhub/cortex-gateway/internal/inference"
	"github.com/cortexhub/cortex-gateway/internal/logging"
	"github.com/cortexhub/cortex-gateway/internal/onboarding"
	"github.com/cortexhub/cortex-gateway/internal/scheduler"
	"github.com/cortexhub/cortex-gateway/internal/server"
)

const (
	configPath = "config.yaml"
	version    = "1.0.0"
)

func main() {
	// Parse CLI flags
	onboardFlag := flag.Bool("onboard", false, "Launch interactive onboarding wizard")
	swarmFlag := flag.Bool("swarm", false, "Auto-discover existing swarm and generate config")
	flag.Parse()

	logger := logging.WithComponent("main")

	logger.Info("Starting Cortex-Gateway", "version", version)

	// Handle --swarm mode
	if *swarmFlag {
		logger.Info("Swarm discovery mode")
		o := onboarding.New(logger, configPath)
		if err := o.SwarmDiscover(); err != nil {
			logger.Error("Swarm discovery failed", "error", err)
			os.Exit(1)
		}
		cfg, err := config.Load(configPath)
		if err != nil {
			logger.Error("Failed to load config after swarm discovery", "error", err)
			os.Exit(1)
		}
		logger.Info("Swarm config generated, starting gateway")
		_ = cfg
		// Fall through to normal startup below by reloading
	}

	// Handle --onboard mode
	if *onboardFlag {
		logger.Info("Onboarding mode")
		o := onboarding.New(logger, configPath)
		if err := o.CLI(); err != nil {
			logger.Error("Onboarding failed", "error", err)
			os.Exit(1)
		}
		logger.Info("Configuration created successfully")
		fmt.Println("\n✅ Config written to config.yaml. Starting gateway...")
	}

	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		logger.Error("Failed to load config", "error", err)
		// No flags and no config = auto-onboard
		if !*onboardFlag && !*swarmFlag {
			logger.Info("No config found, launching onboarding wizard...")
			o := onboarding.New(logger, configPath)
			if err := o.CLI(); err != nil {
				logger.Error("Onboarding failed", "error", err)
				os.Exit(1)
			}
			cfg, err = config.Load(configPath)
			if err != nil {
				logger.Error("Failed to load new config", "error", err)
				os.Exit(1)
			}
			logger.Info("Configuration created and loaded successfully")
		} else {
			logger.Error("Config still missing after setup, exiting")
			os.Exit(1)
		}
	} else {
		logger.Info("Configuration loaded successfully")
	}

	if err := cfg.Validate(); err != nil {
		logger.Error("Invalid config", "error", err)
		os.Exit(1)
	}

	logger.Info("Server listening on", "host", cfg.Server.Host, "port", cfg.Server.Port)

	ctx := context.Background()

	// Initialize discovery
	disc := discovery.NewDiscovery(cfg.Swarm, logger)

	// Initialize brain client
	brainClient := brain.NewClient(&cfg.CortexBrain)

	// Initialize inference router
	inferenceRouter, err := inference.NewRouter(ctx, cfg)
	if err != nil {
		logger.Error("Failed to create inference router", "error", err)
		os.Exit(1)
	}

	// Check inference lane health
	healthResults := inferenceRouter.Health()
	logger.Info("Inference engine health")
	for name, err := range healthResults {
		if err != nil {
			logger.Error("Inference engine error", "engine", name, "error", err)
		} else {
			logger.Info("Inference engine OK", "engine", name)
		}
	}

	// Initialize bridge
	bridgeClient := bridge.NewClient(disc, &cfg.Bridge)

	// Initialize health ring
	healthRing := healthring.NewHealthRing(cfg.HealthRing, disc, logger)

	// Initialize scheduler
	sched := scheduler.NewScheduler(brainClient)
	sched.Start()
	logger.Info("Scheduler started")

	// Start bridge
	if err := bridgeClient.Start(ctx); err != nil {
		logger.Error("Failed to start bridge", "error", err)
	}
	logger.Info("Bridge started")

	// Initialize bus (non-blocking - runs in background)
	var busClient *bus.Client
	go func() {
		var err error
		busClient, err = bus.NewClient(cfg.Bridge.URL)
		if err != nil {
			logger.Error("Failed to create bus client", "error", err)
			busClient = nil
		} else {
			logger.Info("Bus client connected")
		}
	}()

	// Initialize channels
	adapters := []channel.ChannelAdapter{}
	if cfg.Channels.Telegram.Enabled {
		tg := telegram.NewTelegramAdapter(cfg.Channels.Telegram.Token)
		adapters = append(adapters, tg)
		logger.Info("Telegram adapter initialized")
	}
	if cfg.Channels.Discord.Enabled {
		dc := discord.NewDiscordAdapter(cfg.Channels.Discord.Token)
		adapters = append(adapters, dc)
		logger.Info("Discord adapter initialized")
	}
	if cfg.Channels.WebChat.Enabled {
		wc := webchat.NewWebChatAdapter(cfg.Channels.WebChat.Port)
		adapters = append(adapters, wc)
		logger.Info("WebChat adapter initialized")
	}

	// Start adapters
	for _, adapter := range adapters {
		if err := adapter.Start(ctx); err != nil {
			logger.Error("Failed to start adapter", "adapter", adapter.Name(), "error", err)
		} else {
			logger.Info("Adapter started", "adapter", adapter.Name())
		}
	}

	o := onboarding.New(logger, configPath)

	// Create HTTP server
	srv := server.New(cfg, brainClient, inferenceRouter, bridgeClient, disc, healthRing, o, logger)

	// Start server in goroutine
	go func() {
		if err := srv.Start(); err != nil && err != http.ErrServerClosed {
			logger.Error("Server error", "error", err)
		}
	}()

	// Test connections
	testConnections(cfg, logger, disc, inferenceRouter)

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server")

	// Graceful shutdown with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	logger.Info("Stopping adapters")
	for _, adapter := range adapters {
		if err := adapter.Stop(); err != nil {
			logger.Error("Failed to stop adapter", "adapter", adapter.Name(), "error", err)
		} else {
			logger.Info("Adapter stopped", "adapter", adapter.Name())
		}
	}

	logger.Info("Stopping bridge")
	if err := bridgeClient.Stop(); err != nil {
		logger.Error("Failed to stop bridge", "error", err)
	}

	logger.Info("Stopping bus")
	if busClient != nil {
		if err := busClient.Close(); err != nil {
			logger.Error("Failed to close bus", "error", err)
		}
	}

	logger.Info("Stopping scheduler")
	sched.Stop()

	logger.Info("Stopping discovery")
	disc.Shutdown()

	if healthRing != nil {
		logger.Info("Stopping health ring")
		healthRing.Shutdown()
	}

	logger.Info("Stopping HTTP server")
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("Server shutdown error", "error", err)
	}

	logger.Info("Shutdown complete")
}

func testConnections(cfg *config.Config, logger *slog.Logger, disc *discovery.Discovery, infRouter *inference.Router) {
	client := &http.Client{Timeout: 10 * time.Second}
	// Test CortexBrain
	logger.Info("Testing CortexBrain connection")
	resp, err := client.Get(cfg.CortexBrain.URL + "/health")
	if err != nil {
		logger.Error("CortexBrain connection failed", "error", err)
	} else {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			logger.Info("CortexBrain connection OK")
		} else {
			logger.Warn("CortexBrain connection failed", "status", resp.StatusCode)
		}
	}

	// Test swarm
	if disc != nil {
		logger.Info("Testing swarm discovery")
		agents := disc.ListAgents()
		for _, agent := range agents {
			logger.Info("Agent status", "name", agent.Name, "ip", agent.IP, "status", agent.Status)
		}
	}

	// Inference health already checked at startup
}
