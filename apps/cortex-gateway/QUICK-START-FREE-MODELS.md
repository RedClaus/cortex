---
project: Cortex-Gateway
component: UI
phase: Ideation
date_created: 2026-02-06T11:03:05
source: ServerProjectsMac
librarian_indexed: 2026-02-06T11:39:42.381094
---

# Quick Start: Free Coding Models

**Status:** ✅ Deployed and Running
**Gateway Port:** 8080
**Default Lane:** local (FREE)

---

## Available Models

### 1. Local (DEFAULT - FREE)
```bash
# Automatically used for all requests
curl -X POST http://localhost:8080/api/v1/inference \
  -H "Content-Type: application/json" \
  -d '{"prompt": "Write a Go function to check if a number is prime"}'
```

**Models:**
- go-coder:latest (5GB) - Go-specialized
- cortex-coder:latest (9GB) - General coding
- deepseek-coder-v2:latest (9GB) - Complex refactoring

### 2. GLM 4-7 Coding (FREE UNLIMITED)
```bash
curl -X POST http://localhost:8080/api/v1/inference \
  -H "Content-Type: application/json" \
  -d '{"prompt": "Explain this Python algorithm", "lane": "glm"}'
```

### 3. Kimi K2.5 Coding (FREE)
```bash
curl -X POST http://localhost:8080/api/v1/inference \
  -H "Content-Type: application/json" \
  -d '{"prompt": "Refactor this JavaScript code", "lane": "kimi"}'
```

### 4. MiniMax (CHEAP)
```bash
curl -X POST http://localhost:8080/api/v1/inference \
  -H "Content-Type: application/json" \
  -d '{"prompt": "Design a database schema for...", "lane": "minimax"}'
```

---

## Model Selection Guide

| Task Type | Recommended Lane | Why |
|-----------|------------------|-----|
| **Go coding** | `local` (go-coder) | Specialized, fast, free |
| **Simple coding** | `local` (cortex-coder) | Fast, private, free |
| **Complex refactoring** | `local` (deepseek-coder-v2) | Powerful, free |
| **Documentation** | `glm` | Free unlimited, good quality |
| **Code explanation** | `glm` or `kimi` | Both free, good for understanding |
| **Architecture design** | `minimax` | High quality, worth the small cost |

---

## Cost Comparison

| Lane | Cost per 1M tokens | Use for |
|------|-------------------|---------|
| **local** | $0 (FREE) | All coding tasks |
| **glm** | $0 (FREE unlimited) | Complex reasoning, docs |
| **kimi** | $0 (FREE) | Code understanding |
| **minimax** | ~$0.10 (CHEAP) | High-stakes decisions |

**Old default (Grok/Claude):** $15-30 per 1M tokens
**New default (local):** $0 per 1M tokens
**Savings: 100% on most requests**

---

## Quick Commands

### Check Status
```bash
curl http://localhost:8080/health | jq
```

### List All Engines
```bash
curl http://localhost:8080/api/v1/inference/engines | jq
```

### List All Models
```bash
curl http://localhost:8080/api/v1/inference/models | jq
```

### Restart Gateway
```bash
cd ~/ServerProjectsMac/cortex-gateway-test
pkill -9 -f cortex-gateway
./cortex-gateway > cortex-gateway.log 2>&1 &
```

### View Logs
```bash
tail -f ~/ServerProjectsMac/cortex-gateway-test/cortex-gateway.log
```

---

## Integration with Swarm

All swarm agents (harold, pink, red, kentaro) can now use these free models:

```bash
# From any agent
export CORTEX_GATEWAY="http://localhost:8080"

# Use local models (free)
curl -X POST $CORTEX_GATEWAY/api/v1/inference \
  -d '{"prompt": "Your coding task", "lane": "local"}'

# Use GLM (free unlimited)
curl -X POST $CORTEX_GATEWAY/api/v1/inference \
  -d '{"prompt": "Your complex task", "lane": "glm"}'
```

---

**Quick Tip:** If you don't specify a lane, it defaults to `local` (FREE local models). This means you can use the gateway without worrying about costs!

**Documentation:** See `FREE-CODING-MODELS-SUCCESS.md` for full details.
