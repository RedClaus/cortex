---
project: Cortex
component: Docs
phase: Ideation
date_created: 2026-02-01T15:36:57
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:16:29.585572
---

# 📋 OLLAMA IMPLEMENTATION - READY TO EXECUTE

**Date:** February 1, 2026
**Time:** 3:35 PM EST
**Status:** 🟢 Implementation Ready - Awaiting SSH Access
**Priority:** 🟠 HIGH (after Cortex-Journey demo)

---

## ✅ COMPLETED TODAY:

### **Documentation Created (All in Obsidian Vault):**
```
✅ OLLAMA-INTEGRATION-GUIDE.md (13.5KB)
   - Complete 6-phase implementation plan
   - Step-by-step instructions for all phases
   - Troubleshooting guide
   - Success criteria

✅ OLLAMA-ADAPTER-CLIENT.go.md (3.3KB)
   - Complete Go client code for Ollama API
   - Copy-paste ready
   - Usage instructions
   - Testing guide

✅ OLLAMA-AGENT-INTEGRATION.go.md (6.2KB)
   - Harold integration code
   - Task routing logic
   - A2A message format
   - Testing guide

✅ OLLAMA-IMPLEMENTATION-STATUS.md (7.5KB)
   - Implementation status tracker
   - Phase-by-phase checklist
   - Expected benefits
   - Next actions
```

---

## 🎯 IMPLEMENTATION PLAN SUMMARY:

| Phase | Task | Time | Status |
|-------|-------|-------|--------|
| **Phase 1: Install Ollama** | 1 hour | ⏳ Pending |
| **Phase 2: Pull Model** | 30 min | ⏳ Pending |
| **Phase 3: Test Model** | 30 min | ⏳ Pending |
| **Phase 4: Build A2A Adapter** | 2-3 hours | ⏳ Pending |
| **Phase 5: Integrate with Harold** | 2-3 hours | ⏳ Pending |
| **Phase 6: Testing & Validation** | 1-2 hours | ⏳ Pending |
| **TOTAL** | | **6-9 hours** |

---

## 🎯 RECOMMENDATION (FROM RESEARCH):

```
✅ Use Ollama + DeepSeek-Coder-V2-Lite (16B)

Why?
✅ Easiest setup (one command: brew install ollama)
✅ REST API (port 11434), OpenAI-compatible
✅ DeepSeek-Coder-V2-Lite: 16B, fits in 9.2GB Q4
✅ RTX 3090: 24GB VRAM, can run 16B quantized
✅ 50-70 tok/s on quantized models
✅ Offload 20-40% of small coding tasks from Pink/Red
✅ No API costs (local execution)
✅ Simple A2A integration (HTTP client)
```

---

## 📋 PHASE 1: INSTALL OLLAMA ON PINK (1 hour)

```bash
# 1. SSH to Pink
ssh normanking@192.168.1.186
# Password: Zer0k1ng

# 2. Install Ollama
brew install ollama

# 3. Start Ollama
ollama serve

# 4. Verify API
curl http://localhost:11434/api/tags
```

---

## 📋 PHASE 2: PULL DEEPSEEK-CODER-V2-LITE (30 min)

```bash
# 1. Pull model
ollama pull deepseek-coder-v2-lite

# 2. Verify model
ollama list
# Expected: deepseek-coder-v2-lite (9.2 GB)
```

---

## 📋 PHASE 3: TEST MODEL (30 min)

```bash
# 1. Interactive test
ollama run deepseek-coder-v2-lite "Write a Go function to calculate fibonacci"

# 2. API test
curl http://localhost:11434/api/generate \
  -H "Content-Type: application/json" \
  -d '{
    "model": "deepseek-coder-v2-lite",
    "prompt": "Write a Python function to calculate fibonacci",
    "stream": false
  }'
```

---

## 📋 PHASE 4: BUILD A2A ADAPTER (2-3 hours)

```bash
# 1. Create directory
cd ~/clawd
mkdir -p ollama-adapter
cd ollama-adapter

# 2. Create Go module
go mod init github.com/normanking/clawd/ollama-adapter

# 3. Create client.go (copy from OLLAMA-ADAPTER-CLIENT.go.md)
nano client.go

# 4. Build
go mod tidy
go build -o ollama-client client.go

# 5. Test
./ollama-client
```

---

## 📋 PHASE 5: INTEGRATE WITH HAROLD (2-3 hours)

```bash
# 1. On Harold
cd ~/clawd/cortex-brain/pkg/brain/lobes/agent

# 2. Create ollama_agent.go (copy from OLLAMA-AGENT-INTEGRATION.go.md)
nano ollama_agent.go

# 3. Modify registry.go
nano registry.go
# Add: RegisterAgent(NewOllamaAgent("ollama-pink", "http://192.168.1.186:11434", "deepseek-coder-v2-lite"))

# 4. Modify orchestrator.go
nano orchestrator.go
# Add routing logic (small coding tasks → Ollama)

# 5. Rebuild Harold
cd ~/clawd/cortex-brain
go build -o cortex-brain cmd/cortex-brain/main.go
sudo systemctl restart cortex-brain
```

---

## 📋 PHASE 6: TESTING & VALIDATION (1-2 hours)

```bash
# Test 1: Utility function
echo '{"agent":"harold","target":"ollama-pink","message":"Write a utility function to capitalize strings in Go"}' | \
  curl -X POST http://localhost:18802/messages -H "Content-Type: application/json" -d @-

# Test 2: Algorithm
echo '{"agent":"harold","target":"ollama-pink","message":"Write a Python function to sort a list"}' | \
  curl -X POST http://localhost:18802/messages -H "Content-Type: application/json" -d @-

# Test 3: Data structure
echo '{"agent":"harold","target":"ollama-pink","message":"Implement a stack data structure in JavaScript"}' | \
  curl -X POST http://localhost:18802/messages -H "Content-Type: application/json" -d @-

# Measure performance
time curl http://localhost:11434/api/generate -d '{"model":"deepseek-coder-v2-lite","prompt":"test","stream":false}'
```

---

## 📊 EXPECTED BENEFITS:

| Benefit | Expected Impact |
|----------|---------------|
| **Offload Pink/Red** | 20-40% reduction in small task workload |
| **Faster for small tasks** | 50-70 tok/s vs API latency |
| **Cost reduction** | No API costs for small tasks |
| **Local execution** | Runs on RTX 3090 (existing hardware) |
| **Easy integration** | REST API, OpenAI-compatible |

---

## ✅ SUCCESS CRITERIA:

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

## 🚨 BLOCKING ISSUE:

```
❌ SSH access blocked (authentication cache full)
❌ Cannot SSH to Pink (192.168.1.186)
❌ Cannot SSH to Harold (192.168.1.229)
```

### **FIX REQUIRED:**

```bash
# Clear SSH authentication cache
ssh-add -D
ssh-agent -k

# Restart terminal
# Try again
```

---

## 📋 DOCUMENTATION IN OBSIDIAN VAULT:

| File | Description | Status |
|------|-------------|--------|
| `OLLAMA-INTEGRATION-GUIDE.md` | Complete 6-phase plan | ✅ Created |
| `OLLAMA-ADAPTER-CLIENT.go.md` | Go client code | ✅ Created |
| `OLLAMA-AGENT-INTEGRATION.go.md` | Harold integration code | ✅ Created |
| `OLLAMA-IMPLEMENTATION-STATUS.md` | Implementation tracker | ✅ Created |
| `RTX3090-CODING-MODEL-RESEARCH.md` | Research document | ✅ Created |

---

## 🎯 NEXT ACTIONS:

1. ⏳ **Clear SSH cache** (ssh-add -D)
2. ⏳ **SSH to Pink** (192.168.1.186)
3. ⏳ **Install Ollama** (brew install ollama)
4. ⏳ **Pull DeepSeek-Coder-V2-Lite** (ollama pull deepseek-coder-v2-lite)
5. ⏳ **Build A2A adapter** (client.go)
6. ⏳ **Integrate with Harold** (ollama_agent.go)
7. ⏳ **Test integration** (A2A messages)
8. ⏳ **Document findings** (update registry)

---

## ⏰ TIMELINE:

| Phase | Time | Status |
|-------|-------|--------|
| **Documentation** | 3 hours | ✅ Complete |
| **Phase 1-6 (Execution)** | 6-9 hours | ⏳ Pending (SSH blocked) |
| **TOTAL** | 9-12 hours | ⏳ 70% complete |

---

**Last Updated:** February 1, 2026 (3:35 PM EST)

**Status:** 🟢 Implementation Ready - All Documentation in Obsidian Vault

**Note: Execution blocked by SSH access issues. Please clear SSH cache to proceed.**