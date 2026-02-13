---
project: Cortex-Gateway
component: Agents
phase: Ideation
date_created: 2026-02-05T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:21:06.707930
---

# Agent Memory Integration Guide

## Overview

All agents in the Cortex ecosystem now access memory through a unified REST API provided by the Cortex Gateway. This eliminates the need for direct file access and provides a consistent, protocol-agnostic interface.

**Architecture:**
```
┌─────────────────────────────────────────────────────────┐
│                  Agent Ecosystem                        │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐ │
│  │ OpenClaw     │  │ CortexHub CLI│  │Harold Bridge │ │
│  │ (Scripts)    │  │    (ch)      │  │   (A2A)      │ │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘ │
│         │                 │                  │         │
│         └─────────────────┼──────────────────┘         │
│                           │                            │
│                           ▼                            │
│              ┌─────────────────────────┐               │
│              │   Cortex Gateway        │               │
│              │   Memory REST API       │               │
│              │   Port 8080             │               │
│              └────────────┬────────────┘               │
│                           │                            │
│                           ▼                            │
│              ┌─────────────────────────┐               │
│              │  ~/.cortex/memory/      │               │
│              │  - 2026-02-05.md        │               │
│              │  - knowledge.md         │               │
│              └─────────────────────────┘               │
└─────────────────────────────────────────────────────────┘
```

---

## Unified Memory API Endpoints

All agents use these standard endpoints:

| Endpoint | Method | Purpose | Example |
|----------|--------|---------|---------|
| `/api/v1/memories/search` | GET | Search memories | `?q=Norman&limit=10` |
| `/api/v1/memories/store` | POST | Store new memory | `{"content":"...", "type":"episodic"}` |
| `/api/v1/memories/recent` | GET | Get recent memories | `?limit=5` |
| `/api/v1/memories/stats` | GET | Get statistics | N/A |

**Base URL:** `http://localhost:8080/api/v1/memories`

---

## Updated Agent Tools

### 1. **memory-reflect.sh** (OpenClaw)

**Location:** `/Users/normanking/.openclaw/workspace/scripts/memory-reflect.sh`

**Key Changes:**
- ✅ Now uses REST API instead of direct file access
- ✅ All commands work via HTTP requests
- ✅ JSON response parsing with jq
- ✅ Consistent with other agents

**Usage:**
```bash
# Search memories
memory-reflect.sh search "Norman"

# Get recent memories
memory-reflect.sh recent 10

# Store new memory
memory-reflect.sh store "Completed task XYZ" episodic 0.9

# Get stats
memory-reflect.sh stats

# Today's notes
memory-reflect.sh today
```

**Implementation Pattern:**
```bash
# Example: Search function
memory_search() {
    local query="$1"
    response=$(curl -s "${GATEWAY_URL}/search?q=$(printf %s "$query" | jq -sRr @uri)&limit=50")
    echo "$response" | jq -r '.results[] | "  📝 \(.content)"'
}
```

---

### 2. **ch** (CortexHub CLI)

**Location:** `/Users/normanking/.openclaw/workspace/scripts/ch`

**Key Changes:**
- ✅ Removed authentication (no JWT required for now)
- ✅ Simplified to use only Memory API
- ✅ Updated endpoints to `/api/v1/memories/*`
- ✅ Changed base URL from port 18892 to 8080

**Usage:**
```bash
# Search memories
ch search Authentik

# Get recent memories
ch recent 10

# Store episodic memory
ch store "Completed implementation"

# Store high-importance memory
ch pin "Critical decision made"

# Store knowledge
ch learn "Norman prefers k3s over multi-cluster"

# Get stats
ch stats

# Today's notes
ch today
```

**Implementation Pattern:**
```python
# Example: Search function
def cmd_search(query):
    r = api("GET", f"/search?q={urllib.parse.quote(query)}&limit=50")
    for item in r.get("results", []):
        content = item.get("content", "")[:120]
        print(f"  📝 {content}")
```

---

## Agent Integration Patterns

### Pattern 1: Shell Scripts (Bash)

**Prerequisites:**
- `curl` for HTTP requests
- `jq` for JSON parsing

**Template:**
```bash
#!/bin/bash
GATEWAY_URL="http://localhost:8080/api/v1/memories"

# Search memories
search_memory() {
    local query="$1"
    response=$(curl -s "${GATEWAY_URL}/search?q=$(printf %s "$query" | jq -sRr @uri)")
    echo "$response" | jq -r '.results[] | .content'
}

# Store memory
store_memory() {
    local content="$1"
    local type="${2:-episodic}"

    payload=$(jq -n \
        --arg content "$content" \
        --arg type "$type" \
        '{content: $content, type: $type, importance: 0.8}')

    curl -s -X POST "${GATEWAY_URL}/store" \
        -H "Content-Type: application/json" \
        -d "$payload"
}
```

---

### Pattern 2: Python Agents

**Prerequisites:**
- Standard library only (no dependencies)

**Template:**
```python
import json, urllib.request, urllib.parse

GATEWAY = "http://localhost:8080/api/v1/memories"

def api(method, path, data=None):
    """Call Memory API endpoint"""
    headers = {"Content-Type": "application/json"}
    body = json.dumps(data).encode() if data else None
    req = urllib.request.Request(GATEWAY + path, data=body, headers=headers, method=method)

    with urllib.request.urlopen(req, timeout=10) as r:
        return json.loads(r.read())

# Search memories
def search_memory(query):
    r = api("GET", f"/search?q={urllib.parse.quote(query)}&limit=50")
    return r.get("results", [])

# Store memory
def store_memory(content, mem_type="episodic", importance=0.8):
    r = api("POST", "/store", {
        "content": content,
        "type": mem_type,
        "importance": importance
    })
    return r.get("status") == "success"
```

---

### Pattern 3: Go Agents

**Prerequisites:**
- Standard `net/http` and `encoding/json`

**Template:**
```go
package memory

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

const gatewayURL = "http://localhost:8080/api/v1/memories"

type SearchResult struct {
	Query   string  `json:"query"`
	Results []Entry `json:"results"`
	Count   int     `json:"count"`
}

type Entry struct {
	Content    string  `json:"content"`
	Timestamp  string  `json:"timestamp"`
	File       string  `json:"file"`
	Type       string  `json:"type"`
	Importance float64 `json:"importance"`
}

// Search memories
func Search(query string, limit int) (*SearchResult, error) {
	url := fmt.Sprintf("%s/search?q=%s&limit=%d",
		gatewayURL, url.QueryEscape(query), limit)

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result SearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// Store memory
func Store(content, memType string, importance float64) error {
	payload := map[string]interface{}{
		"content":    content,
		"type":       memType,
		"importance": importance,
	}

	body, _ := json.Marshal(payload)
	resp, err := http.Post(gatewayURL+"/store",
		"application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}
```

---

### Pattern 4: JavaScript/TypeScript

**Prerequisites:**
- `fetch` API or `axios`

**Template:**
```javascript
const GATEWAY = 'http://localhost:8080/api/v1/memories';

// Search memories
async function searchMemory(query, limit = 50) {
  const response = await fetch(
    `${GATEWAY}/search?q=${encodeURIComponent(query)}&limit=${limit}`
  );
  const data = await response.json();
  return data.results;
}

// Store memory
async function storeMemory(content, type = 'episodic', importance = 0.8) {
  const response = await fetch(`${GATEWAY}/store`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ content, type, importance })
  });
  const data = await response.json();
  return data.status === 'success';
}

// Get stats
async function getStats() {
  const response = await fetch(`${GATEWAY}/stats`);
  const data = await response.json();
  return data.stats;
}
```

---

## Migration Guide

### Step 1: Identify Direct File Access

Search for these patterns in your agent code:

**Bad (Direct File Access):**
```bash
# Don't do this
grep -i "query" ~/.cortex/memory/*.md
cat ~/.cortex/memory/2026-02-05.md
echo "new memory" >> ~/.cortex/memory/knowledge.md
```

**Good (REST API):**
```bash
# Do this instead
curl "http://localhost:8080/api/v1/memories/search?q=query"
curl -X POST "http://localhost:8080/api/v1/memories/store" \
  -d '{"content":"new memory", "type":"knowledge"}'
```

---

### Step 2: Update Endpoint URLs

| Old Endpoint | New Endpoint | Notes |
|--------------|--------------|-------|
| `/api/admin/memories/all` | `/api/v1/memories/search` | Removed admin namespace |
| `/api/memories/search` | `/api/v1/memories/search` | Added `/v1/` version |
| `/api/memories` | `/api/v1/memories/store` | Changed to `/store` |
| `/api/knowledge` | `/api/v1/memories/store` | Use `type: "knowledge"` |
| `/api/memories/stats` | `/api/v1/memories/stats` | Added `/v1/` version |

---

### Step 3: Update Base URL

| Agent | Old Base URL | New Base URL |
|-------|--------------|--------------|
| CortexHub CLI | `http://192.168.1.186:18892` | `http://localhost:8080` |
| memory-reflect.sh | Direct file access | `http://localhost:8080` |
| Harold Bridge | `http://192.168.1.186:18892` | `http://localhost:8080` |
| OpenClaw scripts | Direct file access | `http://localhost:8080` |

---

### Step 4: Remove Authentication (For Now)

The new Memory API doesn't require authentication yet. Remove JWT/token logic:

**Before:**
```python
def get_token():
    r = api("POST", "/api/auth/login", {"username": USER, "password": PASS})
    return r.get("tokens", {}).get("accessToken", "")

token = get_token()
api("GET", "/api/memories/search", token=token)
```

**After:**
```python
# No authentication needed
api("GET", "/search?q=query")
```

---

## Testing Agent Integration

### Test Checklist

For each agent you update:

1. **Search Test:**
   ```bash
   # Should return results
   <agent> search "Authentik"
   ```

2. **Store Test:**
   ```bash
   # Should store successfully
   <agent> store "Test memory entry"
   ```

3. **Recent Test:**
   ```bash
   # Should show recent memories
   <agent> recent 5
   ```

4. **Stats Test:**
   ```bash
   # Should show statistics
   <agent> stats
   ```

---

### Example Test Session

```bash
# Test memory-reflect.sh
./memory-reflect.sh stats
# Expected: Shows total_entries, episodic_count, knowledge_count

./memory-reflect.sh search "Norman"
# Expected: Returns matching memories

./memory-reflect.sh store "Test from memory-reflect" episodic 0.7
# Expected: "✅ Memory stored successfully!"

# Test CortexHub CLI
python3 ch stats
# Expected: Shows formatted statistics

python3 ch search "Norman"
# Expected: Returns matching memories with icons

python3 ch store "Test from ch"
# Expected: "✅ 📝 Stored (importance=0.8)"

# Verify both wrote to same store
curl "http://localhost:8080/api/v1/memories/search?q=Test" | jq .
# Expected: Both "Test from memory-reflect" and "Test from ch"
```

---

## Benefits of Unified Approach

### 1. **Protocol Agnostic**
- Any agent can access memory via HTTP
- No language-specific dependencies
- No direct file system access needed

### 2. **Consistent Interface**
- Same endpoints for all agents
- Predictable request/response format
- Easier to maintain and debug

### 3. **Scalability**
- Can add caching layer
- Can add authentication when needed
- Can upgrade to distributed storage

### 4. **Future-Proof**
- Easy to add semantic search
- Easy to add A2A protocol layer
- Easy to add access control

---

## Current Agent Status

| Agent | Status | Location | Notes |
|-------|--------|----------|-------|
| **memory-reflect.sh** | ✅ Updated | `.openclaw/workspace/scripts/` | REST API |
| **ch (CortexHub CLI)** | ✅ Updated | `.openclaw/workspace/scripts/` | REST API |
| **a2a-memory-bridge** | ✅ Updated | `.openclaw/workspace/` | A2A → REST API |
| **cortex-gateway** | ✅ Active | `cortex-gateway-test/` | Serving API |
| **Harold Bridge** | ✅ Active | Port 18802 | A2A router (no changes needed) |
| **CortexAvatar** | ⚠️ Pending | `CortexAvatar/` | Needs integration |
| **Salamander** | ⚠️ Pending | `Salamander/` | Needs integration |

---

## Troubleshooting

### Problem: "Failed to connect to Memory API"

**Cause:** Cortex Gateway not running

**Solution:**
```bash
# Check if gateway is running
lsof -i :8080

# If not, start it
cd /Users/normanking/ServerProjectsMac/cortex-gateway-test
./cortex-gateway &
```

---

### Problem: "jq: command not found" (memory-reflect.sh)

**Cause:** jq not installed

**Solution:**
```bash
brew install jq
```

---

### Problem: Empty search results

**Cause:** Query doesn't match any content

**Solution:**
- Try broader search terms
- Check if memory files exist: `ls ~/.cortex/memory/`
- Verify stats show entries: `ch stats`

---

### Problem: "Permission denied" when storing

**Cause:** File permissions on `~/.cortex/memory/`

**Solution:**
```bash
chmod 755 ~/.cortex/memory
chmod 644 ~/.cortex/memory/*.md
```

---

## Adding New Agents

To add a new agent to the unified memory system:

1. **Choose integration pattern** (Bash/Python/Go/JS)
2. **Implement three core functions:**
   - Search
   - Store
   - Stats
3. **Test with existing memory data**
4. **Add to agent status table** above

**Minimal Example (Python):**
```python
import json, urllib.request

GATEWAY = "http://localhost:8080/api/v1/memories"

def search(query):
    url = f"{GATEWAY}/search?q={query}"
    with urllib.request.urlopen(url) as r:
        return json.loads(r.read())["results"]

def store(content):
    req = urllib.request.Request(
        f"{GATEWAY}/store",
        data=json.dumps({"content": content, "type": "episodic"}).encode(),
        headers={"Content-Type": "application/json"}
    )
    with urllib.request.urlopen(req) as r:
        return json.loads(r.read())["status"] == "success"
```

---

## Documentation Links

- **Memory API Documentation:** [MEMORY-API.md](./MEMORY-API.md)
- **Memory API Proposal:** [CortexBrain/Vault/MemoryAPI-Endpoint.md](../CortexBrain/Vault/MemoryAPI-Endpoint.md)
- **Cortex Gateway README:** [README.md](./README.md)

---

## Version History

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-02-05 | Initial unified memory access implementation |

---

*All agents now use a single, unified REST API for memory access. No more direct file operations!*
