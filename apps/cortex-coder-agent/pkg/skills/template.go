// Package skills provides skill management for the Cortex Coder Agent
package skills

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
)

// Context represents the execution context for skill templates
type Context struct {
	Code         string // Selected code
	FilePath     string // Current file path
	PackageName  string // Go package name
	ProjectType  string // Project type (go, python, node, etc.)
	GitBranch    string // Current git branch
	Selection    string // Text selection
	LineNumber   int    // Current line number
	Language     string // Programming language
	FunctionName string // Current function name
	ClassName    string // Current class name
	ProjectPath  string // Project root path
	UserQuery    string // User's query text
	CommandArgs  []string // Command arguments
	Metadata     map[string]interface{} // Additional metadata
}

// NewContext creates a new skill context
func NewContext() *Context {
	return &Context{
		Metadata: make(map[string]interface{}),
	}
}

// WithCode sets the selected code
func (c *Context) WithCode(code string) *Context {
	c.Code = code
	c.Selection = code
	return c
}

// WithSelection sets the text selection
func (c *Context) WithSelection(selection string) *Context {
	c.Selection = selection
	c.Code = selection
	return c
}

// WithLineNumber sets the line number
func (c *Context) WithLineNumber(line int) *Context {
	c.LineNumber = line
	return c
}

// WithLanguage sets the programming language
func (c *Context) WithLanguage(lang string) *Context {
	c.Language = lang
	return c
}

// WithFilePath sets the file path
func (c *Context) WithFilePath(path string) *Context {
	c.FilePath = path
	c.PackageName = filepath.Base(path)
	return c
}

// WithProjectPath sets the project root path
func (c *Context) WithProjectPath(path string) *Context {
	c.ProjectPath = path
	return c
}

// WithGitBranch sets the git branch
func (c *Context) WithGitBranch(branch string) *Context {
	c.GitBranch = branch
	return c
}

// WithUserQuery sets the user query
func (c *Context) WithUserQuery(query string) *Context {
	c.UserQuery = query
	return c
}

// WithCommandArgs sets command arguments
func (c *Context) WithCommandArgs(args []string) *Context {
	c.CommandArgs = args
	return c
}

// WithFunctionName sets the function name
func (c *Context) WithFunctionName(name string) *Context {
	c.FunctionName = name
	return c
}

// WithClassName sets the class name
func (c *Context) WithClassName(name string) *Context {
	c.ClassName = name
	return c
}

// SetMetadata sets a metadata value
func (c *Context) SetMetadata(key string, value interface{}) *Context {
	c.Metadata[key] = value
	return c
}

// Engine handles template execution for skills
type Engine struct {
	funcs template.FuncMap
}

// NewEngine creates a new template engine
func NewEngine() *Engine {
	e := &Engine{
		funcs: make(template.FuncMap),
	}

	// Register built-in functions
	e.funcs["readFile"] = e.readFileFunc()
	e.funcs["gitDiff"] = e.gitDiffFunc()
	e.funcs["join"] = strings.Join
	e.funcs["basename"] = filepath.Base
	e.funcs["dirname"] = filepath.Dir
	e.funcs["ext"] = filepath.Ext
	e.funcs["quote"] = func(s string) string { return "\"" + s + "\"" }
	e.funcs["indent"] = e.indentFunc()
	e.funcs["toUpper"] = strings.ToUpper
	e.funcs["toLower"] = strings.ToLower
	e.funcs["title"] = strings.Title

	return e
}

// Execute parses and executes a template with the given context
func (e *Engine) Execute(tmpl string, ctx *Context) (string, error) {
	t, err := template.New("skill").Funcs(e.funcs).Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, ctx); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// ExecuteFile executes a template file with the given context
func (e *Engine) ExecuteFile(path string, ctx *Context) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read template file: %w", err)
	}

	return e.Execute(string(data), ctx)
}

// readFileFunc returns a function that reads a file
func (e *Engine) readFileFunc() interface{} {
	return func(args ...string) (string, error) {
		if len(args) == 0 {
			return "", fmt.Errorf("readFile requires at least one argument")
		}

		path := args[0]
		// If path is relative, make it relative to project path
		if !filepath.IsAbs(path) && args[1] != "" {
			path = filepath.Join(args[1], path)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("failed to read file %s: %w", path, err)
		}

		// Optional: limit the number of lines
		if len(args) > 2 {
			var n int
			fmt.Sscanf(args[2], "%d", &n)
			if n > 0 {
				lines := strings.Split(string(data), "\n")
				if n < len(lines) {
					return strings.Join(lines[:n], "\n"), nil
				}
			}
		}

		return string(data), nil
	}
}

// gitDiffFunc returns a function that gets git diff
func (e *Engine) gitDiffFunc() interface{} {
	return func(args ...string) (string, error) {
		// Default: show diff for current file
		var file string
		if len(args) > 0 {
			file = args[0]
		}

		cmd := exec.Command("git", "diff", file)
		output, err := cmd.Output()
		if err != nil {
			// Return empty string if not in git repo or no changes
			return "", nil
		}

		return string(output), nil
	}
}

// indentFunc returns a function that indents text
func (e *Engine) indentFunc() interface{} {
	return func(indent string, text string) string {
		lines := strings.Split(text, "\n")
		result := make([]string, len(lines))
		for i, line := range lines {
			if line != "" {
				result[i] = indent + line
			} else {
				result[i] = line
			}
		}
		return strings.Join(result, "\n")
	}
}

// AddFunc adds a custom function to the template engine
func (e *Engine) AddFunc(name string, fn interface{}) {
	e.funcs[name] = fn
}

// SetFuncs sets multiple custom functions
func (e *Engine) SetFuncs(funcs template.FuncMap) {
	for k, v := range funcs {
		e.funcs[k] = v
	}
}
