package indexer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewGenerator(t *testing.T) {
	gen := NewGenerator()
	assert.NotNil(t, gen)
	assert.NotNil(t, gen.analyzer)
	assert.NotNil(t, gen.indexer)
}

func TestNewGeneratorWithOptions(t *testing.T) {
	opts := Options{
		IncludeHidden: true,
		MaxDepth:      5,
	}
	gen := NewGeneratorWithOptions(opts)
	assert.NotNil(t, gen)
	assert.True(t, gen.indexer.opts.IncludeHidden)
	assert.Equal(t, 5, gen.indexer.opts.MaxDepth)
}

func TestGenerateContext_SimpleProject(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a simple Go project structure
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "cmd"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "internal", "handler"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "pkg", "models"), 0755))

	// Create main.go
	mainContent := `package main

import "fmt"

func main() {
	fmt.Println("Hello")
}
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "cmd", "main.go"), []byte(mainContent), 0644))

	// Create handler.go
	handlerContent := `package handler

import "net/http"

type Handler interface {
	ServeHTTP(w http.ResponseWriter, r *http.Request)
}

type UserHandler struct {}

func NewUserHandler() *UserHandler {
	return &UserHandler{}
}
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "internal", "handler", "handler.go"), []byte(handlerContent), 0644))

	// Create model.go
	modelContent := `package models

type User struct {
	ID   string
	Name string
}

type Config struct {
	Port int
}
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "pkg", "models", "user.go"), []byte(modelContent), 0644))

	gen := NewGenerator()
	result, err := gen.GenerateContext(tmpDir)
	require.NoError(t, err)

	// Verify result structure
	assert.NotEmpty(t, result.Content)
	assert.NotEmpty(t, result.ProjectName)
	assert.Contains(t, result.Languages, "go")
	assert.NotEmpty(t, result.EntryPoints)
	assert.Greater(t, result.TotalFiles, 0)
	assert.NotZero(t, result.GeneratedAt)

	// Verify markdown content
	assert.Contains(t, result.Content, "# Project Context:")
	assert.Contains(t, result.Content, "## Overview")
	assert.Contains(t, result.Content, "## Languages")
	assert.Contains(t, result.Content, "## Entry Points")
	assert.Contains(t, result.Content, "cmd/main.go")
}

func TestGenerateContext_MultiLanguageProject(t *testing.T) {
	tmpDir := t.TempDir()

	// Create Go file
	goContent := `package main
func main() {}
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(goContent), 0644))

	// Create Python file
	pyContent := `def main():
    pass

if __name__ == "__main__":
    main()
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "script.py"), []byte(pyContent), 0644))

	// Create TypeScript file
	tsContent := `export function hello(): string {
    return "world";
}
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "index.ts"), []byte(tsContent), 0644))

	gen := NewGenerator()
	result, err := gen.GenerateContext(tmpDir)
	require.NoError(t, err)

	// Should detect all three languages
	assert.Contains(t, result.Languages, "go")
	assert.Contains(t, result.Languages, "python")
	assert.Contains(t, result.Languages, "typescript")
}

func TestGenerateContext_WithInterfaces(t *testing.T) {
	tmpDir := t.TempDir()

	content := `package service

type UserService interface {
	GetUser(id string) (*User, error)
	CreateUser(data CreateUserRequest) (*User, error)
}

type Repository interface {
	Find(id string) (interface{}, error)
	Save(entity interface{}) error
}
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "service.go"), []byte(content), 0644))

	gen := NewGenerator()
	result, err := gen.GenerateContext(tmpDir)
	require.NoError(t, err)

	assert.Contains(t, result.Content, "## Key Interfaces")
	assert.Contains(t, result.Content, "UserService")
	assert.Contains(t, result.Content, "Repository")
}

func TestGenerateContext_WithHTTPEndpoints(t *testing.T) {
	tmpDir := t.TempDir()

	content := `package api

import "net/http"

func SetupRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/users", handleUsers)
	mux.Get("/api/health", healthCheck)
	mux.Post("/api/items", createItem)
}
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "routes.go"), []byte(content), 0644))

	gen := NewGenerator()
	result, err := gen.GenerateContext(tmpDir)
	require.NoError(t, err)

	assert.Contains(t, result.Content, "## Detected APIs")
	assert.Contains(t, result.Content, "/api/users")
}

func TestGenerateContext_NonExistentPath(t *testing.T) {
	gen := NewGenerator()
	_, err := gen.GenerateContext("/nonexistent/path")
	assert.Error(t, err)
}

func TestGenerateTodos_WithTodos(t *testing.T) {
	tmpDir := t.TempDir()

	content := `package main

func main() {
	// TODO: Implement user authentication
	// FIXME: This is broken
	// HACK: Temporary workaround
	// XXX: Needs review
	// NOTE: This is documented
	// BUG: Known issue with concurrent access
}
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(content), 0644))

	gen := NewGenerator()
	result, err := gen.GenerateTodos(tmpDir)
	require.NoError(t, err)

	// Verify result structure
	assert.NotEmpty(t, result.Content)
	assert.Equal(t, 6, result.TotalCount)
	assert.NotZero(t, result.GeneratedAt)

	// Verify by type
	assert.Equal(t, 1, result.ByType["TODO"])
	assert.Equal(t, 1, result.ByType["FIXME"])
	assert.Equal(t, 1, result.ByType["HACK"])
	assert.Equal(t, 1, result.ByType["XXX"])
	assert.Equal(t, 1, result.ByType["NOTE"])
	assert.Equal(t, 1, result.ByType["BUG"])

	// Verify markdown content
	assert.Contains(t, result.Content, "# Code TODOs")
	assert.Contains(t, result.Content, "## Summary")
	assert.Contains(t, result.Content, "Total items: **6**")
}

func TestGenerateTodos_PriorityInference(t *testing.T) {
	tmpDir := t.TempDir()

	content := `package main

func main() {
	// TODO: normal task
	// TODO: URGENT fix needed
	// FIXME: broken feature
	// BUG: security vulnerability
	// NOTE: just a note
	// HACK: workaround
}
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(content), 0644))

	gen := NewGenerator()
	result, err := gen.GenerateTodos(tmpDir)
	require.NoError(t, err)

	// Check priority assignment
	priorities := make(map[string]string)
	for _, item := range result.Items {
		priorities[item.Text] = item.Priority
	}

	// FIXME and BUG should be high
	assert.Equal(t, "high", priorities["broken feature"])
	assert.Equal(t, "high", priorities["security vulnerability"])

	// "URGENT" keyword should make it high
	assert.Equal(t, "high", priorities["URGENT fix needed"])

	// Normal TODO should be medium
	assert.Equal(t, "medium", priorities["normal task"])

	// NOTE should be low
	assert.Equal(t, "low", priorities["just a note"])

	// HACK should be medium
	assert.Equal(t, "medium", priorities["workaround"])
}

func TestGenerateTodos_NoTodos(t *testing.T) {
	tmpDir := t.TempDir()

	content := `package main

func main() {
	// This is a regular comment
	println("Hello, World!")
}
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(content), 0644))

	gen := NewGenerator()
	result, err := gen.GenerateTodos(tmpDir)
	require.NoError(t, err)

	assert.Equal(t, 0, result.TotalCount)
	assert.Empty(t, result.Items)
	assert.Contains(t, result.Content, "Total items: **0**")
}

func TestGenerateTodos_MultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()

	file1 := `package main
// TODO: task in file 1
`
	file2 := `package util
// TODO: task in file 2
// FIXME: bug in file 2
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(file1), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "util.go"), []byte(file2), 0644))

	gen := NewGenerator()
	result, err := gen.GenerateTodos(tmpDir)
	require.NoError(t, err)

	assert.Equal(t, 3, result.TotalCount)

	// Verify files are tracked
	files := make(map[string]bool)
	for _, item := range result.Items {
		files[item.File] = true
	}
	assert.True(t, files["main.go"])
	assert.True(t, files["util.go"])
}

func TestGenerateTodos_VariousFormats(t *testing.T) {
	tmpDir := t.TempDir()

	content := `package main

// TODO: colon format
// TODO - dash format
// TODO without separator
/* TODO: multiline comment */
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(content), 0644))

	gen := NewGenerator()
	result, err := gen.GenerateTodos(tmpDir)
	require.NoError(t, err)

	assert.Equal(t, 4, result.TotalCount)
}

func TestGenerateTodos_NonExistentPath(t *testing.T) {
	gen := NewGenerator()
	_, err := gen.GenerateTodos("/nonexistent/path")
	assert.Error(t, err)
}

func TestGenerateInsights_SimpleProject(t *testing.T) {
	tmpDir := t.TempDir()

	// Create standard directory structure
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "cmd"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "internal"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "pkg"), 0755))

	content := `package main
func main() {}
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "cmd", "main.go"), []byte(content), 0644))

	gen := NewGenerator()
	result, err := gen.GenerateInsights(tmpDir)
	require.NoError(t, err)

	assert.NotEmpty(t, result.Content)
	assert.NotEmpty(t, result.Items)
	assert.NotZero(t, result.GeneratedAt)

	// Should detect standard Go project structure
	assert.Contains(t, result.Content, "# Project Insights")
}

func TestGenerateInsights_WithTests(t *testing.T) {
	tmpDir := t.TempDir()

	mainContent := `package main
func main() {}
`
	testContent := `package main
import "testing"
func TestMain(t *testing.T) {}
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainContent), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main_test.go"), []byte(testContent), 0644))

	gen := NewGenerator()
	result, err := gen.GenerateInsights(tmpDir)
	require.NoError(t, err)

	// Should detect test presence
	hasTestInsight := false
	for _, item := range result.Items {
		if strings.Contains(item.Title, "Test") {
			hasTestInsight = true
			break
		}
	}
	assert.True(t, hasTestInsight, "should detect test files")
}

func TestGenerateInsights_NoTests(t *testing.T) {
	tmpDir := t.TempDir()

	content := `package main
func main() {}
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(content), 0644))

	gen := NewGenerator()
	result, err := gen.GenerateInsights(tmpDir)
	require.NoError(t, err)

	// Should note lack of tests
	hasNoTestInsight := false
	for _, item := range result.Items {
		if strings.Contains(item.Title, "No Tests") {
			hasNoTestInsight = true
			break
		}
	}
	assert.True(t, hasNoTestInsight, "should note missing tests")
}

func TestGenerateInsights_MultiLanguage(t *testing.T) {
	tmpDir := t.TempDir()

	goContent := `package main
func main() {}
`
	pyContent := `def main(): pass
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(goContent), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "script.py"), []byte(pyContent), 0644))

	gen := NewGenerator()
	result, err := gen.GenerateInsights(tmpDir)
	require.NoError(t, err)

	// Should detect multi-language
	hasMultiLangInsight := false
	for _, item := range result.Items {
		if strings.Contains(item.Title, "Multi-language") {
			hasMultiLangInsight = true
			break
		}
	}
	assert.True(t, hasMultiLangInsight, "should detect multi-language codebase")
}

func TestGenerateInsights_LayeredArchitecture(t *testing.T) {
	tmpDir := t.TempDir()

	// Create files that indicate layered architecture
	handlerContent := `package handler
type UserHandler struct {}
`
	serviceContent := `package service
type UserService struct {}
`
	repoContent := `package repository
type UserRepository struct {}
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "handler.go"), []byte(handlerContent), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "service.go"), []byte(serviceContent), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "repository.go"), []byte(repoContent), 0644))

	gen := NewGenerator()
	result, err := gen.GenerateInsights(tmpDir)
	require.NoError(t, err)

	// Should detect layered architecture
	hasLayeredInsight := false
	for _, item := range result.Items {
		if strings.Contains(item.Title, "Layered Architecture") {
			hasLayeredInsight = true
			break
		}
	}
	assert.True(t, hasLayeredInsight, "should detect layered architecture pattern")
}

func TestGenerateInsights_ConfigFiles(t *testing.T) {
	tmpDir := t.TempDir()

	goContent := `package main
func main() {}
`
	configContent := `{"key": "value"}`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(goContent), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "config.json"), []byte(configContent), 0644))

	gen := NewGenerator()
	result, err := gen.GenerateInsights(tmpDir)
	require.NoError(t, err)

	// Should detect config files
	hasConfigInsight := false
	for _, item := range result.Items {
		if strings.Contains(item.Title, "Configuration") {
			hasConfigInsight = true
			break
		}
	}
	assert.True(t, hasConfigInsight, "should detect configuration files")
}

func TestGenerateInsights_NonExistentPath(t *testing.T) {
	gen := NewGenerator()
	_, err := gen.GenerateInsights("/nonexistent/path")
	assert.Error(t, err)
}

func TestSaveToSession(t *testing.T) {
	tmpDir := t.TempDir()
	sessionDir := filepath.Join(tmpDir, "session")

	// Create minimal results
	context := &ContextResult{
		Content:     "# Context\n\nTest content",
		ProjectName: "test",
	}
	todos := &TodoResult{
		Content:    "# TODOs\n\nTest content",
		TotalCount: 0,
	}
	insights := &InsightResult{
		Content: "# Insights\n\nTest content",
	}

	gen := NewGenerator()
	err := gen.SaveToSession(sessionDir, context, todos, insights)
	require.NoError(t, err)

	// Verify files were created
	contextPath := filepath.Join(sessionDir, "context.md")
	todosPath := filepath.Join(sessionDir, "todos.md")
	insightsPath := filepath.Join(sessionDir, "insights.md")

	assert.FileExists(t, contextPath)
	assert.FileExists(t, todosPath)
	assert.FileExists(t, insightsPath)

	// Verify content
	contextContent, err := os.ReadFile(contextPath)
	require.NoError(t, err)
	assert.Equal(t, "# Context\n\nTest content", string(contextContent))

	todosContent, err := os.ReadFile(todosPath)
	require.NoError(t, err)
	assert.Equal(t, "# TODOs\n\nTest content", string(todosContent))

	insightsContent, err := os.ReadFile(insightsPath)
	require.NoError(t, err)
	assert.Equal(t, "# Insights\n\nTest content", string(insightsContent))
}

func TestSaveToSession_PartialResults(t *testing.T) {
	tmpDir := t.TempDir()
	sessionDir := filepath.Join(tmpDir, "session")

	// Only save context
	context := &ContextResult{
		Content:     "# Context\n\nTest content",
		ProjectName: "test",
	}

	gen := NewGenerator()
	err := gen.SaveToSession(sessionDir, context, nil, nil)
	require.NoError(t, err)

	// Only context.md should exist
	assert.FileExists(t, filepath.Join(sessionDir, "context.md"))
	_, err = os.Stat(filepath.Join(sessionDir, "todos.md"))
	assert.True(t, os.IsNotExist(err))
	_, err = os.Stat(filepath.Join(sessionDir, "insights.md"))
	assert.True(t, os.IsNotExist(err))
}

func TestSaveToSession_ExistingDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	sessionDir := filepath.Join(tmpDir, "session")

	// Pre-create directory
	require.NoError(t, os.MkdirAll(sessionDir, 0755))

	context := &ContextResult{
		Content:     "# Context\n\nNew content",
		ProjectName: "test",
	}

	gen := NewGenerator()
	err := gen.SaveToSession(sessionDir, context, nil, nil)
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(sessionDir, "context.md"))
}

func TestHelperFunctions(t *testing.T) {
	t.Run("isSourceFile", func(t *testing.T) {
		assert.True(t, isSourceFile(".go"))
		assert.True(t, isSourceFile(".ts"))
		assert.True(t, isSourceFile(".py"))
		assert.True(t, isSourceFile(".js"))
		assert.False(t, isSourceFile(".md"))
		assert.False(t, isSourceFile(".json"))
		assert.False(t, isSourceFile(".txt"))
	})

	t.Run("isConfigFile", func(t *testing.T) {
		assert.True(t, isConfigFile("config.json"))
		assert.True(t, isConfigFile("settings.yaml"))
		assert.True(t, isConfigFile(".env"))
		assert.True(t, isConfigFile("Dockerfile"))
		assert.True(t, isConfigFile("Makefile"))
		assert.True(t, isConfigFile("package.json"))
		assert.True(t, isConfigFile("go.mod"))
		assert.False(t, isConfigFile("main.go"))
		assert.False(t, isConfigFile("handler.ts"))
	})

	t.Run("formatBytes", func(t *testing.T) {
		assert.Equal(t, "0 B", formatBytes(0))
		assert.Equal(t, "512 B", formatBytes(512))
		assert.Equal(t, "1.0 KB", formatBytes(1024))
		assert.Equal(t, "1.5 KB", formatBytes(1536))
		assert.Equal(t, "1.0 MB", formatBytes(1024*1024))
		assert.Equal(t, "1.0 GB", formatBytes(1024*1024*1024))
	})

	t.Run("inferPriority", func(t *testing.T) {
		assert.Equal(t, "high", inferPriority("FIXME", "any text"))
		assert.Equal(t, "high", inferPriority("BUG", "any text"))
		assert.Equal(t, "high", inferPriority("TODO", "urgent: do this now"))
		assert.Equal(t, "high", inferPriority("TODO", "critical security issue"))
		assert.Equal(t, "medium", inferPriority("TODO", "regular task"))
		assert.Equal(t, "medium", inferPriority("HACK", "workaround"))
		assert.Equal(t, "medium", inferPriority("XXX", "needs review"))
		assert.Equal(t, "low", inferPriority("NOTE", "documentation"))
	})

	t.Run("priorityRank", func(t *testing.T) {
		assert.Equal(t, 0, priorityRank("high"))
		assert.Equal(t, 1, priorityRank("medium"))
		assert.Equal(t, 2, priorityRank("low"))
		assert.Equal(t, 3, priorityRank("unknown"))
	})

	t.Run("truncateList", func(t *testing.T) {
		items := []string{"a", "b", "c", "d", "e"}
		truncated := truncateList(items, 3)
		assert.Len(t, truncated, 4)
		assert.Equal(t, "a", truncated[0])
		assert.Equal(t, "b", truncated[1])
		assert.Equal(t, "c", truncated[2])
		assert.Contains(t, truncated[3], "...and 2 more")

		// No truncation needed
		short := []string{"a", "b"}
		notTruncated := truncateList(short, 3)
		assert.Equal(t, short, notTruncated)
	})

	t.Run("categorizeDependencies", func(t *testing.T) {
		imports := map[string]int{
			"fmt":               5,
			"os":                3,
			"github.com/pkg/errors": 2,
			"./internal/utils":  1,
			"@/components":      1,
		}
		categories := categorizeDependencies(imports)

		assert.Contains(t, categories["Standard Library"], "fmt")
		assert.Contains(t, categories["Standard Library"], "os")
		assert.Contains(t, categories["External"], "github.com/pkg/errors")
		assert.Contains(t, categories["Internal"], "./internal/utils")
		assert.Contains(t, categories["Internal"], "@/components")
	})
}

func TestGenerateContext_EmptyProject(t *testing.T) {
	tmpDir := t.TempDir()

	gen := NewGenerator()
	result, err := gen.GenerateContext(tmpDir)
	require.NoError(t, err)

	assert.Equal(t, 0, result.TotalFiles)
	assert.Empty(t, result.Languages)
	assert.Empty(t, result.EntryPoints)
	assert.Contains(t, result.Content, "Total Files**: 0")
}

func TestGenerateTodos_TypeScriptTodos(t *testing.T) {
	tmpDir := t.TempDir()

	content := `// TODO: Implement authentication
export function login() {
  // FIXME: Handle errors properly
  return fetch('/api/login');
}

/* XXX: This needs refactoring */
export const config = {};
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "auth.ts"), []byte(content), 0644))

	gen := NewGenerator()
	result, err := gen.GenerateTodos(tmpDir)
	require.NoError(t, err)

	assert.Equal(t, 3, result.TotalCount)
	assert.Equal(t, 1, result.ByType["TODO"])
	assert.Equal(t, 1, result.ByType["FIXME"])
	assert.Equal(t, 1, result.ByType["XXX"])
}

func TestGenerateTodos_PythonTodos(t *testing.T) {
	tmpDir := t.TempDir()

	content := `# TODO: Add type hints
def process_data(data):
    # FIXME: Handle None case
    # BUG: Memory leak on large datasets
    return data
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "processor.py"), []byte(content), 0644))

	gen := NewGenerator()
	result, err := gen.GenerateTodos(tmpDir)
	require.NoError(t, err)

	assert.Equal(t, 3, result.TotalCount)
	assert.Equal(t, 1, result.ByType["TODO"])
	assert.Equal(t, 1, result.ByType["FIXME"])
	assert.Equal(t, 1, result.ByType["BUG"])
}

func TestIntegration_FullWorkflow(t *testing.T) {
	// Create a realistic project structure
	tmpDir := t.TempDir()
	sessionDir := filepath.Join(tmpDir, ".session")

	// Create directory structure
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "cmd", "server"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "internal", "handler"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "internal", "service"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "internal", "repository"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "pkg", "models"), 0755))

	// Main entry point
	mainContent := `package main

import "fmt"

func main() {
	// TODO: Add proper initialization
	fmt.Println("Server starting...")
}
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "cmd", "server", "main.go"), []byte(mainContent), 0644))

	// Handler
	handlerContent := `package handler

import "net/http"

type Handler interface {
	ServeHTTP(w http.ResponseWriter, r *http.Request)
}

type UserHandler struct {
	service UserService
}

// FIXME: Add validation
func (h *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	// Implementation
}
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "internal", "handler", "user.go"), []byte(handlerContent), 0644))

	// Handler test
	handlerTestContent := `package handler

import "testing"

func TestCreateUser(t *testing.T) {
	// Test implementation
}
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "internal", "handler", "user_test.go"), []byte(handlerTestContent), 0644))

	// Service
	serviceContent := `package service

type UserService interface {
	Create(data CreateUserRequest) (*User, error)
}

type userService struct {
	repo Repository
}
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "internal", "service", "user.go"), []byte(serviceContent), 0644))

	// Repository
	repoContent := `package repository

// TODO: Add connection pooling
type Repository interface {
	FindByID(id string) (interface{}, error)
}
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "internal", "repository", "repository.go"), []byte(repoContent), 0644))

	// Config file
	configContent := `{"port": 8080}`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "config.json"), []byte(configContent), 0644))

	gen := NewGenerator()

	// Generate all files
	context, err := gen.GenerateContext(tmpDir)
	require.NoError(t, err)

	todos, err := gen.GenerateTodos(tmpDir)
	require.NoError(t, err)

	insights, err := gen.GenerateInsights(tmpDir)
	require.NoError(t, err)

	// Save to session
	err = gen.SaveToSession(sessionDir, context, todos, insights)
	require.NoError(t, err)

	// Verify all files were created
	assert.FileExists(t, filepath.Join(sessionDir, "context.md"))
	assert.FileExists(t, filepath.Join(sessionDir, "todos.md"))
	assert.FileExists(t, filepath.Join(sessionDir, "insights.md"))

	// Verify context content
	assert.Contains(t, context.Content, "Entry Points")
	assert.Contains(t, context.Languages, "go")

	// Verify todos content
	assert.Equal(t, 3, todos.TotalCount) // 1 TODO in main + 1 FIXME in handler + 1 TODO in repo
	assert.Equal(t, 2, todos.ByType["TODO"])
	assert.Equal(t, 1, todos.ByType["FIXME"])

	// Verify insights content
	hasArchitectureInsight := false
	hasTestInsight := false
	for _, item := range insights.Items {
		if strings.Contains(item.Title, "Layered Architecture") {
			hasArchitectureInsight = true
		}
		if strings.Contains(item.Title, "Test") {
			hasTestInsight = true
		}
	}
	assert.True(t, hasArchitectureInsight)
	assert.True(t, hasTestInsight)
}
