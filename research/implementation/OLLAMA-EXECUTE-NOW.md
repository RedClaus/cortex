---
project: Cortex
component: Docs
phase: Build
date_created: 2026-02-01T16:01:37
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:16:29.625902
---

# 🚨 URGENT: OLLAMA IMPLEMENTATION READY - EXECUTE NOW

**Date:** February 1, 2026
**Time:** 4:00 PM EST
**Status:** 🟢 Scripts Created - Ready to Execute
**Priority:** 🟠 HIGH (after Cortex-Journey)

---

## ✅ WHAT'S READY:

### **3 Implementation Scripts Created:**

| Script | Purpose | Location |
|--------|---------|-----------|
| `OLLAMA-DEEPSEEK-IMPLEMENTATION.sh` | Install Ollama + DeepSeek on Pink | ✅ Obsidian Vault |
| `OLLAMA-ADAPTER-BUILD.sh` | Build A2A adapter on Pink | ✅ Obsidian Vault |
| `OLLAMA-HAROLD-INTEGRATION.sh` | Integrate Ollama agent in Harold | ✅ Obsidian Vault |

### **Documentation Created:**

| File | Purpose |
|------|---------|
| `OLLAMA-ADAPTER-CLIENT.go.md` | Go client code (copy-paste) |
| `OLLAMA-AGENT-INTEGRATION.go.md` | Harold integration code (copy-paste) |
| `OLLAMA-QUICK-START.md` | Complete execution guide |

---

## 🎯 EXECUTION STEPS:

### **PART 1: PINK (Install Ollama + DeepSeek)**

```bash
# Step 1: SSH to Pink
ssh normanking@192.168.1.186
# Password: Zer0k1ng

# Step 2: Copy script to Pink
# (On your Mac terminal)
scp ~/ServerProjectsMac/OLLAMA-DEEPSEEK-IMPLEMENTATION.sh normanking@192.168.1.186:~/clawd/

# Step 3: Run script on Pink
cd ~/clawd
chmod +x OLLAMA-DEEPSEEK-IMPLEMENTATION.sh
./OLLAMA-DEEPSEEK-IMPLEMENTATION.sh
```

### **PART 2: PINK (Build A2A Adapter)**

```bash
# Step 1: Copy client code to Pink
# (On your Mac terminal)
cat ~/ServerProjectsMac/OLLAMA-ADAPTER-CLIENT.go.md
# Copy the Go code displayed

# Step 2: Copy script to Pink
scp ~/ServerProjectsMac/OLLAMA-ADAPTER-BUILD.sh normanking@192.168.1.186:~/clawd/

# Step 3: Run script on Pink
cd ~/clawd
chmod +x OLLAMA-ADAPTER-BUILD.sh
./OLLAMA-ADAPTER-BUILD.sh
# Script will ask: "Do you want to overwrite client.go?"
# Answer: y
# Then paste the Go code you copied
```

### **PART 3: HAROLD (Integrate Ollama Agent)**

```bash
# Step 1: SSH to Harold
ssh haroldbot@192.168.1.229
# Password: Zer0k1ng!

# Step 2: Copy agent code to Harold
# (On your Mac terminal)
cat ~/ServerProjectsMac/OLLAMA-AGENT-INTEGRATION.go.md
# Copy the Go code displayed

# Step 3: Copy script to Harold
# (On your Mac terminal)
scp ~/ServerProjectsMac/OLLAMA-HAROLD-INTEGRATION.sh haroldbot@192.168.1.229:~/clawd/

# Step 4: Run script on Harold
cd ~/clawd
chmod +x OLLAMA-HAROLD-INTEGRATION.sh
./OLLAMA-HAROLD-INTEGRATION.sh
# Script will ask to overwrite files - answer: y
# Then paste the Go code when prompted
```

---

## 📋 WHAT EACH SCRIPT DOES:

### **OLLAMA-DEEPSEEK-IMPLEMENTATION.sh (Pink):**

```
✅ Check if running on Pink
✅ Check if Ollama is installed (install via brew if needed)
✅ Start Ollama server (background)
✅ Verify Ollama API (port 11434)
✅ Check DeepSeek-Coder-V2-Lite model (download if needed)
✅ Test DeepSeek-Coder with fibonacci function
```

### **OLLAMA-ADAPTER-BUILD.sh (Pink):**

```
✅ Check if running on Pink
✅ Create ollama-adapter directory
✅ Initialize Go module
✅ Create client.go (you paste code)
✅ Build ollama-client executable
✅ Test: Run ollama-client with fibonacci test
```

### **OLLAMA-HAROLD-INTEGRATION.sh (Harold):**

```
✅ Check if running on Harold
✅ Navigate to agent directory
✅ Create ollama_agent.go (you paste code)
✅ Register Ollama agent in registry.go
✅ Add routing logic in orchestrator.go
✅ Rebuild Harold
✅ Restart Harold
```

---

## 📋 QUICK REFERENCE:

### **Pink IP:** 192.168.1.186
### **Harold IP:** 192.168.1.229
### **Ollama Port:** 11434
### **A2A Bridge Port:** 18802
### **DeepSeek Model:** deepseek-coder-v2-lite (16B, 9.2GB)

---

## 🎯 TESTING:

### **After Implementation:**

```bash
# Test 1: Direct Ollama (on Pink)
ollama run deepseek-coder-v2-lite "Write a Go function to calculate fibonacci"

# Test 2: A2A Messaging (from Harold)
echo '{
  "agent": "harold",
  "target": "ollama-pink",
  "message": "Write a Python function to sort a list"
}' | curl -X POST http://localhost:18802/messages \
  -H "Content-Type: application/json" \
  -d @-

# Check logs
tail -f ~/clawd/logs/cortex-brain.log
```

---

## 📊 EXPECTED BENEFITS:

| Benefit | Expected Impact |
|----------|---------------|
| **Offload Pink/Red** | 20-40% reduction in small task workload |
| **Faster for small tasks** | 50-70 tok/s vs API latency |
| **Cost reduction** | No API costs for small tasks |
| **Local execution** | Runs on RTX 3090 (existing hardware) |

---

## 📋 FILES CREATED (All in Obsidian Vault):

```
OLLAMA-DEEPSEEK-IMPLEMENTATION.sh
OLLAMA-ADAPTER-BUILD.sh
OLLAMA-HAROLD-INTEGRATION.sh
OLLAMA-ADAPTER-CLIENT.go.md
OLLAMA-AGENT-INTEGRATION.go.md
OLLAMA-QUICK-START.md
OLLAMA-IMPLEMENTATION-GUIDE.md
OLLAMA-IMPLEMENTATION-STATUS.md
OLLAMA-IMPLEMENTATION-README.md
```

---

## 🎯 NEXT ACTIONS:

```
1. ⏳ SSH to Pink (ssh normanking@192.168.1.186)
2. ⏳ Run OLLAMA-DEEPSEEK-IMPLEMENTATION.sh
3. ⏳ Copy client code from OLLAMA-ADAPTER-CLIENT.go.md
4. ⏳ Run OLLAMA-ADAPTER-BUILD.sh
5. ⏳ SSH to Harold (ssh haroldbot@192.168.1.229)
6. ⏳ Copy agent code from OLLAMA-AGENT-INTEGRATION.go.md
7. ⏳ Run OLLAMA-HAROLD-INTEGRATION.sh
8. ⏳ Test A2A messaging
9. ⏳ Document results
```

---

**All scripts and documentation are ready in your Obsidian vault (`~/ServerProjectsMac/`)**

**Execute scripts in order: Pink (install + build) → Harold (integration) → Test** 🚀