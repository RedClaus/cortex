---
project: Cortex
component: UI
phase: Build
date_created: 2026-02-01T16:01:18
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:16:29.509926
---

# OLLAMA IMPLEMENTATION - QUICK START GUIDE

**Date:** February 1, 2026
**Status:** Scripts Created - Ready to Execute
**Target:** Pink (Ollama + DeepSeek) → Harold (A2A Agent)

---

## 📋 SCRIPTS CREATED (All in Obsidian Vault):

| Script | Purpose | Location |
|--------|---------|-----------|
| `OLLAMA-DEEPSEEK-IMPLEMENTATION.sh` | Install Ollama + DeepSeek on Pink | `~/ServerProjectsMac/` |
| `OLLAMA-ADAPTER-BUILD.sh` | Build A2A adapter on Pink | `~/ServerProjectsMac/` |
| `OLLAMA-HAROLD-INTEGRATION.sh` | Integrate Ollama agent in Harold | `~/ServerProjectsMac/` |

---

## 🎯 PART 1: INSTALL OLLAMA + DEEPSEEK ON PINK

### **Step 1: SSH to Pink**

```bash
ssh normanking@192.168.1.186
# Password: Zer0k1ng
```

### **Step 2: Run Implementation Script**

```bash
# Copy script to Pink
scp ~/ServerProjectsMac/OLLAMA-DEEPSEEK-IMPLEMENTATION.sh normanking@192.168.1.186:~/clawd/

# On Pink
cd ~/clawd
chmod +x OLLAMA-DEEPSEEK-IMPLEMENTATION.sh
./OLLAMA-DEEPSEEK-IMPLEMENTATION.sh
```

### **What the Script Does:**

```
✅ Step 1: Check if running on Pink
✅ Step 2: Check if Ollama is installed (install via brew if needed)
✅ Step 3: Start Ollama server (background)
✅ Step 4: Verify Ollama API (port 11434)
✅ Step 5: Check DeepSeek-Coder-V2-Lite model (download if needed)
✅ Step 6: Test DeepSeek-Coder with fibonacci function
```

---

## 🎯 PART 2: BUILD A2A ADAPTER ON PINK

### **Step 1: Copy Client Code to Pink**

```bash
# On your Mac
cat ~/ServerProjectsMac/OLLAMA-ADAPTER-CLIENT.go.md

# Copy the Go code
```

### **Step 2: Copy Script to Pink**

```bash
scp ~/ServerProjectsMac/OLLAMA-ADAPTER-BUILD.sh normanking@192.168.1.186:~/clawd/
```

### **Step 3: Run Build Script on Pink**

```bash
# On Pink
cd ~/clawd
chmod +x OLLAMA-ADAPTER-BUILD.sh
./OLLAMA-ADAPTER-BUILD.sh
```

### **What the Script Does:**

```
✅ Step 1: Check if running on Pink
✅ Step 2: Create ollama-adapter directory
✅ Step 3: Initialize Go module
✅ Step 4: Create client.go (you copy code)
✅ Step 5: Build ollama-client executable
✅ Test: Run ollama-client with fibonacci test
```

---

## 🎯 PART 3: INTEGRATE WITH HAROLD

### **Step 1: SSH to Harold**

```bash
ssh haroldbot@192.168.1.229
# Password: Zer0k1ng!
```

### **Step 2: Copy Script to Harold**

```bash
# On your Mac
scp ~/ServerProjectsMac/OLLAMA-HAROLD-INTEGRATION.sh haroldbot@192.168.1.229:~/clawd/
```

### **Step 3: Run Integration Script on Harold**

```bash
# On Harold
cd ~/clawd
chmod +x OLLAMA-HAROLD-INTEGRATION.sh
./OLLAMA-HAROLD-INTEGRATION.sh
```

### **What the Script Does:**

```
✅ Step 1: Check if running on Harold
✅ Step 2: Navigate to agent directory
✅ Step 3: Create ollama_agent.go (you copy code)
✅ Step 4: Register Ollama agent in registry.go
✅ Step 5: Add routing logic in orchestrator.go
✅ Step 6: Rebuild Harold
✅ Step 7: Restart Harold
```

---

## 📋 MANUAL STEPS (if scripts fail):

### **On Pink (192.168.1.186):**

```bash
# 1. Install Ollama
brew install ollama

# 2. Start Ollama
ollama serve &

# 3. Pull model
ollama pull deepseek-coder-v2-lite

# 4. Create adapter
cd ~/clawd
mkdir -p ollama-adapter
cd ollama-adapter
go mod init github.com/normanking/clawd/ollama-adapter

# 5. Create client.go (copy from OLLAMA-ADAPTER-CLIENT.go.md)

# 6. Build
go mod tidy
go build -o ollama-client client.go
```

### **On Harold (192.168.1.229):**

```bash
# 1. Navigate to agent directory
cd ~/clawd/cortex-brain/pkg/brain/lobes/agent

# 2. Create ollama_agent.go (copy from OLLAMA-AGENT-INTEGRATION.go.md)

# 3. Modify registry.go
nano registry.go
# Add: RegisterAgent(NewOllamaAgent("ollama-pink", "http://192.168.1.186:11434", "deepseek-coder-v2-lite"))

# 4. Modify orchestrator.go
nano orchestrator.go
# Add routing logic (see OLLAMA-AGENT-INTEGRATION.go.md)

# 5. Rebuild
cd ~/clawd/cortex-brain
go build -o cortex-brain cmd/cortex-brain/main.go

# 6. Restart
sudo systemctl restart cortex-brain
```

---

## 🎯 TESTING

### **Test 1: Direct Ollama Test (on Pink)**

```bash
# Test fibonacci
ollama run deepseek-coder-v2-lite "Write a Go function to calculate fibonacci numbers"

# Test via API
curl http://localhost:11434/api/generate \
  -H "Content-Type: application/json" \
  -d '{
    "model": "deepseek-coder-v2-lite",
    "prompt": "Write a Python function to sort a list",
    "stream": false
  }'
```

### **Test 2: A2A Test (from Harold)**

```bash
# Send A2A message to Harold
echo '{
  "agent": "harold",
  "target": "ollama-pink",
  "message": "Write a utility function to capitalize strings in Go"
}' | curl -X POST http://localhost:18802/messages \
  -H "Content-Type: application/json" \
  -d @-

# Check Harold logs
tail -f ~/clawd/logs/cortex-brain.log
```

---

## 📊 EXPECTED RESULTS

| Metric | Expected |
|--------|----------|
| **Ollama Server** | Running on port 11434 |
| **Model** | DeepSeek-Coder-V2-Lite (16B, 9.2GB) |
| **Performance** | 50-70 tok/s on RTX 3090 |
| **A2A Adapter** | ollama-client executable |
| **Harold Integration** | ollama-pink agent registered |
| **Task Routing** | Small tasks → Ollama, large tasks → Pink/Red |

---

## 📋 FILES IN OBSIDIAN VAULT:

| File | Purpose |
|------|---------|
| `OLLAMA-DEEPSEEK-IMPLEMENTATION.sh` | Installation script for Pink |
| `OLLAMA-ADAPTER-BUILD.sh` | Build script for Pink |
| `OLLAMA-HAROLD-INTEGRATION.sh` | Integration script for Harold |
| `OLLAMA-ADAPTER-CLIENT.go.md` | Go client code (copy-paste) |
| `OLLAMA-AGENT-INTEGRATION.go.md` | Harold integration code (copy-paste) |
| `OLLAMA-INTEGRATION-GUIDE.md` | Complete documentation |
| `OLLAMA-IMPLEMENTATION-STATUS.md` | Status tracker |
| `OLLAMA-IMPLEMENTATION-README.md` | Quick start guide |
| `OLLAMA-QUICK-START.md` | This file |

---

## 🎯 EXECUTION ORDER:

```
1. ✅ Scripts created in Obsidian vault
2. ⏳ SSH to Pink (192.168.1.186)
3. ⏳ Run OLLAMA-DEEPSEEK-IMPLEMENTATION.sh
4. ⏳ Run OLLAMA-ADAPTER-BUILD.sh (copy client code first)
5. ⏳ SSH to Harold (192.168.1.229)
6. ⏳ Run OLLAMA-HAROLD-INTEGRATION.sh (copy agent code first)
7. ⏳ Test A2A messaging
8. ⏳ Document results
```

---

## 🚨 NOTES:

- All scripts are **interactive** - they will ask for confirmation
- You need to **copy code** from `.md` files to `.go` files
- Scripts will **guide you** through each step
- **Backup** existing files before overwriting

---

**Last Updated:** February 1, 2026 (4:00 PM EST)

**Status:** Scripts Ready - Execute on Pink + Harold

**All files in Obsidian vault: `~/ServerProjectsMac/`**