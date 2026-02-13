// Package agent provides agentic execution capabilities for CortexBrain.
// This file registers agent tools with the capability registrar for discovery
// and invocation through the unified capability system.
package agent

import (
	"context"
	"encoding/json"
	"time"

	"github.com/normanking/cortex/internal/registrar"
)

// RegisterAgentTools registers all agent tools with the registrar.
// This enables tool discovery via intent matching and unified invocation.
//
// Brain alignment: Exposes low-level motor/action capabilities to higher
// cognitive layers through the capability registry, similar to how the
// motor cortex advertises available movement patterns.
func RegisterAgentTools(r *registrar.Registrar) error {
	tools := []*registrar.Capability{
		readFileTool(),
		listDirectoryTool(),
		searchFilesTool(),
		runCommandTool(),
		writeFileTool(),
		webSearchTool(),
	}

	for _, tool := range tools {
		if err := r.Register(tool); err != nil {
			return err
		}
	}

	return nil
}

// readFileTool creates the capability for reading file contents.
func readFileTool() *registrar.Capability {
	return &registrar.Capability{
		ID:          "agent-read-file",
		Name:        "Read File",
		Description: "Read the contents of a file. Use this to read documentation, source code, config files, etc.",
		Version:     "1.0.0",
		Domain:      registrar.DomainStorage,
		Type:        registrar.TypeTool,
		Tags:        []string{"file", "read", "io", "filesystem", "storage"},

		InputTypes:  []string{"path"},
		OutputTypes: []string{"text", "content"},

		IntentPatterns: []string{
			"read file",
			"show file",
			"display file",
			"get file contents",
			"open file",
			"view file",
			"cat file",
			"read documentation",
			"read source code",
			"read config",
			"what does .* contain",
			"show me .*",
		},
		Examples: []string{
			"read the README file",
			"show me config.yaml",
			"what does main.go contain",
			"display the source code",
		},

		Timeout:     30 * time.Second,
		Concurrency: 10, // Allow parallel reads

		Status: registrar.StatusEnabled,

		Handler: placeholderHandler("agent-read-file"),

		Author: "cortexbrain",
		Source: "internal/agent",
		Metadata: map[string]string{
			"risk_level":   "low",
			"original_name": "read_file",
		},
	}
}

// listDirectoryTool creates the capability for listing directory contents.
func listDirectoryTool() *registrar.Capability {
	return &registrar.Capability{
		ID:          "agent-list-directory",
		Name:        "List Directory",
		Description: "List files and directories in a path. Use this to explore project structure.",
		Version:     "1.0.0",
		Domain:      registrar.DomainSystem,
		Type:        registrar.TypeTool,
		Tags:        []string{"directory", "list", "ls", "filesystem", "explore", "navigation"},

		InputTypes:  []string{"path"},
		OutputTypes: []string{"file_list", "directory_list"},

		IntentPatterns: []string{
			"list directory",
			"list files",
			"show directory",
			"ls",
			"what files are in",
			"explore project",
			"show project structure",
			"list folder",
			"directory contents",
			"what's in this folder",
		},
		Examples: []string{
			"list files in the current directory",
			"show the project structure",
			"what files are in src/",
			"ls the directory",
		},

		Timeout:     15 * time.Second,
		Concurrency: 10,

		Status: registrar.StatusEnabled,

		Handler: placeholderHandler("agent-list-directory"),

		Author: "cortexbrain",
		Source: "internal/agent",
		Metadata: map[string]string{
			"risk_level":   "low",
			"original_name": "list_directory",
		},
	}
}

// searchFilesTool creates the capability for searching files by pattern.
func searchFilesTool() *registrar.Capability {
	return &registrar.Capability{
		ID:          "agent-search-files",
		Name:        "Search Files",
		Description: "Search for files matching a pattern. Use this to find documentation, config files, etc.",
		Version:     "1.0.0",
		Domain:      registrar.DomainStorage,
		Type:        registrar.TypeTool,
		Tags:        []string{"search", "find", "glob", "pattern", "filesystem", "discovery"},

		InputTypes:  []string{"pattern", "path"},
		OutputTypes: []string{"file_list", "search_results"},

		IntentPatterns: []string{
			"search files",
			"find files",
			"search for",
			"find .* files",
			"locate files",
			"glob pattern",
			"find documentation",
			"find config files",
			"where is",
			"look for files",
		},
		Examples: []string{
			"find all markdown files",
			"search for config files",
			"find files matching *.go",
			"where are the test files",
		},

		Timeout:     30 * time.Second,
		Concurrency: 5,

		Status: registrar.StatusEnabled,

		Handler: placeholderHandler("agent-search-files"),

		Author: "cortexbrain",
		Source: "internal/agent",
		Metadata: map[string]string{
			"risk_level":   "low",
			"original_name": "search_files",
		},
	}
}

// runCommandTool creates the capability for executing shell commands.
func runCommandTool() *registrar.Capability {
	return &registrar.Capability{
		ID:          "agent-run-command",
		Name:        "Run Command",
		Description: "Execute a shell command. Use this to install dependencies, run builds, start apps, etc.",
		Version:     "1.0.0",
		Domain:      registrar.DomainSystem,
		Type:        registrar.TypeTool,
		Tags:        []string{"shell", "command", "execute", "bash", "terminal", "process"},

		InputTypes:  []string{"command", "working_dir"},
		OutputTypes: []string{"stdout", "stderr", "exit_code"},

		IntentPatterns: []string{
			"run command",
			"execute command",
			"shell command",
			"run .* command",
			"install dependencies",
			"run build",
			"start app",
			"compile",
			"run tests",
			"make",
			"npm",
			"go build",
			"execute script",
		},
		Examples: []string{
			"run go build",
			"execute npm install",
			"run the tests",
			"compile the project",
		},

		Timeout:     120 * time.Second, // Commands may take longer
		Concurrency: 3,                  // Limit concurrent command execution

		Status: registrar.StatusEnabled,

		Handler: placeholderHandler("agent-run-command"),

		Author: "cortexbrain",
		Source: "internal/agent",
		Metadata: map[string]string{
			"risk_level":   "high",
			"original_name": "run_command",
			"requires_confirmation": "true",
		},
	}
}

// writeFileTool creates the capability for writing content to files.
func writeFileTool() *registrar.Capability {
	return &registrar.Capability{
		ID:          "agent-write-file",
		Name:        "Write File",
		Description: "Write content to a file. Use this to create or modify files.",
		Version:     "1.0.0",
		Domain:      registrar.DomainStorage,
		Type:        registrar.TypeTool,
		Tags:        []string{"file", "write", "create", "modify", "io", "filesystem"},

		InputTypes:  []string{"path", "content"},
		OutputTypes: []string{"success", "path"},

		IntentPatterns: []string{
			"write file",
			"create file",
			"save file",
			"modify file",
			"update file",
			"write to",
			"save to",
			"create new file",
			"write content",
			"make file",
		},
		Examples: []string{
			"create a new config file",
			"write the output to results.txt",
			"save the changes to main.go",
			"create README.md",
		},

		Timeout:     30 * time.Second,
		Concurrency: 5,

		Status: registrar.StatusEnabled,

		Handler: placeholderHandler("agent-write-file"),

		Author: "cortexbrain",
		Source: "internal/agent",
		Metadata: map[string]string{
			"risk_level":   "medium",
			"original_name": "write_file",
		},
	}
}

// webSearchTool creates the capability for searching the web.
func webSearchTool() *registrar.Capability {
	return &registrar.Capability{
		ID:          "agent-web-search",
		Name:        "Web Search",
		Description: "Search the web for current information. Use when you need up-to-date information not in your knowledge, recent news, or current documentation.",
		Version:     "1.0.0",
		Domain:      registrar.DomainNetwork,
		Type:        registrar.TypeTool,
		Tags:        []string{"web", "search", "internet", "online", "query", "lookup"},

		InputTypes:  []string{"query"},
		OutputTypes: []string{"search_results", "urls", "snippets"},

		IntentPatterns: []string{
			"search the web",
			"web search",
			"search online",
			"look up",
			"find online",
			"google",
			"search for information",
			"current information",
			"recent news",
			"latest documentation",
			"what is the latest",
		},
		Examples: []string{
			"search for Go best practices 2024",
			"look up the latest Python release",
			"find current documentation for React",
			"search for recent news about AI",
		},

		Timeout:     60 * time.Second, // Network calls may be slow
		Concurrency: 3,                 // Rate limit web searches

		Status: registrar.StatusEnabled,

		Handler: placeholderHandler("agent-web-search"),

		Author: "cortexbrain",
		Source: "internal/agent",
		Metadata: map[string]string{
			"risk_level":   "low",
			"original_name": "web_search",
			"requires_api_key": "TAVILY_API_KEY",
		},
	}
}

// placeholderHandler creates a placeholder handler that returns success.
// Real handlers will be wired in during agent initialization.
func placeholderHandler(toolID string) registrar.CapabilityHandler {
	return func(ctx context.Context, input registrar.CapabilityInput) (registrar.CapabilityOutput, error) {
		response := map[string]interface{}{
			"status":  "placeholder",
			"tool_id": toolID,
			"message": "Handler not yet wired - this is a placeholder",
		}
		data, _ := json.Marshal(response)
		return registrar.CapabilityOutput{
			Type:    "json",
			Data:    data,
			Success: true,
		}, nil
	}
}
