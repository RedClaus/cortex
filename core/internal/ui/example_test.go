package ui

import (
	"fmt"
	"testing"
)

// TestThemeAndStyles demonstrates the theming and styling system.
func TestThemeAndStyles(t *testing.T) {
	tests := []struct {
		name      string
		themeID   string
		wantTheme string
	}{
		{"Default Theme", "default", "Default (VS Code Dark)"},
		{"Dracula Theme", "dracula", "Dracula"},
		{"Nord Theme", "nord", "Nord"},
		{"Gruvbox Theme", "gruvbox", "Gruvbox Dark"},
		{"Fallback to Default", "nonexistent", "Default (VS Code Dark)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Get theme
			theme := GetTheme(tt.themeID)

			// Verify theme name
			if theme.Name != tt.wantTheme {
				t.Errorf("GetTheme(%q) name = %v, want %v", tt.themeID, theme.Name, tt.wantTheme)
			}

			// Create styles from theme
			styles := NewStyles(theme)

			// Verify styles were created
			if styles.Theme().Name != theme.Name {
				t.Errorf("NewStyles theme name mismatch: got %v, want %v", styles.Theme().Name, theme.Name)
			}

			// Test rendering functions
			userMsg := styles.RenderUserMessage("Hello, world!")
			if userMsg == "" {
				t.Error("RenderUserMessage returned empty string")
			}

			assistantMsg := styles.RenderAssistantMessage("Hello! How can I help?")
			if assistantMsg == "" {
				t.Error("RenderAssistantMessage returned empty string")
			}

			systemMsg := styles.RenderSystemMessage("System initialized")
			if systemMsg == "" {
				t.Error("RenderSystemMessage returned empty string")
			}

			errorMsg := styles.RenderError("Something went wrong")
			if errorMsg == "" {
				t.Error("RenderError returned empty string")
			}

			successMsg := styles.RenderSuccess("Operation completed")
			if successMsg == "" {
				t.Error("RenderSuccess returned empty string")
			}
		})
	}
}

// TestAllThemes verifies all built-in themes are accessible.
func TestAllThemes(t *testing.T) {
	themeNames := ThemeNames()

	if len(themeNames) < 4 {
		t.Errorf("Expected at least 4 themes, got %d", len(themeNames))
	}

	// Verify each theme can be retrieved and used
	for _, name := range themeNames {
		theme := GetTheme(name)
		styles := NewStyles(theme)

		if styles.Theme().Name == "" {
			t.Errorf("Theme %q has empty name", name)
		}
	}
}

// TestStyleRendering verifies all rendering functions work.
func TestStyleRendering(t *testing.T) {
	styles := NewStyles(ThemeDefault)

	tests := []struct {
		name   string
		render func() string
	}{
		{"UserMessage", func() string { return styles.RenderUserMessage("test") }},
		{"AssistantMessage", func() string { return styles.RenderAssistantMessage("test") }},
		{"SystemMessage", func() string { return styles.RenderSystemMessage("test") }},
		{"Error", func() string { return styles.RenderError("test") }},
		{"Success", func() string { return styles.RenderSuccess("test") }},
		{"Code", func() string { return styles.RenderCode("test") }},
		{"Badge", func() string { return styles.RenderBadge("test") }},
		{"FooterNormal", func() string { return styles.RenderFooter("normal", "test") }},
		{"FooterYolo", func() string { return styles.RenderFooter("yolo", "test") }},
		{"FooterPlan", func() string { return styles.RenderFooter("plan", "test") }},
		{"HorizontalLine", func() string { return styles.RenderHorizontalLine(10) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.render()
			if result == "" {
				t.Errorf("%s rendered empty string", tt.name)
			}
		})
	}
}

// ExampleNewStyles demonstrates creating and using styles.
func ExampleNewStyles() {
	// Create styles from Dracula theme
	styles := NewStyles(ThemeDracula)

	// Render a user message
	userMsg := styles.RenderUserMessage("What is Cortex?")
	fmt.Println(userMsg)

	// Render an assistant message
	assistantMsg := styles.RenderAssistantMessage("Cortex is an AI orchestration system.")
	fmt.Println(assistantMsg)

	// Render a system message
	systemMsg := styles.RenderSystemMessage("Session started")
	fmt.Println(systemMsg)
}

// ExampleGetTheme demonstrates retrieving themes.
func ExampleGetTheme() {
	// Get the Nord theme
	theme := GetTheme("nord")
	fmt.Println(theme.Name)
	// Output: Nord
}

// ExampleThemeNames demonstrates listing all available themes.
func ExampleThemeNames() {
	themes := ThemeNames()
	fmt.Printf("Available themes: %d\n", len(themes))
}
