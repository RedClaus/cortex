// Command cortex-plugin provides CLI for CortexBrain plugin management.
//
// Usage:
//
//	cortex-plugin list                  # List installed plugins
//	cortex-plugin search <query>        # Search marketplace
//	cortex-plugin install <name|url>    # Install a plugin
//	cortex-plugin remove <name>         # Remove a plugin
//	cortex-plugin update <name>         # Update a plugin
//	cortex-plugin info <name>           # Show plugin details
//	cortex-plugin marketplace           # Browse marketplace
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"
)

const version = "1.0.0"

var pluginsDir string

func init() {
	// Default plugins directory
	home, _ := os.UserHomeDir()
	pluginsDir = filepath.Join(home, "ServerProjectsMac", "CortexBrain", "plugins")

	// Override from environment
	if dir := os.Getenv("CORTEX_PLUGINS_DIR"); dir != "" {
		pluginsDir = dir
	}
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]

	switch cmd {
	case "list", "ls":
		cmdList()
	case "search", "find":
		if len(os.Args) < 3 {
			fmt.Println("Usage: cortex-plugin search <query>")
			os.Exit(1)
		}
		cmdSearch(os.Args[2])
	case "install", "add":
		if len(os.Args) < 3 {
			fmt.Println("Usage: cortex-plugin install <name|url|path>")
			os.Exit(1)
		}
		cmdInstall(os.Args[2])
	case "remove", "rm", "uninstall":
		if len(os.Args) < 3 {
			fmt.Println("Usage: cortex-plugin remove <name>")
			os.Exit(1)
		}
		cmdRemove(os.Args[2])
	case "update", "upgrade":
		if len(os.Args) < 3 {
			fmt.Println("Usage: cortex-plugin update <name>")
			os.Exit(1)
		}
		cmdUpdate(os.Args[2])
	case "info", "show":
		if len(os.Args) < 3 {
			fmt.Println("Usage: cortex-plugin info <name>")
			os.Exit(1)
		}
		cmdInfo(os.Args[2])
	case "marketplace", "market", "browse":
		cmdMarketplace()
	case "categories", "cats":
		cmdCategories()
	case "featured":
		cmdFeatured()
	case "version", "-v", "--version":
		fmt.Printf("cortex-plugin version %s\n", version)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`CortexBrain Plugin Manager

Usage:
  cortex-plugin <command> [arguments]

Commands:
  list, ls              List installed plugins
  search <query>        Search the marketplace
  install <source>      Install a plugin (name, URL, or path)
  remove <name>         Remove an installed plugin
  update <name>         Update a plugin to latest version
  info <name>           Show plugin details
  marketplace           Browse all marketplace plugins
  categories            List plugin categories
  featured              Show featured plugins
  version               Show version
  help                  Show this help

Sources for install:
  - Plugin name:    cortex-plugin install gateflow
  - GitHub URL:     cortex-plugin install https://github.com/user/plugin
  - Local path:     cortex-plugin install /path/to/plugin

Environment:
  CORTEX_PLUGINS_DIR    Override plugins directory

Examples:
  cortex-plugin search verilog
  cortex-plugin install gateflow
  cortex-plugin list
  cortex-plugin info gateflow`)
}

func cmdList() {
	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		fmt.Printf("Error reading plugins directory: %v\n", err)
		os.Exit(1)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tVERSION\tDESCRIPTION")
	fmt.Fprintln(w, "----\t-------\t-----------")

	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		jsonPath := filepath.Join(pluginsDir, entry.Name(), ".claude-plugin", "plugin.json")
		data, err := os.ReadFile(jsonPath)
		if err != nil {
			continue
		}

		var pj map[string]interface{}
		if err := json.Unmarshal(data, &pj); err != nil {
			continue
		}

		name := getString(pj, "name", entry.Name())
		version := getString(pj, "version", "?")
		desc := truncate(getString(pj, "description", ""), 50)

		fmt.Fprintf(w, "%s\t%s\t%s\n", name, version, desc)
	}
	w.Flush()
}

func cmdSearch(query string) {
	registry := loadMarketplace()
	if registry == nil {
		fmt.Println("Failed to load marketplace")
		os.Exit(1)
	}

	query = strings.ToLower(query)
	found := false

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tVERSION\tCATEGORIES\tDESCRIPTION")
	fmt.Fprintln(w, "----\t-------\t----------\t-----------")

	plugins := registry["plugins"].([]interface{})
	for _, p := range plugins {
		plugin := p.(map[string]interface{})

		// Search in name, description, keywords
		name := getString(plugin, "name", "")
		desc := getString(plugin, "description", "")
		keywords := getStringSlice(plugin, "keywords")

		match := strings.Contains(strings.ToLower(name), query) ||
			strings.Contains(strings.ToLower(desc), query)

		if !match {
			for _, kw := range keywords {
				if strings.Contains(strings.ToLower(kw), query) {
					match = true
					break
				}
			}
		}

		if match {
			found = true
			version := getString(plugin, "version", "?")
			cats := strings.Join(getStringSlice(plugin, "categories"), ", ")
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", name, version, cats, truncate(desc, 40))
		}
	}
	w.Flush()

	if !found {
		fmt.Printf("No plugins found matching '%s'\n", query)
	}
}

func cmdInstall(source string) {
	fmt.Printf("Installing plugin from: %s\n", source)

	var repoURL string
	var destName string

	if strings.HasPrefix(source, "https://") || strings.HasPrefix(source, "git@") {
		// Direct URL
		repoURL = source
		parts := strings.Split(strings.TrimSuffix(source, ".git"), "/")
		destName = parts[len(parts)-1]
	} else if strings.HasPrefix(source, "/") || strings.HasPrefix(source, "./") {
		// Local path
		fmt.Println("Installing from local path...")
		destName = filepath.Base(source)
		destPath := filepath.Join(pluginsDir, destName)

		// Copy
		if err := copyDir(source, destPath); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✓ Installed '%s' from local path\n", destName)
		return
	} else {
		// Marketplace name
		registry := loadMarketplace()
		if registry == nil {
			fmt.Println("Failed to load marketplace")
			os.Exit(1)
		}

		plugins := registry["plugins"].([]interface{})
		for _, p := range plugins {
			plugin := p.(map[string]interface{})
			if getString(plugin, "name", "") == source {
				repoURL = getString(plugin, "repository", "")
				destName = source
				break
			}
		}

		if repoURL == "" {
			fmt.Printf("Plugin '%s' not found in marketplace\n", source)
			os.Exit(1)
		}
	}

	destPath := filepath.Join(pluginsDir, destName)

	// Check if exists
	if _, err := os.Stat(destPath); err == nil {
		fmt.Printf("Plugin '%s' already installed\n", destName)
		os.Exit(1)
	}

	// Clone
	fmt.Printf("Cloning %s...\n", repoURL)
	cmd := fmt.Sprintf("git clone --depth 1 %s %s", repoURL, destPath)
	if err := runCmd(cmd); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	// Validate
	jsonPath := filepath.Join(destPath, ".claude-plugin", "plugin.json")
	if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
		os.RemoveAll(destPath)
		fmt.Println("Error: Invalid plugin (missing .claude-plugin/plugin.json)")
		os.Exit(1)
	}

	fmt.Printf("✓ Installed '%s'\n", destName)
}

func cmdRemove(name string) {
	pluginPath := filepath.Join(pluginsDir, name)

	if _, err := os.Stat(pluginPath); os.IsNotExist(err) {
		fmt.Printf("Plugin '%s' is not installed\n", name)
		os.Exit(1)
	}

	fmt.Printf("Removing plugin '%s'...\n", name)
	if err := os.RemoveAll(pluginPath); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Removed '%s'\n", name)
}

func cmdUpdate(name string) {
	pluginPath := filepath.Join(pluginsDir, name)

	if _, err := os.Stat(pluginPath); os.IsNotExist(err) {
		fmt.Printf("Plugin '%s' is not installed\n", name)
		os.Exit(1)
	}

	fmt.Printf("Updating plugin '%s'...\n", name)
	cmd := fmt.Sprintf("git -C %s pull --rebase", pluginPath)
	if err := runCmd(cmd); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Updated '%s'\n", name)
}

func cmdInfo(name string) {
	// Check installed first
	pluginPath := filepath.Join(pluginsDir, name)
	jsonPath := filepath.Join(pluginPath, ".claude-plugin", "plugin.json")

	data, err := os.ReadFile(jsonPath)
	if err != nil {
		// Try marketplace
		registry := loadMarketplace()
		if registry != nil {
			plugins := registry["plugins"].([]interface{})
			for _, p := range plugins {
				plugin := p.(map[string]interface{})
				if getString(plugin, "name", "") == name {
					printPluginInfo(plugin, false)
					return
				}
			}
		}
		fmt.Printf("Plugin '%s' not found\n", name)
		os.Exit(1)
	}

	var pj map[string]interface{}
	json.Unmarshal(data, &pj)
	printPluginInfo(pj, true)
}

func cmdMarketplace() {
	registry := loadMarketplace()
	if registry == nil {
		fmt.Println("Failed to load marketplace")
		os.Exit(1)
	}

	fmt.Println("CortexBrain Plugin Marketplace")
	fmt.Println("==============================")
	fmt.Println()

	plugins := registry["plugins"].([]interface{})

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tVERSION\tAUTHOR\tCATEGORIES\tDESCRIPTION")
	fmt.Fprintln(w, "----\t-------\t------\t----------\t-----------")

	for _, p := range plugins {
		plugin := p.(map[string]interface{})
		name := getString(plugin, "name", "")
		version := getString(plugin, "version", "?")
		author := getString(plugin, "author", "?")
		cats := strings.Join(getStringSlice(plugin, "categories"), ", ")
		desc := truncate(getString(plugin, "description", ""), 35)

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", name, version, author, cats, desc)
	}
	w.Flush()

	fmt.Printf("\nTotal: %d plugins\n", len(plugins))
	fmt.Println("\nUse 'cortex-plugin install <name>' to install")
}

func cmdCategories() {
	registry := loadMarketplace()
	if registry == nil {
		fmt.Println("Failed to load marketplace")
		os.Exit(1)
	}

	fmt.Println("Plugin Categories")
	fmt.Println("=================")
	fmt.Println()

	cats := registry["categories"].(map[string]interface{})
	for key, val := range cats {
		cat := val.(map[string]interface{})
		fmt.Printf("  %s - %s\n", key, getString(cat, "description", ""))
	}
}

func cmdFeatured() {
	registry := loadMarketplace()
	if registry == nil {
		fmt.Println("Failed to load marketplace")
		os.Exit(1)
	}

	fmt.Println("Featured Plugins")
	fmt.Println("================")
	fmt.Println()

	plugins := registry["plugins"].([]interface{})
	for _, p := range plugins {
		plugin := p.(map[string]interface{})
		if getBool(plugin, "featured") {
			name := getString(plugin, "name", "")
			desc := getString(plugin, "description", "")
			fmt.Printf("★ %s\n  %s\n\n", name, desc)
		}
	}
}

// Helper functions

func loadMarketplace() map[string]interface{} {
	// Try local first
	localPath := filepath.Join(pluginsDir, "marketplace.json")
	data, err := os.ReadFile(localPath)
	if err != nil {
		fmt.Println("Could not load local marketplace.json")
		return nil
	}

	var registry map[string]interface{}
	if err := json.Unmarshal(data, &registry); err != nil {
		fmt.Printf("Invalid marketplace.json: %v\n", err)
		return nil
	}

	return registry
}

func getString(m map[string]interface{}, key, def string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return def
}

func getBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}

func getStringSlice(m map[string]interface{}, key string) []string {
	if v, ok := m[key].([]interface{}); ok {
		var result []string
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func printPluginInfo(plugin map[string]interface{}, installed bool) {
	fmt.Printf("Name:        %s\n", getString(plugin, "name", ""))
	fmt.Printf("Version:     %s\n", getString(plugin, "version", ""))
	fmt.Printf("Description: %s\n", getString(plugin, "description", ""))

	if author, ok := plugin["author"].(map[string]interface{}); ok {
		fmt.Printf("Author:      %s\n", getString(author, "name", ""))
		if gh := getString(author, "github", ""); gh != "" {
			fmt.Printf("GitHub:      %s\n", gh)
		}
	} else if author := getString(plugin, "author", ""); author != "" {
		fmt.Printf("Author:      %s\n", author)
	}

	fmt.Printf("License:     %s\n", getString(plugin, "license", ""))
	fmt.Printf("Repository:  %s\n", getString(plugin, "repository", ""))

	if cats := getStringSlice(plugin, "categories"); len(cats) > 0 {
		fmt.Printf("Categories:  %s\n", strings.Join(cats, ", "))
	}
	if kw := getStringSlice(plugin, "keywords"); len(kw) > 0 {
		fmt.Printf("Keywords:    %s\n", strings.Join(kw, ", "))
	}
	if agents := getStringSlice(plugin, "agents"); len(agents) > 0 {
		fmt.Printf("Agents:      %s\n", strings.Join(agents, ", "))
	}
	if skills := getStringSlice(plugin, "skills"); len(skills) > 0 {
		fmt.Printf("Skills:      %s\n", strings.Join(skills, ", "))
	}
	if commands := getStringSlice(plugin, "commands"); len(commands) > 0 {
		fmt.Printf("Commands:    %s\n", strings.Join(commands, ", "))
	}

	if installed {
		fmt.Printf("Status:      Installed\n")
	} else {
		fmt.Printf("Status:      Not installed\n")
	}
}

func runCmd(cmdStr string) error {
	parts := strings.Fields(cmdStr)
	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func copyDir(src, dst string) error {
	cmd := fmt.Sprintf("cp -r %s %s", src, dst)
	return runCmd(cmd)
}
