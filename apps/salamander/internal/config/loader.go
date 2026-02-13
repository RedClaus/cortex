// Package config provides YAML configuration loading and validation for Salamander TUI.
//
// This package handles parsing YAML configuration files, validating their contents,
// and providing sensible defaults for optional fields. It supports loading from
// files or byte slices, and can merge configurations for layered defaults.
package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/normanking/salamander/pkg/schema"
	"gopkg.in/yaml.v3"
)

// Valid layout types for Salamander TUI.
var validLayoutTypes = map[string]bool{
	"chat":      true,
	"dashboard": true,
	"split":     true,
	"custom":    true,
}

// Valid backend types for Salamander TUI.
var validBackendTypes = map[string]bool{
	"a2a":    true,
	"direct": true,
	"mock":   true,
}

// validKeyPattern matches valid keybinding patterns.
// Supports modifiers (ctrl, alt, shift, meta) combined with keys.
var validKeyPattern = regexp.MustCompile(`^(ctrl\+|alt\+|shift\+|meta\+)*(([a-z0-9])|f[1-9]|f1[0-2]|space|enter|tab|backspace|delete|escape|up|down|left|right|home|end|pageup|pagedown)$`)

// LoadConfig loads and parses a YAML configuration file from the given path.
// It returns the parsed configuration or an error if the file cannot be read
// or contains invalid YAML.
func LoadConfig(path string) (*schema.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	cfg, err := LoadConfigFromBytes(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", path, err)
	}

	return cfg, nil
}

// LoadConfigFromBytes parses YAML configuration from a byte slice.
// It returns the parsed configuration or an error if the YAML is invalid.
func LoadConfigFromBytes(data []byte) (*schema.Config, error) {
	var cfg schema.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}

	return &cfg, nil
}

// ValidateConfig validates a configuration for required fields and valid values.
// It checks:
//   - Required fields: Version, App.Name, Layout.Type, Backend.Type
//   - Valid Layout.Type values: "chat", "dashboard", "split", "custom"
//   - Valid Backend.Type values: "a2a", "direct", "mock"
//   - Menu items have unique IDs
//   - Keybindings have valid key patterns
func ValidateConfig(cfg *schema.Config) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}

	// Validate required fields
	if cfg.Version == "" {
		return fmt.Errorf("version is required")
	}

	if cfg.App.Name == "" {
		return fmt.Errorf("app.name is required")
	}

	if cfg.Layout.Type == "" {
		return fmt.Errorf("layout.type is required")
	}

	if cfg.Backend.Type == "" {
		return fmt.Errorf("backend.type is required")
	}

	// Validate layout type
	if !validLayoutTypes[cfg.Layout.Type] {
		return fmt.Errorf("invalid layout.type %q: must be one of chat, dashboard, split, custom", cfg.Layout.Type)
	}

	// Validate backend type
	if !validBackendTypes[cfg.Backend.Type] {
		return fmt.Errorf("invalid backend.type %q: must be one of a2a, direct, mock", cfg.Backend.Type)
	}

	// Validate menu item IDs are unique
	if err := validateMenuIDs(cfg.Menus); err != nil {
		return err
	}

	// Validate keybindings
	if err := validateKeybindings(cfg.Keybindings); err != nil {
		return err
	}

	return nil
}

// validateMenuIDs checks that all menu and menu item IDs are unique.
func validateMenuIDs(menus []schema.MenuConfig) error {
	seenMenuIDs := make(map[string]bool)
	seenItemIDs := make(map[string]bool)

	for _, menu := range menus {
		if menu.ID != "" {
			if seenMenuIDs[menu.ID] {
				return fmt.Errorf("duplicate menu ID: %q", menu.ID)
			}
			seenMenuIDs[menu.ID] = true
		}

		if err := validateMenuItems(menu.Items, seenItemIDs); err != nil {
			return err
		}
	}

	return nil
}

// validateMenuItems recursively validates menu item IDs, including submenus.
func validateMenuItems(items []schema.MenuItemConfig, seenIDs map[string]bool) error {
	for _, item := range items {
		if item.ID != "" {
			if seenIDs[item.ID] {
				return fmt.Errorf("duplicate menu item ID: %q", item.ID)
			}
			seenIDs[item.ID] = true
		}

		// Check submenu items recursively
		if item.Submenu != nil {
			if err := validateMenuItems(item.Submenu.Items, seenIDs); err != nil {
				return err
			}
		}
	}

	return nil
}

// validateKeybindings checks that all keybindings have valid key patterns.
func validateKeybindings(keybindings []schema.KeybindingConfig) error {
	for _, kb := range keybindings {
		if kb.Key == "" {
			return fmt.Errorf("keybinding has empty key")
		}

		key := strings.ToLower(kb.Key)
		if !validKeyPattern.MatchString(key) {
			return fmt.Errorf("invalid keybinding key %q: must be a valid key combination (e.g., ctrl+c, alt+enter, f1)", kb.Key)
		}
	}

	return nil
}

// MergeConfig merges two configurations, with override values taking precedence.
// Non-zero values in override replace values in base. Slices are replaced entirely,
// not appended. Returns a new Config without modifying the inputs.
func MergeConfig(base, override *schema.Config) *schema.Config {
	if base == nil {
		if override == nil {
			return nil
		}
		return copyConfig(override)
	}

	if override == nil {
		return copyConfig(base)
	}

	result := copyConfig(base)

	// Merge scalar fields (override if non-empty)
	if override.Version != "" {
		result.Version = override.Version
	}

	// Merge App
	result.App = mergeAppConfig(result.App, override.App)

	// Merge Theme
	result.Theme = mergeThemeConfig(result.Theme, override.Theme)

	// Merge Layout
	result.Layout = mergeLayoutConfig(result.Layout, override.Layout)

	// Replace Menus entirely if override has menus
	if len(override.Menus) > 0 {
		result.Menus = override.Menus
	}

	// Replace Keybindings entirely if override has keybindings
	if len(override.Keybindings) > 0 {
		result.Keybindings = override.Keybindings
	}

	// Merge Backend
	result.Backend = mergeBackendConfig(result.Backend, override.Backend)

	// Replace Extensions entirely if override has extensions
	if len(override.Extensions) > 0 {
		result.Extensions = override.Extensions
	}

	return result
}

// mergeAppConfig merges two AppConfig structs.
func mergeAppConfig(base, override schema.AppConfig) schema.AppConfig {
	result := base

	if override.Name != "" {
		result.Name = override.Name
	}
	if override.Description != "" {
		result.Description = override.Description
	}
	if override.Version != "" {
		result.Version = override.Version
	}
	if override.Author != "" {
		result.Author = override.Author
	}
	if override.WelcomeMessage != "" {
		result.WelcomeMessage = override.WelcomeMessage
	}
	// For bools, we always take the override value
	result.ShowStatusBar = override.ShowStatusBar || base.ShowStatusBar
	result.ShowTitleBar = override.ShowTitleBar || base.ShowTitleBar

	return result
}

// mergeThemeConfig merges two ThemeConfig structs.
func mergeThemeConfig(base, override schema.ThemeConfig) schema.ThemeConfig {
	result := base

	if override.Name != "" {
		result.Name = override.Name
	}
	if override.Mode != "" {
		result.Mode = override.Mode
	}

	// Merge colors
	result.Colors = mergeColorPalette(result.Colors, override.Colors)

	return result
}

// mergeColorPalette merges two ColorPalette structs.
func mergeColorPalette(base, override schema.ColorPalette) schema.ColorPalette {
	result := base

	if override.Primary != "" {
		result.Primary = override.Primary
	}
	if override.Secondary != "" {
		result.Secondary = override.Secondary
	}
	if override.Background != "" {
		result.Background = override.Background
	}
	if override.Surface != "" {
		result.Surface = override.Surface
	}
	if override.Text != "" {
		result.Text = override.Text
	}
	if override.TextMuted != "" {
		result.TextMuted = override.TextMuted
	}
	if override.Border != "" {
		result.Border = override.Border
	}
	if override.Error != "" {
		result.Error = override.Error
	}
	if override.Success != "" {
		result.Success = override.Success
	}
	if override.Warning != "" {
		result.Warning = override.Warning
	}
	if override.Info != "" {
		result.Info = override.Info
	}
	if override.Accent != "" {
		result.Accent = override.Accent
	}

	return result
}

// mergeLayoutConfig merges two LayoutConfig structs.
func mergeLayoutConfig(base, override schema.LayoutConfig) schema.LayoutConfig {
	result := base

	if override.Type != "" {
		result.Type = override.Type
	}
	if len(override.Components) > 0 {
		result.Components = override.Components
	}

	return result
}

// mergeBackendConfig merges two BackendConfig structs.
func mergeBackendConfig(base, override schema.BackendConfig) schema.BackendConfig {
	result := base

	if override.Type != "" {
		result.Type = override.Type
	}
	if override.URL != "" {
		result.URL = override.URL
	}
	if override.AuthToken != "" {
		result.AuthToken = override.AuthToken
	}
	if override.AuthScheme != "" {
		result.AuthScheme = override.AuthScheme
	}
	if override.Timeout > 0 {
		result.Timeout = override.Timeout
	}
	if override.RetryCount > 0 {
		result.RetryCount = override.RetryCount
	}
	// For streaming, take override if true
	result.Streaming = override.Streaming || base.Streaming

	return result
}

// copyConfig creates a shallow copy of a Config.
// Note: Slices reference the same underlying arrays.
func copyConfig(cfg *schema.Config) *schema.Config {
	if cfg == nil {
		return nil
	}

	result := *cfg
	return &result
}

// DefaultConfig returns a configuration with sensible defaults.
// This can be used as a base configuration that users can override.
func DefaultConfig() *schema.Config {
	return &schema.Config{
		Version: "1.0",
		App: schema.AppConfig{
			Name:           "Salamander App",
			Description:    "A YAML-driven TUI application",
			ShowStatusBar:  true,
			ShowTitleBar:   true,
			WelcomeMessage: "Welcome to Salamander!",
		},
		Theme: schema.ThemeConfig{
			Name: "default",
			Mode: "dark",
			Colors: schema.ColorPalette{
				Primary:    "#7C3AED",
				Secondary:  "#A78BFA",
				Background: "#1E1E2E",
				Surface:    "#313244",
				Text:       "#CDD6F4",
				TextMuted:  "#6C7086",
				Border:     "#45475A",
				Error:      "#F38BA8",
				Success:    "#A6E3A1",
				Warning:    "#F9E2AF",
				Info:       "#89B4FA",
				Accent:     "#F5C2E7",
			},
		},
		Layout: schema.LayoutConfig{
			Type: "chat",
			Components: []schema.ComponentConfig{
				{
					ID:   "main-chat",
					Type: "chat",
					Position: schema.PositionConfig{
						Anchor: "center",
					},
					Size: schema.SizeConfig{
						Width:  "100%",
						Height: "100%",
					},
				},
			},
		},
		Menus: []schema.MenuConfig{},
		Keybindings: []schema.KeybindingConfig{
			{
				Key: "ctrl+c",
				Action: schema.ActionConfig{
					Type: "quit",
				},
				Description: "Quit the application",
				Context:     "global",
			},
			{
				Key: "ctrl+l",
				Action: schema.ActionConfig{
					Type:    "command",
					Command: "clear",
				},
				Description: "Clear the screen",
				Context:     "global",
			},
		},
		Backend: schema.BackendConfig{
			Type:       "mock",
			Timeout:    30,
			RetryCount: 3,
			Streaming:  true,
		},
		Extensions: []schema.ExtensionConfig{},
	}
}
