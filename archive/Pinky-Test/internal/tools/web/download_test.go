package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/normanking/pinky/internal/tools"
)

func TestDownloadTool_Name(t *testing.T) {
	dl := NewDownloadTool()
	if dl.Name() != "web_download" {
		t.Errorf("expected name 'web_download', got '%s'", dl.Name())
	}
}

func TestDownloadTool_Category(t *testing.T) {
	dl := NewDownloadTool()
	if dl.Category() != tools.CategoryWeb {
		t.Errorf("expected category 'web', got '%s'", dl.Category())
	}
}

func TestDownloadTool_RiskLevel(t *testing.T) {
	dl := NewDownloadTool()
	if dl.RiskLevel() != tools.RiskLow {
		t.Errorf("expected risk level 'low', got '%s'", dl.RiskLevel())
	}
}

func TestDownloadTool_Validate(t *testing.T) {
	dl := NewDownloadTool()

	tests := []struct {
		name    string
		input   *tools.ToolInput
		wantErr bool
	}{
		{
			name: "valid URL",
			input: &tools.ToolInput{
				Args: map[string]any{"url": "https://example.com/file.txt"},
			},
			wantErr: false,
		},
		{
			name: "missing URL",
			input: &tools.ToolInput{
				Args: map[string]any{},
			},
			wantErr: true,
		},
		{
			name: "invalid URL scheme",
			input: &tools.ToolInput{
				Args: map[string]any{"url": "ftp://example.com/file.txt"},
			},
			wantErr: true,
		},
		{
			name: "invalid URL",
			input: &tools.ToolInput{
				Args: map[string]any{"url": "not a url"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := dl.Validate(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDownloadTool_Validate_AllowedDirs(t *testing.T) {
	tmpDir := t.TempDir()
	dl := NewDownloadTool(WithAllowedDirs([]string{tmpDir}))

	tests := []struct {
		name        string
		destination string
		wantErr     bool
	}{
		{
			name:        "allowed directory",
			destination: filepath.Join(tmpDir, "file.txt"),
			wantErr:     false,
		},
		{
			name:        "disallowed directory",
			destination: "/etc/file.txt",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := &tools.ToolInput{
				Args: map[string]any{
					"url":         "https://example.com/file.txt",
					"destination": tt.destination,
				},
			}
			err := dl.Validate(input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDownloadTool_Execute(t *testing.T) {
	// Create a test server
	fileContent := "Hello, Pinky! This is a test file."
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fileContent))
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	destination := filepath.Join(tmpDir, "downloaded.txt")

	dl := NewDownloadTool()
	input := &tools.ToolInput{
		Args: map[string]any{
			"url":         server.URL + "/file.txt",
			"destination": destination,
		},
	}

	output, err := dl.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !output.Success {
		t.Errorf("expected success, got error: %s", output.Error)
	}

	// Verify file was created
	content, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("failed to read downloaded file: %v", err)
	}

	if string(content) != fileContent {
		t.Errorf("expected content '%s', got '%s'", fileContent, string(content))
	}

	// Check artifacts
	if len(output.Artifacts) != 1 {
		t.Errorf("expected 1 artifact, got %d", len(output.Artifacts))
	}
}

func TestDownloadTool_Execute_AutoFilename(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test content"))
	}))
	defer server.Close()

	tmpDir := t.TempDir()

	dl := NewDownloadTool()
	input := &tools.ToolInput{
		Args: map[string]any{
			"url": server.URL + "/myfile.txt",
		},
		WorkingDir: tmpDir,
	}

	output, err := dl.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !output.Success {
		t.Errorf("expected success, got error: %s", output.Error)
	}

	// Verify file was created with auto-detected filename
	expectedPath := filepath.Join(tmpDir, "myfile.txt")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("expected file at %s to exist", expectedPath)
	}
}

func TestDownloadTool_Execute_ErrorResponse(t *testing.T) {
	// Create a test server that returns 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Not Found"))
	}))
	defer server.Close()

	tmpDir := t.TempDir()

	dl := NewDownloadTool()
	input := &tools.ToolInput{
		Args: map[string]any{
			"url":         server.URL + "/missing.txt",
			"destination": filepath.Join(tmpDir, "file.txt"),
		},
	}

	output, err := dl.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if output.Success {
		t.Error("expected failure for 404 response")
	}
}

func TestDownloadTool_Execute_FileTooLarge(t *testing.T) {
	// Create a test server that claims a large file
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "999999999999") // Very large
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("small content"))
	}))
	defer server.Close()

	tmpDir := t.TempDir()

	dl := NewDownloadTool(WithMaxFileSize(1024)) // 1KB limit
	input := &tools.ToolInput{
		Args: map[string]any{
			"url":         server.URL + "/large.txt",
			"destination": filepath.Join(tmpDir, "file.txt"),
		},
	}

	output, err := dl.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if output.Success {
		t.Error("expected failure for file too large")
	}

	if !strings.Contains(output.Error, "too large") {
		t.Errorf("expected 'too large' error, got: %s", output.Error)
	}
}

func TestDownloadTool_WithOptions(t *testing.T) {
	dl := NewDownloadTool(
		WithDownloadTimeout(10*time.Second),
		WithMaxFileSize(1024),
		WithAllowedDirs([]string{"/tmp", "/var"}),
	)

	if dl.maxFileSize != 1024 {
		t.Errorf("expected maxFileSize 1024, got %d", dl.maxFileSize)
	}
	if len(dl.allowedDirs) != 2 {
		t.Errorf("expected 2 allowed dirs, got %d", len(dl.allowedDirs))
	}
}
