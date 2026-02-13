package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// ===========================================================================
// READ TOOL
// ===========================================================================

// ReadTool reads file contents.
type ReadTool struct {
	// MaxFileSize limits readable file size (default: 10MB)
	maxFileSize int64

	// AllowedExtensions restricts readable file types (empty = all)
	allowedExtensions []string

	// BlockedPaths are paths that cannot be read
	blockedPaths []*regexp.Regexp
}

// ReadOption configures the ReadTool.
type ReadOption func(*ReadTool)

// WithMaxFileSize sets the maximum readable file size.
func WithMaxFileSize(size int64) ReadOption {
	return func(r *ReadTool) {
		r.maxFileSize = size
	}
}

// WithAllowedExtensions restricts readable file types.
func WithAllowedExtensions(exts []string) ReadOption {
	return func(r *ReadTool) {
		r.allowedExtensions = exts
	}
}

// NewReadTool creates a new file read tool.
func NewReadTool(opts ...ReadOption) *ReadTool {
	r := &ReadTool{
		maxFileSize: 10 * 1024 * 1024, // 10MB default
		blockedPaths: []*regexp.Regexp{
			regexp.MustCompile(`/etc/shadow`),
			regexp.MustCompile(`/etc/passwd`),
			regexp.MustCompile(`\.ssh/id_`),
			regexp.MustCompile(`\.ssh/authorized_keys`),
			regexp.MustCompile(`\.aws/credentials`),
			regexp.MustCompile(`\.kube/config`),
			regexp.MustCompile(`\.netrc`),
			regexp.MustCompile(`\.npmrc`),
			regexp.MustCompile(`\.pypirc`),
			regexp.MustCompile(`\.env$`),
			regexp.MustCompile(`\.env\.local$`),
			regexp.MustCompile(`credentials\.json$`),
			regexp.MustCompile(`secrets\.ya?ml$`),
		},
	}

	for _, opt := range opts {
		opt(r)
	}

	return r
}

func (r *ReadTool) Name() ToolType { return ToolRead }

func (r *ReadTool) Validate(req *ToolRequest) error {
	if req.Tool != ToolRead {
		return fmt.Errorf("wrong tool type: expected %s, got %s", ToolRead, req.Tool)
	}

	if strings.TrimSpace(req.Input) == "" {
		return fmt.Errorf("file path cannot be empty")
	}

	// Resolve path
	path := req.Input
	if req.WorkingDir != "" && !filepath.IsAbs(path) {
		path = filepath.Join(req.WorkingDir, path)
	}

	// Check existence
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("file not found: %s", path)
	}

	if info.IsDir() {
		return fmt.Errorf("path is a directory, not a file: %s", path)
	}

	// Check size
	if info.Size() > r.maxFileSize {
		return fmt.Errorf("file too large: %d bytes (max: %d)", info.Size(), r.maxFileSize)
	}

	// Check extension restrictions
	if len(r.allowedExtensions) > 0 {
		ext := strings.ToLower(filepath.Ext(path))
		allowed := false
		for _, allowedExt := range r.allowedExtensions {
			if ext == allowedExt {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("file extension %s not allowed", ext)
		}
	}

	return nil
}

func (r *ReadTool) AssessRisk(req *ToolRequest) RiskLevel {
	path := resolvePath(req.Input, req.WorkingDir)

	// Check blocked paths
	for _, pattern := range r.blockedPaths {
		if pattern.MatchString(path) {
			return RiskHigh // Sensitive file access
		}
	}

	// Check for system files
	if strings.HasPrefix(path, "/etc/") || strings.HasPrefix(path, "/sys/") ||
		strings.HasPrefix(path, "/proc/") {
		return RiskMedium
	}

	return RiskNone
}

func (r *ReadTool) Execute(ctx context.Context, req *ToolRequest) (*ToolResult, error) {
	start := time.Now()
	path := resolvePath(req.Input, req.WorkingDir)

	// Read file
	content, err := os.ReadFile(path)
	if err != nil {
		return &ToolResult{
			Tool:     ToolRead,
			Success:  false,
			Error:    err.Error(),
			Duration: time.Since(start),
		}, err
	}

	// Get file info for metadata
	info, _ := os.Stat(path)
	var metadata map[string]interface{}
	if info != nil {
		metadata = map[string]interface{}{
			"size":     info.Size(),
			"mode":     info.Mode().String(),
			"mod_time": info.ModTime().Format(time.RFC3339),
		}
	}

	return &ToolResult{
		Tool:     ToolRead,
		Success:  true,
		Output:   string(content),
		Duration: time.Since(start),
		Metadata: metadata,
	}, nil
}

// ===========================================================================
// WRITE TOOL
// ===========================================================================

// WriteTool writes content to files.
type WriteTool struct {
	// CreateDirs if true, creates parent directories
	createDirs bool

	// BackupExisting if true, creates .bak before overwriting
	backupExisting bool

	// BlockedPaths are paths that cannot be written
	blockedPaths []*regexp.Regexp
}

// WriteOption configures the WriteTool.
type WriteOption func(*WriteTool)

// WithCreateDirs enables automatic directory creation.
func WithCreateDirs(create bool) WriteOption {
	return func(w *WriteTool) {
		w.createDirs = create
	}
}

// WithBackupExisting enables backup before overwrite.
func WithBackupExisting(backup bool) WriteOption {
	return func(w *WriteTool) {
		w.backupExisting = backup
	}
}

// NewWriteTool creates a new file write tool.
func NewWriteTool(opts ...WriteOption) *WriteTool {
	w := &WriteTool{
		createDirs:     true,
		backupExisting: false,
		blockedPaths: []*regexp.Regexp{
			regexp.MustCompile(`^/etc/`),
			regexp.MustCompile(`^/usr/`),
			regexp.MustCompile(`^/bin/`),
			regexp.MustCompile(`^/sbin/`),
			regexp.MustCompile(`^/boot/`),
			regexp.MustCompile(`^/sys/`),
			regexp.MustCompile(`^/proc/`),
			regexp.MustCompile(`^/dev/`),
			regexp.MustCompile(`\.ssh/`),
		},
	}

	for _, opt := range opts {
		opt(w)
	}

	return w
}

func (w *WriteTool) Name() ToolType { return ToolWrite }

func (w *WriteTool) Validate(req *ToolRequest) error {
	if req.Tool != ToolWrite {
		return fmt.Errorf("wrong tool type: expected %s, got %s", ToolWrite, req.Tool)
	}

	if strings.TrimSpace(req.Input) == "" {
		return fmt.Errorf("file path cannot be empty")
	}

	// Content is in params
	if req.Params == nil {
		return fmt.Errorf("params required (content)")
	}

	if _, ok := req.Params["content"]; !ok {
		return fmt.Errorf("content parameter required")
	}

	path := resolvePath(req.Input, req.WorkingDir)

	// Check blocked paths
	for _, pattern := range w.blockedPaths {
		if pattern.MatchString(path) {
			return fmt.Errorf("path is blocked: %s", path)
		}
	}

	// Check parent directory exists or can be created
	dir := filepath.Dir(path)
	if !w.createDirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			return fmt.Errorf("parent directory does not exist: %s", dir)
		}
	}

	return nil
}

func (w *WriteTool) AssessRisk(req *ToolRequest) RiskLevel {
	path := resolvePath(req.Input, req.WorkingDir)

	// Check for sensitive locations
	for _, pattern := range w.blockedPaths {
		if pattern.MatchString(path) {
			return RiskCritical
		}
	}

	// Check for executable files
	ext := strings.ToLower(filepath.Ext(path))
	executableExts := []string{".sh", ".bash", ".py", ".rb", ".pl", ".exe", ".bat", ".cmd"}
	for _, execExt := range executableExts {
		if ext == execExt {
			return RiskMedium
		}
	}

	// Check for config files
	configNames := []string{".env", ".bashrc", ".zshrc", ".profile", "config", "settings"}
	base := filepath.Base(path)
	for _, configName := range configNames {
		if strings.Contains(strings.ToLower(base), configName) {
			return RiskMedium
		}
	}

	// Check if overwriting existing file
	if _, err := os.Stat(path); err == nil {
		return RiskLow // File exists, overwriting
	}

	return RiskNone
}

func (w *WriteTool) Execute(ctx context.Context, req *ToolRequest) (*ToolResult, error) {
	start := time.Now()
	path := resolvePath(req.Input, req.WorkingDir)

	content, ok := req.Params["content"].(string)
	if !ok {
		return &ToolResult{
			Tool:     ToolWrite,
			Success:  false,
			Error:    "content must be a string",
			Duration: time.Since(start),
		}, fmt.Errorf("content must be a string")
	}

	// Create directories if needed
	if w.createDirs {
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return &ToolResult{
				Tool:     ToolWrite,
				Success:  false,
				Error:    fmt.Sprintf("failed to create directory: %v", err),
				Duration: time.Since(start),
			}, err
		}
	}

	// Backup existing file if configured
	if w.backupExisting {
		if _, err := os.Stat(path); err == nil {
			backupPath := path + ".bak"
			if err := os.Rename(path, backupPath); err != nil {
				return &ToolResult{
					Tool:     ToolWrite,
					Success:  false,
					Error:    fmt.Sprintf("failed to create backup: %v", err),
					Duration: time.Since(start),
				}, err
			}
		}
	}

	// Write file
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return &ToolResult{
			Tool:     ToolWrite,
			Success:  false,
			Error:    err.Error(),
			Duration: time.Since(start),
		}, err
	}

	return &ToolResult{
		Tool:     ToolWrite,
		Success:  true,
		Output:   fmt.Sprintf("Wrote %d bytes to %s", len(content), path),
		Duration: time.Since(start),
		Metadata: map[string]interface{}{
			"path":       path,
			"bytes":      len(content),
			"lines":      strings.Count(content, "\n") + 1,
			"created_at": time.Now().Format(time.RFC3339),
		},
	}, nil
}

// ===========================================================================
// EDIT TOOL
// ===========================================================================

// EditTool performs precise text replacements in files.
type EditTool struct {
	// BackupBeforeEdit if true, creates .bak before editing
	backupBeforeEdit bool

	// MaxFileSize for editable files
	maxFileSize int64
}

// EditOption configures the EditTool.
type EditOption func(*EditTool)

// WithBackupBeforeEdit enables backup before editing.
func WithBackupBeforeEdit(backup bool) EditOption {
	return func(e *EditTool) {
		e.backupBeforeEdit = backup
	}
}

// NewEditTool creates a new file edit tool.
func NewEditTool(opts ...EditOption) *EditTool {
	e := &EditTool{
		backupBeforeEdit: false,
		maxFileSize:      10 * 1024 * 1024, // 10MB
	}

	for _, opt := range opts {
		opt(e)
	}

	return e
}

func (e *EditTool) Name() ToolType { return ToolEdit }

func (e *EditTool) Validate(req *ToolRequest) error {
	if req.Tool != ToolEdit {
		return fmt.Errorf("wrong tool type: expected %s, got %s", ToolEdit, req.Tool)
	}

	if strings.TrimSpace(req.Input) == "" {
		return fmt.Errorf("file path cannot be empty")
	}

	if req.Params == nil {
		return fmt.Errorf("params required (old_string, new_string)")
	}

	if _, ok := req.Params["old_string"]; !ok {
		return fmt.Errorf("old_string parameter required")
	}

	if _, ok := req.Params["new_string"]; !ok {
		return fmt.Errorf("new_string parameter required")
	}

	path := resolvePath(req.Input, req.WorkingDir)

	// Check file exists
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("file not found: %s", path)
	}

	if info.IsDir() {
		return fmt.Errorf("path is a directory: %s", path)
	}

	if info.Size() > e.maxFileSize {
		return fmt.Errorf("file too large for editing: %d bytes", info.Size())
	}

	return nil
}

func (e *EditTool) AssessRisk(req *ToolRequest) RiskLevel {
	path := resolvePath(req.Input, req.WorkingDir)

	// System files are high risk
	if strings.HasPrefix(path, "/etc/") || strings.HasPrefix(path, "/usr/") {
		return RiskHigh
	}

	// Config files are medium risk
	ext := strings.ToLower(filepath.Ext(path))
	configExts := []string{".conf", ".cfg", ".config", ".ini", ".yaml", ".yml", ".toml"}
	for _, configExt := range configExts {
		if ext == configExt {
			return RiskMedium
		}
	}

	// Check for replacing with empty (deletion)
	if newStr, ok := req.Params["new_string"].(string); ok {
		if newStr == "" {
			return RiskLow // Deletion operation
		}
	}

	return RiskNone
}

func (e *EditTool) Execute(ctx context.Context, req *ToolRequest) (*ToolResult, error) {
	start := time.Now()
	path := resolvePath(req.Input, req.WorkingDir)

	oldString, _ := req.Params["old_string"].(string)
	newString, _ := req.Params["new_string"].(string)

	// Get replace_all option
	replaceAll := false
	if val, ok := req.Params["replace_all"].(bool); ok {
		replaceAll = val
	}

	// Read current content
	content, err := os.ReadFile(path)
	if err != nil {
		return &ToolResult{
			Tool:     ToolEdit,
			Success:  false,
			Error:    err.Error(),
			Duration: time.Since(start),
		}, err
	}

	contentStr := string(content)

	// Check if old_string exists
	occurrences := strings.Count(contentStr, oldString)
	if occurrences == 0 {
		return &ToolResult{
			Tool:     ToolEdit,
			Success:  false,
			Error:    fmt.Sprintf("old_string not found in file: %q", truncateString(oldString, 50)),
			Duration: time.Since(start),
		}, fmt.Errorf("old_string not found")
	}

	// Check for ambiguity
	if occurrences > 1 && !replaceAll {
		return &ToolResult{
			Tool:     ToolEdit,
			Success:  false,
			Error:    fmt.Sprintf("old_string found %d times. Use replace_all=true or provide more context", occurrences),
			Duration: time.Since(start),
			Metadata: map[string]interface{}{
				"occurrences": occurrences,
			},
		}, fmt.Errorf("ambiguous edit: %d occurrences", occurrences)
	}

	// Backup if configured
	if e.backupBeforeEdit {
		backupPath := path + ".bak"
		if err := os.WriteFile(backupPath, content, 0644); err != nil {
			return &ToolResult{
				Tool:     ToolEdit,
				Success:  false,
				Error:    fmt.Sprintf("failed to create backup: %v", err),
				Duration: time.Since(start),
			}, err
		}
	}

	// Perform replacement
	var newContent string
	var replacements int
	if replaceAll {
		newContent = strings.ReplaceAll(contentStr, oldString, newString)
		replacements = occurrences
	} else {
		newContent = strings.Replace(contentStr, oldString, newString, 1)
		replacements = 1
	}

	// Write back
	if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
		return &ToolResult{
			Tool:     ToolEdit,
			Success:  false,
			Error:    err.Error(),
			Duration: time.Since(start),
		}, err
	}

	return &ToolResult{
		Tool:     ToolEdit,
		Success:  true,
		Output:   fmt.Sprintf("Replaced %d occurrence(s) in %s", replacements, path),
		Duration: time.Since(start),
		Metadata: map[string]interface{}{
			"path":         path,
			"replacements": replacements,
			"old_length":   len(oldString),
			"new_length":   len(newString),
		},
	}, nil
}

// ===========================================================================
// HELPERS
// ===========================================================================

// resolvePath resolves a path relative to a working directory.
func resolvePath(path, workingDir string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	if workingDir != "" {
		return filepath.Clean(filepath.Join(workingDir, path))
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return absPath
}

// truncateString truncates a string with ellipsis.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
