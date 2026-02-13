package tools

// ToolsConfig contains configuration for all tools
type ToolsConfig struct {
	Shell     *ShellConfig
	Files     *FilesConfig
	Web       *WebConfig
	WebSearch *WebSearchConfig
	Git       *GitConfig
	Code      *CodeConfig
	System    *SystemConfig
}

// DefaultToolsConfig returns default configuration for all tools
func DefaultToolsConfig() *ToolsConfig {
	return &ToolsConfig{
		Shell:     DefaultShellConfig(),
		Files:    DefaultFilesConfig(),
		Web:       DefaultWebConfig(),
		WebSearch: DefaultWebSearchConfig(),
		Git:       DefaultGitConfig(),
		Code:      DefaultCodeConfig(),
		System:    DefaultSystemConfig(),
	}
}

// NewDefaultRegistry creates a registry with all default tools registered
func NewDefaultRegistry(cfg *ToolsConfig) *Registry {
	if cfg == nil {
		cfg = DefaultToolsConfig()
	}

	r := NewRegistry()

	// Register all tools
	r.Register(NewShellTool(cfg.Shell))
	r.Register(NewFilesTool(cfg.Files))
	r.Register(NewWebTool(cfg.Web))
	r.Register(NewWebSearchTool(cfg.WebSearch)) // Tavily web search for real-time info
	r.Register(NewGitTool(cfg.Git))
	r.Register(NewCodeTool(cfg.Code))
	r.Register(NewSystemTool(cfg.System))
	r.Register(NewAPITool())

	return r
}

// Categories returns all tool categories
func Categories() []ToolCategory {
	return []ToolCategory{
		CategoryShell,
		CategoryFiles,
		CategoryWeb,
		CategoryAPI,
		CategoryGit,
		CategoryCode,
		CategorySystem,
	}
}

// RiskLevels returns all risk levels in order of severity
func RiskLevels() []RiskLevel {
	return []RiskLevel{
		RiskLow,
		RiskMedium,
		RiskHigh,
	}
}

// CategoryDescription returns a human-readable description of a category
func CategoryDescription(cat ToolCategory) string {
	switch cat {
	case CategoryShell:
		return "Shell command execution"
	case CategoryFiles:
		return "File system operations"
	case CategoryWeb:
		return "Web content fetching"
	case CategoryAPI:
		return "REST API calls"
	case CategoryGit:
		return "Git version control"
	case CategoryCode:
		return "Code execution"
	case CategorySystem:
		return "System operations"
	default:
		return "Unknown category"
	}
}

// RiskDescription returns a human-readable description of a risk level
func RiskDescription(risk RiskLevel) string {
	switch risk {
	case RiskLow:
		return "Low risk - read-only or non-destructive operations"
	case RiskMedium:
		return "Medium risk - may modify data or state"
	case RiskHigh:
		return "High risk - can execute arbitrary code or commands"
	default:
		return "Unknown risk level"
	}
}
