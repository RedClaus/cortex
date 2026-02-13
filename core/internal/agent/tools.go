// Package agent provides agentic capabilities for Cortex.
// It enables the LLM to use tools like reading files, executing commands,
// and performing multi-step tasks autonomously.
package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/normanking/cortex/internal/logging"
)

// ═══════════════════════════════════════════════════════════════════════════════
// TOOL DEFINITIONS
// ═══════════════════════════════════════════════════════════════════════════════

// Tool represents a tool the agent can use.
type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  []Parameter `json:"parameters"`
}

// Parameter describes a tool parameter.
type Parameter struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

// ToolCall represents a request to execute a tool.
type ToolCall struct {
	Name   string            `json:"name"`
	Params map[string]string `json:"params"`
}

// ToolResult represents the result of a tool execution.
type ToolResult struct {
	Tool    string `json:"tool"`
	Success bool   `json:"success"`
	Output  string `json:"output"`
	Error   string `json:"error,omitempty"`
}

// AvailableTools returns all tools the agent can use.
func AvailableTools() []Tool {
	return []Tool{
		{
			Name:        "read_file",
			Description: "Read the contents of a file. Use this to read documentation, source code, config files, etc.",
			Parameters: []Parameter{
				{Name: "path", Type: "string", Description: "Path to the file to read (relative or absolute)", Required: true},
			},
		},
		{
			Name:        "list_directory",
			Description: "List files and directories in a path. Use this to explore project structure.",
			Parameters: []Parameter{
				{Name: "path", Type: "string", Description: "Directory path to list (default: current directory)", Required: false},
			},
		},
		{
			Name:        "search_files",
			Description: "Search for files matching a pattern. Use this to find documentation, config files, etc.",
			Parameters: []Parameter{
				{Name: "pattern", Type: "string", Description: "Glob pattern like '*.md' or 'README*'", Required: true},
				{Name: "path", Type: "string", Description: "Directory to search in (default: current directory)", Required: false},
			},
		},
		{
			Name:        "run_command",
			Description: "Execute a shell command. Use this to install dependencies, run builds, start apps, etc.",
			Parameters: []Parameter{
				{Name: "command", Type: "string", Description: "The shell command to execute", Required: true},
				{Name: "working_dir", Type: "string", Description: "Working directory for the command (optional)", Required: false},
			},
		},
		{
			Name:        "write_file",
			Description: "Write content to a file. Use this to create or modify files.",
			Parameters: []Parameter{
				{Name: "path", Type: "string", Description: "Path to the file to write", Required: true},
				{Name: "content", Type: "string", Description: "Content to write to the file", Required: true},
			},
		},
		{
			Name:        "web_search",
			Description: "Search the web for current information. Use when you need up-to-date information not in your knowledge, recent news, or current documentation.",
			Parameters: []Parameter{
				{Name: "query", Type: "string", Description: "Search query - be specific and include technical terms", Required: true},
			},
		},
		// Memory tools (MemGPT-style)
		{
			Name:        "recall_memory_search",
			Description: "Search past conversations for relevant context. Use when you need information from previous interactions with the user.",
			Parameters: []Parameter{
				{Name: "query", Type: "string", Description: "What to search for in conversation history", Required: true},
				{Name: "limit", Type: "integer", Description: "Maximum results to return (default: 5)", Required: false},
			},
		},
		{
			Name:        "core_memory_read",
			Description: "Read persistent facts about the user or current project. Use to recall user preferences, environment, or project details.",
			Parameters: []Parameter{
				{Name: "section", Type: "string", Description: "Which section to read: 'user' for user facts, 'project' for project context", Required: true},
			},
		},
		{
			Name:        "core_memory_append",
			Description: "Remember a new fact about the user. Use when learning user preferences, environment details, or important context.",
			Parameters: []Parameter{
				{Name: "fact", Type: "string", Description: "The fact to remember about the user", Required: true},
			},
		},
		{
			Name:        "archival_memory_search",
			Description: "Search the long-term knowledge base for lessons, solutions, and learned information. Use when you need historical solutions or accumulated knowledge.",
			Parameters: []Parameter{
				{Name: "query", Type: "string", Description: "What to search for in the knowledge base", Required: true},
				{Name: "scope", Type: "string", Description: "Limit search to: 'personal', 'team', 'global', or leave empty for all", Required: false},
				{Name: "limit", Type: "integer", Description: "Maximum results to return (default: 5)", Required: false},
			},
		},
		{
			Name:        "archival_memory_insert",
			Description: "Store a lesson or solution in the long-term knowledge base. Use after successfully solving a problem to help with similar tasks in the future.",
			Parameters: []Parameter{
				{Name: "content", Type: "string", Description: "The lesson or solution to store", Required: true},
				{Name: "title", Type: "string", Description: "Short title for the knowledge item", Required: false},
				{Name: "tags", Type: "string", Description: "Comma-separated tags for categorization", Required: false},
			},
		},
	}
}

// ToolsDescription returns a formatted string describing all available tools.
func ToolsDescription() string {
	var sb strings.Builder
	sb.WriteString("## Available Tools\n\n")
	sb.WriteString("You can use these tools by including a tool call in your response.\n")
	sb.WriteString("Format: <tool>tool_name</tool><params>{\"param\": \"value\"}</params>\n\n")

	for _, tool := range AvailableTools() {
		sb.WriteString(fmt.Sprintf("### %s\n", tool.Name))
		sb.WriteString(fmt.Sprintf("%s\n", tool.Description))
		sb.WriteString("Parameters:\n")
		for _, p := range tool.Parameters {
			req := ""
			if p.Required {
				req = " (required)"
			}
			sb.WriteString(fmt.Sprintf("  - %s (%s)%s: %s\n", p.Name, p.Type, req, p.Description))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// ═══════════════════════════════════════════════════════════════════════════════
// TOOL EXECUTOR
// ═══════════════════════════════════════════════════════════════════════════════

// MemoryToolsInterface defines the interface for memory operations.
// This allows the agent to access memory tools without importing the memory package.
type MemoryToolsInterface interface {
	ExecuteTool(ctx context.Context, userID, toolName, argsJSON string) (*MemoryToolResult, error)
}

// MemoryToolResult mirrors memory.ToolResult for the agent package.
type MemoryToolResult struct {
	Success   bool   `json:"success"`
	Result    string `json:"result"`
	Error     string `json:"error,omitempty"`
	LatencyMs int64  `json:"latency_ms"`
}

// Executor handles tool execution for the agent.
type Executor struct {
	workingDir     string
	log            *logging.Logger
	memoryTools    MemoryToolsInterface // Optional memory tools interface
	userID         string               // User ID for memory operations
	processCleanup *ProcessCleanup      // Track spawned processes for cleanup
}

// NewExecutor creates a new tool executor.
func NewExecutor(workingDir string) *Executor {
	if workingDir == "" {
		workingDir, _ = os.Getwd()
	}
	cortexDir := filepath.Join(os.Getenv("HOME"), ".cortex")
	return &Executor{
		workingDir:     workingDir,
		log:            logging.Global(),
		userID:         "default-user", // Default user ID for memory operations
		processCleanup: NewProcessCleanup(cortexDir),
	}
}

// SetMemoryTools configures memory tools for the executor.
func (e *Executor) SetMemoryTools(mt MemoryToolsInterface, userID string) {
	e.memoryTools = mt
	if userID != "" {
		e.userID = userID
	}
}

// Execute runs a tool and returns the result.
func (e *Executor) Execute(ctx context.Context, call *ToolCall) *ToolResult {
	e.log.Debug("[Agent] Executing tool: %s with params: %v", call.Name, call.Params)

	switch call.Name {
	case "read_file":
		return e.readFile(call.Params["path"])
	case "list_directory":
		return e.listDirectory(call.Params["path"])
	case "search_files":
		return e.searchFiles(call.Params["pattern"], call.Params["path"])
	case "run_command":
		return e.runCommand(ctx, call.Params["command"], call.Params["working_dir"])
	case "write_file":
		return e.writeFile(call.Params["path"], call.Params["content"])
	case "web_search":
		return e.webSearch(ctx, call.Params["query"])
	// MemGPT-style memory tools
	case "recall_memory_search":
		return e.executeMemoryTool(ctx, call)
	case "core_memory_read":
		return e.executeMemoryTool(ctx, call)
	case "core_memory_append":
		return e.executeMemoryTool(ctx, call)
	case "archival_memory_search":
		return e.executeMemoryTool(ctx, call)
	case "archival_memory_insert":
		return e.executeMemoryTool(ctx, call)
	default:
		return &ToolResult{
			Tool:    call.Name,
			Success: false,
			Error:   fmt.Sprintf("Unknown tool: %s", call.Name),
		}
	}
}

// executeMemoryTool handles all memory tool calls by delegating to the memory tools interface.
func (e *Executor) executeMemoryTool(ctx context.Context, call *ToolCall) *ToolResult {
	if e.memoryTools == nil {
		e.log.Warn("[Agent] Memory tools not configured, cannot execute %s", call.Name)
		return &ToolResult{
			Tool:    call.Name,
			Success: false,
			Error:   "Memory tools not configured. Memory features are disabled.",
		}
	}

	// Convert params to JSON for the memory tools interface
	argsJSON, err := json.Marshal(call.Params)
	if err != nil {
		return &ToolResult{
			Tool:    call.Name,
			Success: false,
			Error:   fmt.Sprintf("Failed to marshal params: %v", err),
		}
	}

	e.log.Info("[Agent] Executing memory tool: %s (user: %s)", call.Name, e.userID)

	// Execute the memory tool
	result, err := e.memoryTools.ExecuteTool(ctx, e.userID, call.Name, string(argsJSON))
	if err != nil {
		e.log.Error("[Agent] Memory tool %s failed: %v", call.Name, err)
		return &ToolResult{
			Tool:    call.Name,
			Success: false,
			Error:   fmt.Sprintf("Memory tool execution failed: %v", err),
		}
	}

	e.log.Info("[Agent] Memory tool %s completed (success=%v, latency=%dms)", call.Name, result.Success, result.LatencyMs)

	return &ToolResult{
		Tool:    call.Name,
		Success: result.Success,
		Output:  result.Result,
		Error:   result.Error,
	}
}

// readFile reads a file and returns its contents.
func (e *Executor) readFile(path string) *ToolResult {
	if path == "" {
		return &ToolResult{Tool: "read_file", Success: false, Error: "path parameter is required"}
	}

	// Resolve relative paths
	if !filepath.IsAbs(path) {
		path = filepath.Join(e.workingDir, path)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return &ToolResult{
			Tool:    "read_file",
			Success: false,
			Error:   fmt.Sprintf("Failed to read file: %v", err),
		}
	}

	// Truncate very large files
	output := string(content)
	if len(output) > 50000 {
		output = output[:50000] + "\n\n[... truncated, file too large ...]"
	}

	return &ToolResult{
		Tool:    "read_file",
		Success: true,
		Output:  output,
	}
}

// listDirectory lists contents of a directory.
func (e *Executor) listDirectory(path string) *ToolResult {
	if path == "" {
		path = e.workingDir
	} else if !filepath.IsAbs(path) {
		path = filepath.Join(e.workingDir, path)
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return &ToolResult{
			Tool:    "list_directory",
			Success: false,
			Error:   fmt.Sprintf("Failed to list directory: %v", err),
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Contents of %s:\n\n", path))

	for _, entry := range entries {
		info, _ := entry.Info()
		typeIndicator := "  "
		if entry.IsDir() {
			typeIndicator = "📁"
		} else {
			typeIndicator = "📄"
		}

		size := ""
		if info != nil && !entry.IsDir() {
			size = fmt.Sprintf(" (%d bytes)", info.Size())
		}

		sb.WriteString(fmt.Sprintf("%s %s%s\n", typeIndicator, entry.Name(), size))
	}

	return &ToolResult{
		Tool:    "list_directory",
		Success: true,
		Output:  sb.String(),
	}
}

// searchFiles searches for files matching a pattern.
// Has safeguards to prevent hanging on large directory trees.
func (e *Executor) searchFiles(pattern, path string) *ToolResult {
	if pattern == "" {
		return &ToolResult{Tool: "search_files", Success: false, Error: "pattern parameter is required"}
	}

	if path == "" {
		path = e.workingDir
	} else if !filepath.IsAbs(path) {
		path = filepath.Join(e.workingDir, path)
	}

	const maxResults = 100        // Stop after finding this many matches
	const maxDepth = 5            // Don't recurse deeper than this
	const maxFilesScanned = 10000 // Stop after scanning this many files

	var matches []string
	var filesScanned int
	var hitLimit bool
	basePath := path

	err := filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Check depth limit
		relPath, _ := filepath.Rel(basePath, p)
		depth := strings.Count(relPath, string(filepath.Separator))
		if depth > maxDepth {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if info.IsDir() {
			// Skip hidden directories and common non-essential dirs
			name := info.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" || name == "Library" {
				return filepath.SkipDir
			}
			return nil
		}

		filesScanned++
		if filesScanned > maxFilesScanned {
			hitLimit = true
			return filepath.SkipAll
		}

		matched, _ := filepath.Match(pattern, info.Name())
		if matched {
			displayPath, _ := filepath.Rel(basePath, p)
			matches = append(matches, displayPath)
			if len(matches) >= maxResults {
				hitLimit = true
				return filepath.SkipAll
			}
		}
		return nil
	})

	if err != nil && err != filepath.SkipAll {
		return &ToolResult{
			Tool:    "search_files",
			Success: false,
			Error:   fmt.Sprintf("Search failed: %v", err),
		}
	}

	if len(matches) == 0 {
		return &ToolResult{
			Tool:    "search_files",
			Success: true,
			Output:  fmt.Sprintf("No files found matching pattern '%s' in %s (searched %d files, max depth %d)", pattern, path, filesScanned, maxDepth),
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d files matching '%s':\n\n", len(matches), pattern))
	for _, m := range matches {
		sb.WriteString(fmt.Sprintf("  %s\n", m))
	}
	if hitLimit {
		sb.WriteString(fmt.Sprintf("\n(Search limited: scanned %d files, max depth %d)", filesScanned, maxDepth))
	}

	return &ToolResult{
		Tool:    "search_files",
		Success: true,
		Output:  sb.String(),
	}
}

// CommandTimeout is the default timeout for command execution.
// Commands that may take longer should be run with explicit timeouts or limits.
const CommandTimeout = 30 * time.Second

// runCommand executes a shell command with a 30-second timeout.
// For commands that may take longer, users should add their own timeout or limit.
func (e *Executor) runCommand(ctx context.Context, command, workingDir string) *ToolResult {
	if command == "" {
		return &ToolResult{Tool: "run_command", Success: false, Error: "command parameter is required"}
	}

	if workingDir == "" {
		workingDir = e.workingDir
	} else if !filepath.IsAbs(workingDir) {
		workingDir = filepath.Join(e.workingDir, workingDir)
	}

	e.log.Info("[Agent] Running command: %s in %s", command, workingDir)

	// Create command with 30-second timeout (prevents hanging on slow commands)
	cmdCtx, cancel := context.WithTimeout(ctx, CommandTimeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "sh", "-c", command)
	cmd.Dir = workingDir

	// Use process group so we can kill all child processes on timeout
	// This is critical for piped commands like "du ... | sort" where killing
	// just the shell doesn't stop the subprocess
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// Run command with timeout monitoring
	startTime := time.Now()

	// Use a channel to handle the command completion
	var output []byte
	var cmdErr error
	done := make(chan struct{})

	// Capture output via pipes since we need to track the process
	var outputBuf bytes.Buffer
	cmd.Stdout = &outputBuf
	cmd.Stderr = &outputBuf

	// Start command in background
	if err := cmd.Start(); err != nil {
		return &ToolResult{
			Tool:    "run_command",
			Success: false,
			Error:   fmt.Sprintf("Failed to start command: %v", err),
		}
	}

	// Track the process for cleanup if we crash
	if e.processCleanup != nil && cmd.Process != nil {
		e.processCleanup.TrackProcess(cmd.Process.Pid, command)
	}

	go func() {
		cmdErr = cmd.Wait()
		output = outputBuf.Bytes()
		close(done)
	}()

	// Wait for either completion or timeout
	select {
	case <-done:
		// Command completed normally - untrack the process
		if e.processCleanup != nil && cmd.Process != nil {
			e.processCleanup.UntrackProcess(cmd.Process.Pid)
		}
	case <-cmdCtx.Done():
		// Timeout - kill the entire process group
		if cmd.Process != nil {
			// Untrack before killing
			if e.processCleanup != nil {
				e.processCleanup.UntrackProcess(cmd.Process.Pid)
			}
			// Kill the process group (negative PID kills the group)
			syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		// Wait for the goroutine to finish
		<-done
	}

	elapsed := time.Since(startTime)

	result := &ToolResult{
		Tool:   "run_command",
		Output: string(output),
	}

	if cmdErr != nil {
		// Check if it was a timeout
		if cmdCtx.Err() == context.DeadlineExceeded {
			result.Success = false
			result.Error = fmt.Sprintf("Command timed out after %.0f seconds. Try a more specific query or limit output (e.g., 'ls -lhS ~/ | head -20' instead of 'du -sh ~/*')", elapsed.Seconds())
			if len(output) > 0 {
				result.Output = string(output) + "\n\n[Command timed out - partial output shown above]"
			}
		} else if ctx.Err() != nil {
			// Parent context was cancelled (user interrupted)
			result.Success = false
			result.Error = "Command interrupted by user"
		} else {
			result.Success = false
			result.Error = fmt.Sprintf("Command failed: %v", cmdErr)
			if len(output) > 0 {
				result.Output = string(output)
			}
		}
	} else {
		result.Success = true
		// Add timing info for commands that took a while
		if elapsed > 5*time.Second {
			result.Output = fmt.Sprintf("[Completed in %.1fs]\n%s", elapsed.Seconds(), result.Output)
		}

		// If this was a cd command, update the working directory persistently
		// We look for "cd X && pwd" pattern and extract the output (which is the new directory)
		if strings.HasPrefix(strings.TrimSpace(command), "cd ") && strings.Contains(command, "&& pwd") {
			newDir := strings.TrimSpace(result.Output)
			if newDir != "" && filepath.IsAbs(newDir) {
				e.workingDir = newDir
				e.log.Info("[Agent] Working directory updated to: %s", newDir)
			}
		}
	}

	// Truncate very long output
	if len(result.Output) > 20000 {
		result.Output = result.Output[:20000] + "\n\n[... output truncated ...]"
	}

	return result
}

// writeFile writes content to a file.
func (e *Executor) writeFile(path, content string) *ToolResult {
	if path == "" {
		return &ToolResult{Tool: "write_file", Success: false, Error: "path parameter is required"}
	}
	if content == "" {
		return &ToolResult{Tool: "write_file", Success: false, Error: "content parameter is required"}
	}

	if !filepath.IsAbs(path) {
		path = filepath.Join(e.workingDir, path)
	}

	// Create parent directories if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return &ToolResult{
			Tool:    "write_file",
			Success: false,
			Error:   fmt.Sprintf("Failed to create directory: %v", err),
		}
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return &ToolResult{
			Tool:    "write_file",
			Success: false,
			Error:   fmt.Sprintf("Failed to write file: %v", err),
		}
	}

	return &ToolResult{
		Tool:    "write_file",
		Success: true,
		Output:  fmt.Sprintf("Successfully wrote %d bytes to %s", len(content), path),
	}
}

// webSearch searches the web using the Tavily API.
func (e *Executor) webSearch(ctx context.Context, query string) *ToolResult {
	if query == "" {
		return &ToolResult{Tool: "web_search", Success: false, Error: "query parameter is required"}
	}

	apiKey := os.Getenv("TAVILY_API_KEY")
	if apiKey == "" {
		return &ToolResult{
			Tool:    "web_search",
			Success: false,
			Error:   "TAVILY_API_KEY not configured. Set it via /setkey tavily or in ~/.cortex/.env",
		}
	}

	e.log.Info("[Agent] Searching web for: %s", query)

	// Build Tavily request
	reqBody := map[string]interface{}{
		"api_key":        apiKey,
		"query":          query,
		"search_depth":   "basic",
		"max_results":    5,
		"include_answer": true,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return &ToolResult{Tool: "web_search", Success: false, Error: fmt.Sprintf("Failed to build request: %v", err)}
	}

	// Make HTTP request with timeout
	client := &http.Client{Timeout: 30 * time.Second}
	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.tavily.com/search", bytes.NewReader(body))
	if err != nil {
		return &ToolResult{Tool: "web_search", Success: false, Error: fmt.Sprintf("Failed to create request: %v", err)}
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(httpReq)
	if err != nil {
		return &ToolResult{Tool: "web_search", Success: false, Error: fmt.Sprintf("Search request failed: %v", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &ToolResult{Tool: "web_search", Success: false, Error: fmt.Sprintf("Search API returned status %d", resp.StatusCode)}
	}

	// Parse response
	var result struct {
		Answer  string `json:"answer"`
		Query   string `json:"query"`
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return &ToolResult{Tool: "web_search", Success: false, Error: fmt.Sprintf("Failed to parse response: %v", err)}
	}

	// Format results as XML for prompt injection defense
	var sb strings.Builder
	sb.WriteString("<web_search_results>\n")

	if result.Answer != "" {
		sb.WriteString("  <summary>\n")
		sb.WriteString(fmt.Sprintf("    %s\n", escapeXMLContent(result.Answer)))
		sb.WriteString("  </summary>\n")
	}

	sb.WriteString("  <sources>\n")
	for i, r := range result.Results {
		sb.WriteString(fmt.Sprintf("    <source rank=\"%d\">\n", i+1))
		sb.WriteString(fmt.Sprintf("      <title>%s</title>\n", escapeXMLContent(r.Title)))
		sb.WriteString(fmt.Sprintf("      <url>%s</url>\n", escapeXMLContent(r.URL)))
		content := r.Content
		if len(content) > 500 {
			content = content[:497] + "..."
		}
		sb.WriteString(fmt.Sprintf("      <content>%s</content>\n", escapeXMLContent(content)))
		sb.WriteString("    </source>\n")
	}
	sb.WriteString("  </sources>\n")
	sb.WriteString("</web_search_results>")

	e.log.Info("[Agent] Web search returned %d results", len(result.Results))

	return &ToolResult{
		Tool:    "web_search",
		Success: true,
		Output:  sb.String(),
	}
}

// escapeXMLContent escapes special characters for XML.
func escapeXMLContent(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}

// ═══════════════════════════════════════════════════════════════════════════════
// TOOL CALL PARSING
// ═══════════════════════════════════════════════════════════════════════════════

// ParseToolCalls extracts tool calls from LLM response text.
// Looks for patterns like: <tool>tool_name</tool><params>{"key": "value"}</params>
// Also handles alternate format: <tool_name>params</tool_name>
func ParseToolCalls(response string) ([]*ToolCall, string) {
	var calls []*ToolCall
	cleanedResponse := response

	// Try the canonical format first
	calls, cleanedResponse = parseCanonicalToolCalls(cleanedResponse)

	// If no canonical calls found, try alternate formats
	if len(calls) == 0 {
		calls, cleanedResponse = parseAlternateToolCalls(cleanedResponse)
	}

	return calls, strings.TrimSpace(cleanedResponse)
}

// parseCanonicalToolCalls parses the canonical format: <tool>name</tool><params>{...}</params>
func parseCanonicalToolCalls(response string) ([]*ToolCall, string) {
	var calls []*ToolCall
	cleanedResponse := response

	// Find all tool calls
	for {
		toolStart := strings.Index(cleanedResponse, "<tool>")
		if toolStart == -1 {
			break
		}

		toolEnd := strings.Index(cleanedResponse[toolStart:], "</tool>")
		if toolEnd == -1 {
			break
		}
		toolEnd += toolStart

		paramsStart := strings.Index(cleanedResponse[toolEnd:], "<params>")
		if paramsStart == -1 {
			break
		}
		paramsStart += toolEnd

		paramsEnd := strings.Index(cleanedResponse[paramsStart:], "</params>")
		if paramsEnd == -1 {
			break
		}
		paramsEnd += paramsStart

		// Extract tool name and params
		toolName := cleanedResponse[toolStart+6 : toolEnd]
		paramsJSON := cleanedResponse[paramsStart+8 : paramsEnd]

		// Clean up common LLM output errors:
		// 1. Trailing > before </params> (e.g., {"key":"value"}>)
		// 2. Leading/trailing whitespace
		paramsJSON = strings.TrimSpace(paramsJSON)
		paramsJSON = strings.TrimSuffix(paramsJSON, ">")
		paramsJSON = strings.TrimPrefix(paramsJSON, "<")

		// Parse params
		var params map[string]string
		if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
			// Try to extract just the JSON object if there's extra content
			if start := strings.Index(paramsJSON, "{"); start >= 0 {
				if end := strings.LastIndex(paramsJSON, "}"); end > start {
					paramsJSON = paramsJSON[start : end+1]
					if err2 := json.Unmarshal([]byte(paramsJSON), &params); err2 == nil {
						// Successfully parsed after cleanup
					} else {
						params = make(map[string]string)
					}
				} else {
					params = make(map[string]string)
				}
			} else {
				params = make(map[string]string)
			}
		}

		calls = append(calls, &ToolCall{
			Name:   strings.TrimSpace(toolName),
			Params: params,
		})

		// Remove the tool call from response
		cleanedResponse = cleanedResponse[:toolStart] + cleanedResponse[paramsEnd+9:]
	}

	return calls, cleanedResponse
}

// parseAlternateToolCalls parses alternate format: <tool_name>params</tool_name>
// This handles cases where LLMs output <web_search>query="..."</web_search>
func parseAlternateToolCalls(response string) ([]*ToolCall, string) {
	var calls []*ToolCall
	cleanedResponse := response

	// Known tool names to look for
	toolNames := []string{"web_search", "read_file", "list_directory", "search_files", "run_command", "edit_file", "create_file"}

	for _, toolName := range toolNames {
		openTag := "<" + toolName + ">"
		closeTag := "</" + toolName + ">"

		for {
			start := strings.Index(cleanedResponse, openTag)
			if start == -1 {
				break
			}

			end := strings.Index(cleanedResponse[start:], closeTag)
			if end == -1 {
				break
			}
			end += start

			// Extract params content between tags
			paramsContent := cleanedResponse[start+len(openTag) : end]
			paramsContent = strings.TrimSpace(paramsContent)

			params := make(map[string]string)

			// Try to parse as JSON first
			if strings.HasPrefix(paramsContent, "{") {
				json.Unmarshal([]byte(paramsContent), &params)
			} else {
				// Parse key="value" or key=value format
				parts := strings.Split(paramsContent, " ")
				for _, part := range parts {
					if idx := strings.Index(part, "="); idx > 0 {
						key := part[:idx]
						val := strings.Trim(part[idx+1:], "\"'")
						params[key] = val
					} else if len(parts) == 1 {
						// Single value without key - assume it's the main parameter
						params["query"] = strings.Trim(part, "\"'")
					}
				}
			}

			calls = append(calls, &ToolCall{
				Name:   toolName,
				Params: params,
			})

			// Remove the tool call from response
			cleanedResponse = cleanedResponse[:start] + cleanedResponse[end+len(closeTag):]
		}
	}

	return calls, cleanedResponse
}
