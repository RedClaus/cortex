// Package tools provides the tool execution framework for Pinky
package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Common errors for API tool
var (
	ErrInvalidURL        = errors.New("invalid URL")
	ErrDomainNotAllowed  = errors.New("domain not in allowed list")
	ErrMethodNotAllowed  = errors.New("HTTP method not allowed")
	ErrRequestFailed     = errors.New("HTTP request failed")
	ErrResponseTooLarge  = errors.New("response body too large")
	ErrInvalidJSON       = errors.New("invalid JSON body")
)

// HTTPMethod represents allowed HTTP methods
type HTTPMethod string

const (
	MethodGET    HTTPMethod = "GET"
	MethodPOST   HTTPMethod = "POST"
	MethodPUT    HTTPMethod = "PUT"
	MethodDELETE HTTPMethod = "DELETE"
	MethodPATCH  HTTPMethod = "PATCH"
	MethodHEAD   HTTPMethod = "HEAD"
)

// APITool executes HTTP API requests
type APITool struct {
	client          *http.Client
	allowedDomains  []string // Empty means all domains allowed
	blockedDomains  []string // Always blocked
	maxResponseSize int64    // Max response body size in bytes
	defaultTimeout  time.Duration
	userAgent       string
}

// APIToolOption configures the API tool
type APIToolOption func(*APITool)

// WithAllowedDomains sets the allowed domains for API requests
func WithAllowedDomains(domains []string) APIToolOption {
	return func(t *APITool) {
		t.allowedDomains = domains
	}
}

// WithBlockedDomains sets domains that are always blocked (replaces defaults)
func WithBlockedDomains(domains []string) APIToolOption {
	return func(t *APITool) {
		t.blockedDomains = domains
	}
}

// WithTestMode disables localhost blocking for testing
func WithTestMode() APIToolOption {
	return func(t *APITool) {
		// Remove localhost-related entries for testing
		t.blockedDomains = []string{
			"metadata.google.internal",
			"169.254.169.254",
		}
	}
}

// WithTimeout sets the default request timeout
func WithTimeout(d time.Duration) APIToolOption {
	return func(t *APITool) {
		t.defaultTimeout = d
		// Note: client timeout is set when client is created in NewAPITool
	}
}

// WithMaxResponseSize sets the maximum response body size
func WithMaxResponseSize(size int64) APIToolOption {
	return func(t *APITool) {
		t.maxResponseSize = size
	}
}

// WithUserAgent sets the User-Agent header
func WithUserAgent(ua string) APIToolOption {
	return func(t *APITool) {
		t.userAgent = ua
	}
}

// NewAPITool creates a new API tool with the given options
func NewAPITool(opts ...APIToolOption) *APITool {
	t := &APITool{
		blockedDomains: []string{
			"localhost",
			"127.0.0.1",
			"0.0.0.0",
			"::1",
			"metadata.google.internal",      // GCP metadata
			"169.254.169.254",               // AWS/Azure metadata
			"[::1]",                         // IPv6 localhost
			"metadata",                      // Generic metadata
			"instance-data",                 // EC2 instance metadata
		},
		maxResponseSize: 10 * 1024 * 1024, // 10MB default
		defaultTimeout:  30 * time.Second,
		userAgent:       "Pinky/1.0",
	}

	// Apply options first so blockedDomains can be customized
	for _, opt := range opts {
		opt(t)
	}

	// Create client with redirect validation AFTER blockedDomains is set
	t.client = &http.Client{
		Timeout: t.defaultTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errors.New("too many redirects")
			}
			// SECURITY: Re-validate redirect URL against blocked domains
			// This prevents SSRF via redirect chains to internal services
			host := strings.ToLower(req.URL.Hostname())
			for _, blocked := range t.blockedDomains {
				if host == strings.ToLower(blocked) || strings.HasSuffix(host, "."+strings.ToLower(blocked)) {
					return fmt.Errorf("redirect to blocked domain: %s", host)
				}
			}
			// Also block private IP ranges
			if isPrivateIP(host) {
				return fmt.Errorf("redirect to private IP: %s", host)
			}
			return nil
		},
	}

	return t
}

// isPrivateIP checks if a hostname is a private IP address
func isPrivateIP(host string) bool {
	// Check for common private IP patterns
	privatePatterns := []string{
		"10.",
		"192.168.",
		"172.16.", "172.17.", "172.18.", "172.19.",
		"172.20.", "172.21.", "172.22.", "172.23.",
		"172.24.", "172.25.", "172.26.", "172.27.",
		"172.28.", "172.29.", "172.30.", "172.31.",
		"169.254.", // Link-local
		"fc",       // IPv6 private
		"fd",       // IPv6 private
		"fe80:",    // IPv6 link-local
	}
	host = strings.ToLower(host)
	for _, pattern := range privatePatterns {
		if strings.HasPrefix(host, pattern) {
			return true
		}
	}
	return false
}

// Name returns the tool name
func (t *APITool) Name() string {
	return "api"
}

// Description returns the tool description
func (t *APITool) Description() string {
	return "Execute HTTP API requests (GET, POST, PUT, DELETE, PATCH) and trigger webhooks"
}

// Category returns the tool category
func (t *APITool) Category() ToolCategory {
	return CategoryAPI
}

// RiskLevel returns the risk level based on the HTTP method
func (t *APITool) RiskLevel() RiskLevel {
	return RiskLow // Default to low; actual risk determined per-request
}

// RiskLevelForMethod returns the risk level for a specific HTTP method
func (t *APITool) RiskLevelForMethod(method HTTPMethod) RiskLevel {
	switch method {
	case MethodGET, MethodHEAD:
		return RiskLow
	case MethodPOST, MethodPUT, MethodPATCH, MethodDELETE:
		return RiskMedium
	default:
		return RiskMedium
	}
}

// Validate checks if the input is valid for API execution
func (t *APITool) Validate(input *ToolInput) error {
	// Get URL from args
	urlStr, ok := input.Args["url"].(string)
	if !ok || urlStr == "" {
		return fmt.Errorf("%w: url is required", ErrInvalidURL)
	}

	// Parse and validate URL
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidURL, err)
	}

	// Check scheme
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("%w: scheme must be http or https", ErrInvalidURL)
	}

	// Check blocked domains
	host := strings.ToLower(parsedURL.Hostname())
	for _, blocked := range t.blockedDomains {
		if host == strings.ToLower(blocked) || strings.HasSuffix(host, "."+strings.ToLower(blocked)) {
			return fmt.Errorf("%w: %s", ErrDomainNotAllowed, host)
		}
	}

	// Check allowed domains if specified
	if len(t.allowedDomains) > 0 {
		allowed := false
		for _, domain := range t.allowedDomains {
			domain = strings.ToLower(domain)
			if host == domain || strings.HasSuffix(host, "."+domain) {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("%w: %s", ErrDomainNotAllowed, host)
		}
	}

	// Validate method if specified
	if methodStr, ok := input.Args["method"].(string); ok && methodStr != "" {
		method := HTTPMethod(strings.ToUpper(methodStr))
		switch method {
		case MethodGET, MethodPOST, MethodPUT, MethodDELETE, MethodPATCH, MethodHEAD:
			// Valid
		default:
			return fmt.Errorf("%w: %s", ErrMethodNotAllowed, methodStr)
		}
	}

	// Validate JSON body if present
	if body, ok := input.Args["body"].(string); ok && body != "" {
		if !json.Valid([]byte(body)) {
			// Check if it's meant to be JSON
			if contentType, ok := input.Args["content_type"].(string); ok {
				if strings.Contains(strings.ToLower(contentType), "json") {
					return ErrInvalidJSON
				}
			}
		}
	}

	return nil
}

// Execute performs the HTTP request
func (t *APITool) Execute(ctx context.Context, input *ToolInput) (*ToolOutput, error) {
	start := time.Now()

	// Validate first
	if err := t.Validate(input); err != nil {
		return &ToolOutput{
			Success:  false,
			Error:    err.Error(),
			Duration: time.Since(start),
		}, err
	}

	// Build request
	req, err := t.buildRequest(ctx, input)
	if err != nil {
		return &ToolOutput{
			Success:  false,
			Error:    err.Error(),
			Duration: time.Since(start),
		}, err
	}

	// Execute request
	resp, err := t.client.Do(req)
	if err != nil {
		return &ToolOutput{
			Success:  false,
			Error:    fmt.Sprintf("request failed: %v", err),
			Duration: time.Since(start),
		}, fmt.Errorf("%w: %v", ErrRequestFailed, err)
	}
	defer resp.Body.Close()

	// Read response with size limit
	limitReader := io.LimitReader(resp.Body, t.maxResponseSize+1)
	body, err := io.ReadAll(limitReader)
	if err != nil {
		return &ToolOutput{
			Success:  false,
			Error:    fmt.Sprintf("failed to read response: %v", err),
			Duration: time.Since(start),
		}, err
	}

	if int64(len(body)) > t.maxResponseSize {
		return &ToolOutput{
			Success:  false,
			Error:    fmt.Sprintf("response too large (max %d bytes)", t.maxResponseSize),
			Duration: time.Since(start),
		}, ErrResponseTooLarge
	}

	// Build output
	output := &ToolOutput{
		Success:  resp.StatusCode >= 200 && resp.StatusCode < 300,
		Duration: time.Since(start),
	}

	// Format response
	result := &APIResponse{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Headers:    make(map[string]string),
		Body:       string(body),
	}

	for key := range resp.Header {
		result.Headers[key] = resp.Header.Get(key)
	}

	// Try to pretty-print JSON responses
	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		var prettyJSON bytes.Buffer
		if err := json.Indent(&prettyJSON, body, "", "  "); err == nil {
			result.Body = prettyJSON.String()
		}
	}

	// Serialize response
	outputBytes, _ := json.MarshalIndent(result, "", "  ")
	output.Output = string(outputBytes)

	if !output.Success {
		output.Error = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	return output, nil
}

// APIResponse represents the structured API response
type APIResponse struct {
	StatusCode int               `json:"status_code"`
	Status     string            `json:"status"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
}

// buildRequest constructs the HTTP request from input
func (t *APITool) buildRequest(ctx context.Context, input *ToolInput) (*http.Request, error) {
	urlStr := input.Args["url"].(string)

	// Get method (default GET)
	method := MethodGET
	if methodStr, ok := input.Args["method"].(string); ok && methodStr != "" {
		method = HTTPMethod(strings.ToUpper(methodStr))
	}

	// Get body
	var bodyReader io.Reader
	if body, ok := input.Args["body"].(string); ok && body != "" {
		bodyReader = strings.NewReader(body)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, string(method), urlStr, bodyReader)
	if err != nil {
		return nil, err
	}

	// Set default headers
	req.Header.Set("User-Agent", t.userAgent)

	// Set content type for body
	if bodyReader != nil {
		contentType := "application/json"
		if ct, ok := input.Args["content_type"].(string); ok && ct != "" {
			contentType = ct
		}
		req.Header.Set("Content-Type", contentType)
	}

	// Set custom headers
	if headers, ok := input.Args["headers"].(map[string]any); ok {
		for key, value := range headers {
			if strVal, ok := value.(string); ok {
				req.Header.Set(key, strVal)
			}
		}
	}

	// Handle auth
	if auth, ok := input.Args["auth"].(map[string]any); ok {
		t.applyAuth(req, auth)
	}

	// Handle query params
	if params, ok := input.Args["params"].(map[string]any); ok {
		q := req.URL.Query()
		for key, value := range params {
			if strVal, ok := value.(string); ok {
				q.Set(key, strVal)
			} else {
				q.Set(key, fmt.Sprintf("%v", value))
			}
		}
		req.URL.RawQuery = q.Encode()
	}

	return req, nil
}

// applyAuth applies authentication to the request
func (t *APITool) applyAuth(req *http.Request, auth map[string]any) {
	authType, _ := auth["type"].(string)

	switch strings.ToLower(authType) {
	case "bearer":
		if token, ok := auth["token"].(string); ok {
			req.Header.Set("Authorization", "Bearer "+token)
		}
	case "basic":
		username, _ := auth["username"].(string)
		password, _ := auth["password"].(string)
		req.SetBasicAuth(username, password)
	case "api_key":
		key, _ := auth["key"].(string)
		value, _ := auth["value"].(string)
		location, _ := auth["location"].(string)
		switch strings.ToLower(location) {
		case "query":
			q := req.URL.Query()
			q.Set(key, value)
			req.URL.RawQuery = q.Encode()
		default: // header
			req.Header.Set(key, value)
		}
	}
}

// SetAllowedDomains updates the allowed domains list
func (t *APITool) SetAllowedDomains(domains []string) {
	t.allowedDomains = domains
}

// AddAllowedDomain adds a domain to the allowed list
func (t *APITool) AddAllowedDomain(domain string) {
	t.allowedDomains = append(t.allowedDomains, domain)
}

// IsDomainAllowed checks if a domain is allowed
func (t *APITool) IsDomainAllowed(domain string) bool {
	domain = strings.ToLower(domain)

	// Check blocked first
	for _, blocked := range t.blockedDomains {
		if domain == strings.ToLower(blocked) || strings.HasSuffix(domain, "."+strings.ToLower(blocked)) {
			return false
		}
	}

	// If no allowed list, all non-blocked domains are allowed
	if len(t.allowedDomains) == 0 {
		return true
	}

	// Check allowed list
	for _, allowed := range t.allowedDomains {
		allowed = strings.ToLower(allowed)
		if domain == allowed || strings.HasSuffix(domain, "."+allowed) {
			return true
		}
	}

	return false
}

// WebhookPayload represents a webhook trigger payload
type WebhookPayload struct {
	Event     string         `json:"event"`
	Timestamp time.Time      `json:"timestamp"`
	Data      map[string]any `json:"data"`
	Source    string         `json:"source"`
}

// TriggerWebhook sends a webhook notification
func (t *APITool) TriggerWebhook(ctx context.Context, webhookURL string, payload *WebhookPayload) (*ToolOutput, error) {
	// Set timestamp if not set
	if payload.Timestamp.IsZero() {
		payload.Timestamp = time.Now()
	}
	if payload.Source == "" {
		payload.Source = "pinky"
	}

	// Serialize payload
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize webhook payload: %w", err)
	}

	// Build input for Execute
	input := &ToolInput{
		Command: "webhook",
		Args: map[string]any{
			"url":          webhookURL,
			"method":       "POST",
			"body":         string(body),
			"content_type": "application/json",
		},
	}

	return t.Execute(ctx, input)
}
