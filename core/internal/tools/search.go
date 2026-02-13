package tools

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/normanking/cortex/internal/search"
)

// ===========================================================================
// GLOB TOOL
// ===========================================================================

// GlobTool finds files matching glob patterns.
type GlobTool struct {
	// MaxResults limits the number of results returned
	maxResults int

	// IgnorePatterns are patterns to skip (e.g., node_modules)
	ignorePatterns []string

	// FollowSymlinks if true, follows symbolic links
	followSymlinks bool
}

// GlobOption configures the GlobTool.
type GlobOption func(*GlobTool)

// WithMaxResults sets the maximum number of glob results.
func WithMaxResults(max int) GlobOption {
	return func(g *GlobTool) {
		g.maxResults = max
	}
}

// WithIgnorePatterns sets patterns to ignore.
func WithIgnorePatterns(patterns []string) GlobOption {
	return func(g *GlobTool) {
		g.ignorePatterns = patterns
	}
}

// NewGlobTool creates a new glob tool.
func NewGlobTool(opts ...GlobOption) *GlobTool {
	g := &GlobTool{
		maxResults: 1000,
		ignorePatterns: []string{
			"node_modules",
			".git",
			"vendor",
			"__pycache__",
			".venv",
			"venv",
			".idea",
			".vscode",
			"dist",
			"build",
			"target",
		},
		followSymlinks: false,
	}

	for _, opt := range opts {
		opt(g)
	}

	return g
}

func (g *GlobTool) Name() ToolType { return ToolGlob }

func (g *GlobTool) Validate(req *ToolRequest) error {
	if req.Tool != ToolGlob {
		return fmt.Errorf("wrong tool type: expected %s, got %s", ToolGlob, req.Tool)
	}

	if strings.TrimSpace(req.Input) == "" {
		return fmt.Errorf("glob pattern cannot be empty")
	}

	// Validate pattern syntax
	_, err := filepath.Match(req.Input, "test")
	if err != nil {
		return fmt.Errorf("invalid glob pattern: %v", err)
	}

	return nil
}

func (g *GlobTool) AssessRisk(req *ToolRequest) RiskLevel {
	// Glob is read-only, always safe
	return RiskNone
}

func (g *GlobTool) Execute(ctx context.Context, req *ToolRequest) (*ToolResult, error) {
	start := time.Now()
	pattern := req.Input
	baseDir := req.WorkingDir
	if baseDir == "" {
		var err error
		baseDir, err = os.Getwd()
		if err != nil {
			return &ToolResult{
				Tool:     ToolGlob,
				Success:  false,
				Error:    err.Error(),
				Duration: time.Since(start),
			}, err
		}
	}

	// Use the new FileSearcher for optimized file search
	searcher := search.NewFileSearcher()
	searcher.SetMaxFiles(g.maxResults)
	searcher.SetIgnoreDirs(g.ignorePatterns)

	// Normalize pattern for FileSearcher
	searchPattern := pattern
	if !strings.HasPrefix(pattern, "**") && !filepath.IsAbs(pattern) {
		// For non-recursive patterns, just use the base name
		searchPattern = filepath.Base(pattern)
	}

	// Perform the search
	results, err := searcher.Search(ctx, baseDir, searchPattern)
	if err != nil && err != context.Canceled {
		return &ToolResult{
			Tool:     ToolGlob,
			Success:  false,
			Error:    err.Error(),
			Duration: time.Since(start),
		}, err
	}

	// Sort by modification time (newest first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].ModTime.After(results[j].ModTime)
	})

	// Convert results to relative paths
	var matches []string
	for _, result := range results {
		relPath, err := filepath.Rel(baseDir, result.Path)
		if err != nil {
			relPath = result.Path
		}
		matches = append(matches, relPath)
	}

	return &ToolResult{
		Tool:     ToolGlob,
		Success:  true,
		Output:   strings.Join(matches, "\n"),
		Duration: time.Since(start),
		Metadata: map[string]interface{}{
			"count":         len(matches),
			"pattern":       req.Input,
			"search_method": searcher.GetLastMethod(),
		},
	}, nil
}

// ===========================================================================
// GREP TOOL
// ===========================================================================

// GrepTool searches file contents using regex.
type GrepTool struct {
	// MaxResults limits the number of matching files
	maxResults int

	// MaxMatchesPerFile limits matches shown per file
	maxMatchesPerFile int

	// ContextLines to show before/after matches
	contextLines int

	// IgnorePatterns for directories to skip
	ignorePatterns []string

	// MaxFileSize for searchable files
	maxFileSize int64
}

// GrepOption configures the GrepTool.
type GrepOption func(*GrepTool)

// WithGrepMaxResults sets the maximum number of matching files.
func WithGrepMaxResults(max int) GrepOption {
	return func(g *GrepTool) {
		g.maxResults = max
	}
}

// WithContextLines sets lines of context around matches.
func WithContextLines(lines int) GrepOption {
	return func(g *GrepTool) {
		g.contextLines = lines
	}
}

// NewGrepTool creates a new grep tool.
func NewGrepTool(opts ...GrepOption) *GrepTool {
	g := &GrepTool{
		maxResults:        100,
		maxMatchesPerFile: 10,
		contextLines:      0,
		ignorePatterns: []string{
			"node_modules",
			".git",
			"vendor",
			"__pycache__",
			".venv",
		},
		maxFileSize: 1 * 1024 * 1024, // 1MB
	}

	for _, opt := range opts {
		opt(g)
	}

	return g
}

func (g *GrepTool) Name() ToolType { return ToolGrep }

func (g *GrepTool) Validate(req *ToolRequest) error {
	if req.Tool != ToolGrep {
		return fmt.Errorf("wrong tool type: expected %s, got %s", ToolGrep, req.Tool)
	}

	if strings.TrimSpace(req.Input) == "" {
		return fmt.Errorf("search pattern cannot be empty")
	}

	// Validate regex
	_, err := regexp.Compile(req.Input)
	if err != nil {
		return fmt.Errorf("invalid regex pattern: %v", err)
	}

	return nil
}

func (g *GrepTool) AssessRisk(req *ToolRequest) RiskLevel {
	// Grep is read-only, always safe
	return RiskNone
}

// GrepMatch represents a single grep match.
type GrepMatch struct {
	File       string `json:"file"`
	LineNumber int    `json:"line_number"`
	Line       string `json:"line"`
	Context    struct {
		Before []string `json:"before,omitempty"`
		After  []string `json:"after,omitempty"`
	} `json:"context,omitempty"`
}

func (g *GrepTool) Execute(ctx context.Context, req *ToolRequest) (*ToolResult, error) {
	start := time.Now()

	pattern, err := regexp.Compile(req.Input)
	if err != nil {
		return &ToolResult{
			Tool:     ToolGrep,
			Success:  false,
			Error:    err.Error(),
			Duration: time.Since(start),
		}, err
	}

	baseDir := req.WorkingDir
	if baseDir == "" {
		baseDir, _ = os.Getwd()
	}

	// Get file filter from params
	fileGlob := "**/*"
	if fg, ok := req.Params["glob"].(string); ok {
		fileGlob = fg
	}

	// Case insensitive option
	caseInsensitive := false
	if ci, ok := req.Params["case_insensitive"].(bool); ok && ci {
		caseInsensitive = true
		pattern = regexp.MustCompile("(?i)" + req.Input)
	}

	var results []GrepMatch
	filesSearched := 0

	err = filepath.WalkDir(baseDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Skip ignored directories
		if d.IsDir() {
			for _, ignore := range g.ignorePatterns {
				if d.Name() == ignore {
					return filepath.SkipDir
				}
			}
			return nil
		}

		// Check file size
		info, err := d.Info()
		if err != nil || info.Size() > g.maxFileSize {
			return nil
		}

		// Check glob pattern
		relPath, _ := filepath.Rel(baseDir, path)
		if fileGlob != "**/*" {
			matched, _ := filepath.Match(fileGlob, filepath.Base(path))
			if !matched {
				return nil
			}
		}

		// Skip binary files (simple heuristic)
		ext := strings.ToLower(filepath.Ext(path))
		binaryExts := []string{".exe", ".dll", ".so", ".dylib", ".png", ".jpg", ".gif", ".pdf", ".zip", ".tar", ".gz"}
		for _, binExt := range binaryExts {
			if ext == binExt {
				return nil
			}
		}

		// Search file
		file, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer file.Close()

		filesSearched++
		scanner := bufio.NewScanner(file)
		lineNum := 0
		matchCount := 0

		for scanner.Scan() {
			lineNum++
			line := scanner.Text()

			if pattern.MatchString(line) {
				match := GrepMatch{
					File:       relPath,
					LineNumber: lineNum,
					Line:       line,
				}
				results = append(results, match)
				matchCount++

				if matchCount >= g.maxMatchesPerFile {
					break
				}
			}
		}

		if len(results) >= g.maxResults {
			return fmt.Errorf("limit reached")
		}

		return nil
	})

	if err != nil && err.Error() != "limit reached" && err != context.Canceled {
		return &ToolResult{
			Tool:     ToolGrep,
			Success:  false,
			Error:    err.Error(),
			Duration: time.Since(start),
		}, err
	}

	// Format output
	var output strings.Builder
	for _, match := range results {
		output.WriteString(fmt.Sprintf("%s:%d: %s\n", match.File, match.LineNumber, match.Line))
	}

	_ = caseInsensitive // Avoid unused variable warning

	return &ToolResult{
		Tool:     ToolGrep,
		Success:  true,
		Output:   strings.TrimRight(output.String(), "\n"),
		Duration: time.Since(start),
		Metadata: map[string]interface{}{
			"matches":        len(results),
			"files_searched": filesSearched,
			"pattern":        req.Input,
		},
	}, nil
}
