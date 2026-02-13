// Package manager provides plugin management for CortexBrain.
// It handles plugin discovery, installation, updates, and removal.
package manager

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// MarketplaceRegistry represents the plugin marketplace.
type MarketplaceRegistry struct {
	Version     string                    `json:"version"`
	Updated     string                    `json:"updated"`
	Description string                    `json:"description"`
	RegistryURL string                    `json:"registry_url"`
	Categories  map[string]Category       `json:"categories"`
	Plugins     []MarketplacePlugin       `json:"plugins"`
	Featured    []string                  `json:"featured"`
	Sources     []RegistrySource          `json:"sources"`
}

// Category represents a plugin category.
type Category struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
}

// MarketplacePlugin represents a plugin in the marketplace.
type MarketplacePlugin struct {
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Description string   `json:"description"`
	Author      string   `json:"author"`
	Repository  string   `json:"repository"`
	Categories  []string `json:"categories"`
	Keywords    []string `json:"keywords"`
	License     string   `json:"license"`
	Stars       int      `json:"stars"`
	Downloads   int      `json:"downloads"`
	Verified    bool     `json:"verified"`
	Featured    bool     `json:"featured"`
	Agents      []string `json:"agents"`
	Skills      []string `json:"skills"`
	Commands    []string `json:"commands"`
}

// RegistrySource represents a marketplace source.
type RegistrySource struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Priority int    `json:"priority"`
}

// PluginJSON represents the plugin.json file.
type PluginJSON struct {
	Name        string      `json:"name"`
	Version     string      `json:"version"`
	Description string      `json:"description"`
	Author      AuthorInfo  `json:"author"`
	Keywords    []string    `json:"keywords"`
	License     string      `json:"license"`
	Repository  string      `json:"repository"`
	CortexBrain *CortexMeta `json:"cortexbrain,omitempty"`
}

// AuthorInfo represents plugin author information.
type AuthorInfo struct {
	Name   string `json:"name"`
	GitHub string `json:"github,omitempty"`
	Email  string `json:"email,omitempty"`
}

// CortexMeta represents CortexBrain-specific metadata.
type CortexMeta struct {
	MinVersion   string   `json:"minVersion,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
	Triggers     []string `json:"triggers,omitempty"`
	Dependencies []string `json:"dependencies,omitempty"`
	Tools        []string `json:"tools,omitempty"`
	Lobes        []string `json:"lobes,omitempty"`
}

// InstalledPlugin represents a locally installed plugin.
type InstalledPlugin struct {
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	Path        string    `json:"path"`
	InstalledAt time.Time `json:"installed_at"`
	Source      string    `json:"source"` // "marketplace", "github", "local"
}

// Manager handles plugin operations.
type Manager struct {
	pluginsDir   string
	marketplaceURL string
	registry     *MarketplaceRegistry
	installed    map[string]*InstalledPlugin
}

// NewManager creates a new plugin manager.
func NewManager(pluginsDir string) *Manager {
	return &Manager{
		pluginsDir:   pluginsDir,
		marketplaceURL: "https://raw.githubusercontent.com/RedClaus/CortexBrain/main/plugins/marketplace.json",
		installed:    make(map[string]*InstalledPlugin),
	}
}

// LoadMarketplace fetches the marketplace registry.
func (m *Manager) LoadMarketplace() error {
	// Try local first
	localPath := filepath.Join(m.pluginsDir, "marketplace.json")
	if data, err := os.ReadFile(localPath); err == nil {
		return json.Unmarshal(data, &m.registry)
	}

	// Fetch from remote
	resp, err := http.Get(m.marketplaceURL)
	if err != nil {
		return fmt.Errorf("failed to fetch marketplace: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read marketplace: %w", err)
	}

	return json.Unmarshal(data, &m.registry)
}

// LoadInstalled scans the plugins directory for installed plugins.
func (m *Manager) LoadInstalled() error {
	entries, err := os.ReadDir(m.pluginsDir)
	if err != nil {
		return fmt.Errorf("failed to read plugins dir: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pluginPath := filepath.Join(m.pluginsDir, entry.Name())
		jsonPath := filepath.Join(pluginPath, ".claude-plugin", "plugin.json")

		data, err := os.ReadFile(jsonPath)
		if err != nil {
			continue // Not a valid plugin
		}

		var pj PluginJSON
		if err := json.Unmarshal(data, &pj); err != nil {
			continue
		}

		m.installed[pj.Name] = &InstalledPlugin{
			Name:        pj.Name,
			Version:     pj.Version,
			Path:        pluginPath,
			InstalledAt: time.Now(), // Would need to store this
			Source:      "unknown",
		}
	}

	return nil
}

// Search finds plugins matching a query.
func (m *Manager) Search(query string) []MarketplacePlugin {
	query = strings.ToLower(query)
	var results []MarketplacePlugin

	for _, p := range m.registry.Plugins {
		// Check name
		if strings.Contains(strings.ToLower(p.Name), query) {
			results = append(results, p)
			continue
		}

		// Check description
		if strings.Contains(strings.ToLower(p.Description), query) {
			results = append(results, p)
			continue
		}

		// Check keywords
		for _, kw := range p.Keywords {
			if strings.Contains(strings.ToLower(kw), query) {
				results = append(results, p)
				break
			}
		}

		// Check categories
		for _, cat := range p.Categories {
			if strings.Contains(strings.ToLower(cat), query) {
				results = append(results, p)
				break
			}
		}
	}

	return results
}

// Install installs a plugin from marketplace, GitHub, or local path.
func (m *Manager) Install(source string) error {
	// Determine source type
	if strings.HasPrefix(source, "https://github.com") || strings.HasPrefix(source, "git@github.com") {
		return m.installFromGitHub(source)
	} else if strings.HasPrefix(source, "/") || strings.HasPrefix(source, "./") {
		return m.installFromLocal(source)
	} else {
		return m.installFromMarketplace(source)
	}
}

// installFromMarketplace installs a plugin by name from the marketplace.
func (m *Manager) installFromMarketplace(name string) error {
	var plugin *MarketplacePlugin
	for _, p := range m.registry.Plugins {
		if p.Name == name {
			plugin = &p
			break
		}
	}

	if plugin == nil {
		return fmt.Errorf("plugin '%s' not found in marketplace", name)
	}

	return m.installFromGitHub(plugin.Repository)
}

// installFromGitHub clones a plugin from GitHub.
func (m *Manager) installFromGitHub(repoURL string) error {
	// Extract repo name
	parts := strings.Split(strings.TrimSuffix(repoURL, ".git"), "/")
	repoName := parts[len(parts)-1]

	destPath := filepath.Join(m.pluginsDir, repoName)

	// Check if already exists
	if _, err := os.Stat(destPath); err == nil {
		return fmt.Errorf("plugin '%s' already installed at %s", repoName, destPath)
	}

	// Clone the repository
	cmd := exec.Command("git", "clone", "--depth", "1", repoURL, destPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to clone plugin: %w", err)
	}

	// Validate plugin structure
	if err := m.validatePlugin(destPath); err != nil {
		os.RemoveAll(destPath)
		return fmt.Errorf("invalid plugin structure: %w", err)
	}

	return nil
}

// installFromLocal copies a plugin from a local path.
func (m *Manager) installFromLocal(srcPath string) error {
	// Get plugin name from path
	name := filepath.Base(srcPath)
	destPath := filepath.Join(m.pluginsDir, name)

	// Validate source
	if err := m.validatePlugin(srcPath); err != nil {
		return fmt.Errorf("invalid plugin at source: %w", err)
	}

	// Copy to plugins directory
	cmd := exec.Command("cp", "-r", srcPath, destPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to copy plugin: %w", err)
	}

	return nil
}

// validatePlugin checks if a directory is a valid plugin.
func (m *Manager) validatePlugin(pluginPath string) error {
	// Check for plugin.json
	jsonPath := filepath.Join(pluginPath, ".claude-plugin", "plugin.json")
	if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
		return fmt.Errorf("missing .claude-plugin/plugin.json")
	}

	// Validate JSON
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return fmt.Errorf("failed to read plugin.json: %w", err)
	}

	var pj PluginJSON
	if err := json.Unmarshal(data, &pj); err != nil {
		return fmt.Errorf("invalid plugin.json: %w", err)
	}

	// Check required fields
	if pj.Name == "" {
		return fmt.Errorf("plugin.json missing 'name' field")
	}
	if pj.Version == "" {
		return fmt.Errorf("plugin.json missing 'version' field")
	}

	return nil
}

// Remove uninstalls a plugin.
func (m *Manager) Remove(name string) error {
	plugin, ok := m.installed[name]
	if !ok {
		return fmt.Errorf("plugin '%s' is not installed", name)
	}

	if err := os.RemoveAll(plugin.Path); err != nil {
		return fmt.Errorf("failed to remove plugin: %w", err)
	}

	delete(m.installed, name)
	return nil
}

// Update updates a plugin to the latest version.
func (m *Manager) Update(name string) error {
	plugin, ok := m.installed[name]
	if !ok {
		return fmt.Errorf("plugin '%s' is not installed", name)
	}

	// Git pull
	cmd := exec.Command("git", "-C", plugin.Path, "pull", "--rebase")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// List returns all installed plugins.
func (m *Manager) List() []*InstalledPlugin {
	var plugins []*InstalledPlugin
	for _, p := range m.installed {
		plugins = append(plugins, p)
	}
	return plugins
}

// ListMarketplace returns all marketplace plugins.
func (m *Manager) ListMarketplace() []MarketplacePlugin {
	if m.registry == nil {
		return nil
	}
	return m.registry.Plugins
}

// GetCategories returns available categories.
func (m *Manager) GetCategories() map[string]Category {
	if m.registry == nil {
		return nil
	}
	return m.registry.Categories
}

// ByCategory returns plugins in a category.
func (m *Manager) ByCategory(category string) []MarketplacePlugin {
	var results []MarketplacePlugin
	for _, p := range m.registry.Plugins {
		for _, cat := range p.Categories {
			if cat == category {
				results = append(results, p)
				break
			}
		}
	}
	return results
}

// Featured returns featured plugins.
func (m *Manager) Featured() []MarketplacePlugin {
	var results []MarketplacePlugin
	for _, p := range m.registry.Plugins {
		if p.Featured {
			results = append(results, p)
		}
	}
	return results
}
