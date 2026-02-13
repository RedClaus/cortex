// Package web provides web-related tools for Pinky: fetching URLs,
// parsing HTML, and downloading files.
package web

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/normanking/pinky/internal/tools"
)

// FetchTool fetches content from a URL and returns the response body.
type FetchTool struct {
	client      *http.Client
	maxBodySize int64  // Maximum response body size to read
	userAgent   string // User-Agent header value
}

// FetchOption configures the FetchTool.
type FetchOption func(*FetchTool)

// WithTimeout sets the HTTP client timeout.
func WithTimeout(d time.Duration) FetchOption {
	return func(f *FetchTool) {
		f.client.Timeout = d
	}
}

// WithMaxBodySize sets the maximum response body size.
func WithMaxBodySize(size int64) FetchOption {
	return func(f *FetchTool) {
		f.maxBodySize = size
	}
}

// WithUserAgent sets the User-Agent header.
func WithUserAgent(ua string) FetchOption {
	return func(f *FetchTool) {
		f.userAgent = ua
	}
}

// NewFetchTool creates a new FetchTool with sensible defaults.
func NewFetchTool(opts ...FetchOption) *FetchTool {
	f := &FetchTool{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		maxBodySize: 10 * 1024 * 1024, // 10MB default
		userAgent:   "Pinky/1.0",
	}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

func (f *FetchTool) Name() string {
	return "web_fetch"
}

func (f *FetchTool) Description() string {
	return "Fetch content from a URL. Returns the response body as text. Supports GET, POST, PUT, DELETE methods."
}

func (f *FetchTool) Category() tools.ToolCategory {
	return tools.CategoryWeb
}

func (f *FetchTool) RiskLevel() tools.RiskLevel {
	return tools.RiskLow
}

func (f *FetchTool) Validate(input *tools.ToolInput) error {
	url, ok := input.Args["url"].(string)
	if !ok || url == "" {
		return fmt.Errorf("url is required")
	}

	// Basic URL validation
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return fmt.Errorf("url must start with http:// or https://")
	}

	// Validate method if provided
	if method, ok := input.Args["method"].(string); ok {
		method = strings.ToUpper(method)
		switch method {
		case "GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS":
			// Valid methods
		default:
			return fmt.Errorf("invalid HTTP method: %s", method)
		}
	}

	return nil
}

func (f *FetchTool) Execute(ctx context.Context, input *tools.ToolInput) (*tools.ToolOutput, error) {
	start := time.Now()

	url := input.Args["url"].(string)

	// Default to GET method
	method := "GET"
	if m, ok := input.Args["method"].(string); ok {
		method = strings.ToUpper(m)
	}

	// Build request body if provided
	var body io.Reader
	if bodyStr, ok := input.Args["body"].(string); ok && bodyStr != "" {
		body = strings.NewReader(bodyStr)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return &tools.ToolOutput{
			Success:  false,
			Error:    fmt.Sprintf("failed to create request: %v", err),
			Duration: time.Since(start),
		}, nil
	}

	// Set User-Agent
	req.Header.Set("User-Agent", f.userAgent)

	// Set custom headers if provided
	if headers, ok := input.Args["headers"].(map[string]any); ok {
		for k, v := range headers {
			if vs, ok := v.(string); ok {
				req.Header.Set(k, vs)
			}
		}
	}

	// Set Content-Type for POST/PUT if body is provided
	if body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return &tools.ToolOutput{
			Success:  false,
			Error:    fmt.Sprintf("request failed: %v", err),
			Duration: time.Since(start),
		}, nil
	}
	defer resp.Body.Close()

	// Read response body with size limit
	limitedReader := io.LimitReader(resp.Body, f.maxBodySize)
	respBody, err := io.ReadAll(limitedReader)
	if err != nil {
		return &tools.ToolOutput{
			Success:  false,
			Error:    fmt.Sprintf("failed to read response: %v", err),
			Duration: time.Since(start),
		}, nil
	}

	// Build output with status info
	output := fmt.Sprintf("Status: %s\nContent-Length: %d\n\n%s",
		resp.Status,
		len(respBody),
		string(respBody),
	)

	return &tools.ToolOutput{
		Success:   resp.StatusCode >= 200 && resp.StatusCode < 400,
		Output:    output,
		Duration:  time.Since(start),
		Artifacts: []string{url},
	}, nil
}
