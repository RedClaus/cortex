package tools

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// WebTool handles HTTP requests
type WebTool struct {
	client       *http.Client
	maxBodySize  int64
	allowedHosts []string
	blockedHosts []string
	userAgent    string
}

// WebConfig configures the web tool
type WebConfig struct {
	Timeout      time.Duration
	MaxBodySize  int64
	AllowedHosts []string
	BlockedHosts []string
	UserAgent    string
}

// DefaultWebConfig returns sensible defaults
func DefaultWebConfig() *WebConfig {
	return &WebConfig{
		Timeout:      30 * time.Second,
		MaxBodySize:  5 * 1024 * 1024, // 5MB
		AllowedHosts: nil,             // Allow all by default
		BlockedHosts: nil,
		UserAgent:    "Pinky/1.0",
	}
}

// NewWebTool creates a new web tool
func NewWebTool(cfg *WebConfig) *WebTool {
	if cfg == nil {
		cfg = DefaultWebConfig()
	}

	return &WebTool{
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
		maxBodySize:  cfg.MaxBodySize,
		allowedHosts: cfg.AllowedHosts,
		blockedHosts: cfg.BlockedHosts,
		userAgent:    cfg.UserAgent,
	}
}

func (t *WebTool) Name() string           { return "web" }
func (t *WebTool) Category() ToolCategory { return CategoryWeb }
func (t *WebTool) RiskLevel() RiskLevel   { return RiskLow }

func (t *WebTool) Description() string {
	return "Fetch web content. Supports GET and POST requests."
}

// Spec returns the tool specification for LLM function calling
func (t *WebTool) Spec() *ToolSpec {
	return &ToolSpec{
		Name:        t.Name(),
		Description: t.Description(),
		Category:    t.Category(),
		RiskLevel:   t.RiskLevel(),
		Parameters: &ParamSchema{
			Type: "object",
			Properties: map[string]*ParamProp{
				"url": {
					Type:        "string",
					Description: "The URL to fetch",
				},
				"method": {
					Type:        "string",
					Description: "HTTP method (GET, POST, etc.)",
					Enum:        []string{"GET", "POST", "PUT", "DELETE", "HEAD"},
					Default:     "GET",
				},
				"headers": {
					Type:        "object",
					Description: "HTTP headers to include",
				},
				"body": {
					Type:        "string",
					Description: "Request body (for POST/PUT)",
				},
			},
			Required: []string{"url"},
		},
	}
}

// Validate checks if the input is valid
func (t *WebTool) Validate(input *ToolInput) error {
	if input == nil {
		return errors.New("input is nil")
	}

	rawURL, ok := input.Args["url"].(string)
	if !ok || rawURL == "" {
		return errors.New("url is required")
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("invalid URL scheme: %s (must be http or https)", parsedURL.Scheme)
	}

	// Check host restrictions
	if !t.isHostAllowed(parsedURL.Host) {
		return fmt.Errorf("host not allowed: %s", parsedURL.Host)
	}

	return nil
}

// Execute performs the HTTP request
func (t *WebTool) Execute(ctx context.Context, input *ToolInput) (*ToolOutput, error) {
	rawURL := input.Args["url"].(string)

	method := "GET"
	if m, ok := input.Args["method"].(string); ok && m != "" {
		method = strings.ToUpper(m)
	}

	var bodyReader io.Reader
	if body, ok := input.Args["body"].(string); ok && body != "" {
		bodyReader = strings.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, rawURL, bodyReader)
	if err != nil {
		return &ToolOutput{
			Success: false,
			Error:   fmt.Sprintf("failed to create request: %v", err),
		}, nil
	}

	// Set user agent
	req.Header.Set("User-Agent", t.userAgent)

	// Set custom headers
	if headers, ok := input.Args["headers"].(map[string]any); ok {
		for k, v := range headers {
			if vs, ok := v.(string); ok {
				req.Header.Set(k, vs)
			}
		}
	}

	start := time.Now()
	resp, err := t.client.Do(req)
	if err != nil {
		return &ToolOutput{
			Success: false,
			Error:   fmt.Sprintf("request failed: %v", err),
		}, nil
	}
	defer resp.Body.Close()

	// Limit body size
	limitedReader := io.LimitReader(resp.Body, t.maxBodySize)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return &ToolOutput{
			Success: false,
			Error:   fmt.Sprintf("failed to read response: %v", err),
		}, nil
	}

	duration := time.Since(start)

	// Build response info
	var output strings.Builder
	output.WriteString(fmt.Sprintf("Status: %s\n", resp.Status))
	output.WriteString(fmt.Sprintf("Content-Type: %s\n", resp.Header.Get("Content-Type")))
	output.WriteString(fmt.Sprintf("Content-Length: %d\n\n", len(body)))
	output.Write(body)

	return &ToolOutput{
		Success:  resp.StatusCode >= 200 && resp.StatusCode < 400,
		Output:   output.String(),
		Duration: duration,
	}, nil
}

func (t *WebTool) isHostAllowed(host string) bool {
	// Check blocked hosts first
	for _, blocked := range t.blockedHosts {
		if strings.HasSuffix(host, blocked) || host == blocked {
			return false
		}
	}

	// If allowed hosts are specified, check them
	if len(t.allowedHosts) > 0 {
		for _, allowed := range t.allowedHosts {
			if strings.HasSuffix(host, allowed) || host == allowed {
				return true
			}
		}
		return false
	}

	return true
}
