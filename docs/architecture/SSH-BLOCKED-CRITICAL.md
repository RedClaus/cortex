---
project: Cortex
component: Docs
phase: Build
date_created: 2026-02-01T16:03:08
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:16:29.549026
---

# 🛑 SSH BLOCKED - CANNOT IMPLEMENT OLLAMA FROM THIS ENVIRONMENT

**Date:** February 1, 2026
**Time:** 4:10 PM EST
**Status:** 🔴 BLOCKED - SSH Access Issues

---

## 🚨 THE PROBLEM:

```
❌ OpenClaw sandbox cannot SSH to Pink (192.168.1.186)
❌ OpenClaw sandbox cannot SSH to Harold (192.168.1.229)
❌ Error: "Too many authentication failures"
❌ Cannot execute implementation scripts
```

---

## 🎯 SOLUTION: YOU MUST EXECUTE MANUALLY

**I cannot execute the implementation scripts from this environment. You need to run them manually.**

---

## 📋 IMPLEMENTATION STEPS (FOR YOU TO EXECUTE):

### **PART 1: INSTALL OLLAMA ON PINK**

```bash
# 1. SSH to Pink
ssh normanking@192.168.1.186
# Password: Zer0k1ng

# 2. Copy script to Pink
# (On your Mac terminal)
scp ~/ServerProjectsMac/OLLAMA-DEEPSEEK-IMPLEMENTATION.sh normanking@192.168.1.186:~/clawd/

# 3. Run script on Pink
cd ~/clawd
chmod +x OLLAMA-DEEPSEEK-IMPLEMENTATION.sh
./OLLAMA-DEEPSEEK-IMPLEMENTATION.sh
```

**What the script does:**
- ✅ Checks if Ollama is installed (installs via brew if needed)
- ✅ Starts Ollama server (background)
- ✅ Downloads DeepSeek-Coder-V2-Lite (9.2GB)
- ✅ Tests with fibonacci function

---

### **PART 2: BUILD A2A ADAPTER ON PINK**

```bash
# 1. On your Mac - open the client code file
cat ~/ServerProjectsMac/OLLAMA-ADAPTER-CLIENT.go.md
# Copy the Go code displayed

# 2. Copy script to Pink
# (On your Mac terminal)
scp ~/ServerProjectsMac/OLLAMA-ADAPTER-BUILD.sh normanking@192.168.1.186:~/clawd/

# 3. Run script on Pink
cd ~/clawd
chmod +x OLLAMA-ADAPTER-BUILD.sh
./OLLAMA-ADAPTER-BUILD.sh

# 4. When prompted "Do you want to overwrite client.go?", type: y
# 5. Paste the Go code you copied
# 6. Save and exit (Ctrl+X, then y, then Enter)
```

**What the script does:**
- ✅ Creates `ollama-adapter/` directory
- ✅ Initializes Go module
- ✅ Creates `client.go` (you paste code)
- ✅ Builds `ollama-client` executable
- ✅ Tests with fibonacci

---

### **PART 3: INTEGRATE WITH HAROLD**

```bash
# 1. SSH to Harold
ssh haroldbot@192.168.1.229
# Password: Zer0k1ng!

# 2. On your Mac - open the agent integration code file
cat ~/ServerProjectsMac/OLLAMA-AGENT-INTEGRATION.go.md
# Copy the Go code displayed

# 3. Copy script to Harold
# (On your Mac terminal)
scp ~/ServerProjectsMac/OLLAMA-HAROLD-INTEGRATION.sh haroldbot@192.168.1.229:~/clawd/

# 4. Run script on Harold
cd ~/clawd
chmod +x OLLAMA-HAROLD-INTEGRATION.sh
./OLLAMA-HAROLD-INTEGRATION.sh

# 5. When prompted "Do you want to overwrite ollama_agent.go?", type: y
# 6. Paste the Go code you copied (for ollama_agent.go)
# 7. Save and exit (Ctrl+X, then y, then Enter)

# 8. When prompted to update registry.go, type: y
# 9. Edit registry.go and add:
#    RegisterAgent(NewOllamaAgent(
#        "ollama-pink",
#        "http://192.168.1.186:11434",
#        "deepseek-coder-v2-lite",
#    ))

# 10. When prompted to update orchestrator.go, type: y
# 11. Edit orchestrator.go and add the routing logic from OLLAMA-AGENT-INTEGRATION.go.md

# 12. Save and exit (Ctrl+X, then y, then Enter)
```

**What the script does:**
- ✅ Creates `ollama_agent.go` (you paste code)
- ✅ Registers Ollama agent in `registry.go` (you edit)
- ✅ Adds routing logic in `orchestrator.go` (you edit)
- ✅ Rebuilds Harold
- ✅ Restarts Harold

---

## 🎯 TESTING (AFTER IMPLEMENTATION):

### **Test 1: Direct Ollama (on Pink)**

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

### **Test 2: A2A Adapter (on Pink)**

```bash
# Test the ollama-client
cd ~/clawd/ollama-adapter
./ollama-client

# Expected: Generated fibonacci function
```

### **Test 3: A2A Messaging (from Harold)**

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

## 📋 FILES IN YOUR OBSIDIAN VAULT:

| File | Purpose | Status |
|------|---------|--------|
| `OLLAMA-DEEPSEEK-IMPLEMENTATION.sh` | Install Ollama on Pink | ✅ Ready |
| `OLLAMA-ADAPTER-BUILD.sh` | Build A2A adapter | ✅ Ready |
| `OLLAMA-HAROLD-INTEGRATION.sh` | Integrate with Harold | ✅ Ready |
| `OLLAMA-ADAPTER-CLIENT.go.md` | Go client code | ✅ Ready |
| `OLLAMA-AGENT-INTEGRATION.go.md` | Harold integration code | ✅ Ready |
| `OLLAMA-EXECUTE-NOW.md` | Quick reference | ✅ Ready |
| `OLLAMA-QUICK-START.md` | Complete guide | ✅ Ready |

---

## 🎯 SUMMARY:

```
🔴 I CANNOT IMPLEMENT FROM THIS ENVIRONMENT
🔴 SSH IS BLOCKED IN OPENCLAW SANDBOX
🔴 YOU MUST EXECUTE MANUALLY

✅ All scripts are ready in your Obsidian vault
✅ All code is ready (copy-paste from .md files)
✅ All documentation is ready
✅ Execution steps are documented

📋 YOU NEED TO:
   1. SSH to Pink → Run OLLAMA-DEEPSEEK-IMPLEMENTATION.sh
   2. SSH to Pink → Run OLLAMA-ADAPTER-BUILD.sh (copy code)
   3. SSH to Harold → Run OLLAMA-HAROLD-INTEGRATION.sh (copy code)
   4. Test → Verify A2A messaging works
```

---

## 🚨 NEXT ACTIONS (FOR YOU):

```
1. ⏳ Open terminal on your Mac
2. ⏳ SSH to Pink (ssh normanking@192.168.1.186)
3. ⏳ Run OLLAMA-DEEPSEEK-IMPLEMENTATION.sh
4. ⏳ Copy code from OLLAMA-ADAPTER-CLIENT.go.md
5. ⏳ Run OLLAMA-ADAPTER-BUILD.sh
6. ⏳ SSH to Harold (ssh haroldbot@192.168.1.229)
7. ⏳ Copy code from OLLAMA-AGENT-INTEGRATION.go.md
8. ⏳ Run OLLAMA-HAROLD-INTEGRATION.sh
9. ⏳ Test A2A messaging
10. ⏳ Verify task routing works
```

---

**I cannot execute the implementation from this environment. All scripts and code are ready in your Obsidian vault - you need to run them manually.**

**Open `OLLAMA-EXECUTE-NOW.md` in your Obsidian vault for step-by-step instructions!** 🚀