package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/normanking/pinky/internal/tools"
)

func TestFetchTool_Name(t *testing.T) {
	f := NewFetchTool()
	if f.Name() != "web_fetch" {
		t.Errorf("expected name 'web_fetch', got '%s'", f.Name())
	}
}

func TestFetchTool_Category(t *testing.T) {
	f := NewFetchTool()
	if f.Category() != tools.CategoryWeb {
		t.Errorf("expected category 'web', got '%s'", f.Category())
	}
}

func TestFetchTool_RiskLevel(t *testing.T) {
	f := NewFetchTool()
	if f.RiskLevel() != tools.RiskLow {
		t.Errorf("expected risk level 'low', got '%s'", f.RiskLevel())
	}
}

func TestFetchTool_Validate(t *testing.T) {
	f := NewFetchTool()

	tests := []struct {
		name    string
		input   *tools.ToolInput
		wantErr bool
	}{
		{
			name: "valid GET request",
			input: &tools.ToolInput{
				Args: map[string]any{"url": "https://example.com"},
			},
			wantErr: false,
		},
		{
			name: "valid POST request",
			input: &tools.ToolInput{
				Args: map[string]any{
					"url":    "https://example.com",
					"method": "POST",
				},
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
				Args: map[string]any{"url": "ftp://example.com"},
			},
			wantErr: true,
		},
		{
			name: "invalid method",
			input: &tools.ToolInput{
				Args: map[string]any{
					"url":    "https://example.com",
					"method": "INVALID",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := f.Validate(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFetchTool_Execute_GET(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET request, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello, Pinky!"))
	}))
	defer server.Close()

	f := NewFetchTool()
	input := &tools.ToolInput{
		Args: map[string]any{"url": server.URL},
	}

	output, err := f.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !output.Success {
		t.Errorf("expected success, got error: %s", output.Error)
	}

	if output.Output == "" {
		t.Error("expected non-empty output")
	}
}

func TestFetchTool_Execute_POST(t *testing.T) {
	// Create a test server that expects POST
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST request, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Posted!"))
	}))
	defer server.Close()

	f := NewFetchTool()
	input := &tools.ToolInput{
		Args: map[string]any{
			"url":    server.URL,
			"method": "POST",
			"body":   "test=data",
		},
	}

	output, err := f.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !output.Success {
		t.Errorf("expected success, got error: %s", output.Error)
	}
}

func TestFetchTool_Execute_CustomHeaders(t *testing.T) {
	// Create a test server that checks headers
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Custom-Header") != "test-value" {
			t.Error("expected custom header to be set")
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	f := NewFetchTool()
	input := &tools.ToolInput{
		Args: map[string]any{
			"url": server.URL,
			"headers": map[string]any{
				"X-Custom-Header": "test-value",
			},
		},
	}

	output, err := f.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !output.Success {
		t.Errorf("expected success, got error: %s", output.Error)
	}
}

func TestFetchTool_Execute_ErrorResponse(t *testing.T) {
	// Create a test server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	f := NewFetchTool()
	input := &tools.ToolInput{
		Args: map[string]any{"url": server.URL},
	}

	output, err := f.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// 5xx is not success
	if output.Success {
		t.Error("expected failure for 5xx response")
	}
}

func TestFetchTool_WithOptions(t *testing.T) {
	f := NewFetchTool(
		WithTimeout(10*time.Second),
		WithMaxBodySize(1024),
		WithUserAgent("TestAgent/1.0"),
	)

	if f.maxBodySize != 1024 {
		t.Errorf("expected maxBodySize 1024, got %d", f.maxBodySize)
	}
	if f.userAgent != "TestAgent/1.0" {
		t.Errorf("expected userAgent 'TestAgent/1.0', got '%s'", f.userAgent)
	}
}
