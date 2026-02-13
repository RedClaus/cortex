// Package schema defines the YAML schema types for Salamander TUI configuration.
//
// Salamander is a YAML-driven TUI framework that allows users to define
// their interface through configuration files rather than code.
package schema

// ═══════════════════════════════════════════════════════════════════════════════
// ROOT CONFIGURATION
// ═══════════════════════════════════════════════════════════════════════════════

// Config is the root configuration for a Salamander TUI application.
type Config struct {
	// Version of the schema (e.g., "1.0")
	Version string `yaml:"version"`

	// App contains application-level settings
	App AppConfig `yaml:"app"`

	// Theme defines colors and styling
	Theme ThemeConfig `yaml:"theme,omitempty"`

	// Layout defines the screen layout
	Layout LayoutConfig `yaml:"layout"`

	// Menus defines the command menus
	Menus []MenuConfig `yaml:"menus"`

	// Keybindings defines custom key mappings
	Keybindings []KeybindingConfig `yaml:"keybindings,omitempty"`

	// Backend configures the A2A connection
	Backend BackendConfig `yaml:"backend"`

	// Extensions for custom functionality
	Extensions []ExtensionConfig `yaml:"extensions,omitempty"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// APP CONFIGURATION
// ═══════════════════════════════════════════════════════════════════════════════

// AppConfig contains application-level settings.
type AppConfig struct {
	// Name of the application (shown in title bar)
	Name string `yaml:"name"`

	// Description shown in help
	Description string `yaml:"description,omitempty"`

	// Version of this configuration
	Version string `yaml:"version,omitempty"`

	// Author information
	Author string `yaml:"author,omitempty"`

	// ShowStatusBar toggles the status bar
	ShowStatusBar bool `yaml:"show_status_bar"`

	// ShowTitleBar toggles the title bar
	ShowTitleBar bool `yaml:"show_title_bar"`

	// WelcomeMessage shown on startup
	WelcomeMessage string `yaml:"welcome_message,omitempty"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// THEME CONFIGURATION
// ═══════════════════════════════════════════════════════════════════════════════

// ThemeConfig defines the visual styling.
type ThemeConfig struct {
	// Name of the theme
	Name string `yaml:"name"`

	// Mode: "dark" or "light"
	Mode string `yaml:"mode"`

	// Colors defines the color palette
	Colors ColorPalette `yaml:"colors"`

	// Styles for specific components
	Styles ComponentStyles `yaml:"styles,omitempty"`
}

// ColorPalette defines the color values.
type ColorPalette struct {
	Primary    string `yaml:"primary"`
	Secondary  string `yaml:"secondary"`
	Background string `yaml:"background"`
	Surface    string `yaml:"surface"`
	Text       string `yaml:"text"`
	TextMuted  string `yaml:"text_muted"`
	Border     string `yaml:"border"`
	Error      string `yaml:"error"`
	Success    string `yaml:"success"`
	Warning    string `yaml:"warning"`
	Info       string `yaml:"info"`
	Accent     string `yaml:"accent"`
}

// ComponentStyles defines styles for specific components.
type ComponentStyles struct {
	Input     StyleConfig `yaml:"input,omitempty"`
	Message   StyleConfig `yaml:"message,omitempty"`
	Menu      StyleConfig `yaml:"menu,omitempty"`
	StatusBar StyleConfig `yaml:"status_bar,omitempty"`
}

// StyleConfig defines styling for a component.
type StyleConfig struct {
	Background  string `yaml:"background,omitempty"`
	Foreground  string `yaml:"foreground,omitempty"`
	Border      string `yaml:"border,omitempty"`
	BorderStyle string `yaml:"border_style,omitempty"` // "none", "single", "double", "rounded"
	Padding     []int  `yaml:"padding,omitempty"`      // [top, right, bottom, left] or [vertical, horizontal]
	Margin      []int  `yaml:"margin,omitempty"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// LAYOUT CONFIGURATION
// ═══════════════════════════════════════════════════════════════════════════════

// LayoutConfig defines the screen layout.
type LayoutConfig struct {
	// Type: "chat", "dashboard", "split", "custom"
	Type string `yaml:"type"`

	// Components in the layout
	Components []ComponentConfig `yaml:"components"`
}

// ComponentConfig defines a UI component.
type ComponentConfig struct {
	// ID is the unique identifier
	ID string `yaml:"id"`

	// Type: "chat", "input", "status", "panel", "list", "markdown", "code"
	Type string `yaml:"type"`

	// Position in the layout
	Position PositionConfig `yaml:"position,omitempty"`

	// Size constraints
	Size SizeConfig `yaml:"size,omitempty"`

	// Style overrides
	Style StyleConfig `yaml:"style,omitempty"`

	// Component-specific options
	Options map[string]interface{} `yaml:"options,omitempty"`
}

// PositionConfig defines component position.
type PositionConfig struct {
	// Row in a grid layout (0-based)
	Row int `yaml:"row,omitempty"`

	// Column in a grid layout (0-based)
	Column int `yaml:"column,omitempty"`

	// Anchor: "top", "bottom", "left", "right", "center"
	Anchor string `yaml:"anchor,omitempty"`
}

// SizeConfig defines component size.
type SizeConfig struct {
	// Width: absolute (e.g., 80) or percentage (e.g., "50%")
	Width string `yaml:"width,omitempty"`

	// Height: absolute or percentage
	Height string `yaml:"height,omitempty"`

	// MinWidth constraint
	MinWidth int `yaml:"min_width,omitempty"`

	// MaxWidth constraint
	MaxWidth int `yaml:"max_width,omitempty"`

	// MinHeight constraint
	MinHeight int `yaml:"min_height,omitempty"`

	// MaxHeight constraint
	MaxHeight int `yaml:"max_height,omitempty"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// MENU CONFIGURATION
// ═══════════════════════════════════════════════════════════════════════════════

// MenuConfig defines a command menu.
type MenuConfig struct {
	// ID is the unique identifier
	ID string `yaml:"id"`

	// Trigger is how to open this menu (e.g., "/" for slash commands)
	Trigger string `yaml:"trigger"`

	// Title shown at top of menu
	Title string `yaml:"title,omitempty"`

	// Items in the menu
	Items []MenuItemConfig `yaml:"items"`

	// Style overrides
	Style StyleConfig `yaml:"style,omitempty"`

	// MaxVisible items before scrolling
	MaxVisible int `yaml:"max_visible,omitempty"`

	// Filterable enables type-to-filter
	Filterable bool `yaml:"filterable"`
}

// MenuItemConfig defines a menu item.
type MenuItemConfig struct {
	// ID is the unique identifier
	ID string `yaml:"id"`

	// Label shown in the menu
	Label string `yaml:"label"`

	// Description shown next to label
	Description string `yaml:"description,omitempty"`

	// Category for grouping
	Category string `yaml:"category,omitempty"`

	// Icon (emoji or nerd font)
	Icon string `yaml:"icon,omitempty"`

	// Shortcut key
	Shortcut string `yaml:"shortcut,omitempty"`

	// Action to perform when selected
	Action ActionConfig `yaml:"action"`

	// Submenu for cascading menus
	Submenu *MenuConfig `yaml:"submenu,omitempty"`

	// Visible condition (expression)
	Visible string `yaml:"visible,omitempty"`

	// Enabled condition (expression)
	Enabled string `yaml:"enabled,omitempty"`
}

// ActionConfig defines what happens when a menu item is selected.
type ActionConfig struct {
	// Type: "command", "submenu", "a2a_request", "set_variable", "open_dialog", "quit"
	Type string `yaml:"type"`

	// Command to execute (for type="command")
	Command string `yaml:"command,omitempty"`

	// Args for the command
	Args map[string]interface{} `yaml:"args,omitempty"`

	// Message to send to A2A agent (for type="a2a_request")
	Message string `yaml:"message,omitempty"`

	// Variable to set (for type="set_variable")
	Variable string `yaml:"variable,omitempty"`

	// Value to set
	Value interface{} `yaml:"value,omitempty"`

	// Dialog to open (for type="open_dialog")
	Dialog string `yaml:"dialog,omitempty"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// KEYBINDING CONFIGURATION
// ═══════════════════════════════════════════════════════════════════════════════

// KeybindingConfig defines a keyboard shortcut.
type KeybindingConfig struct {
	// Key combination (e.g., "ctrl+c", "alt+enter", "f1")
	Key string `yaml:"key"`

	// Action to perform
	Action ActionConfig `yaml:"action"`

	// Description for help
	Description string `yaml:"description,omitempty"`

	// Context where this keybinding is active (e.g., "input", "menu", "global")
	Context string `yaml:"context,omitempty"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// BACKEND CONFIGURATION
// ═══════════════════════════════════════════════════════════════════════════════

// BackendConfig configures the A2A connection.
type BackendConfig struct {
	// Type: "a2a", "direct", "mock"
	Type string `yaml:"type"`

	// URL for A2A agent
	URL string `yaml:"url,omitempty"`

	// AuthToken for authentication
	AuthToken string `yaml:"auth_token,omitempty"`

	// AuthScheme: "Bearer", "Basic", etc.
	AuthScheme string `yaml:"auth_scheme,omitempty"`

	// Timeout in seconds
	Timeout int `yaml:"timeout,omitempty"`

	// RetryCount for failed requests
	RetryCount int `yaml:"retry_count,omitempty"`

	// Streaming enables SSE streaming
	Streaming bool `yaml:"streaming"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// EXTENSION CONFIGURATION
// ═══════════════════════════════════════════════════════════════════════════════

// ExtensionConfig defines a custom extension.
type ExtensionConfig struct {
	// ID is the unique identifier
	ID string `yaml:"id"`

	// Name of the extension
	Name string `yaml:"name"`

	// Type: "builtin", "plugin", "script"
	Type string `yaml:"type"`

	// Source for the extension (path or URL)
	Source string `yaml:"source,omitempty"`

	// Config for the extension
	Config map[string]interface{} `yaml:"config,omitempty"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// DATA SOURCES (for dynamic menus)
// ═══════════════════════════════════════════════════════════════════════════════

// DataSourceConfig defines a dynamic data source for menus.
type DataSourceConfig struct {
	// ID is the unique identifier
	ID string `yaml:"id"`

	// Type: "a2a_skill", "api", "command", "static"
	Type string `yaml:"type"`

	// URL for API calls
	URL string `yaml:"url,omitempty"`

	// Command to execute
	Command string `yaml:"command,omitempty"`

	// StaticItems for static data
	StaticItems []MenuItemConfig `yaml:"static_items,omitempty"`

	// Transform expression to convert response to menu items
	Transform string `yaml:"transform,omitempty"`

	// Cache duration in seconds
	CacheDuration int `yaml:"cache_duration,omitempty"`
}
