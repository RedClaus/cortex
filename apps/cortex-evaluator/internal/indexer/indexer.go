// Package indexer provides project directory scanning and indexing capabilities.
package indexer

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FileType represents the type of a file in the project.
type FileType string

const (
	FileTypeRegular   FileType = "regular"
	FileTypeDirectory FileType = "directory"
	FileTypeSymlink   FileType = "symlink"
	FileTypeOther     FileType = "other"
)

// FileInfo represents metadata about a single file in the project.
type FileInfo struct {
	Path         string    `json:"path"`          // Relative path from project root
	AbsolutePath string    `json:"absolute_path"` // Full absolute path
	Size         int64     `json:"size"`          // Size in bytes (0 for directories)
	Type         FileType  `json:"type"`          // File type classification
	ModTime      time.Time `json:"mod_time"`      // Last modification time
	Extension    string    `json:"extension"`     // File extension (empty for directories)
}

// Manifest represents the complete index of a project directory.
type Manifest struct {
	RootPath    string     `json:"root_path"`    // Absolute path to project root
	Files       []FileInfo `json:"files"`        // All indexed files
	TotalFiles  int        `json:"total_files"`  // Count of regular files
	TotalDirs   int        `json:"total_dirs"`   // Count of directories
	TotalSize   int64      `json:"total_size"`   // Sum of all file sizes
	IndexedAt   time.Time  `json:"indexed_at"`   // When indexing completed
	ElapsedTime float64    `json:"elapsed_time"` // Indexing duration in seconds
}

// Progress reports the current state of indexing.
type Progress struct {
	CurrentPath   string // Path currently being processed
	FilesScanned  int    // Number of files scanned so far
	DirsScanned   int    // Number of directories scanned so far
	BytesScanned  int64  // Total bytes scanned so far
	TotalEstimate int    // Estimated total files (if known, else 0)
}

// ProgressCallback is called during indexing to report progress.
// Return an error to cancel indexing.
type ProgressCallback func(Progress) error

// Options configures the indexing behavior.
type Options struct {
	// FollowSymlinks determines whether to follow symbolic links.
	FollowSymlinks bool

	// MaxDepth limits recursion depth. 0 means unlimited.
	MaxDepth int

	// ExcludePatterns are additional patterns to exclude beyond .gitignore.
	ExcludePatterns []string

	// IncludeHidden determines whether to include hidden files (starting with .).
	IncludeHidden bool

	// ProgressCallback reports indexing progress. Can be nil.
	ProgressCallback ProgressCallback
}

// DefaultOptions returns sensible default indexing options.
func DefaultOptions() Options {
	return Options{
		FollowSymlinks: false,
		MaxDepth:       0, // unlimited
		IncludeHidden:  false,
		ExcludePatterns: []string{
			".git",
			"node_modules",
			"__pycache__",
			".DS_Store",
			"*.pyc",
			"*.pyo",
			".venv",
			"venv",
		},
	}
}

// Indexer scans and indexes project directories.
type Indexer struct {
	opts           Options
	gitignoreRules []gitignoreRule
}

// gitignoreRule represents a single .gitignore pattern.
type gitignoreRule struct {
	pattern  string
	negation bool
	dirOnly  bool
}

// New creates a new Indexer with the given options.
func New(opts Options) *Indexer {
	return &Indexer{
		opts: opts,
	}
}

// IndexProject scans and indexes the directory at the given path.
func (idx *Indexer) IndexProject(path string) (*Manifest, error) {
	startTime := time.Now()

	// Resolve to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	// Verify the path exists and is a directory
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, &NotADirectoryError{Path: absPath}
	}

	// Load .gitignore rules from project root
	idx.gitignoreRules = nil
	gitignorePath := filepath.Join(absPath, ".gitignore")
	if rules, err := parseGitignore(gitignorePath); err == nil {
		idx.gitignoreRules = rules
	}

	manifest := &Manifest{
		RootPath:  absPath,
		Files:     make([]FileInfo, 0, 1000),
		IndexedAt: time.Now(),
	}

	progress := Progress{}

	// Walk the directory tree
	err = filepath.WalkDir(absPath, func(filePath string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			// Skip permission errors, but continue walking
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(absPath, filePath)
		if err != nil {
			return nil
		}

		// Skip root directory itself
		if relPath == "." {
			return nil
		}

		// Check depth limit
		if idx.opts.MaxDepth > 0 {
			depth := strings.Count(relPath, string(filepath.Separator)) + 1
			if depth > idx.opts.MaxDepth {
				if d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
		}

		// Check if should be ignored
		if idx.shouldIgnore(relPath, d.IsDir()) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}

		// Get file info
		fileInfo, err := d.Info()
		if err != nil {
			return nil // Skip files we can't stat
		}

		fileType := determineFileType(d)
		ext := ""
		if fileType == FileTypeRegular {
			ext = filepath.Ext(filePath)
		}

		fi := FileInfo{
			Path:         relPath,
			AbsolutePath: filePath,
			Size:         fileInfo.Size(),
			Type:         fileType,
			ModTime:      fileInfo.ModTime(),
			Extension:    ext,
		}

		manifest.Files = append(manifest.Files, fi)

		// Update counters
		switch fileType {
		case FileTypeRegular:
			manifest.TotalFiles++
			manifest.TotalSize += fileInfo.Size()
			progress.FilesScanned++
			progress.BytesScanned += fileInfo.Size()
		case FileTypeDirectory:
			manifest.TotalDirs++
			progress.DirsScanned++
		}

		// Report progress
		if idx.opts.ProgressCallback != nil {
			progress.CurrentPath = relPath
			if err := idx.opts.ProgressCallback(progress); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	manifest.ElapsedTime = time.Since(startTime).Seconds()
	return manifest, nil
}

// shouldIgnore returns true if the path should be excluded from indexing.
func (idx *Indexer) shouldIgnore(relPath string, isDir bool) bool {
	baseName := filepath.Base(relPath)

	// Check hidden files
	if !idx.opts.IncludeHidden && strings.HasPrefix(baseName, ".") {
		return true
	}

	// Check additional exclude patterns
	for _, pattern := range idx.opts.ExcludePatterns {
		if matchPattern(pattern, relPath, baseName, isDir) {
			return true
		}
	}

	// Check .gitignore rules
	ignored := false
	for _, rule := range idx.gitignoreRules {
		if rule.dirOnly && !isDir {
			continue
		}
		if matchGitignorePattern(rule.pattern, relPath, baseName) {
			ignored = !rule.negation
		}
	}

	return ignored
}

// matchPattern checks if a path matches a simple glob pattern.
func matchPattern(pattern, relPath, baseName string, isDir bool) bool {
	// Handle directory-only patterns (ending with /)
	if strings.HasSuffix(pattern, "/") {
		if !isDir {
			return false
		}
		pattern = strings.TrimSuffix(pattern, "/")
	}

	// Try matching against basename
	if matched, _ := filepath.Match(pattern, baseName); matched {
		return true
	}

	// Try matching against full relative path
	if matched, _ := filepath.Match(pattern, relPath); matched {
		return true
	}

	// Handle patterns with directory separators
	if strings.Contains(pattern, "/") || strings.Contains(pattern, string(filepath.Separator)) {
		normalizedPattern := filepath.FromSlash(pattern)
		if matched, _ := filepath.Match(normalizedPattern, relPath); matched {
			return true
		}
	}

	return false
}

// matchGitignorePattern matches a .gitignore pattern against a path.
func matchGitignorePattern(pattern, relPath, baseName string) bool {
	// Handle patterns starting with /
	if trimmed, found := strings.CutPrefix(pattern, "/"); found {
		// Must match from root
		if matched, _ := filepath.Match(trimmed, relPath); matched {
			return true
		}
		return false
	}

	// Handle ** (match any path)
	if strings.Contains(pattern, "**") {
		// Simplified: treat ** as matching any directory depth
		parts := strings.Split(pattern, "**")
		if len(parts) == 2 {
			prefix := strings.TrimSuffix(parts[0], "/")
			suffix := strings.TrimPrefix(parts[1], "/")
			if prefix == "" && suffix == "" {
				return true
			}
			if prefix != "" && !strings.HasPrefix(relPath, prefix) {
				return false
			}
			if suffix != "" {
				if matched, _ := filepath.Match(suffix, baseName); matched {
					return true
				}
			}
			return prefix != "" && suffix == ""
		}
	}

	// Try matching against basename
	if matched, _ := filepath.Match(pattern, baseName); matched {
		return true
	}

	// Try matching against full path
	if matched, _ := filepath.Match(pattern, relPath); matched {
		return true
	}

	return false
}

// parseGitignore reads and parses a .gitignore file.
func parseGitignore(path string) ([]gitignoreRule, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var rules []gitignoreRule
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		rule := gitignoreRule{}

		// Check for negation
		if strings.HasPrefix(line, "!") {
			rule.negation = true
			line = strings.TrimPrefix(line, "!")
		}

		// Check for directory-only pattern
		if strings.HasSuffix(line, "/") {
			rule.dirOnly = true
			line = strings.TrimSuffix(line, "/")
		}

		rule.pattern = line
		rules = append(rules, rule)
	}

	return rules, scanner.Err()
}

// determineFileType classifies a file based on its DirEntry.
func determineFileType(d fs.DirEntry) FileType {
	if d.IsDir() {
		return FileTypeDirectory
	}

	info, err := d.Info()
	if err != nil {
		return FileTypeOther
	}

	mode := info.Mode()
	switch {
	case mode.IsRegular():
		return FileTypeRegular
	case mode&os.ModeSymlink != 0:
		return FileTypeSymlink
	default:
		return FileTypeOther
	}
}

// NotADirectoryError indicates the path is not a directory.
type NotADirectoryError struct {
	Path string
}

func (e *NotADirectoryError) Error() string {
	return "not a directory: " + e.Path
}
