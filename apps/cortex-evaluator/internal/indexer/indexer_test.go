package indexer

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	opts := DefaultOptions()
	idx := New(opts)

	assert.NotNil(t, idx)
	assert.Equal(t, opts.FollowSymlinks, idx.opts.FollowSymlinks)
	assert.Equal(t, opts.MaxDepth, idx.opts.MaxDepth)
}

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()

	assert.False(t, opts.FollowSymlinks)
	assert.Equal(t, 0, opts.MaxDepth)
	assert.False(t, opts.IncludeHidden)
	assert.Contains(t, opts.ExcludePatterns, ".git")
	assert.Contains(t, opts.ExcludePatterns, "node_modules")
}

func TestIndexProject_BasicDirectory(t *testing.T) {
	// Create a temporary directory structure
	tmpDir := t.TempDir()

	// Create some files and directories
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "src"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# Test"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "src", "main.go"), []byte("package main"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "src", "util.go"), []byte("package main"), 0644))

	opts := DefaultOptions()
	opts.IncludeHidden = true // Include hidden for this test
	idx := New(opts)

	manifest, err := idx.IndexProject(tmpDir)
	require.NoError(t, err)

	assert.Equal(t, tmpDir, manifest.RootPath)
	assert.Equal(t, 3, manifest.TotalFiles)  // README.md, main.go, util.go
	assert.Equal(t, 1, manifest.TotalDirs)   // src
	assert.Greater(t, manifest.TotalSize, int64(0))
	assert.NotZero(t, manifest.IndexedAt)
	assert.Greater(t, manifest.ElapsedTime, float64(0))
}

func TestIndexProject_NonExistentPath(t *testing.T) {
	idx := New(DefaultOptions())

	_, err := idx.IndexProject("/nonexistent/path/that/does/not/exist")
	assert.Error(t, err)
}

func TestIndexProject_NotADirectory(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-file-*.txt")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	idx := New(DefaultOptions())

	_, err = idx.IndexProject(tmpFile.Name())
	assert.Error(t, err)

	var notDirErr *NotADirectoryError
	assert.True(t, errors.As(err, &notDirErr))
}

func TestIndexProject_RespectsGitignore(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .gitignore
	gitignore := `
# Ignore build directory
build/
*.log
temp*.txt
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".gitignore"), []byte(gitignore), 0644))

	// Create files that should be indexed
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "config.yaml"), []byte("key: value"), 0644))

	// Create files that should be ignored
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "build"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "build", "output.exe"), []byte("binary"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "app.log"), []byte("log content"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "temp123.txt"), []byte("temp"), 0644))

	opts := DefaultOptions()
	opts.IncludeHidden = true // Include .gitignore itself
	idx := New(opts)

	manifest, err := idx.IndexProject(tmpDir)
	require.NoError(t, err)

	// Verify only expected files are indexed
	paths := make([]string, 0, len(manifest.Files))
	for _, f := range manifest.Files {
		paths = append(paths, f.Path)
	}

	assert.Contains(t, paths, "main.go")
	assert.Contains(t, paths, "config.yaml")
	assert.Contains(t, paths, ".gitignore")
	assert.NotContains(t, paths, "build")
	assert.NotContains(t, paths, filepath.Join("build", "output.exe"))
	assert.NotContains(t, paths, "app.log")
	assert.NotContains(t, paths, "temp123.txt")
}

func TestIndexProject_ExcludesHiddenByDefault(t *testing.T) {
	tmpDir := t.TempDir()

	// Create regular file
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "visible.txt"), []byte("content"), 0644))

	// Create hidden file
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".hidden"), []byte("hidden"), 0644))

	// Create hidden directory
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".config"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".config", "settings.json"), []byte("{}"), 0644))

	idx := New(DefaultOptions())

	manifest, err := idx.IndexProject(tmpDir)
	require.NoError(t, err)

	paths := make([]string, 0, len(manifest.Files))
	for _, f := range manifest.Files {
		paths = append(paths, f.Path)
	}

	assert.Contains(t, paths, "visible.txt")
	assert.NotContains(t, paths, ".hidden")
	assert.NotContains(t, paths, ".config")
}

func TestIndexProject_IncludeHiddenOption(t *testing.T) {
	tmpDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "visible.txt"), []byte("content"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".hidden"), []byte("hidden"), 0644))

	opts := DefaultOptions()
	opts.IncludeHidden = true
	idx := New(opts)

	manifest, err := idx.IndexProject(tmpDir)
	require.NoError(t, err)

	paths := make([]string, 0, len(manifest.Files))
	for _, f := range manifest.Files {
		paths = append(paths, f.Path)
	}

	assert.Contains(t, paths, "visible.txt")
	assert.Contains(t, paths, ".hidden")
}

func TestIndexProject_MaxDepth(t *testing.T) {
	tmpDir := t.TempDir()

	// Create nested structure: level1/level2/level3/file.txt
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "level1", "level2", "level3"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "root.txt"), []byte("root"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "level1", "l1.txt"), []byte("l1"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "level1", "level2", "l2.txt"), []byte("l2"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "level1", "level2", "level3", "l3.txt"), []byte("l3"), 0644))

	opts := DefaultOptions()
	opts.IncludeHidden = true
	opts.MaxDepth = 2 // Only go 2 levels deep
	idx := New(opts)

	manifest, err := idx.IndexProject(tmpDir)
	require.NoError(t, err)

	paths := make([]string, 0, len(manifest.Files))
	for _, f := range manifest.Files {
		paths = append(paths, f.Path)
	}

	assert.Contains(t, paths, "root.txt")
	assert.Contains(t, paths, "level1")
	assert.Contains(t, paths, filepath.Join("level1", "l1.txt"))
	// level2 is at depth 2, should be included
	assert.Contains(t, paths, filepath.Join("level1", "level2"))
	// Files inside level2 are at depth 3, should be excluded
	assert.NotContains(t, paths, filepath.Join("level1", "level2", "l2.txt"))
}

func TestIndexProject_ProgressCallback(t *testing.T) {
	tmpDir := t.TempDir()

	// Create some files
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("content1"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "file2.txt"), []byte("content2"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "file3.txt"), []byte("content3"), 0644))

	var progressCalls []Progress
	opts := DefaultOptions()
	opts.IncludeHidden = true
	opts.ProgressCallback = func(p Progress) error {
		progressCalls = append(progressCalls, p)
		return nil
	}
	idx := New(opts)

	_, err := idx.IndexProject(tmpDir)
	require.NoError(t, err)

	// Should have received progress updates for each file
	assert.Len(t, progressCalls, 3)

	// Last progress should have all files counted
	lastProgress := progressCalls[len(progressCalls)-1]
	assert.Equal(t, 3, lastProgress.FilesScanned)
}

func TestIndexProject_ProgressCallbackCancellation(t *testing.T) {
	tmpDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("content1"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "file2.txt"), []byte("content2"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "file3.txt"), []byte("content3"), 0644))

	cancelErr := errors.New("cancelled")
	callCount := 0

	opts := DefaultOptions()
	opts.IncludeHidden = true
	opts.ProgressCallback = func(p Progress) error {
		callCount++
		if callCount >= 2 {
			return cancelErr
		}
		return nil
	}
	idx := New(opts)

	_, err := idx.IndexProject(tmpDir)
	assert.ErrorIs(t, err, cancelErr)
	assert.Equal(t, 2, callCount)
}

func TestIndexProject_FileTypes(t *testing.T) {
	tmpDir := t.TempDir()

	// Create regular file
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("content"), 0644))

	// Create directory
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "subdir"), 0755))

	opts := DefaultOptions()
	opts.IncludeHidden = true
	idx := New(opts)

	manifest, err := idx.IndexProject(tmpDir)
	require.NoError(t, err)

	var fileInfo, dirInfo FileInfo
	for _, f := range manifest.Files {
		if f.Path == "file.txt" {
			fileInfo = f
		} else if f.Path == "subdir" {
			dirInfo = f
		}
	}

	assert.Equal(t, FileTypeRegular, fileInfo.Type)
	assert.Equal(t, ".txt", fileInfo.Extension)
	assert.Greater(t, fileInfo.Size, int64(0))

	assert.Equal(t, FileTypeDirectory, dirInfo.Type)
	assert.Empty(t, dirInfo.Extension)
}

func TestIndexProject_FileManifestContents(t *testing.T) {
	tmpDir := t.TempDir()

	content := "hello world"
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "test.go"), []byte(content), 0644))

	opts := DefaultOptions()
	opts.IncludeHidden = true
	idx := New(opts)

	manifest, err := idx.IndexProject(tmpDir)
	require.NoError(t, err)

	require.Len(t, manifest.Files, 1)
	fi := manifest.Files[0]

	assert.Equal(t, "test.go", fi.Path)
	assert.Equal(t, filepath.Join(tmpDir, "test.go"), fi.AbsolutePath)
	assert.Equal(t, int64(len(content)), fi.Size)
	assert.Equal(t, FileTypeRegular, fi.Type)
	assert.Equal(t, ".go", fi.Extension)
	assert.False(t, fi.ModTime.IsZero())
}

func TestIndexProject_ExcludePatterns(t *testing.T) {
	tmpDir := t.TempDir()

	// Create files
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "keep.txt"), []byte("keep"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "skip.tmp"), []byte("skip"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "vendor"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "vendor", "lib.go"), []byte("lib"), 0644))

	opts := DefaultOptions()
	opts.IncludeHidden = true
	opts.ExcludePatterns = append(opts.ExcludePatterns, "*.tmp", "vendor/")
	idx := New(opts)

	manifest, err := idx.IndexProject(tmpDir)
	require.NoError(t, err)

	paths := make([]string, 0, len(manifest.Files))
	for _, f := range manifest.Files {
		paths = append(paths, f.Path)
	}

	assert.Contains(t, paths, "keep.txt")
	assert.NotContains(t, paths, "skip.tmp")
	assert.NotContains(t, paths, "vendor")
}

func TestParseGitignore(t *testing.T) {
	tmpDir := t.TempDir()
	gitignorePath := filepath.Join(tmpDir, ".gitignore")

	content := `
# Comment line
*.log
build/
!important.log

temp/
node_modules
`
	require.NoError(t, os.WriteFile(gitignorePath, []byte(content), 0644))

	rules, err := parseGitignore(gitignorePath)
	require.NoError(t, err)

	require.Len(t, rules, 5)

	assert.Equal(t, "*.log", rules[0].pattern)
	assert.False(t, rules[0].negation)
	assert.False(t, rules[0].dirOnly)

	assert.Equal(t, "build", rules[1].pattern)
	assert.False(t, rules[1].negation)
	assert.True(t, rules[1].dirOnly)

	assert.Equal(t, "important.log", rules[2].pattern)
	assert.True(t, rules[2].negation)
	assert.False(t, rules[2].dirOnly)
}

func TestParseGitignore_NonExistent(t *testing.T) {
	_, err := parseGitignore("/nonexistent/.gitignore")
	assert.Error(t, err)
}

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		relPath  string
		baseName string
		isDir    bool
		expected bool
	}{
		{"exact match", "file.txt", "file.txt", "file.txt", false, true},
		{"glob match", "*.go", "main.go", "main.go", false, true},
		{"no match", "*.go", "main.txt", "main.txt", false, false},
		{"dir only match", "build/", "build", "build", true, true},
		{"dir only no match file", "build/", "build", "build", false, false},
		{"nested path", "src/*.go", filepath.Join("src", "main.go"), "main.go", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchPattern(tt.pattern, tt.relPath, tt.baseName, tt.isDir)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDetermineFileType(t *testing.T) {
	tmpDir := t.TempDir()

	// Create regular file
	filePath := filepath.Join(tmpDir, "regular.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("content"), 0644))

	// Create directory
	dirPath := filepath.Join(tmpDir, "directory")
	require.NoError(t, os.MkdirAll(dirPath, 0755))

	entries, err := os.ReadDir(tmpDir)
	require.NoError(t, err)

	for _, entry := range entries {
		ft := determineFileType(entry)
		if entry.Name() == "regular.txt" {
			assert.Equal(t, FileTypeRegular, ft)
		} else if entry.Name() == "directory" {
			assert.Equal(t, FileTypeDirectory, ft)
		}
	}
}

func TestNotADirectoryError(t *testing.T) {
	err := &NotADirectoryError{Path: "/some/path"}
	assert.Equal(t, "not a directory: /some/path", err.Error())
}

func TestIndexProject_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	idx := New(DefaultOptions())
	manifest, err := idx.IndexProject(tmpDir)
	require.NoError(t, err)

	assert.Equal(t, tmpDir, manifest.RootPath)
	assert.Empty(t, manifest.Files)
	assert.Equal(t, 0, manifest.TotalFiles)
	assert.Equal(t, 0, manifest.TotalDirs)
	assert.Equal(t, int64(0), manifest.TotalSize)
}

func TestIndexProject_RelativePathInput(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("test"), 0644))

	// Change to tmpDir to test relative path
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(originalDir)

	require.NoError(t, os.Chdir(tmpDir))

	opts := DefaultOptions()
	opts.IncludeHidden = true
	idx := New(opts)

	// Index with relative path "."
	manifest, err := idx.IndexProject(".")
	require.NoError(t, err)

	// Should resolve to absolute path (on macOS, /var may be symlinked to /private/var)
	// Just check that it's a valid absolute path containing the temp dir name
	assert.True(t, filepath.IsAbs(manifest.RootPath))
	assert.Contains(t, manifest.RootPath, filepath.Base(tmpDir))
	assert.Len(t, manifest.Files, 1)
}
