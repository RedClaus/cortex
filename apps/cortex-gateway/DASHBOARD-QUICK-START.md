---
project: Cortex-Gateway
component: UI
phase: Design
date_created: 2026-02-06T11:13:40
source: ServerProjectsMac
librarian_indexed: 2026-02-06T11:39:42.346781
---

# 🚀 Swarm Dashboard - Quick Start

**Real-time monitoring for your Cortex coding swarm!**

---

## Deploy to Pink (One Command)

```bash
cd /Users/normanking/ServerProjectsMac/cortex-gateway-test
./deploy-dashboard-pink.sh
```

**This will:**
1. ✅ Build dashboard for Linux ARM64
2. ✅ Copy to pink (192.168.1.186)
3. ✅ Install as systemd service
4. ✅ Start automatically

**Expected output:**
```
🚀 Deploying Cortex Swarm Dashboard to Pink...
📦 Building dashboard for Linux (arm64)...
📤 Copying binary to pink...
📤 Copying service file...
⚙️  Installing service...
✅ Service installed and started
✅ Dashboard deployed successfully!
🌐 Access at: http://192.168.1.186:18888
```

---

## Access Dashboard

**Open in your browser:**
```
http://192.168.1.186:18888
```

**From any device on the network:**
- MacBook: http://192.168.1.186:18888
- Pink: http://localhost:18888
- Harold: http://192.168.1.186:18888

---

## What You'll See

### 1. Agent Status (Live)
Each agent card shows:
- Name & IP address
- Online/Offline/Busy status
- Running services (bridge, ollama, gateway, etc.)
- Current coding task
- Recent reasoning/thinking

**Example:**
```
┌─────────────────────────────┐
│ harold                       │
│ 192.168.1.128   [UP • BUSY] │
│                              │
│ Services:                    │
│ • bridge: 18802              │
│ • gateway: 18789             │
│                              │
│ Current Task:                │
│ Implementing WebSocket...    │
│                              │
│ Recent Activity:             │
│ • Started Phase 1 P1-2       │
│ • Built service registry     │
└─────────────────────────────┘
```

### 2. Inference Engines
Shows all 4 coding models:
- **ollama-pink** - Local FREE models (go-coder, cortex-coder, deepseek)
- **glm-coding** - GLM 4-7 FREE unlimited
- **kimi-coding** - Kimi K2.5 FREE
- **minimax** - MiniMax cheap

### 3. Activity Timeline
Last 10 agent activities:
- What each agent is doing
- When they started
- Memory type (episodic/knowledge)

### 4. Live Updates
- Updates automatically every 5 seconds
- Green pulsing dot = Live connection
- No page refresh needed!

---

## Common Tasks

### View Logs
```bash
ssh norman@192.168.1.186 'sudo journalctl -u swarm-dashboard -f'
```

### Restart Dashboard
```bash
ssh norman@192.168.1.186 'sudo systemctl restart swarm-dashboard'
```

### Check Status
```bash
ssh norman@192.168.1.186 'sudo systemctl status swarm-dashboard'
```

### Redeploy (after making changes)
```bash
cd /Users/normanking/ServerProjectsMac/cortex-gateway-test
./deploy-dashboard-pink.sh
```

---

## Troubleshooting

### Dashboard not loading?

**1. Check if it's running:**
```bash
ssh norman@192.168.1.186 'sudo systemctl status swarm-dashboard'
```

**2. Check logs:**
```bash
ssh norman@192.168.1.186 'sudo journalctl -u swarm-dashboard -n 20'
```

**3. Verify port is open:**
```bash
ssh norman@192.168.1.186 'sudo lsof -i :18888'
```

### No agent data?

**Make sure cortex-gateway is running:**
```bash
ssh norman@192.168.1.186
cd /home/norman/cortex-gateway
./cortex-gateway > cortex-gateway.log 2>&1 &
```

### Connection issues?

**Test gateway from pink:**
```bash
ssh norman@192.168.1.186 'curl http://localhost:8080/api/v1/swarm/agents'
```

---

## Features

✅ **Real-time agent status** (up/down/busy)
✅ **Live task monitoring** (see what agents are coding)
✅ **Reasoning stream** (watch agents think)
✅ **Activity timeline** (last 10 activities)
✅ **Inference engine status** (4 FREE/cheap models)
✅ **Server-Sent Events** (no polling, instant updates)
✅ **Beautiful UI** (gradient background, smooth animations)
✅ **Auto-reconnect** (handles network drops)

---

## Architecture

```
Browser (You)
    ↓
http://192.168.1.186:18888
    ↓
Swarm Dashboard (Port 18888)
    ↓
Cortex Gateway (Port 8080)
    ↓
Memory API, Swarm Discovery, Health Ring
```

**Update flow:**
1. Dashboard polls gateway APIs every 5 seconds
2. Enriches agent data with recent memories
3. Streams updates to browser via SSE
4. Browser renders without page refresh

---

## Next Steps

Once dashboard is deployed:

1. **Open in browser:** http://192.168.1.186:18888
2. **Watch agents work:** See live status and tasks
3. **Monitor swarm health:** Check if all agents are online
4. **Track reasoning:** See what agents are thinking

---

## Files Created

| File | Purpose |
|------|---------|
| `cmd/swarm-dashboard/main.go` | Dashboard source code |
| `swarm-dashboard` | macOS binary (for testing) |
| `deploy-dashboard-pink.sh` | One-click deployment script |
| `swarm-dashboard.service` | systemd service file |
| `SWARM-DASHBOARD-README.md` | Full documentation |
| `DASHBOARD-QUICK-START.md` | This quick start guide |

---

## Ready to Deploy!

Just run:
```bash
./deploy-dashboard-pink.sh
```

Then open http://192.168.1.186:18888 and watch your swarm in action! 🎉

---

**Full Documentation:** See `SWARM-DASHBOARD-README.md` for advanced features, configuration, and troubleshooting.
