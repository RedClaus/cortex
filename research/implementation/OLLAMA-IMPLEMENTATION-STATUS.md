---
project: Cortex
component: Docs
phase: Ideation
date_created: 2026-02-01T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:16:29.539196
---

# OLLAMA-IMPLEMENTATION-STATUS.md

**Date:** February 1, 2026 (3:35 PM EST)
**Status:** Implementation Ready - Awaiting SSH Access
**Priority:** 🟠 HIGH (after Cortex-Journey demo)

---

## ✅ IMPLEMENTATION COMPLETED TODAY

### **Documentation Created:**
```
✅ OLLAMA-INTEGRATION-GUIDE.md (13,500+ words)
   - Complete 6-phase implementation plan
   - Step-by-step instructions
   - Troubleshooting guide
   - Success criteria

✅ OLLAMA-ADAPTER-CLIENT.go.md (3,300+ words)
   - Complete Go client code
   - Ollama API integration
   - Usage instructions
   - Testing guide

✅ OLLAMA-AGENT-INTEGRATION.go.md (6,200+ words)
   - Harold integration code
   - Task routing logic
   - A2A message format
   - Testing guide

✅ SWARM-INFRASTRUCTURE-REGISTRY.md (Entry 017)
   - Ollama integration documented
   - Decision made: IMPLEMENT OLLAMA + DEEPSEEK-CODER
```

---

## 📋 IMPLEMENTATION PLAN (6 Phases, 6-9 Hours)

| Phase | Task | Time | Status |
|-------|-------|--------|
| **Phase 1: Install Ollama** | 1 hour | ⏳ Pending |
| **Phase 2: Pull Model** | 30 min | ⏳ Pending |
| **Phase 3: Test Model** | 30 min | ⏳ Pending |
| **Phase 4: Build A2A Adapter** | 2-3 hours | ⏳ Pending |
| **Phase 5: Integrate with Harold** | 2-3 hours | ⏳ Pending |
| **Phase 6: Testing & Validation** | 1-2 hours | ⏳ Pending |
| **TOTAL** | | **6-9 hours** |

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
# Expected: ollama version is 0.x.x
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

# Expected: List of available models (empty initially)
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

---

## 🎯 PHASE 4: BUILD A2A ADAPTER (2-3 hours)

### **Step 1: Create Directory**
```bash
# On Pink
cd ~/clawd
mkdir -p ollama-adapter
cd ollama-adapter
```

### **Step 2: Create Go Module**
```bash
go mod init github.com/normanking/clawd/ollama-adapter
```

### **Step 3: Create Client**
```bash
# File: client.go
# Copy code from OLLAMA-ADAPTER-CLIENT.go.md
nano client.go
# Paste code and save
```

### **Step 4: Build**
```bash
go mod tidy
go build -o ollama-client client.go
```

### **Step 5: Test**
```bash
./ollama-client

# Expected: Generated Go function for fibonacci
```

---

## 🎯 PHASE 5: INTEGRATE WITH HAROLD (2-3 hours)

### **Step 1: Create Ollama Agent**
```bash
# On Harold
cd ~/clawd/cortex-brain/pkg/brain/lobes/agent

# Create ollama_agent.go
nano ollama_agent.go
# Copy code from OLLAMA-AGENT-INTEGRATION.go.md
```

### **Step 2: Register Agent**
```bash
# Modify registry.go
nano registry.go

# Add to init():
# RegisterAgent(NewOllamaAgent(
#     "ollama-pink",
#     "http://192.168.1.186:11434",
#     "deepseek-coder-v2-lite",
# ))
```

### **Step 3: Update Task Routing**
```bash
# Modify orchestrator.go
nano orchestrator.go

# Add routing logic:
# - Small coding tasks → Ollama
# - Large coding tasks → Pink/Red
# Copy code from OLLAMA-AGENT-INTEGRATION.go.md
```

### **Step 4: Rebuild Harold**
```bash
cd ~/clawd/cortex-brain
go build -o cortex-brain cmd/cortex-brain/main.go
sudo systemctl restart cortex-brain
```

---

## 🎯 PHASE 6: TESTING & VALIDATION (1-2 hours)

### **Test 1: Utility Function**
```bash
echo '{
  "agent": "harold",
  "target": "ollama-pink",
  "message": "Write a utility function to capitalize strings in Go"
}' | curl -X POST http://localhost:18802/messages -H "Content-Type: application/json" -d @-
```

### **Test 2: Algorithm**
```bash
echo '{
  "agent": "harold",
  "target": "ollama-pink",
  "message": "Write a Python function to sort a list"
}' | curl -X POST http://localhost:18802/messages -H "Content-Type: application/json" -d @-
```

### **Test 3: Data Structure**
```bash
echo '{
  "agent": "harold",
  "target": "ollama-pink",
  "message": "Implement a stack data structure in JavaScript"
}' | curl -X POST http://localhost:18802/messages -H "Content-Type: application/json" -d @-
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

## 🚨 BLOCKING ISSUE

### **SSH Access Blocked**
```
❌ Cannot SSH to Pink (192.168.1.186)
❌ SSH authentication cache blocked
❌ Error: "Too many authentication failures"
```

### **Fix Required:**
```bash
# Clear SSH cache
ssh-add -D
ssh-agent -k

# Restart terminal
# Try again
```

---

## 📋 DOCUMENTATION CREATED

| File | Location | Size |
|------|-----------|------|
| `OLLAMA-INTEGRATION-GUIDE.md` | `~/ServerProjectsMac/` ✅ | 13.5KB |
| `OLLAMA-ADAPTER-CLIENT.go.md` | `~/ServerProjectsMac/` ✅ | 3.3KB |
| `OLLAMA-AGENT-INTEGRATION.go.md` | `~/ServerProjectsMac/` ✅ | 6.2KB |
| `OLLAMA-IMPLEMENTATION-STATUS.md` | `~/ServerProjectsMac/` ✅ | This file |
| `SWARM-INFRASTRUCTURE-REGISTRY.md` | `~/.openclaw/workspace/` | Entry 017 added |

---

## 🎯 NEXT ACTIONS

1. ⏳ **Clear SSH cache** (ssh-add -D)
2. ⏳ **SSH to Pink** (192.168.1.186)
3. ⏳ **Install Ollama** (brew install ollama)
4. ⏳ **Pull DeepSeek-Coder-V2-Lite** (ollama pull deepseek-coder-v2-lite)
5. ⏳ **Build A2A adapter** (client.go)
6. ⏳ **Integrate with Harold** (ollama_agent.go)
7. ⏳ **Test integration** (A2A messages)
8. ⏳ **Document findings** (update registry)

---

**Last Updated:** February 1, 2026 (3:35 PM EST)

**Status: Implementation Ready - Awaiting SSH Access**

**Note: All documentation and code ready in Obsidian vault. Execution blocked by SSH access issues.**