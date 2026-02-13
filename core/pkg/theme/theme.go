// Package theme provides a structured color palette system for the Cortex TUI.
// It defines semantic colors optimized for high contrast and readability across
// light and dark themes.
package theme

// ═══════════════════════════════════════════════════════════════════════════════
// PALETTE DEFINITION
// ═══════════════════════════════════════════════════════════════════════════════

// Palette defines the complete color system for a theme.
// All colors are hex codes (e.g., "#58a6ff").
type Palette struct {
	// ─────────────────────────────────────────────────────────────────────────
	// Metadata
	// ─────────────────────────────────────────────────────────────────────────
	Name string `json:"name"` // Human-readable name (e.g., "Midnight")
	ID   string `json:"id"`   // Machine ID (e.g., "midnight")
	Type string `json:"type"` // "light" or "dark"

	// ─────────────────────────────────────────────────────────────────────────
	// Base Colors
	// ─────────────────────────────────────────────────────────────────────────
	Background string `json:"background"` // Main background (#0d1117)
	Foreground string `json:"foreground"` // Primary text (#e6edf3)
	Selection  string `json:"selection"`  // Highlighted/selected item background
	Border     string `json:"border"`     // Borders and dividers

	// ─────────────────────────────────────────────────────────────────────────
	// Semantic Colors
	// ─────────────────────────────────────────────────────────────────────────
	Primary   string `json:"primary"`   // Main action, focus, accent (#58a6ff)
	Secondary string `json:"secondary"` // Subtitles, secondary info (#8b949e)
	Success   string `json:"success"`   // Success states, confirmations (#3fb950)
	Warning   string `json:"warning"`   // Warnings, caution (#d29922)
	Error     string `json:"error"`     // Error states, destructive (#f85149)

	// ─────────────────────────────────────────────────────────────────────────
	// Extended UI Colors
	// ─────────────────────────────────────────────────────────────────────────
	Muted   string `json:"muted"`   // Dim/muted text (#484f58)
	Accent  string `json:"accent"`  // Accent highlights (usually same as Primary)
	Surface string `json:"surface"` // Elevated surfaces, cards (#161b22)

	// ─────────────────────────────────────────────────────────────────────────
	// Code Colors (for syntax highlighting)
	// ─────────────────────────────────────────────────────────────────────────
	Code        string `json:"code"`         // Code text color (#3fb950)
	CodeBg      string `json:"code_bg"`      // Code block background (#21262d)
	CodeKeyword string `json:"code_keyword"` // Keywords (#ff7b72)
	CodeString  string `json:"code_string"`  // Strings (#a5d6ff)
	CodeComment string `json:"code_comment"` // Comments (#8b949e)
	CodeFunc    string `json:"code_func"`    // Functions (#d2a8ff)
	CodeNumber  string `json:"code_number"`  // Numbers (#79c0ff)

	// ─────────────────────────────────────────────────────────────────────────
	// Multicolor Accents (for Neon/Aurora themes)
	// ─────────────────────────────────────────────────────────────────────────
	Accent2 string `json:"accent2,omitempty"` // Secondary accent (magenta/teal)
	Accent3 string `json:"accent3,omitempty"` // Tertiary accent (yellow/coral)
}

// IsDark returns true if this is a dark theme.
func (p Palette) IsDark() bool {
	return p.Type == "dark"
}

// IsMulticolor returns true if this theme has multiple accent colors.
func (p Palette) IsMulticolor() bool {
	return p.Accent2 != "" && p.Accent3 != ""
}

// ═══════════════════════════════════════════════════════════════════════════════
// FALLBACK HELPERS
// ═══════════════════════════════════════════════════════════════════════════════

// GetMuted returns the muted color or falls back to secondary.
func (p Palette) GetMuted() string {
	if p.Muted != "" {
		return p.Muted
	}
	return p.Secondary
}

// GetAccent returns the accent color or falls back to primary.
func (p Palette) GetAccent() string {
	if p.Accent != "" {
		return p.Accent
	}
	return p.Primary
}

// GetSurface returns the surface color or falls back to a blend of bg.
func (p Palette) GetSurface() string {
	if p.Surface != "" {
		return p.Surface
	}
	return p.Selection
}

// GetSelectionForeground returns a contrasting text color for selected items.
func (p Palette) GetSelectionForeground() string {
	if p.IsDark() {
		return p.Background
	}
	return p.Foreground
}

// GetCode returns the code text color or falls back to Success.
func (p Palette) GetCode() string {
	if p.Code != "" {
		return p.Code
	}
	return p.Success
}

// GetCodeBg returns the code block background or falls back to Surface.
func (p Palette) GetCodeBg() string {
	if p.CodeBg != "" {
		return p.CodeBg
	}
	return p.GetSurface()
}

// GetCodeKeyword returns keyword color or falls back to Error (red).
func (p Palette) GetCodeKeyword() string {
	if p.CodeKeyword != "" {
		return p.CodeKeyword
	}
	return p.Error
}

// GetCodeString returns string color or falls back to Primary.
func (p Palette) GetCodeString() string {
	if p.CodeString != "" {
		return p.CodeString
	}
	return p.Primary
}

// GetCodeComment returns comment color or falls back to Muted.
func (p Palette) GetCodeComment() string {
	if p.CodeComment != "" {
		return p.CodeComment
	}
	return p.GetMuted()
}

// GetCodeFunc returns function color or falls back to Warning.
func (p Palette) GetCodeFunc() string {
	if p.CodeFunc != "" {
		return p.CodeFunc
	}
	return p.Warning
}

// GetCodeNumber returns number color or falls back to Primary.
func (p Palette) GetCodeNumber() string {
	if p.CodeNumber != "" {
		return p.CodeNumber
	}
	return p.Primary
}

// ═══════════════════════════════════════════════════════════════════════════════
// THEME REGISTRY - 6 High Contrast Themes
// ═══════════════════════════════════════════════════════════════════════════════

// Registry holds all available themes.
var Registry = map[string]Palette{
	// ─────────────────────────────────────────────────────────────────────────
	// DARK THEMES
	// ─────────────────────────────────────────────────────────────────────────

	// Midnight - Clean dark theme with blue accent (GitHub Dark inspired)
	"midnight": {
		Name:       "Midnight",
		ID:         "midnight",
		Type:       "dark",
		Background: "#0d1117",
		Foreground: "#e6edf3",
		Selection:  "#1f3a5f",
		Border:     "#30363d",
		Primary:    "#58a6ff",
		Secondary:  "#8b949e",
		Success:    "#3fb950",
		Warning:    "#d29922",
		Error:      "#f85149",
		Muted:      "#484f58",
		Accent:     "#58a6ff",
		Surface:    "#161b22",
		// Code colors
		Code:        "#3fb950",
		CodeBg:      "#21262d",
		CodeKeyword: "#ff7b72",
		CodeString:  "#a5d6ff",
		CodeComment: "#8b949e",
		CodeFunc:    "#d2a8ff",
		CodeNumber:  "#79c0ff",
	},

	// Neon - Cyberpunk multicolor theme (cyan/magenta/yellow)
	"neon": {
		Name:       "Neon",
		ID:         "neon",
		Type:       "dark",
		Background: "#000000",
		Foreground: "#e0f7ff",
		Selection:  "#1a1a3e",
		Border:     "#00fff530",
		Primary:    "#00fff5", // Cyan
		Secondary:  "#8892b0",
		Success:    "#00ff88",
		Warning:    "#ffd93d",
		Error:      "#ff2e63",
		Muted:      "#4a5568",
		Accent:     "#00fff5",
		Surface:    "#0a0a0f",
		// Multicolor accents
		Accent2: "#ff00ff", // Magenta
		Accent3: "#ffff00", // Yellow
		// Code colors (neon style)
		Code:        "#00ff88",
		CodeBg:      "#0a0a0f",
		CodeKeyword: "#ff2e63",
		CodeString:  "#ffd93d",
		CodeComment: "#4a5568",
		CodeFunc:    "#ff00ff",
		CodeNumber:  "#00fff5",
	},

	// Obsidian - Minimal monochrome dark theme
	"obsidian": {
		Name:       "Obsidian",
		ID:         "obsidian",
		Type:       "dark",
		Background: "#1a1a1a",
		Foreground: "#d4d4d4",
		Selection:  "#3a3a3a",
		Border:     "#404040",
		Primary:    "#ffffff",
		Secondary:  "#a0a0a0",
		Success:    "#b0b0b0",
		Warning:    "#d0d0d0",
		Error:      "#ff6b6b",
		Muted:      "#606060",
		Accent:     "#ffffff",
		Surface:    "#252525",
		// Code colors (monochrome with slight variation)
		Code:        "#c0c0c0",
		CodeBg:      "#202020",
		CodeKeyword: "#ffffff",
		CodeString:  "#a0a0a0",
		CodeComment: "#606060",
		CodeFunc:    "#e0e0e0",
		CodeNumber:  "#d0d0d0",
	},

	// ─────────────────────────────────────────────────────────────────────────
	// LIGHT THEMES
	// ─────────────────────────────────────────────────────────────────────────

	// Paper - Clean high-contrast light theme
	"paper": {
		Name:       "Paper",
		ID:         "paper",
		Type:       "light",
		Background: "#ffffff",
		Foreground: "#1f2328",
		Selection:  "#dbeafe",
		Border:     "#d1d9e0",
		Primary:    "#0969da",
		Secondary:  "#57606a",
		Success:    "#1a7f37",
		Warning:    "#9a6700",
		Error:      "#cf222e",
		Muted:      "#8c959f",
		Accent:     "#0969da",
		Surface:    "#f6f8fa",
		// Code colors
		Code:        "#1a7f37",
		CodeBg:      "#f6f8fa",
		CodeKeyword: "#cf222e",
		CodeString:  "#0a3069",
		CodeComment: "#8c959f",
		CodeFunc:    "#8250df",
		CodeNumber:  "#0969da",
	},

	// Aurora - Light theme with multicolor accents (purple/teal/coral)
	"aurora": {
		Name:       "Aurora",
		ID:         "aurora",
		Type:       "light",
		Background: "#fafafa",
		Foreground: "#2e3440",
		Selection:  "#e8d5f0",
		Border:     "#d8dee9",
		Primary:    "#8b5cf6", // Purple
		Secondary:  "#6b7280",
		Success:    "#059669",
		Warning:    "#d97706",
		Error:      "#dc2626",
		Muted:      "#9ca3af",
		Accent:     "#8b5cf6",
		Surface:    "#f3f4f6",
		// Multicolor accents
		Accent2: "#0d9488", // Teal
		Accent3: "#f97316", // Coral/Orange
		// Code colors
		Code:        "#059669",
		CodeBg:      "#f3f4f6",
		CodeKeyword: "#dc2626",
		CodeString:  "#0d9488",
		CodeComment: "#9ca3af",
		CodeFunc:    "#8b5cf6",
		CodeNumber:  "#2563eb",
	},

	// Daylight - Warm high-contrast light theme
	"daylight": {
		Name:       "Daylight",
		ID:         "daylight",
		Type:       "light",
		Background: "#fffbf5",
		Foreground: "#3d3d3d",
		Selection:  "#fff3cd",
		Border:     "#e5ddd5",
		Primary:    "#c45500",
		Secondary:  "#6b6b6b",
		Success:    "#2d6a4f",
		Warning:    "#b86e00",
		Error:      "#c41e3a",
		Muted:      "#8b8b8b",
		Accent:     "#c45500",
		Surface:    "#faf5ef",
		// Code colors (warm palette)
		Code:        "#2d6a4f",
		CodeBg:      "#faf5ef",
		CodeKeyword: "#c41e3a",
		CodeString:  "#0066cc",
		CodeComment: "#8b8b8b",
		CodeFunc:    "#7c3aed",
		CodeNumber:  "#c45500",
	},
}

// ═══════════════════════════════════════════════════════════════════════════════
// REGISTRY ACCESS
// ═══════════════════════════════════════════════════════════════════════════════

// DefaultTheme is used when no theme is specified or theme is not found.
const DefaultTheme = "midnight"

// Get safely returns a theme or falls back to the default.
func Get(id string) Palette {
	if t, ok := Registry[id]; ok {
		return t
	}
	return Registry[DefaultTheme]
}

// Exists checks if a theme ID is valid.
func Exists(id string) bool {
	_, ok := Registry[id]
	return ok
}

// List returns all theme IDs in display order.
func List() []string {
	// Return in consistent order: dark themes first, then light
	return []string{
		"midnight", "neon", "obsidian", // Dark
		"paper", "aurora", "daylight", // Light
	}
}

// ListDark returns only dark theme IDs.
func ListDark() []string {
	return []string{"midnight", "neon", "obsidian"}
}

// ListLight returns only light theme IDs.
func ListLight() []string {
	return []string{"paper", "aurora", "daylight"}
}

// Next returns the next theme ID in the cycle.
func Next(current string) string {
	themes := List()
	for i, id := range themes {
		if id == current {
			return themes[(i+1)%len(themes)]
		}
	}
	return DefaultTheme
}

// Prev returns the previous theme ID in the cycle.
func Prev(current string) string {
	themes := List()
	for i, id := range themes {
		if id == current {
			if i == 0 {
				return themes[len(themes)-1]
			}
			return themes[i-1]
		}
	}
	return DefaultTheme
}
