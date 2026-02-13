package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/RedClaus/cortex-coder-agent/internal/version"
	"github.com/RedClaus/cortex-coder-agent/pkg/config"
	"github.com/RedClaus/cortex-coder-agent/pkg/cortexbrain"
	"github.com/RedClaus/cortex-coder-agent/pkg/session"
	"github.com/RedClaus/cortex-coder-agent/pkg/tui"
	"github.com/spf13/cobra"
)

var (
	cfgFile string
	mode    string
	verbose bool
)

var rootCmd = &cobra.Command{
	Use:   "coder",
	Short: "Cortex Coder Agent - AI-powered coding assistant",
	Long: `Cortex Coder Agent is an AI-powered coding assistant with skills, 
extensions, and a beautiful TUI interface.

Features:
  • Skill-based code generation and modification
  • Interactive TUI with BubbleTea
  • Extension system for custom tools
  • JSON-RPC API for integrations
  • CortexBrain integration for knowledge recall

Configuration:
  The agent looks for configuration in:
  1. --config flag (explicit path)
  2. $HOME/.config/cortex-coder/config.yaml
  3. ./config.yaml (current directory)

Environment Variables:
  CORTEXBRAIN_URL     - CortexBrain API URL
  CORTEXBRAIN_WSURL   - CortexBrain WebSocket URL
  CORTEXBRAIN_TOKEN   - Authentication token
  CORTEX_AGENT_MODE   - Default mode (interactive, json, rpc)`,
	Version: version.Version,
	RunE:    runRoot,
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/cortex-coder/config.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	// Local flags
	rootCmd.Flags().StringVar(&mode, "mode", "", "operation mode: interactive, json, rpc (overrides config)")

	// Add subcommands
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(healthCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(sessionCmd)
	rootCmd.AddCommand(tuiCmd)

	// Config subcommands
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configValidateCmd)

	// Session subcommands
	sessionCmd.AddCommand(sessionListCmd)
	sessionCmd.AddCommand(sessionLoadCmd)
	sessionCmd.AddCommand(sessionDeleteCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runRoot(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Override mode if specified via CLI flag
	if mode != "" {
		cfg.Agent.Mode = mode
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "Configuration loaded from: %s\n", config.GetConfigPath())
		fmt.Fprintf(os.Stderr, "Mode: %s\n", cfg.Agent.Mode)
		fmt.Fprintf(os.Stderr, "CortexBrain URL: %s\n", cfg.CortexBrain.URL)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Execute based on mode
	switch cfg.Agent.Mode {
	case "interactive":
		return runInteractiveMode(cfg)
	case "json":
		return runJSONMode(cfg)
	case "rpc":
		return runRPCMode(cfg)
	default:
		return fmt.Errorf("unknown mode: %s", cfg.Agent.Mode)
	}
}

func runInteractiveMode(cfg *config.Config) error {
	// Get project path (current directory or specified)
	projectPath, _ := os.Getwd()
	
	// Parse save interval
	saveInterval := 30 * time.Second
	if cfg.Session.SaveInterval != "" {
		if d, err := time.ParseDuration(cfg.Session.SaveInterval); err == nil {
			saveInterval = d
		}
	}
	
	// Create app config
	appConfig := tui.AppConfig{
		Theme:        tui.Theme(cfg.TUI.Theme),
		RootPath:     projectPath,
		SessionID:    generateSessionID(),
		SessionName:  filepath.Base(projectPath),
		AutoSave:     cfg.Session.AutoSave,
		SaveInterval: saveInterval,
	}
	
	// Run the TUI
	return tui.RunApp(appConfig)
}

func runJSONMode(cfg *config.Config) error {
	// Read JSON request from stdin
	decoder := json.NewDecoder(os.Stdin)
	
	var request map[string]interface{}
	if err := decoder.Decode(&request); err != nil {
		return fmt.Errorf("failed to parse JSON input: %w", err)
	}

	// Create CortexBrain client
	client := cortexbrain.NewClient(
		cfg.CortexBrain.URL,
		cfg.CortexBrain.WSURL,
		cfg.CortexBrain.Token,
	)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	if err := client.Ping(ctx); err != nil {
		response := map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("CortexBrain connection failed: %v", err),
		}
		return json.NewEncoder(os.Stdout).Encode(response)
	}

	// Process the request
	response := map[string]interface{}{
		"success":             true,
		"mode":                "json",
		"request":             request,
		"cortexbrain_connected": true,
		"timestamp":           time.Now().Format(time.RFC3339),
		"message":             "JSON mode active (full implementation in Phase 2)",
	}

	return json.NewEncoder(os.Stdout).Encode(response)
}

func runRPCMode(cfg *config.Config) error {
	fmt.Println("Starting Cortex Coder Agent in RPC mode...")
	fmt.Println("JSON-RPC server would start here (Phase 4)")
	fmt.Println()
	fmt.Println("Press Ctrl+C to exit")
	
	// Placeholder - RPC server will be implemented in Phase 4
	time.Sleep(1 * time.Second)
	return nil
}

// ============== TUI Command ==============

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch the TUI interface",
	Long:  `Launch the interactive TUI interface for the Cortex Coder Agent.`,
	RunE:  runTUICmd,
}

func runTUICmd(cmd *cobra.Command, args []string) error {
	// Get project path
	projectPath := "."
	if len(args) > 0 {
		projectPath = args[0]
	}
	
	// Convert to absolute path
	if !filepath.IsAbs(projectPath) {
		cwd, _ := os.Getwd()
		projectPath = filepath.Join(cwd, projectPath)
	}
	
	// Load config for settings
	cfg, _ := config.Load(cfgFile)
	if cfg == nil {
		cfg = config.DefaultConfig()
	}
	
	// Parse save interval
	saveInterval := 30 * time.Second
	if cfg.Session.SaveInterval != "" {
		if d, err := time.ParseDuration(cfg.Session.SaveInterval); err == nil {
			saveInterval = d
		}
	}
	
	// Create app config
	appConfig := tui.AppConfig{
		Theme:        tui.Theme(cfg.TUI.Theme),
		RootPath:     projectPath,
		SessionID:    generateSessionID(),
		SessionName:  filepath.Base(projectPath),
		AutoSave:     cfg.Session.AutoSave,
		SaveInterval: saveInterval,
	}
	
	// Run the TUI
	return tui.RunApp(appConfig)
}

// ============== Session Commands ==============

var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Manage sessions",
	Long:  `List, load, and manage coding sessions.`,
}

var sessionListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all sessions",
	Long:  `List all saved coding sessions.`,
	RunE:  runSessionList,
}

var sessionLoadCmd = &cobra.Command{
	Use:   "load <name>",
	Short: "Load a session",
	Long:  `Load a previously saved session and launch the TUI.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runSessionLoad,
}

var sessionDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a session",
	Long:  `Delete a saved session.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runSessionDelete,
}

func runSessionList(cmd *cobra.Command, args []string) error {
	manager, err := session.NewManager()
	if err != nil {
		return fmt.Errorf("failed to create session manager: %w", err)
	}
	
	sessions, err := manager.ListSessions()
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}
	
	if len(sessions) == 0 {
		fmt.Println("No saved sessions found.")
		fmt.Println("Start the TUI to create a session.")
		return nil
	}
	
	fmt.Println("Saved Sessions:")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Printf("%-8s %-20s %-30s %8s %8s\n", "ID", "Name", "Project", "Files", "Messages")
	fmt.Println(strings.Repeat("-", 70))
	
	for _, s := range sessions {
		fmt.Printf("%-8s %-20s %-30s %8d %8d\n",
			s.ID[:8],
			s.Name,
			truncatePath(s.ProjectPath, 30),
			s.FileCount,
			s.MessageCount,
		)
	}
	
	return nil
}

func runSessionLoad(cmd *cobra.Command, args []string) error {
	sessionName := args[0]
	
	manager, err := session.NewManager()
	if err != nil {
		return fmt.Errorf("failed to create session manager: %w", err)
	}
	
	// Find session by name
	sessions, err := manager.ListSessions()
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}
	
	var targetID string
	for _, s := range sessions {
		if s.Name == sessionName || s.ID == sessionName || s.ID[:8] == sessionName {
			targetID = s.ID
			break
		}
	}
	
	if targetID == "" {
		return fmt.Errorf("session not found: %s", sessionName)
	}
	
	// Load session
	sess, err := manager.LoadSession(targetID)
	if err != nil {
		return fmt.Errorf("failed to load session: %w", err)
	}
	
	fmt.Printf("Loaded session: %s\n", sess.Name)
	fmt.Printf("Project: %s\n", sess.ProjectPath)
	fmt.Printf("Files: %d\n", len(sess.Files))
	fmt.Printf("Messages: %d\n", len(sess.Messages))
	fmt.Println()
	
	// Load config for settings
	cfg, _ := config.Load(cfgFile)
	if cfg == nil {
		cfg = config.DefaultConfig()
	}
	
	// Parse save interval
	saveInterval := 30 * time.Second
	if cfg.Session.SaveInterval != "" {
		if d, err := time.ParseDuration(cfg.Session.SaveInterval); err == nil {
			saveInterval = d
		}
	}
	
	// Create app config
	appConfig := tui.AppConfig{
		Theme:        tui.Theme(cfg.TUI.Theme),
		RootPath:     sess.ProjectPath,
		SessionID:    sess.ID,
		SessionName:  sess.Name,
		AutoSave:     cfg.Session.AutoSave,
		SaveInterval: saveInterval,
	}
	
	// Run the TUI
	return tui.RunApp(appConfig)
}

func runSessionDelete(cmd *cobra.Command, args []string) error {
	sessionName := args[0]
	
	manager, err := session.NewManager()
	if err != nil {
		return fmt.Errorf("failed to create session manager: %w", err)
	}
	
	// Find session by name
	sessions, err := manager.ListSessions()
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}
	
	var targetID string
	for _, s := range sessions {
		if s.Name == sessionName || s.ID == sessionName || s.ID[:8] == sessionName {
			targetID = s.ID
			break
		}
	}
	
	if targetID == "" {
		return fmt.Errorf("session not found: %s", sessionName)
	}
	
	// Delete session
	if err := manager.DeleteSession(targetID); err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}
	
	fmt.Printf("Session '%s' deleted successfully.\n", sessionName)
	return nil
}

// ============== Config Command ==============

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
	Long:  `Initialize, view, and validate the Cortex Coder Agent configuration.`,
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new configuration file",
	Long: `Creates a new configuration file at ~/.config/cortex-coder/config.yaml
with default values. If the file already exists, it will not be overwritten
unless --force is specified.`,
	RunE: runConfigInit,
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Display current configuration",
	Long:  `Loads and displays the current configuration from file and environment variables.`,
	RunE:  runConfigShow,
}

var configValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate configuration",
	Long:  `Validates the current configuration and reports any errors.`,
	RunE:  runConfigValidate,
}

func runConfigInit(cmd *cobra.Command, args []string) error {
	force, _ := cmd.Flags().GetBool("force")

	configPath := config.GetConfigPath()
	
	if config.ConfigFileExists() && !force {
		return fmt.Errorf("configuration file already exists at %s (use --force to overwrite)", configPath)
	}

	cfg := config.DefaultConfig()
	
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	fmt.Printf("Configuration file created at: %s\n", configPath)
	fmt.Println("\nYou can now edit this file to customize your settings.")
	fmt.Println("Environment variables can also be used to override specific values:")
	fmt.Println("  CORTEXBRAIN_URL   - CortexBrain API URL")
	fmt.Println("  CORTEXBRAIN_TOKEN - Authentication token")
	
	return nil
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Display in YAML-like format
	fmt.Println("# Current Configuration")
	fmt.Printf("# Source: %s\n", config.GetConfigPath())
	fmt.Println()
	
	fmt.Println("agent:")
	fmt.Printf("  name: %s\n", cfg.Agent.Name)
	fmt.Printf("  mode: %s\n", cfg.Agent.Mode)
	
	fmt.Println()
	fmt.Println("cortexbrain:")
	fmt.Printf("  url: %s\n", cfg.CortexBrain.URL)
	fmt.Printf("  ws_url: %s\n", cfg.CortexBrain.WSURL)
	if cfg.CortexBrain.Token != "" {
		fmt.Println("  token: ******** (set)")
	} else {
		fmt.Println("  token: (not set)")
	}
	
	fmt.Println()
	fmt.Println("tui:")
	fmt.Printf("  theme: %s\n", cfg.TUI.Theme)
	fmt.Printf("  show_line_numbers: %t\n", cfg.TUI.ShowLineNumbers)
	fmt.Printf("  tab_size: %d\n", cfg.TUI.TabSize)
	
	fmt.Println()
	fmt.Println("skills:")
	fmt.Printf("  auto_load: %t\n", cfg.Skills.AutoLoad)
	fmt.Println("  directories:")
	for _, dir := range cfg.Skills.Directories {
		fmt.Printf("    - %s\n", dir)
	}
	
	fmt.Println()
	fmt.Println("session:")
	fmt.Printf("  auto_save: %t\n", cfg.Session.AutoSave)
	fmt.Printf("  save_interval: %s\n", cfg.Session.SaveInterval)

	return nil
}

func runConfigValidate(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		fmt.Printf("❌ Configuration is invalid: %v\n", err)
		return err
	}

	fmt.Println("✅ Configuration is valid")
	
	// Additional validation - try to connect to CortexBrain
	fmt.Println("\nTesting CortexBrain connection...")
	client := cortexbrain.NewClient(
		cfg.CortexBrain.URL,
		cfg.CortexBrain.WSURL,
		cfg.CortexBrain.Token,
	)
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	health, err := client.HealthCheck(ctx)
	if err != nil {
		fmt.Printf("⚠️  CortexBrain connection failed: %v\n", err)
		fmt.Println("   The agent will still work but some features may be unavailable.")
	} else {
		fmt.Printf("✅ CortexBrain connection successful (version: %s, status: %s)\n", 
			health.Version, health.Status)
	}

	return nil
}

// ============== Health Command ==============

var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Check CortexBrain health",
	Long:  `Performs a health check against the configured CortexBrain instance.`,
	RunE:  runHealth,
}

func runHealth(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	client := cortexbrain.NewClient(
		cfg.CortexBrain.URL,
		cfg.CortexBrain.WSURL,
		cfg.CortexBrain.Token,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	health, err := client.HealthCheck(ctx)
	if err != nil {
		fmt.Printf("❌ CortexBrain is unhealthy: %v\n", err)
		fmt.Printf("   URL: %s\n", cfg.CortexBrain.URL)
		os.Exit(1)
	}

	fmt.Println("✅ CortexBrain is healthy")
	fmt.Printf("   URL:      %s\n", cfg.CortexBrain.URL)
	fmt.Printf("   Version:  %s\n", health.Version)
	fmt.Printf("   Status:   %s\n", health.Status)
	fmt.Printf("   Uptime:   %s\n", health.Uptime)
	fmt.Printf("   Time:     %s\n", health.Timestamp.Format(time.RFC3339))

	return nil
}

// ============== Version Command ==============

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Long:  `Display detailed version information about the Cortex Coder Agent.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Cortex Coder Agent\n")
		fmt.Printf("  Version:    %s\n", version.Version)
		fmt.Printf("  Build Time: %s\n", version.BuildTime)
		fmt.Printf("  Git Commit: %s\n", version.GitCommit)
	},
}

func init() {
	configInitCmd.Flags().Bool("force", false, "Overwrite existing configuration file")
}

// Helper functions

func generateSessionID() string {
	// Generate a simple session ID based on timestamp
	return fmt.Sprintf("session_%d", time.Now().UnixNano())
}

func truncatePath(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}
	return "..." + path[len(path)-maxLen+3:]
}
