package tools

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// FilesTool handles file system operations
type FilesTool struct {
	allowedPaths []string
	deniedPaths  []string
	maxFileSize  int64
	baseDir      string
}

// FilesConfig configures the files tool
type FilesConfig struct {
	AllowedPaths []string
	DeniedPaths  []string
	MaxFileSize  int64
	BaseDir      string
}

// DefaultFilesConfig returns sensible defaults
func DefaultFilesConfig() *FilesConfig {
	home, _ := os.UserHomeDir()
	return &FilesConfig{
		AllowedPaths: []string{home},
		DeniedPaths:  []string{"/etc", "/usr", "/bin", "/sbin", "/var"},
		MaxFileSize:  10 * 1024 * 1024, // 10MB
		BaseDir:      home,
	}
}

// NewFilesTool creates a new files tool
func NewFilesTool(cfg *FilesConfig) *FilesTool {
	if cfg == nil {
		cfg = DefaultFilesConfig()
	}

	return &FilesTool{
		allowedPaths: cfg.AllowedPaths,
		deniedPaths:  cfg.DeniedPaths,
		maxFileSize:  cfg.MaxFileSize,
		baseDir:      cfg.BaseDir,
	}
}

func (t *FilesTool) Name() string           { return "files" }
func (t *FilesTool) Category() ToolCategory { return CategoryFiles }
func (t *FilesTool) RiskLevel() RiskLevel   { return RiskMedium }

func (t *FilesTool) Description() string {
	return "Read, write, and manage files. Operations: read, write, append, delete, move, copy, list, search."
}

// Spec returns the tool specification for LLM function calling
func (t *FilesTool) Spec() *ToolSpec {
	return &ToolSpec{
		Name:        t.Name(),
		Description: t.Description(),
		Category:    t.Category(),
		RiskLevel:   t.RiskLevel(),
		Parameters: &ParamSchema{
			Type: "object",
			Properties: map[string]*ParamProp{
				"operation": {
					Type:        "string",
					Description: "The file operation to perform",
					Enum:        []string{"read", "write", "append", "delete", "move", "copy", "list", "exists", "search"},
				},
				"path": {
					Type:        "string",
					Description: "The file path to operate on",
				},
				"pattern": {
					Type:        "string",
					Description: "Search pattern for matching file names (for search operation)",
				},
				"content": {
					Type:        "string",
					Description: "Content to write (for write/append operations)",
				},
				"destination": {
					Type:        "string",
					Description: "Destination path (for move/copy operations)",
				},
			},
			Required: []string{"operation", "path"},
		},
	}
}

// Validate checks if the input is valid
func (t *FilesTool) Validate(input *ToolInput) error {
	if input == nil {
		return errors.New("input is nil")
	}

	op, ok := input.Args["operation"].(string)
	if !ok || op == "" {
		return errors.New("operation is required")
	}

	path, ok := input.Args["path"].(string)
	if !ok || path == "" {
		return errors.New("path is required")
	}

	// Validate operation
	validOps := []string{"read", "write", "append", "delete", "move", "copy", "list", "exists", "search"}
	valid := false
	for _, v := range validOps {
		if v == op {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("invalid operation: %s", op)
	}

	// Check path is allowed
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	if !t.isPathAllowed(absPath) {
		return fmt.Errorf("path is not allowed: %s", absPath)
	}

	// For write operations, check content is provided
	if op == "write" || op == "append" {
		if _, ok := input.Args["content"].(string); !ok {
			return errors.New("content is required for write/append operations")
		}
	}

	// For search, check pattern is provided
	if op == "search" {
		if _, ok := input.Args["pattern"].(string); !ok {
			return errors.New("pattern is required for search operation")
		}
	}

	// For move/copy, check destination
	if op == "move" || op == "copy" {
		dest, ok := input.Args["destination"].(string)
		if !ok || dest == "" {
			return errors.New("destination is required for move/copy operations")
		}
		absDest, err := filepath.Abs(dest)
		if err != nil {
			return fmt.Errorf("invalid destination: %w", err)
		}
		if !t.isPathAllowed(absDest) {
			return fmt.Errorf("destination is not allowed: %s", absDest)
		}
	}

	return nil
}

// Execute performs the file operation
func (t *FilesTool) Execute(ctx context.Context, input *ToolInput) (*ToolOutput, error) {
	op := input.Args["operation"].(string)
	path := input.Args["path"].(string)

	// Make path absolute
	if !filepath.IsAbs(path) {
		if input.WorkingDir != "" {
			path = filepath.Join(input.WorkingDir, path)
		} else {
			path = filepath.Join(t.baseDir, path)
		}
	}

	switch op {
	case "read":
		return t.read(ctx, path)
	case "write":
		content := input.Args["content"].(string)
		return t.write(ctx, path, content, false)
	case "append":
		content := input.Args["content"].(string)
		return t.write(ctx, path, content, true)
	case "delete":
		return t.delete(ctx, path)
	case "move":
		dest := input.Args["destination"].(string)
		return t.move(ctx, path, dest)
	case "copy":
		dest := input.Args["destination"].(string)
		return t.copyFile(ctx, path, dest)
	case "list":
		return t.list(ctx, path)
	case "exists":
		return t.exists(ctx, path)
	case "search":
		pattern := input.Args["pattern"].(string)
		return t.search(ctx, path, pattern)
	default:
		return &ToolOutput{
			Success: false,
			Error:   fmt.Sprintf("unknown operation: %s", op),
		}, nil
	}
}

func (t *FilesTool) read(ctx context.Context, path string) (*ToolOutput, error) {
	info, err := os.Stat(path)
	if err != nil {
		return &ToolOutput{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	if info.IsDir() {
		return &ToolOutput{
			Success: false,
			Error:   "path is a directory, use 'list' operation",
		}, nil
	}

	if info.Size() > t.maxFileSize {
		return &ToolOutput{
			Success: false,
			Error:   fmt.Sprintf("file too large: %d bytes (max %d)", info.Size(), t.maxFileSize),
		}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return &ToolOutput{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &ToolOutput{
		Success: true,
		Output:  string(data),
	}, nil
}

func (t *FilesTool) write(ctx context.Context, path, content string, append bool) (*ToolOutput, error) {
	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return &ToolOutput{
			Success: false,
			Error:   fmt.Sprintf("failed to create directory: %v", err),
		}, nil
	}

	var file *os.File
	var err error

	if append {
		file, err = os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	} else {
		file, err = os.Create(path)
	}

	if err != nil {
		return &ToolOutput{
			Success: false,
			Error:   err.Error(),
		}, nil
	}
	defer file.Close()

	n, err := file.WriteString(content)
	if err != nil {
		return &ToolOutput{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	action := "wrote"
	if append {
		action = "appended"
	}

	return &ToolOutput{
		Success:   true,
		Output:    fmt.Sprintf("%s %d bytes to %s", action, n, path),
		Artifacts: []string{path},
	}, nil
}

func (t *FilesTool) delete(ctx context.Context, path string) (*ToolOutput, error) {
	info, err := os.Stat(path)
	if err != nil {
		return &ToolOutput{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	if info.IsDir() {
		err = os.RemoveAll(path)
	} else {
		err = os.Remove(path)
	}

	if err != nil {
		return &ToolOutput{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &ToolOutput{
		Success: true,
		Output:  fmt.Sprintf("deleted %s", path),
	}, nil
}

func (t *FilesTool) move(ctx context.Context, src, dest string) (*ToolOutput, error) {
	if err := os.Rename(src, dest); err != nil {
		return &ToolOutput{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &ToolOutput{
		Success:   true,
		Output:    fmt.Sprintf("moved %s to %s", src, dest),
		Artifacts: []string{dest},
	}, nil
}

func (t *FilesTool) copyFile(ctx context.Context, src, dest string) (*ToolOutput, error) {
	srcFile, err := os.Open(src)
	if err != nil {
		return &ToolOutput{
			Success: false,
			Error:   err.Error(),
		}, nil
	}
	defer srcFile.Close()

	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return &ToolOutput{
			Success: false,
			Error:   fmt.Sprintf("failed to create destination directory: %v", err),
		}, nil
	}

	destFile, err := os.Create(dest)
	if err != nil {
		return &ToolOutput{
			Success: false,
			Error:   err.Error(),
		}, nil
	}
	defer destFile.Close()

	n, err := io.Copy(destFile, srcFile)
	if err != nil {
		return &ToolOutput{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &ToolOutput{
		Success:   true,
		Output:    fmt.Sprintf("copied %d bytes from %s to %s", n, src, dest),
		Artifacts: []string{dest},
	}, nil
}

func (t *FilesTool) list(ctx context.Context, path string) (*ToolOutput, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return &ToolOutput{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	var lines []string
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		prefix := "-"
		if entry.IsDir() {
			prefix = "d"
		}

		lines = append(lines, fmt.Sprintf("%s %10d %s %s",
			prefix,
			info.Size(),
			info.ModTime().Format("2006-01-02 15:04"),
			entry.Name(),
		))
	}

	return &ToolOutput{
		Success: true,
		Output:  strings.Join(lines, "\n"),
	}, nil
}

func (t *FilesTool) exists(ctx context.Context, path string) (*ToolOutput, error) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return &ToolOutput{
			Success: true,
			Output:  "false",
		}, nil
	}
	if err != nil {
		return &ToolOutput{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	fileType := "file"
	if info.IsDir() {
		fileType = "directory"
	}

	return &ToolOutput{
		Success: true,
		Output:  fmt.Sprintf("true (%s, %d bytes)", fileType, info.Size()),
	}, nil
}

func (t *FilesTool) search(ctx context.Context, basePath, pattern string) (*ToolOutput, error) {
	var matches []string

	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip inaccessible paths
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		matched, _ := filepath.Match(pattern, info.Name())
		if matched {
			relPath, _ := filepath.Rel(basePath, path)
			matches = append(matches, relPath)
		}
		return nil
	})

	if err != nil && err != context.Canceled {
		return &ToolOutput{
			Success: false,
			Error:   fmt.Sprintf("search failed: %v", err),
		}, nil
	}

	return &ToolOutput{
		Success:   true,
		Output:    strings.Join(matches, "\n"),
		Artifacts: matches,
	}, nil
}

func (t *FilesTool) isPathAllowed(path string) bool {
	// Resolve symlinks to get the real path
	// This prevents symlink-based path traversal attacks
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		// If path doesn't exist yet (for write operations), check parent
		parentPath := filepath.Dir(path)
		realParent, err := filepath.EvalSymlinks(parentPath)
		if err != nil {
			// Can't resolve, deny by default for safety
			return false
		}
		// Reconstruct path with resolved parent
		realPath = filepath.Join(realParent, filepath.Base(path))
	}

	// Convert to absolute path for consistent comparison
	absPath, err := filepath.Abs(realPath)
	if err != nil {
		return false
	}

	// Check denied paths first
	for _, denied := range t.deniedPaths {
		absDenied, err := filepath.Abs(denied)
		if err != nil {
			continue
		}
		if absPath == absDenied || strings.HasPrefix(absPath, absDenied+string(filepath.Separator)) {
			return false
		}
	}

	// If allowed paths are specified, check them
	if len(t.allowedPaths) > 0 {
		for _, allowed := range t.allowedPaths {
			absAllowed, err := filepath.Abs(allowed)
			if err != nil {
				continue
			}
			if absPath == absAllowed || strings.HasPrefix(absPath, absAllowed+string(filepath.Separator)) {
				return true
			}
		}
		return false
	}

	return true
}
