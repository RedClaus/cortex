// Package skills provides skill management for the Cortex Coder Agent
package skills

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// ContextBuilder builds skill context from session state
type ContextBuilder struct {
	ctx *Context
}

// NewContextBuilder creates a new context builder
func NewContextBuilder() *ContextBuilder {
	return &ContextBuilder{
		ctx: NewContext(),
	}
}

// Build returns the built context
func (b *ContextBuilder) Build() *Context {
	return b.ctx
}

// FromSelection sets context from a text selection
func (b *ContextBuilder) FromSelection(selection string, lineNumber int) *ContextBuilder {
	b.ctx.Selection = selection
	b.ctx.Code = selection
	b.ctx.LineNumber = lineNumber
	return b
}

// FromFile sets context from a file
func (b *ContextBuilder) FromFile(path string) *ContextBuilder {
	b.ctx.FilePath = path

	// Extract package name from Go files
	if strings.HasSuffix(path, ".go") {
		b.ctx.PackageName = filepath.Base(path)
		b.ctx.Language = "go"
	}

	// Detect language from extension
	ext := filepath.Ext(path)
	switch ext {
	case ".py":
		b.ctx.Language = "python"
	case ".js", ".ts", ".jsx", ".tsx":
		b.ctx.Language = "javascript"
	case ".java":
		b.ctx.Language = "java"
	case ".rs":
		b.ctx.Language = "rust"
	case ".go":
		b.ctx.Language = "go"
	case ".cpp", ".cc", ".hpp", ".h":
		b.ctx.Language = "cpp"
	case ".c":
		b.ctx.Language = "c"
	case ".rb":
		b.ctx.Language = "ruby"
	case ".php":
		b.ctx.Language = "php"
	case ".swift":
		b.ctx.Language = "swift"
	case ".kt", ".kts":
		b.ctx.Language = "kotlin"
	}

	return b
}

// FromProject sets context from project information
func (b *ContextBuilder) FromProject(projectPath string) *ContextBuilder {
	b.ctx.ProjectPath = projectPath

	// Detect project type
	if b.detectProjectType(projectPath) {
		return b
	}

	// Check for common project files
	files, _ := os.ReadDir(projectPath)
	for _, f := range files {
		name := f.Name()
		switch {
		case name == "go.mod" || name == "go.sum":
			b.ctx.ProjectType = "go"
			b.ctx.Language = "go"
		case name == "package.json" || name == "yarn.lock" || name == "pnpm-lock.yaml":
			b.ctx.ProjectType = "node"
			b.ctx.Language = "javascript"
		case name == "pyproject.toml" || name == "requirements.txt" || name == "setup.py":
			b.ctx.ProjectType = "python"
			b.ctx.Language = "python"
		case name == "Cargo.toml" || name == "Cargo.lock":
			b.ctx.ProjectType = "rust"
			b.ctx.Language = "rust"
		case name == "pom.xml" || name == "build.gradle":
			b.ctx.ProjectType = "java"
			b.ctx.Language = "java"
		case name == "Makefile":
			b.ctx.ProjectType = "make"
		}
	}

	return b
}

// detectProjectType attempts to detect project type from structure
func (b *ContextBuilder) detectProjectType(projectPath string) bool {
	if b.ctx.ProjectType != "" {
		return true
	}

	// Check if it's a git repository
	if _, err := exec.LookPath("git"); err == nil {
		if b.getGitBranch() != "" {
			return true
		}
	}

	return false
}

// FromGit sets context from git information
func (b *ContextBuilder) FromGit() *ContextBuilder {
	b.ctx.GitBranch = b.getGitBranch()
	return b
}

// getGitBranch returns the current git branch
func (b *ContextBuilder) getGitBranch() string {
	if _, err := exec.LookPath("git"); err != nil {
		return ""
	}

	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(output))
}

// FromCommand sets context from command invocation
func (b *ContextBuilder) FromCommand(query string, args []string) *ContextBuilder {
	b.ctx.UserQuery = query
	b.ctx.CommandArgs = args
	return b
}

// FromFunction sets the current function name
func (b *ContextBuilder) FromFunction(name string) *ContextBuilder {
	b.ctx.FunctionName = name
	return b
}

// FromClass sets the current class name
func (b *ContextBuilder) FromClass(name string) *ContextBuilder {
	b.ctx.ClassName = name
	return b
}

// WithMetadata adds additional metadata
func (b *ContextBuilder) WithMetadata(key string, value interface{}) *ContextBuilder {
	b.ctx.Metadata[key] = value
	return b
}

// BuildDefault builds a default context for the current working directory
func (ContextBuilder) BuildDefault() *Context {
	ctx := NewContext()

	// Get current working directory
	wd, _ := os.Getwd()
	ctx.ProjectPath = wd

	// Detect project type
	files, _ := os.ReadDir(wd)
	for _, f := range files {
		name := f.Name()
		switch {
		case name == "go.mod" || name == "go.sum":
			ctx.ProjectType = "go"
			ctx.Language = "go"
		case name == "package.json":
			ctx.ProjectType = "node"
			ctx.Language = "javascript"
		case name == "pyproject.toml" || name == "requirements.txt":
			ctx.ProjectType = "python"
			ctx.Language = "python"
		}
	}

	// Get git branch
	if _, err := exec.LookPath("git"); err == nil {
		cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
		if output, err := cmd.Output(); err == nil {
			ctx.GitBranch = strings.TrimSpace(string(output))
		}
	}

	// Get file path if available
	ctx.FilePath = wd
	ctx.PackageName = filepath.Base(wd)

	return ctx
}

// Clone clones the builder for reuse
func (b *ContextBuilder) Clone() *ContextBuilder {
	clone := *b
	clone.ctx = &Context{
		Code:        b.ctx.Code,
		FilePath:    b.ctx.FilePath,
		PackageName: b.ctx.PackageName,
		ProjectType: b.ctx.ProjectType,
		GitBranch:   b.ctx.GitBranch,
		Selection:   b.ctx.Selection,
		LineNumber:  b.ctx.LineNumber,
		Language:    b.ctx.Language,
		FunctionName: b.ctx.FunctionName,
		ClassName:    b.ctx.ClassName,
		ProjectPath:  b.ctx.ProjectPath,
		UserQuery:   b.ctx.UserQuery,
		CommandArgs: b.ctx.CommandArgs,
		Metadata:    make(map[string]interface{}),
	}
	for k, v := range b.ctx.Metadata {
		clone.ctx.Metadata[k] = v
	}
	return &clone
}

// DetectLanguage detects the programming language from file content
func (b *ContextBuilder) DetectLanguage(content string) string {
	// Simple heuristics for language detection
	var lang string
	switch {
	case strings.Contains(content, "func ") || strings.Contains(content, "package "):
		lang = "go"
	case strings.Contains(content, "def ") || strings.Contains(content, "import ") && strings.Contains(content, ":"):
		lang = "python"
	case strings.Contains(content, "function ") || strings.Contains(content, "const ") || strings.Contains(content, "let "):
		lang = "javascript"
	case strings.Contains(content, "struct ") || strings.Contains(content, "impl "):
		lang = "rust"
	case strings.Contains(content, "public class ") || strings.Contains(content, "public static void main"):
		lang = "java"
	default:
		lang = b.ctx.Language
	}
	if lang != "" {
		b.ctx.Language = lang
	}
	return lang
}

// GetOS returns the current operating system
func (b *ContextBuilder) GetOS() string {
	return runtime.GOOS
}

// GetArch returns the current architecture
func (b *ContextBuilder) GetArch() string {
	return runtime.GOARCH
}
