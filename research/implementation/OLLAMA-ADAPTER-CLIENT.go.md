---
project: Cortex
component: Docs
phase: Build
date_created: 2026-02-01T15:33:22
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:16:29.605520
---

# OLLAMA-ADAPTER-CLIENT.go

```go
// File: ~/clawd/ollama-adapter/client.go
package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "time"
)

// Ollama coding request
type OllamaRequest struct {
    Model  string `json:"model"`
    Prompt string `json:"prompt"`
    Stream bool   `json:"stream"`
}

// Ollama coding response
type OllamaResponse struct {
    Model    string `json:"model"`
    Response string `json:"response"`
    Done     bool   `json:"done"`
}

// Ollama client
type OllamaClient struct {
    BaseURL    string
    HTTPClient *http.Client
}

// New Ollama client
func NewOllamaClient(baseURL string) *OllamaClient {
    return &OllamaClient{
        BaseURL: baseURL,
        HTTPClient: &http.Client{
            Timeout: 60 * time.Second,
        },
    }
}

// Send coding task to Ollama
func (c *OllamaClient) SendCodingTask(model, prompt string) (string, error) {
    req := OllamaRequest{
        Model:  model,
        Prompt: prompt,
        Stream: false,
    }

    reqBytes, err := json.Marshal(req)
    if err != nil {
        return "", fmt.Errorf("failed to marshal request: %w", err)
    }

    resp, err := c.HTTPClient.Post(
        c.BaseURL+"/api/generate",
        "application/json",
        bytes.NewReader(reqBytes),
    )
    if err != nil {
        return "", fmt.Errorf("failed to send request: %w", err)
    }
    defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return "", fmt.Errorf("failed to read response: %w", err)
    }

    var ollamaResp OllamaResponse
    if err := json.Unmarshal(body, &ollamaResp); err != nil {
        return "", fmt.Errorf("failed to unmarshal response: %w", err)
    }

    return ollamaResp.Response, nil
}

// Test Ollama client
func main() {
    client := NewOllamaClient("http://localhost:11434")

    code, err := client.SendCodingTask(
        "deepseek-coder-v2-lite",
        "Write a Go function to calculate fibonacci numbers",
    )
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }

    fmt.Printf("Generated code:\n%s\n", code)
}
```

---

## HOW TO USE

```bash
# 1. On Pink (192.168.1.186)
cd ~/clawd
mkdir -p ollama-adapter
cd ollama-adapter

# 2. Create Go module
go mod init github.com/normanking/clawd/ollama-adapter

# 3. Save client.go (file above)
nano client.go
# Paste the code and save

# 4. Build
go build -o ollama-client client.go

# 5. Test (after Ollama is installed and running)
./ollama-client

# Expected output:
# Generated code:
# func fibonacci(n int) int {
#     if n <= 1 {
#         return n
#     }
#     return fibonacci(n-1) + fibonacci(n-2)
# }
```

---

## DEPENDENCIES

```bash
# No external dependencies required
# Uses only Go standard library (encoding/json, net/http, io)
```

---

## TESTING

```bash
# Test 1: Fibonacci function
./ollama-client

# Test 2: API test (after building)
curl http://localhost:11434/api/generate \
  -H "Content-Type: application/json" \
  -d '{
    "model": "deepseek-coder-v2-lite",
    "prompt": "Write a Python function to calculate fibonacci",
    "stream": false
  }'
```

---

## NOTES

- Uses standard HTTP POST to Ollama API
- Timeout: 60 seconds
- Supports any Ollama model (change model name)
- Returns generated code as string
- Error handling included