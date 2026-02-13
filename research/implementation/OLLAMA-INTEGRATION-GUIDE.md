---
project: Cortex
component: UI
phase: Build
date_created: 2026-02-01T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:16:29.636523
---

# OLLAMA-INTEGRATION-GUIDE.md

**Date:** February 1, 2026
**Objective:** Install Ollama + DeepSeek-Coder-V2-Lite on Pink (RTX 3090) for small coding tasks

---

## 📋 OVERVIEW

**Target:** Pink (192.168.1.186) - NVIDIA RTX 3090 (24GB VRAM)
**Model:** DeepSeek-Coder-V2-Lite (16B, quantized to 9.2GB)
**Framework:** Ollama
**Purpose:** Offload small coding tasks from Pink/Red agents

---

## 🎯 PHASE 1: INSTALL OLLAMA ON PINK (1 hour)

### **Step 1: SSH to Pink**

```bash
ssh normanking@192.168.1.186
# Password: Zer0k1ng
```

### **Step 2: Install Ollama**

```bash
# Install Ollama (macOS)
brew install ollama

# Or if on Linux:
# curl -fsSL https://ollama.com/install.sh | sh
```

### **Step 3: Verify Installation**

```bash
ollama --version
# Expected output: ollama version is 0.x.x
```

### **Step 4: Start Ollama Service**

```bash
# Start Ollama server (runs in background)
ollama serve

# Or run as service (macOS):
brew services start ollama

# Or run as service (Linux):
sudo systemctl start ollama
```

### **Step 5: Verify Ollama Server**

```bash
# Check if Ollama server is running
curl http://localhost:11434/api/tags

# Expected output: List of available models (empty initially)
```

---

## 🎯 PHASE 2: PULL DEEPSEEK-CODER-V2-LITE (30 min)

### **Step 1: Pull Model**

```bash
# Pull DeepSeek-Coder-V2-Lite model
ollama pull deepseek-coder-v2-lite

# This downloads ~9.2GB (quantized model)
```

### **Step 2: Verify Model**

```bash
# Check available models
ollama list

# Expected output:
# NAME                    ID              SIZE      MODIFIED
# deepseek-coder-v2-lite   sha256:xxxxx    9.2 GB    2026-02-01
```

---

## 🎯 PHASE 3: TEST MODEL (30 min)

### **Step 1: Interactive Test**

```bash
# Test DeepSeek-Coder with a simple coding task
ollama run deepseek-coder-v2-lite "Write a Go function to calculate fibonacci numbers"
```

### **Step 2: API Test**

```bash
# Test Ollama API
curl http://localhost:11434/api/generate \
  -H "Content-Type: application/json" \
  -d '{
    "model": "deepseek-coder-v2-lite",
    "prompt": "Write a Python function to calculate fibonacci numbers",
    "stream": false
  }'
```

### **Step 3: Verify API Output**

```bash
# Expected output: JSON with "response" field containing generated code
{
  "model": "deepseek-coder-v2-lite",
  "response": "def fibonacci(n):\n    if n <= 1:\n        return n\n    return fibonacci(n-1) + fibonacci(n-2)\n",
  "done": true
}
```

---

## 🎯 PHASE 4: BUILD A2A ADAPTER (2-3 hours)

### **Step 1: Create Ollama A2A Adapter Directory**

```bash
# On Pink
cd ~/clawd
mkdir -p ollama-adapter
cd ollama-adapter
```

### **Step 2: Create Go Module**

```bash
# Initialize Go module
go mod init github.com/normanking/clawd/ollama-adapter
```

### **Step 3: Create Ollama Client (Go)**

```go
// File: ollama-adapter/client.go
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

### **Step 4: Build and Test Adapter**

```bash
# Download dependencies
go mod tidy

# Build
go build -o ollama-client client.go

# Test
./ollama-client

# Expected output: Generated Go function for fibonacci
```

---

## 🎯 PHASE 5: INTEGRATE WITH HAROLD (2-3 hours)

### **Step 1: Add Ollama Agent to Harold**

```bash
# On Harold (192.168.1.229)
cd ~/clawd/cortex-brain/pkg/brain/lobes/agent

# Create Ollama agent file
touch ollama_agent.go
```

### **Step 2: Implement Ollama Agent**

```go
// File: cortex-brain/pkg/brain/lobes/agent/ollama_agent.go
package agent

import (
    "context"
    "fmt"
    "github.com/normanking/clawd/ollama-adapter"
)

// Ollama agent
type OllamaAgent struct {
    name   string
    client *ollama_adapter.OllamaClient
    model  string
}

// New Ollama agent
func NewOllamaAgent(name, baseURL, model string) *OllamaAgent {
    return &OllamaAgent{
        name:   name,
        client: ollama_adapter.NewOllamaClient(baseURL),
        model:  model,
    }
}

// Execute coding task
func (o *OllamaAgent) ExecuteTask(ctx context.Context, task string) (string, error) {
    return o.client.SendCodingTask(o.model, task)
}

// Get agent name
func (o *OllamaAgent) Name() string {
    return o.name
}
```

### **Step 3: Register Ollama Agent in Harold**

```go
// File: cortex-brain/pkg/brain/lobes/agent/registry.go (modify existing)

// Add Ollama agent registration
func init() {
    // Register Ollama agent (Pink: 192.168.1.186)
    RegisterAgent(NewOllamaAgent(
        "ollama-pink",
        "http://192.168.1.186:11434",
        "deepseek-coder-v2-lite",
    ))

    // Existing agents...
    RegisterAgent(NewPinkAgent(...))
    RegisterAgent(NewRedAgent(...))
}
```

### **Step 4: Update Harold's Task Routing**

```go
// File: cortex-brain/pkg/brain/lobes/agent/orchestrator.go (modify existing)

// Add Ollama agent to task routing
func (o *Orchestrator) RouteTask(ctx context.Context, task string) (string, error) {
    // For small coding tasks (< 500 lines), use Ollama
    if o.isSmallCodingTask(task) {
        return o.agents["ollama-pink"].ExecuteTask(ctx, task)
    }

    // For large coding tasks, use Pink/Red
    if o.isCodingTask(task) {
        return o.agents["pink"].ExecuteTask(ctx, task)
    }

    // Default: Use other agents...
}

// Check if task is small coding task
func (o *Orchestrator) isSmallCodingTask(task string) bool {
    // Heuristic: < 500 lines, utility function, algorithm
    keywords := []string{
        "utility function",
        "helper function",
        "calculate",
        "algorithm",
        "fibonacci",
        "factorial",
        "data structure",
        "simple",
    }

    for _, keyword := range keywords {
        if contains(task, keyword) {
            return true
        }
    }

    return false
}
```

### **Step 5: Rebuild Harold**

```bash
# On Harold
cd ~/clawd/cortex-brain

# Rebuild
go build -o cortex-brain cmd/cortex-brain/main.go

# Restart Harold
sudo systemctl restart cortex-brain

# Or if running as service:
sudo systemctl restart cortex-brain
```

---

## 🎯 PHASE 6: TESTING & VALIDATION (1-2 hours)

### **Step 1: Test Small Coding Tasks**

```bash
# Test 1: Utility function
# Send A2A message to Harold for small coding task
echo '{"agent":"harold","target":"ollama-pink","message":"Write a utility function to capitalize strings in Go"}' | \
  curl -X POST http://localhost:18802/messages -H "Content-Type: application/json" -d @-

# Test 2: Algorithm
echo '{"agent":"harold","target":"ollama-pink","message":"Write a Python function to sort a list"}' | \
  curl -X POST http://localhost:18802/messages -H "Content-Type: application/json" -d @-

# Test 3: Data structure
echo '{"agent":"harold","target":"ollama-pink","message":"Implement a stack data structure in JavaScript"}' | \
  curl -X POST http://localhost:18802/messages -H "Content-Type: application/json" -d @-
```

### **Step 2: Measure Performance**

```bash
# Measure tokens/sec (use Ollama API timing)
time curl http://localhost:11434/api/generate \
  -H "Content-Type: application/json" \
  -d '{
    "model": "deepseek-coder-v2-lite",
    "prompt": "Write a Go function to calculate fibonacci",
    "stream": false
  }'

# Expected: 50-70 tok/s on RTX 3090 (quantized)
```

### **Step 3: Compare with Pink/Red**

```bash
# Test Pink (GLM-4.7)
# Test Red (GLM-4.7)
# Compare:
# - Speed (tokens/sec)
# - Code quality
# - API latency
# - Cost (Ollama: $0, GLM-4.7: API costs)
```

### **Step 4: Validate Quality**

```bash
# Test code correctness
# Test 1: Fibonacci function (verify output)
# Test 2: Sorting function (verify sorted output)
# Test 3: Stack data structure (verify push/pop operations)

# Test code style
# - Does code follow best practices?
# - Is code readable?
# - Are error handlers included?
```

---

## 📊 EXPECTED BENEFITS

| Benefit | Expected Impact |
|----------|---------------|
| **Offload Pink/Red** | 20-40% reduction in small task workload |
| **Faster for small tasks** | 50-70 tok/s vs API latency |
| **Cost reduction** | No API costs for small tasks |
| **Local execution** | Runs on RTX 3090 (existing hardware) |
| **Easy integration** | REST API, OpenAI-compatible |

---

## 📋 SUCCESS CRITERIA

```
✅ Ollama installed on Pink
✅ DeepSeek-Coder-V2-Lite model downloaded
✅ Ollama API working (port 11434)
✅ A2A adapter built and tested
✅ Ollama agent registered in Harold
✅ Task routing updated (small tasks → Ollama)
✅ Test small coding tasks (utility, algorithm, data structure)
✅ Measure performance (50-70 tok/s expected)
✅ Compare with Pink/Red (speed, quality, cost)
✅ Document findings in SWARM-INFRASTRUCTURE-REGISTRY.md
```

---

## 📋 TROUBLESHOOTING

### **Issue: Ollama installation fails**

```bash
# Check if Homebrew is installed
brew --version

# Install Homebrew (if not installed)
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"

# Install Ollama
brew install ollama
```

### **Issue: Model download fails**

```bash
# Check disk space
df -h

# Check network connectivity
ping ollama.com

# Retry model download
ollama pull deepseek-coder-v2-lite
```

### **Issue: Ollama API not responding**

```bash
# Check if Ollama server is running
ps aux | grep ollama

# Start Ollama server
ollama serve

# Check port 11434
netstat -tlnp | grep 11434
```

### **Issue: A2A adapter fails to connect**

```bash
# Test Ollama API manually
curl http://localhost:11434/api/generate -d '{"model":"deepseek-coder-v2-lite","prompt":"test"}'

# Check network connectivity (from Harold)
ping 192.168.1.186
curl http://192.168.1.186:11434/api/tags

# Check firewall settings
sudo ufw status
```

---

## 📋 DOCUMENTATION

### **Files to Create:**

| File | Description |
|------|-------------|
| `~/clawd/ollama-adapter/client.go` | Ollama Go client |
| `~/clawd/ollama-adapter/go.mod` | Go module file |
| `~/clawd/cortex-brain/pkg/brain/lobes/agent/ollama_agent.go` | Ollama agent implementation |
| `~/clawd/cortex-brain/pkg/brain/lobes/agent/registry.go` | Agent registry (modified) |
| `~/clawd/cortex-brain/pkg/brain/lobes/agent/orchestrator.go` | Task routing (modified) |

### **Documentation to Update:**

| File | Description |
|------|-------------|
| `SWARM-INFRASTRUCTURE-REGISTRY.md` | Add Ollama integration entry |
| `NORMAN-PRIORITY-TASK-LIST.md` | Update RTX 3090 Coding Model task status |
| `OLLAMA-INTEGRATION-GUIDE.md` | This file (integration guide) |

---

## ⏰ TIMELINE

| Phase | Time | Status |
|-------|-------|--------|
| **Phase 1: Install Ollama** | 1 hour | ⏳ Pending |
| **Phase 2: Pull Model** | 30 min | ⏳ Pending |
| **Phase 3: Test Model** | 30 min | ⏳ Pending |
| **Phase 4: Build A2A Adapter** | 2-3 hours | ⏳ Pending |
| **Phase 5: Integrate with Harold** | 2-3 hours | ⏳ Pending |
| **Phase 6: Testing & Validation** | 1-2 hours | ⏳ Pending |
| **TOTAL** | 6-9 hours | ⏳ Pending |

---

## 🎯 NEXT ACTIONS

1. ⏳ **Install Ollama on Pink** (SSH to Pink, run brew install ollama)
2. ⏳ **Pull DeepSeek-Coder-V2-Lite** (ollama pull deepseek-coder-v2-lite)
3. ⏳ **Test Ollama API** (curl http://localhost:11434/api/generate)
4. ⏳ **Build A2A adapter** (create ollama-adapter/client.go)
5. ⏳ **Integrate with Harold** (add ollama_agent.go, update registry)
6. ⏳ **Test integration** (send A2A messages, verify routing)
7. ⏳ **Document findings** (update SWARM-INFRASTRUCTURE-REGISTRY.md)

---

**Last Updated:** February 1, 2026 (3:30 PM EST)

**Next Step:** Install Ollama on Pink

---

**Note: This is a blocking task due to SSH access issues. Please clear SSH cache first.**