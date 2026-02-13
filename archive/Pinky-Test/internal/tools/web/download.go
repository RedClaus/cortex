package web

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/normanking/pinky/internal/tools"
)

// DownloadTool downloads files from URLs to the local filesystem.
type DownloadTool struct {
	client       *http.Client
	maxFileSize  int64    // Maximum file size to download
	allowedDirs  []string // Directories where downloads are allowed
	userAgent    string
}

// DownloadOption configures the DownloadTool.
type DownloadOption func(*DownloadTool)

// WithDownloadTimeout sets the HTTP client timeout for downloads.
func WithDownloadTimeout(d time.Duration) DownloadOption {
	return func(dl *DownloadTool) {
		dl.client.Timeout = d
	}
}

// WithMaxFileSize sets the maximum file size for downloads.
func WithMaxFileSize(size int64) DownloadOption {
	return func(dl *DownloadTool) {
		dl.maxFileSize = size
	}
}

// WithAllowedDirs sets the directories where downloads are permitted.
func WithAllowedDirs(dirs []string) DownloadOption {
	return func(dl *DownloadTool) {
		dl.allowedDirs = dirs
	}
}

// NewDownloadTool creates a new DownloadTool with sensible defaults.
func NewDownloadTool(opts ...DownloadOption) *DownloadTool {
	dl := &DownloadTool{
		client: &http.Client{
			Timeout: 5 * time.Minute, // Downloads can take a while
		},
		maxFileSize: 100 * 1024 * 1024, // 100MB default
		userAgent:   "Pinky/1.0",
	}
	for _, opt := range opts {
		opt(dl)
	}
	return dl
}

func (dl *DownloadTool) Name() string {
	return "web_download"
}

func (dl *DownloadTool) Description() string {
	return "Download a file from a URL to the local filesystem. Specify the destination path or use the filename from the URL."
}

func (dl *DownloadTool) Category() tools.ToolCategory {
	return tools.CategoryWeb
}

func (dl *DownloadTool) RiskLevel() tools.RiskLevel {
	return tools.RiskLow
}

func (dl *DownloadTool) Validate(input *tools.ToolInput) error {
	urlStr, ok := input.Args["url"].(string)
	if !ok || urlStr == "" {
		return fmt.Errorf("url is required")
	}

	// Validate URL format
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("invalid url: %v", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("url must use http or https scheme")
	}

	// Validate destination if provided
	if dest, ok := input.Args["destination"].(string); ok && dest != "" {
		// Check if destination directory is in allowed list
		if len(dl.allowedDirs) > 0 {
			absPath, err := filepath.Abs(dest)
			if err != nil {
				return fmt.Errorf("invalid destination path: %v", err)
			}
			allowed := false
			for _, dir := range dl.allowedDirs {
				absDir, _ := filepath.Abs(dir)
				if strings.HasPrefix(absPath, absDir) {
					allowed = true
					break
				}
			}
			if !allowed {
				return fmt.Errorf("destination must be within allowed directories")
			}
		}

		// Check if parent directory exists
		parentDir := filepath.Dir(dest)
		if _, err := os.Stat(parentDir); os.IsNotExist(err) {
			return fmt.Errorf("destination directory does not exist: %s", parentDir)
		}
	}

	return nil
}

func (dl *DownloadTool) Execute(ctx context.Context, input *tools.ToolInput) (*tools.ToolOutput, error) {
	start := time.Now()

	urlStr := input.Args["url"].(string)

	// Determine destination path
	var destination string
	if dest, ok := input.Args["destination"].(string); ok && dest != "" {
		destination = dest
	} else {
		// Extract filename from URL
		parsedURL, _ := url.Parse(urlStr)
		filename := filepath.Base(parsedURL.Path)
		if filename == "" || filename == "/" || filename == "." {
			filename = "download"
		}
		// Use working directory if specified, otherwise current directory
		if input.WorkingDir != "" {
			destination = filepath.Join(input.WorkingDir, filename)
		} else {
			destination = filename
		}
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return &tools.ToolOutput{
			Success:  false,
			Error:    fmt.Sprintf("failed to create request: %v", err),
			Duration: time.Since(start),
		}, nil
	}
	req.Header.Set("User-Agent", dl.userAgent)

	// Execute request
	resp, err := dl.client.Do(req)
	if err != nil {
		return &tools.ToolOutput{
			Success:  false,
			Error:    fmt.Sprintf("download failed: %v", err),
			Duration: time.Since(start),
		}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &tools.ToolOutput{
			Success:  false,
			Error:    fmt.Sprintf("download failed with status: %s", resp.Status),
			Duration: time.Since(start),
		}, nil
	}

	// Check content length against max file size
	if resp.ContentLength > dl.maxFileSize {
		return &tools.ToolOutput{
			Success: false,
			Error: fmt.Sprintf("file too large: %d bytes (max: %d bytes)",
				resp.ContentLength, dl.maxFileSize),
			Duration: time.Since(start),
		}, nil
	}

	// Create destination file
	file, err := os.Create(destination)
	if err != nil {
		return &tools.ToolOutput{
			Success:  false,
			Error:    fmt.Sprintf("failed to create file: %v", err),
			Duration: time.Since(start),
		}, nil
	}
	defer file.Close()

	// Copy with size limit
	limitedReader := io.LimitReader(resp.Body, dl.maxFileSize)
	written, err := io.Copy(file, limitedReader)
	if err != nil {
		os.Remove(destination) // Clean up partial download
		return &tools.ToolOutput{
			Success:  false,
			Error:    fmt.Sprintf("failed to write file: %v", err),
			Duration: time.Since(start),
		}, nil
	}

	absPath, _ := filepath.Abs(destination)

	return &tools.ToolOutput{
		Success: true,
		Output: fmt.Sprintf("Downloaded %d bytes to %s",
			written, absPath),
		Duration:  time.Since(start),
		Artifacts: []string{absPath},
	}, nil
}
