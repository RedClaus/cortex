package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewAPITool(t *testing.T) {
	tool := NewAPITool()
	if tool == nil {
		t.Fatal("NewAPITool returned nil")
	}
	if tool.Name() != "api" {
		t.Errorf("Name() = %q, want %q", tool.Name(), "api")
	}
	if tool.Category() != CategoryAPI {
		t.Errorf("Category() = %v, want %v", tool.Category(), CategoryAPI)
	}
}

func TestAPITool_WithOptions(t *testing.T) {
	tool := NewAPITool(
		WithAllowedDomains([]string{"api.example.com"}),
		WithTimeout(10*time.Second),
		WithMaxResponseSize(1024),
		WithUserAgent("TestAgent/1.0"),
	)

	if len(tool.allowedDomains) != 1 || tool.allowedDomains[0] != "api.example.com" {
		t.Error("WithAllowedDomains not applied")
	}
	if tool.maxResponseSize != 1024 {
		t.Error("WithMaxResponseSize not applied")
	}
	if tool.userAgent != "TestAgent/1.0" {
		t.Error("WithUserAgent not applied")
	}
}

func TestAPITool_Validate_ValidURL(t *testing.T) {
	tool := NewAPITool()

	input := &ToolInput{
		Args: map[string]any{
			"url": "https://api.example.com/users",
		},
	}

	err := tool.Validate(input)
	if err != nil {
		t.Errorf("Validate failed for valid URL: %v", err)
	}
}

func TestAPITool_Validate_InvalidURL(t *testing.T) {
	tool := NewAPITool()

	tests := []struct {
		name string
		url  string
	}{
		{"empty", ""},
		{"no scheme", "api.example.com/users"},
		{"ftp scheme", "ftp://example.com/file"},
		{"invalid format", "not-a-url"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := &ToolInput{
				Args: map[string]any{
					"url": tt.url,
				},
			}
			err := tool.Validate(input)
			if err == nil {
				t.Errorf("Validate should fail for %q", tt.url)
			}
		})
	}
}

func TestAPITool_Validate_BlockedDomains(t *testing.T) {
	tool := NewAPITool()

	blockedURLs := []string{
		"http://localhost/api",
		"http://127.0.0.1/api",
		"http://169.254.169.254/metadata",
		"http://metadata.google.internal/computeMetadata",
	}

	for _, url := range blockedURLs {
		t.Run(url, func(t *testing.T) {
			input := &ToolInput{
				Args: map[string]any{"url": url},
			}
			err := tool.Validate(input)
			if err == nil {
				t.Errorf("Should block %q", url)
			}
		})
	}
}

func TestAPITool_Validate_AllowedDomains(t *testing.T) {
	tool := NewAPITool(WithAllowedDomains([]string{"api.github.com", "api.openai.com"}))

	tests := []struct {
		url     string
		allowed bool
	}{
		{"https://api.github.com/users", true},
		{"https://api.openai.com/v1/chat", true},
		{"https://subdomain.api.github.com/test", true},
		{"https://api.example.com/users", false},
		{"https://evil.com/steal", false},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			input := &ToolInput{
				Args: map[string]any{"url": tt.url},
			}
			err := tool.Validate(input)
			if tt.allowed && err != nil {
				t.Errorf("Should allow %q: %v", tt.url, err)
			}
			if !tt.allowed && err == nil {
				t.Errorf("Should block %q", tt.url)
			}
		})
	}
}

func TestAPITool_Validate_Methods(t *testing.T) {
	tool := NewAPITool()

	validMethods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "get", "post"}
	invalidMethods := []string{"INVALID", "OPTIONS", "CONNECT"}

	for _, method := range validMethods {
		input := &ToolInput{
			Args: map[string]any{
				"url":    "https://api.example.com/test",
				"method": method,
			},
		}
		if err := tool.Validate(input); err != nil {
			t.Errorf("Should allow method %q: %v", method, err)
		}
	}

	for _, method := range invalidMethods {
		input := &ToolInput{
			Args: map[string]any{
				"url":    "https://api.example.com/test",
				"method": method,
			},
		}
		if err := tool.Validate(input); err == nil {
			t.Errorf("Should block method %q", method)
		}
	}
}

func TestAPITool_Execute_GET(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Expected GET, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"message": "hello"})
	}))
	defer server.Close()

	tool := NewAPITool(WithTestMode())
	input := &ToolInput{
		Args: map[string]any{
			"url":    server.URL + "/test",
			"method": "GET",
		},
	}

	output, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !output.Success {
		t.Errorf("Expected success, got error: %s", output.Error)
	}
	if !strings.Contains(output.Output, "hello") {
		t.Errorf("Output should contain 'hello': %s", output.Output)
	}
}

func TestAPITool_Execute_POST(t *testing.T) {
	var receivedBody map[string]string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected application/json content type")
		}

		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"status": "created"})
	}))
	defer server.Close()

	tool := NewAPITool(WithTestMode())
	input := &ToolInput{
		Args: map[string]any{
			"url":    server.URL + "/users",
			"method": "POST",
			"body":   `{"name": "test"}`,
		},
	}

	output, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !output.Success {
		t.Errorf("Expected success, got error: %s", output.Error)
	}
	if receivedBody["name"] != "test" {
		t.Errorf("Body not received correctly: %v", receivedBody)
	}
}

func TestAPITool_Execute_WithHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Custom-Header") != "custom-value" {
			t.Error("Custom header not set")
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("Auth header not set")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tool := NewAPITool(WithTestMode())
	input := &ToolInput{
		Args: map[string]any{
			"url": server.URL + "/test",
			"headers": map[string]any{
				"X-Custom-Header": "custom-value",
			},
			"auth": map[string]any{
				"type":  "bearer",
				"token": "test-token",
			},
		},
	}

	output, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !output.Success {
		t.Errorf("Expected success: %s", output.Error)
	}
}

func TestAPITool_Execute_WithBasicAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if !ok || username != "user" || password != "pass" {
			t.Error("Basic auth not set correctly")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tool := NewAPITool(WithTestMode())
	input := &ToolInput{
		Args: map[string]any{
			"url": server.URL + "/test",
			"auth": map[string]any{
				"type":     "basic",
				"username": "user",
				"password": "pass",
			},
		},
	}

	output, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !output.Success {
		t.Errorf("Expected success: %s", output.Error)
	}
}

func TestAPITool_Execute_WithQueryParams(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") != "1" {
			t.Error("Query param 'page' not set")
		}
		if r.URL.Query().Get("limit") != "10" {
			t.Error("Query param 'limit' not set")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tool := NewAPITool(WithTestMode())
	input := &ToolInput{
		Args: map[string]any{
			"url": server.URL + "/users",
			"params": map[string]any{
				"page":  "1",
				"limit": 10, // Test non-string value
			},
		},
	}

	output, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !output.Success {
		t.Errorf("Expected success: %s", output.Error)
	}
}

func TestAPITool_Execute_ErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
	}))
	defer server.Close()

	tool := NewAPITool(WithTestMode())
	input := &ToolInput{
		Args: map[string]any{
			"url": server.URL + "/missing",
		},
	}

	output, err := tool.Execute(context.Background(), input)
	// Should not return an error, but Success should be false
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if output.Success {
		t.Error("Should not be successful for 404")
	}
	if !strings.Contains(output.Error, "404") {
		t.Errorf("Error should mention 404: %s", output.Error)
	}
}

func TestAPITool_Execute_ResponseTooLarge(t *testing.T) {
	// Create a server that returns a large response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Write more than 1KB
		for i := 0; i < 2000; i++ {
			w.Write([]byte("x"))
		}
	}))
	defer server.Close()

	tool := NewAPITool(WithTestMode(), WithMaxResponseSize(1024))
	input := &ToolInput{
		Args: map[string]any{
			"url": server.URL + "/large",
		},
	}

	output, err := tool.Execute(context.Background(), input)
	if err != ErrResponseTooLarge {
		t.Errorf("Expected ErrResponseTooLarge, got: %v", err)
	}
	if output.Success {
		t.Error("Should not be successful for large response")
	}
}

func TestAPITool_RiskLevelForMethod(t *testing.T) {
	tool := NewAPITool()

	tests := []struct {
		method HTTPMethod
		risk   RiskLevel
	}{
		{MethodGET, RiskLow},
		{MethodHEAD, RiskLow},
		{MethodPOST, RiskMedium},
		{MethodPUT, RiskMedium},
		{MethodPATCH, RiskMedium},
		{MethodDELETE, RiskMedium},
	}

	for _, tt := range tests {
		risk := tool.RiskLevelForMethod(tt.method)
		if risk != tt.risk {
			t.Errorf("RiskLevelForMethod(%s) = %v, want %v", tt.method, risk, tt.risk)
		}
	}
}

func TestAPITool_IsDomainAllowed(t *testing.T) {
	tool := NewAPITool(WithAllowedDomains([]string{"api.github.com", "example.com"}))

	tests := []struct {
		domain  string
		allowed bool
	}{
		{"api.github.com", true},
		{"example.com", true},
		{"sub.example.com", true},
		{"api.example.com", true},
		{"localhost", false},
		{"127.0.0.1", false},
		{"other.com", false},
	}

	for _, tt := range tests {
		result := tool.IsDomainAllowed(tt.domain)
		if result != tt.allowed {
			t.Errorf("IsDomainAllowed(%q) = %v, want %v", tt.domain, result, tt.allowed)
		}
	}
}

func TestAPITool_IsDomainAllowed_NoAllowedList(t *testing.T) {
	tool := NewAPITool() // No allowed domains = all non-blocked allowed

	if !tool.IsDomainAllowed("api.example.com") {
		t.Error("Should allow any non-blocked domain when no allowed list")
	}
	if tool.IsDomainAllowed("localhost") {
		t.Error("Should still block localhost")
	}
}

func TestAPITool_TriggerWebhook(t *testing.T) {
	var receivedPayload WebhookPayload

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		json.NewDecoder(r.Body).Decode(&receivedPayload)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tool := NewAPITool(WithTestMode())
	payload := &WebhookPayload{
		Event: "user.created",
		Data: map[string]any{
			"user_id": "123",
			"email":   "test@example.com",
		},
	}

	output, err := tool.TriggerWebhook(context.Background(), server.URL+"/webhook", payload)
	if err != nil {
		t.Fatalf("TriggerWebhook failed: %v", err)
	}
	if !output.Success {
		t.Errorf("Expected success: %s", output.Error)
	}

	if receivedPayload.Event != "user.created" {
		t.Errorf("Event = %q, want %q", receivedPayload.Event, "user.created")
	}
	if receivedPayload.Source != "pinky" {
		t.Errorf("Source = %q, want %q", receivedPayload.Source, "pinky")
	}
	if receivedPayload.Data["user_id"] != "123" {
		t.Error("Data not received correctly")
	}
}

func TestAPITool_Execute_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tool := NewAPITool(WithTestMode())
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	input := &ToolInput{
		Args: map[string]any{
			"url": server.URL + "/slow",
		},
	}

	_, err := tool.Execute(ctx, input)
	if err == nil {
		t.Error("Expected error for cancelled context")
	}
}

func TestAPITool_AddAllowedDomain(t *testing.T) {
	tool := NewAPITool()

	if tool.IsDomainAllowed("newdomain.com") == false && len(tool.allowedDomains) > 0 {
		// If there are already allowed domains, this domain should not be allowed
	}

	tool.SetAllowedDomains([]string{"existing.com"})
	tool.AddAllowedDomain("newdomain.com")

	if !tool.IsDomainAllowed("newdomain.com") {
		t.Error("AddAllowedDomain should add domain to allowed list")
	}
	if !tool.IsDomainAllowed("existing.com") {
		t.Error("Should still allow existing domain")
	}
}
