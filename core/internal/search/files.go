// Package search provides high-performance file search using native tools.
// It uses platform-specific tools (mdfind on macOS, ripgrep, find) with
// a Go fallback for maximum performance.
package search

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// FileSearchResult represents a single file match.
type FileSearchResult struct {
	Path    string    // Absolute path to the file
	Name    string    // Base name of the file
	Size    int64     // File size in bytes
	ModTime time.Time // Last modification time
}

// FileSearcher provides optimized file search functionality.
type FileSearcher struct {
	maxFiles     int      // Maximum number of files to return
	ignoreDirs   []string // Directories to skip
	hasMdfind    bool     // Whether mdfind is available (macOS)
	hasRipgrep   bool     // Whether ripgrep is available
	hasFind      bool     // Whether find is available (Unix)
	lastMethod   string   // Last search method used
}

// NewFileSearcher creates a new FileSearcher with sensible defaults.
func NewFileSearcher() *FileSearcher {
	fs := &FileSearcher{
		maxFiles: 50000,
		ignoreDirs: []string{
			"node_modules",
			"vendor",
			".git",
			"__pycache__",
			"target",
			"build",
			"dist",
			".cache",
			".venv",
			"venv",
			".idea",
			".vscode",
		},
	}

	// Detect available tools
	fs.hasMdfind = runtime.GOOS == "darwin" && commandExists("mdfind")
	fs.hasRipgrep = commandExists("rg")
	fs.hasFind = commandExists("find")

	return fs
}

// SetMaxFiles sets the maximum number of files to return.
func (fs *FileSearcher) SetMaxFiles(max int) {
	fs.maxFiles = max
}

// SetIgnoreDirs sets the directories to ignore during search.
func (fs *FileSearcher) SetIgnoreDirs(dirs []string) {
	fs.ignoreDirs = dirs
}

// GetLastMethod returns the search method used in the last Search call.
func (fs *FileSearcher) GetLastMethod() string {
	return fs.lastMethod
}

// Search finds files matching the pattern in the given directory.
// It tries the fastest available method first and falls back to slower methods.
func (fs *FileSearcher) Search(ctx context.Context, dir, pattern string) ([]FileSearchResult, error) {
	// Ensure directory is absolute
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Try fastest method first
	if fs.hasMdfind {
		if results, err := fs.searchMdfind(ctx, absDir, pattern); err == nil && len(results) > 0 {
			fs.lastMethod = "mdfind"
			return results, nil
		}
	}

	if fs.hasRipgrep {
		if results, err := fs.searchRipgrep(ctx, absDir, pattern); err == nil {
			fs.lastMethod = "ripgrep"
			return results, nil
		}
	}

	if fs.hasFind && runtime.GOOS != "windows" {
		if results, err := fs.searchFind(ctx, absDir, pattern); err == nil {
			fs.lastMethod = "find"
			return results, nil
		}
	}

	// Fall back to Go implementation
	fs.lastMethod = "go-walk"
	return fs.searchGoWalk(ctx, absDir, pattern)
}

// searchMdfind uses macOS Spotlight to search files (fastest on macOS).
func (fs *FileSearcher) searchMdfind(ctx context.Context, dir, pattern string) ([]FileSearchResult, error) {
	// Convert glob pattern to mdfind name pattern
	namePattern := pattern
	if strings.HasPrefix(pattern, "**/") {
		namePattern = pattern[3:]
	} else if strings.HasPrefix(pattern, "**\\") {
		namePattern = pattern[3:]
	}
	namePattern = strings.TrimPrefix(namePattern, "*")

	cmd := exec.CommandContext(ctx, "mdfind", "-onlyin", dir, "-name", namePattern)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("mdfind failed: %w", err)
	}

	return fs.parseFileList(ctx, output)
}

// searchRipgrep uses ripgrep to list files (very fast).
func (fs *FileSearcher) searchRipgrep(ctx context.Context, dir, pattern string) ([]FileSearchResult, error) {
	// Convert glob pattern to ripgrep glob
	globPattern := pattern
	if strings.HasPrefix(pattern, "**/") {
		globPattern = pattern[3:]
	} else if strings.HasPrefix(pattern, "**\\") {
		globPattern = pattern[3:]
	}

	args := []string{"--files", "-g", globPattern}

	// Add ignore patterns
	for _, ignore := range fs.ignoreDirs {
		args = append(args, "-g", "!"+ignore)
	}

	args = append(args, dir)

	cmd := exec.CommandContext(ctx, "rg", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ripgrep failed: %w", err)
	}

	return fs.parseFileList(ctx, output)
}

// searchFind uses Unix find command.
func (fs *FileSearcher) searchFind(ctx context.Context, dir, pattern string) ([]FileSearchResult, error) {
	// Convert glob pattern to find pattern
	namePattern := pattern
	if strings.HasPrefix(pattern, "**/") {
		namePattern = pattern[3:]
	} else if strings.HasPrefix(pattern, "**\\") {
		namePattern = pattern[3:]
	}

	args := []string{dir, "-type", "f", "-name", namePattern}

	// Add prune patterns for ignored directories
	if len(fs.ignoreDirs) > 0 {
		pruneArgs := []string{}
		for i, ignore := range fs.ignoreDirs {
			if i > 0 {
				pruneArgs = append(pruneArgs, "-o")
			}
			pruneArgs = append(pruneArgs, "-name", ignore)
		}
		args = []string{dir, "("}
		args = append(args, pruneArgs...)
		args = append(args, ")", "-prune", "-o", "-type", "f", "-name", namePattern, "-print")
	}

	cmd := exec.CommandContext(ctx, "find", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("find failed: %w", err)
	}

	return fs.parseFileList(ctx, output)
}

// searchGoWalk uses Go's filepath.WalkDir as a fallback.
func (fs *FileSearcher) searchGoWalk(ctx context.Context, dir, pattern string) ([]FileSearchResult, error) {
	var results []FileSearchResult
	count := 0

	// Extract the file pattern from the glob
	filePattern := pattern
	if strings.HasPrefix(pattern, "**/") {
		filePattern = pattern[3:]
	} else if strings.HasPrefix(pattern, "**\\") {
		filePattern = pattern[3:]
	}

	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Skip ignored directories
		if d.IsDir() {
			name := d.Name()
			for _, ignore := range fs.ignoreDirs {
				if name == ignore {
					return filepath.SkipDir
				}
			}
			return nil
		}

		// Check if file matches pattern
		matched, err := filepath.Match(filePattern, d.Name())
		if err != nil || !matched {
			return nil
		}

		// Get file info
		info, err := d.Info()
		if err != nil {
			return nil
		}

		results = append(results, FileSearchResult{
			Path:    path,
			Name:    d.Name(),
			Size:    info.Size(),
			ModTime: info.ModTime(),
		})
		count++

		// Limit results
		if count >= fs.maxFiles {
			return fmt.Errorf("limit reached")
		}

		return nil
	})

	if err != nil && err.Error() != "limit reached" && err != context.Canceled {
		return nil, err
	}

	return results, nil
}

// parseFileList parses the output of file listing commands.
func (fs *FileSearcher) parseFileList(ctx context.Context, output []byte) ([]FileSearchResult, error) {
	var results []FileSearchResult
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	count := 0

	for scanner.Scan() {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Skip ignored directories
		skip := false
		for _, ignore := range fs.ignoreDirs {
			if strings.Contains(line, "/"+ignore+"/") || strings.Contains(line, "\\"+ignore+"\\") {
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		// Get file info
		info, err := os.Stat(line)
		if err != nil {
			continue // Skip files we can't stat
		}

		// Skip directories
		if info.IsDir() {
			continue
		}

		results = append(results, FileSearchResult{
			Path:    line,
			Name:    filepath.Base(line),
			Size:    info.Size(),
			ModTime: info.ModTime(),
		})
		count++

		if count >= fs.maxFiles {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return results, fmt.Errorf("failed to parse output: %w", err)
	}

	return results, nil
}

// commandExists checks if a command is available in PATH.
func commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}
