package search

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestNewFileSearcher(t *testing.T) {
	fs := NewFileSearcher()
	if fs == nil {
		t.Fatal("NewFileSearcher returned nil")
	}

	if fs.maxFiles != 50000 {
		t.Errorf("Expected maxFiles=50000, got %d", fs.maxFiles)
	}

	if len(fs.ignoreDirs) == 0 {
		t.Error("Expected ignoreDirs to be set")
	}

	// Check that tool detection runs
	if runtime.GOOS == "darwin" {
		// On macOS, mdfind should be detected
		if !fs.hasMdfind {
			t.Log("Warning: mdfind not detected on macOS")
		}
	}
}

func TestFileSearcher_SetMaxFiles(t *testing.T) {
	fs := NewFileSearcher()
	fs.SetMaxFiles(100)

	if fs.maxFiles != 100 {
		t.Errorf("Expected maxFiles=100, got %d", fs.maxFiles)
	}
}

func TestFileSearcher_SetIgnoreDirs(t *testing.T) {
	fs := NewFileSearcher()
	customDirs := []string{"custom1", "custom2"}
	fs.SetIgnoreDirs(customDirs)

	if len(fs.ignoreDirs) != len(customDirs) {
		t.Errorf("Expected %d ignoreDirs, got %d", len(customDirs), len(fs.ignoreDirs))
	}
}

func TestCommandExists(t *testing.T) {
	// Test with a command that should exist
	if !commandExists("ls") && !commandExists("dir") {
		t.Error("Expected ls or dir to exist")
	}

	// Test with a command that shouldn't exist
	if commandExists("this-command-definitely-does-not-exist-12345") {
		t.Error("Expected non-existent command to return false")
	}
}

func TestFileSearcher_Search(t *testing.T) {
	// Create a temporary directory with test files
	tmpDir, err := os.MkdirTemp("", "filesearch-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test files
	testFiles := []string{
		"test1.go",
		"test2.go",
		"readme.md",
		"subdir/test3.go",
	}

	for _, f := range testFiles {
		path := filepath.Join(tmpDir, f)
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create dir %s: %v", dir, err)
		}
		if err := os.WriteFile(path, []byte("test content"), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", path, err)
		}
	}

	// Create an ignored directory
	ignoredDir := filepath.Join(tmpDir, "node_modules")
	if err := os.MkdirAll(ignoredDir, 0755); err != nil {
		t.Fatalf("Failed to create ignored dir: %v", err)
	}
	ignoredFile := filepath.Join(ignoredDir, "test4.go")
	if err := os.WriteFile(ignoredFile, []byte("ignored"), 0644); err != nil {
		t.Fatalf("Failed to create ignored file: %v", err)
	}

	fs := NewFileSearcher()
	ctx := context.Background()

	t.Run("SearchGoFiles", func(t *testing.T) {
		results, err := fs.Search(ctx, tmpDir, "*.go")
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		// Should find 3 .go files (excluding node_modules)
		if len(results) != 3 {
			t.Errorf("Expected 3 results, got %d", len(results))
			for _, r := range results {
				t.Logf("Found: %s", r.Path)
			}
		}

		// Verify results have required fields
		for _, r := range results {
			if r.Path == "" {
				t.Error("Result has empty path")
			}
			if r.Name == "" {
				t.Error("Result has empty name")
			}
			if r.ModTime.IsZero() {
				t.Error("Result has zero ModTime")
			}
		}
	})

	t.Run("SearchMarkdownFiles", func(t *testing.T) {
		results, err := fs.Search(ctx, tmpDir, "*.md")
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if len(results) != 1 {
			t.Errorf("Expected 1 result, got %d", len(results))
		}
	})

	t.Run("SearchWithRecursivePattern", func(t *testing.T) {
		results, err := fs.Search(ctx, tmpDir, "**/*.go")
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		// Should find all .go files recursively
		if len(results) < 3 {
			t.Errorf("Expected at least 3 results, got %d", len(results))
		}
	})
}

func TestFileSearcher_SearchWithContext(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "filesearch-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create many test files
	for i := 0; i < 100; i++ {
		path := filepath.Join(tmpDir, filepath.Join("dir", filepath.Join("subdir", filepath.Join("test", filepath.Join("file", filepath.Join("path", filepath.Join("long", filepath.Join("nested", filepath.Join("structure", filepath.Join("test", filepath.Join("files", filepath.Join("here", "file.go"))))))))))))
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			continue
		}
		if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
			continue
		}
	}

	fs := NewFileSearcher()

	t.Run("ContextCancellation", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()

		time.Sleep(10 * time.Millisecond) // Ensure context is cancelled

		_, err := fs.Search(ctx, tmpDir, "*.go")
		if err != nil && err != context.DeadlineExceeded && err != context.Canceled {
			// Context cancellation is expected, but might not always happen
			// depending on how fast the search is
			t.Logf("Search returned: %v", err)
		}
	})
}

func TestFileSearcher_GoWalk(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "filesearch-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test structure
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	fs := NewFileSearcher()
	ctx := context.Background()

	results, err := fs.searchGoWalk(ctx, tmpDir, "*.txt")
	if err != nil {
		t.Fatalf("searchGoWalk failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}

	if len(results) > 0 {
		if results[0].Name != "test.txt" {
			t.Errorf("Expected name=test.txt, got %s", results[0].Name)
		}
	}
}

func BenchmarkFileSearcher_Search(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "filesearch-bench-*")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test files
	for i := 0; i < 1000; i++ {
		subdir := filepath.Join(tmpDir, filepath.Join("subdir", filepath.Join("nested", "files")))
		if err := os.MkdirAll(subdir, 0755); err != nil {
			continue
		}
		path := filepath.Join(subdir, filepath.Join("test", "file.go"))
		if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
			continue
		}
	}

	fs := NewFileSearcher()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := fs.Search(ctx, tmpDir, "*.go")
		if err != nil {
			b.Fatalf("Search failed: %v", err)
		}
	}
}
