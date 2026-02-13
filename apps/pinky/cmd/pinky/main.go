// Pinky - AI Agent Gateway
// "The same thing we do every night, Brain—try to take over the world!"
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/normanking/pinky/internal/agent"
	"github.com/normanking/pinky/internal/brain"
	"github.com/normanking/pinky/internal/config"
	"github.com/normanking/pinky/internal/logging"
	"github.com/normanking/pinky/internal/permissions"
	"github.com/normanking/pinky/internal/tools"
	"github.com/normanking/pinky/internal/tui"
	"github.com/normanking/pinky/internal/webui"
	"github.com/normanking/pinky/internal/wizard"
)

var (
	version   = "1.0.0"
	buildTime = "unknown"
)

func main() {
	// Parse flags
	configPath := flag.String("config", "", "Path to config file")
	showVersion := flag.Bool("version", false, "Show version")
	runWizard := flag.Bool("wizard", false, "Run setup wizard")
	tuiMode := flag.Bool("tui", false, "Start in TUI mode")
	flag.Parse()

	if *showVersion {
		fmt.Printf("Pinky v%s (built %s)\n", version, buildTime)
		os.Exit(0)
	}

	// Load or create configuration first (needed for logging config)
	cfg, err := config.Load(*configPath)
	if err != nil || *runWizard {
		// Use default logger for wizard
		logger := logging.New()
		logger.Info("Running setup wizard...")
		cfg, err = wizard.Run()
		if err != nil {
			logger.Error("Wizard failed", "error", err)
			os.Exit(1)
		}
		logger.Info("Configuration saved successfully")
	}

	// Initialize logging with config
	logger := logging.NewWithConfig(cfg.Logging.Level, cfg.Logging.Format, cfg.Logging.File)
	defer logger.Close()
	logger.Info("Starting Pinky", "version", version)

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		logger.Info("Shutting down...")
		cancel()
	}()

	// Start Pinky
	if *tuiMode {
		if err := runTUI(ctx, cfg, logger); err != nil {
			logger.Error("TUI error", "error", err)
			os.Exit(1)
		}
	} else {
		if err := runServer(ctx, cfg, logger); err != nil {
			logger.Error("Server error", "error", err)
			os.Exit(1)
		}
	}
}

func runTUI(ctx context.Context, cfg *config.Config, logger *logging.Logger) error {
	logger.Info("Starting TUI mode", "brain_mode", cfg.Brain.Mode)

	// 1. Create the Brain using factory (respects config mode)
	brn, err := brain.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create brain: %w", err)
	}
	if err := brn.Ping(ctx); err != nil {
		logger.Warn("Brain ping failed, continuing anyway", "error", err)
	}
	logger.Info("Brain connected", "mode", brn.Mode())

	// 2. Create the Tool Registry with default tools
	registry := tools.NewDefaultRegistry(nil)

	// 3. Create the Permission Service
	permSvc := permissions.NewService(permissions.Tier(cfg.Permissions.DefaultTier))
	if err := permSvc.Load(); err != nil {
		logger.Warn("Failed to load approvals", "error", err)
	}

	// 4. Create the Agent Loop
	loop := agent.New(agent.Config{
		Brain:        brn,
		Tools:        registry,
		Permissions:  permSvc,
		MaxToolCalls: 10,
	})

	// 5. Create TUI
	t := tui.New(tui.Options{
		Config: cfg,
	})

	// 6. Wire up the agent loop with TUI callbacks
	loop.SetApprovalHandler(func(req *permissions.ApprovalRequest) (*permissions.ApprovalResponse, error) {
		return t.RequestApproval(req), nil
	})

	loop.SetToolStartHandler(func(name, command string) {
		t.UpdateToolStatus(name, "running", command)
	})

	loop.SetToolCompleteHandler(func(name string, output *tools.ToolOutput) {
		status := "success"
		if !output.Success {
			status = "failed"
		}
		t.UpdateToolStatus(name, status, output.Output)
	})

	loop.SetResponseHandler(func(content string) {
		t.SendResponse(content)
	})

	// 7. Start message processing goroutine
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg := <-t.Messages():
				if msg == "" {
					continue
				}

				// Handle slash commands
				if handled := handleSlashCommand(msg, brn, t); handled {
					continue
				}

				// Process the message through the agent loop
				resp, err := loop.Process(ctx, &agent.Request{
					UserID:     "tui-user",
					Content:    msg,
					Channel:    "tui",
					WorkingDir: mustGetwd(),
				})

				if err != nil {
					logger.Error("Agent loop error", "error", err)
					t.SendResponse(fmt.Sprintf("Error: %v", err))
					continue
				}

				// Response is already sent via the callback, log the summary
				logger.Debug("Request processed",
					"tools_used", len(resp.ToolsUsed),
					"tokens", resp.TotalTokens,
					"duration", resp.ResponseTime,
				)
			}
		}
	}()

	// 8. Run the TUI
	if err := t.Run(ctx); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	return nil
}

// mustGetwd returns the current working directory or "." on error.
func mustGetwd() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

func runServer(ctx context.Context, cfg *config.Config, logger *logging.Logger) error {
	logger.Info("Server mode starting",
		"api_port", cfg.Server.Port,
		"webui_port", cfg.Server.WebUIPort,
		"brain_mode", cfg.Brain.Mode,
	)

	// Create the Brain using factory (respects config mode)
	brn, err := brain.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create brain: %w", err)
	}
	if err := brn.Ping(ctx); err != nil {
		logger.Warn("Brain ping failed, continuing anyway", "error", err)
	}
	logger.Info("Brain connected", "mode", brn.Mode())

	// Create tool registry and build tool specs for WebUI
	registry := tools.NewDefaultRegistry(nil)
	toolSpecs := buildToolSpecs(registry)

	// Start WebUI server with lane switching (only for EmbeddedBrain)
	webuiServer := webui.New(cfg, logger)
	if ls, ok := brn.(LaneSwitcher); ok {
		webuiServer.SetLaneSwitcher(ls)
	} else {
		logger.Info("Lane switching disabled in remote brain mode")
	}
	webuiServer.SetBrain(brn)
	webuiServer.SetTools(toolSpecs)
	return webuiServer.Start(ctx)
}

// buildToolSpecs converts registered tools to brain.ToolSpec format.
func buildToolSpecs(registry *tools.Registry) []brain.ToolSpec {
	registeredTools := registry.List()
	specs := make([]brain.ToolSpec, 0, len(registeredTools))

	for _, t := range registeredTools {
		spec := brain.ToolSpec{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  make(map[string]brain.ParameterSpec),
		}

		// Add standard parameters based on tool category
		switch t.Category() {
		case tools.CategoryShell:
			spec.Parameters["command"] = brain.ParameterSpec{
				Type:        "string",
				Description: "The shell command to execute",
				Required:    true,
			}
		case tools.CategoryFiles:
			spec.Parameters["path"] = brain.ParameterSpec{
				Type:        "string",
				Description: "The file path",
				Required:    true,
			}
			spec.Parameters["operation"] = brain.ParameterSpec{
				Type:        "string",
				Description: "The operation (read, write, delete, etc.)",
				Required:    true,
			}
		case tools.CategoryWeb:
			spec.Parameters["query"] = brain.ParameterSpec{
				Type:        "string",
				Description: "The search query or URL",
				Required:    true,
			}
		}

		specs = append(specs, spec)
	}

	return specs
}

// LaneSwitcher interface for brain lane management
type LaneSwitcher interface {
	SetLane(name string) error
	GetLane() string
	GetLanes() []brain.LaneInfo
	SetAutoLLM(enabled bool)
	GetAutoLLM() bool
}

// handleSlashCommand processes slash commands and returns true if handled
func handleSlashCommand(msg string, brn brain.Brain, t *tui.TUI) bool {
	// Check if it's a command
	if len(msg) == 0 || msg[0] != '/' {
		return false
	}

	// Get the lane switcher interface
	ls, ok := brn.(LaneSwitcher)
	if !ok {
		t.SendResponse("⚠️ Lane switching not available with current brain mode")
		return true
	}

	cmd := strings.ToLower(strings.TrimSpace(msg))

	switch {
	case cmd == "/fast":
		if err := ls.SetLane("fast"); err != nil {
			t.SendResponse(fmt.Sprintf("❌ %v", err))
		} else {
			lanes := ls.GetLanes()
			var info string
			for _, l := range lanes {
				if l.Name == "fast" {
					info = fmt.Sprintf("%s/%s", l.Engine, l.Model)
					break
				}
			}
			t.SendResponse(fmt.Sprintf("⚡ Switched to **fast** lane (%s)", info))
		}
		return true

	case cmd == "/local":
		if err := ls.SetLane("local"); err != nil {
			t.SendResponse(fmt.Sprintf("❌ %v", err))
		} else {
			lanes := ls.GetLanes()
			var info string
			for _, l := range lanes {
				if l.Name == "local" {
					info = fmt.Sprintf("%s/%s", l.Engine, l.Model)
					break
				}
			}
			t.SendResponse(fmt.Sprintf("🏠 Switched to **local** lane (%s)", info))
		}
		return true

	case cmd == "/smart":
		if err := ls.SetLane("smart"); err != nil {
			t.SendResponse(fmt.Sprintf("❌ %v", err))
		} else {
			lanes := ls.GetLanes()
			var info string
			for _, l := range lanes {
				if l.Name == "smart" {
					info = fmt.Sprintf("%s/%s", l.Engine, l.Model)
					break
				}
			}
			t.SendResponse(fmt.Sprintf("🧠 Switched to **smart** lane (%s)", info))
		}
		return true

	case cmd == "/auto" || cmd == "/auto on":
		ls.SetAutoLLM(true)
		t.SendResponse("🔀 **AutoLLM** enabled - lane will be selected based on task complexity")
		return true

	case cmd == "/auto off":
		ls.SetAutoLLM(false)
		current := ls.GetLane()
		t.SendResponse(fmt.Sprintf("🔀 **AutoLLM** disabled - using **%s** lane", current))
		return true

	case cmd == "/lanes" || cmd == "/status":
		lanes := ls.GetLanes()
		autoLLM := ls.GetAutoLLM()

		var sb strings.Builder
		sb.WriteString("**Inference Lanes**\n\n")

		for _, l := range lanes {
			marker := "  "
			if l.Active {
				marker = "▶ "
			}
			sb.WriteString(fmt.Sprintf("%s**%s** - %s/%s\n", marker, l.Name, l.Engine, l.Model))
		}

		sb.WriteString("\n")
		if autoLLM {
			sb.WriteString("AutoLLM: ✅ ON (auto-selects lane by complexity)\n")
		} else {
			sb.WriteString(fmt.Sprintf("AutoLLM: ❌ OFF (using **%s**)\n", ls.GetLane()))
		}

		sb.WriteString("\n**Commands:** /fast, /local, /smart, /auto [on|off], /lanes")
		t.SendResponse(sb.String())
		return true

	case cmd == "/keys":
		lanes := ls.GetLanes()
		var sb strings.Builder
		sb.WriteString("**API Keys Status**\n\n")

		for _, l := range lanes {
			if l.Engine == "ollama" {
				sb.WriteString(fmt.Sprintf("  **%s** (%s) - No key needed\n", l.Name, l.Engine))
			} else {
				// Check if key is set via config
				keySet := checkAPIKeySet(l.Name, brn)
				if keySet {
					sb.WriteString(fmt.Sprintf("  **%s** (%s) - ✅ Key set\n", l.Name, l.Engine))
				} else {
					sb.WriteString(fmt.Sprintf("  **%s** (%s) - ❌ Key not set\n", l.Name, l.Engine))
				}
			}
		}

		sb.WriteString("\n**Set a key:** /setkey <lane> <api-key>")
		sb.WriteString("\n**Example:** /setkey fast gsk_xxx...")
		sb.WriteString("\n\n💡 Or run `pinky --wizard` for guided setup")
		t.SendResponse(sb.String())
		return true

	case strings.HasPrefix(cmd, "/setkey "):
		parts := strings.SplitN(msg, " ", 3)
		if len(parts) < 3 {
			t.SendResponse("❌ Usage: /setkey <lane> <api-key>\n\nExample: /setkey fast gsk_xxx...")
			return true
		}
		lane := strings.ToLower(parts[1])
		apiKey := parts[2]

		// Validate lane exists
		validLanes := []string{"fast", "smart", "openai"}
		valid := false
		for _, vl := range validLanes {
			if lane == vl {
				valid = true
				break
			}
		}
		if !valid {
			t.SendResponse(fmt.Sprintf("❌ Unknown lane: %s\n\nValid lanes: fast, smart, openai", lane))
			return true
		}

		// Set the API key via config
		if err := setAPIKeyInConfig(lane, apiKey); err != nil {
			t.SendResponse(fmt.Sprintf("❌ Failed to save key: %v", err))
			return true
		}

		t.SendResponse(fmt.Sprintf("✅ API key for **%s** lane saved to config", lane))
		return true

	case cmd == "/settings" || cmd == "/config":
		// Show the settings panel
		t.ShowSettings()
		t.SendResponse("⚙️ **Settings panel opened**\nUse arrow keys to navigate, Enter to select, Esc to close")
		return true

	case cmd == "/help" || cmd == "/":
		help := `**Pinky Commands**

**Lane Switching:**
  /fast   - Use fast lane (Groq)
  /local  - Use local lane (Ollama)
  /smart  - Use smart lane (Claude)
  /lanes  - Show all lanes and status

**Settings:**
  /settings - Open inference settings panel

**AutoLLM:**
  /auto      - Enable automatic lane selection
  /auto off  - Disable AutoLLM

**API Keys:**
  /keys            - Show API key status
  /setkey <lane> <key> - Set API key for a lane

**Other:**
  /help   - Show this help

💡 Run 'pinky --wizard' for full setup wizard`
		t.SendResponse(help)
		return true

	default:
		// Unrecognized slash command - show error instead of passing to brain
		t.SendResponse(fmt.Sprintf("❓ Unknown command: %s\nType /help for available commands.", cmd))
		return true
	}

	return false
}

// checkAPIKeySet checks if an API key is configured for a lane
func checkAPIKeySet(lane string, brn brain.Brain) bool {
	// Try to get the config from the brain's inference config
	eb, ok := brn.(*brain.EmbeddedBrain)
	if !ok {
		return false
	}

	// Get lanes info which includes whether key is set
	lanes := eb.GetLanes()
	for _, l := range lanes {
		if l.Name == lane && l.Engine != "ollama" {
			// For cloud providers, check if the lane has a valid key
			// by checking the config
			cfg, err := config.Load("")
			if err != nil {
				return false
			}
			if laneCfg, ok := cfg.Inference.Lanes[lane]; ok {
				key := laneCfg.APIKey
				// If key is an env var reference, expand it
				if len(key) > 3 && key[0] == '$' && key[1] == '{' && key[len(key)-1] == '}' {
					envVar := key[2 : len(key)-1]
					key = os.Getenv(envVar)
				}
				return key != ""
			}
		}
	}
	return false
}

// setAPIKeyInConfig sets an API key in the config file
func setAPIKeyInConfig(lane, apiKey string) error {
	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if laneCfg, ok := cfg.Inference.Lanes[lane]; ok {
		laneCfg.APIKey = apiKey
		cfg.Inference.Lanes[lane] = laneCfg
	} else {
		return fmt.Errorf("lane %s not found in config", lane)
	}

	if err := cfg.Save(""); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	return nil
}
