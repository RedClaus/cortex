// Package main provides the entry point for Salamander TUI.
//
// Salamander is a YAML-driven terminal user interface that connects to
// A2A-compliant AI agents. The interface is fully configurable through
// YAML files, allowing users to customize menus, themes, keybindings,
// and more without writing code.
//
// Usage:
//
//	salamander                      # Use default config
//	salamander --config myapp.yaml  # Use custom config
//	salamander --url http://...     # Connect to specific A2A agent
//	salamander --builder            # Open YAML Builder UI
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/normanking/salamander/internal/app"
	"github.com/normanking/salamander/internal/builder"
	"github.com/normanking/salamander/internal/config"
	"github.com/normanking/salamander/pkg/schema"
)

var (
	configPath  = flag.String("config", "", "Path to YAML configuration file")
	agentURL    = flag.String("url", "", "A2A agent URL (overrides config)")
	builderMode = flag.Bool("builder", false, "Open the YAML Builder UI")
	version     = flag.Bool("version", false, "Show version information")
)

const (
	appVersion = "0.1.0"
	appName    = "Salamander"
)

func main() {
	flag.Parse()

	if *version {
		fmt.Printf("%s v%s\n", appName, appVersion)
		fmt.Println("A YAML-driven TUI for A2A agents")
		os.Exit(0)
	}

	// Find config file
	configFile := findConfig(*configPath)

	// Builder mode
	if *builderMode {
		if err := builder.Run(configFile); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Load configuration
	cfg, err := loadOrCreateConfig(configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Override backend URL if provided via flag
	if *agentURL != "" {
		cfg.Backend.URL = *agentURL
		cfg.Backend.Type = "a2a"
	}

	// Run the TUI application
	if err := app.Run(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// loadOrCreateConfig loads configuration from file or returns defaults
func loadOrCreateConfig(path string) (*schema.Config, error) {
	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Use default config
		return config.DefaultConfig(), nil
	}

	// Load from file
	cfg, err := config.LoadConfig(path)
	if err != nil {
		return nil, err
	}

	// Validate
	if err := config.ValidateConfig(cfg); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// findConfig locates the configuration file
func findConfig(explicit string) string {
	if explicit != "" {
		return explicit
	}

	// Check standard locations
	locations := []string{
		"salamander.yaml",
		"salamander.yml",
		filepath.Join(os.Getenv("HOME"), ".config", "salamander", "config.yaml"),
		filepath.Join(os.Getenv("HOME"), ".salamander.yaml"),
	}

	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			return loc
		}
	}

	// Default to current directory
	return "salamander.yaml"
}
