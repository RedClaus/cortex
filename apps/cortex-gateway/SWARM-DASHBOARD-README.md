---
project: Cortex-Gateway
component: Agents
phase: Ideation
date_created: 2026-02-06T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T11:39:42.338001
---

# 🎯 Cortex Swarm Dashboard

**Real-time monitoring dashboard for the Cortex swarm coding agents**

**Status:** ✅ Built and Ready to Deploy
**Port:** 18888
**Deployment Target:** Pink (192.168.1.186)

---

## Features

### 🤖 Agent Monitoring
- **Live Status:** See which agents are online/offline in real-time
- **Current Tasks:** View what each agent is currently working on
- **Reasoning Stream:** Watch the thinking/reasoning process as agents code
- **Service Discovery:** See all services running on each agent

### 📊 Activity Timeline
- **Recent Activities:** Last 10 agent activities from memory API
- **Timestamped Events:** See when tasks were started/completed
- **Episodic & Knowledge:** Filter by memory type

### 🧠 Inference Engines
- **Engine Status:** All 4 coding engines (local + free cloud)
- **Model List:** See which models are available
- **Live Health:** Engine health checks every 5 seconds

### ⚡ Real-time Updates
- **Server-Sent Events (SSE):** Live streaming, no polling
- **5-Second Refresh:** Dashboard updates every 5 seconds
- **Auto-Reconnect:** Handles connection drops gracefully

---

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Cortex Swarm Dashboard                    │
│                    (Port 18888 on Pink)                      │
└─────────────────┬───────────────────────────────────────────┘
                  │
                  ├─→ /api/v1/swarm/agents         (Agent list)
                  ├─→ /api/v1/healthring/status    (Health data)
                  ├─→ /api/v1/memories/search      (Activity logs)
                  └─→ /api/v1/inference/engines    (LLM engines)
                  │
         ┌────────┴────────┐
         │ cortex-gateway  │
         │  (Port 8080)    │
         └─────────────────┘
```

**Data Flow:**
1. Dashboard polls cortex-gateway APIs every 5 seconds
2. Enriches agent data with recent memory activities
3. Streams updates to all connected browsers via SSE
4. Browsers render live updates without page refresh

---

## Deployment

### Quick Deploy to Pink

```bash
cd /Users/normanking/ServerProjectsMac/cortex-gateway-test
chmod +x deploy-dashboard-pink.sh
./deploy-dashboard-pink.sh
```

**What this does:**
1. Builds dashboard for Linux ARM64
2. Copies binary to pink (192.168.1.186)
3. Installs systemd service
4. Starts dashboard automatically

### Manual Deployment

If the script doesn't work, deploy manually:

```bash
# 1. Build for Linux ARM64
GOOS=linux GOARCH=arm64 go build -o swarm-dashboard-linux-arm64 ./cmd/swarm-dashboard/

# 2. Copy to pink
scp swarm-dashboard-linux-arm64 norman@192.168.1.186:/home/norman/cortex-gateway/swarm-dashboard
scp swarm-dashboard.service norman@192.168.1.186:/tmp/

# 3. SSH to pink and install
ssh norman@192.168.1.186

cd /home/norman/cortex-gateway
chmod +x swarm-dashboard

sudo mv /tmp/swarm-dashboard.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable swarm-dashboard
sudo systemctl start swarm-dashboard
sudo systemctl status swarm-dashboard
```

---

## Access

### From Browser
**Pink (local network):**
```
http://192.168.1.186:18888
```

**From MacBook:**
```
http://192.168.1.186:18888
```

**From any swarm agent:**
```
http://pink:18888
```

### API Endpoints

The dashboard itself exposes:
- `GET /` - Dashboard HTML interface
- `GET /events` - SSE stream for real-time updates (JSON)

---

## Usage

### What You'll See

#### Agent Cards
Each agent shows:
- **Name & IP address**
- **Status badge** (UP/DOWN/BUSY)
- **Services** running on that agent
- **Current task** (if busy coding)
- **Recent reasoning** (last 2-3 activities)

**Example:**
```
┌─────────────────────────────────┐
│ harold                          │
│ 192.168.1.128       [UP • BUSY] │
│                                 │
│ • bridge: 18802                 │
│ • gateway: 18789                │
│                                 │
│ Current Task:                   │
│ Implementing WebSocket handler  │
│ for real-time agent comms...    │
│                                 │
│ Recent Activity:                │
│ Started Phase 1 task P1-2...    │
│ Built service registry schema...│
└─────────────────────────────────┘
```

#### Inference Engines
Shows all 4 coding engines:
- ollama-pink (local, FREE)
- glm-coding (GLM 4-7, FREE)
- kimi-coding (Kimi K2.5, FREE)
- minimax (cheap)

#### Activity Timeline
Last 10 agent activities with:
- Timestamp
- Memory type (episodic/knowledge)
- Activity description

---

## Service Management (on Pink)

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

### Stop Dashboard
```bash
ssh norman@192.168.1.186 'sudo systemctl stop swarm-dashboard'
```

### Update Dashboard
```bash
cd /Users/normanking/ServerProjectsMac/cortex-gateway-test
./deploy-dashboard-pink.sh  # Rebuilds and redeploys
```

---

## Configuration

### Dashboard Configuration (cmd/swarm-dashboard/main.go)

```go
const (
    gatewayURL    = "http://localhost:8080"  // Cortex gateway API
    dashboardPort = 18888                    // Dashboard HTTP port
)
```

**To change:**
1. Edit `cmd/swarm-dashboard/main.go`
2. Rebuild: `go build -o swarm-dashboard ./cmd/swarm-dashboard/`
3. Redeploy to pink

### Update Interval

Currently updates every **5 seconds**. To change:

```go
// In main(), line ~245
ticker := time.NewTicker(5 * time.Second)  // Change to 10 * time.Second for 10s
```

---

## Troubleshooting

### Dashboard not accessible

**Check if running:**
```bash
ssh norman@192.168.1.186 'sudo systemctl status swarm-dashboard'
```

**Check if port is open:**
```bash
ssh norman@192.168.1.186 'sudo lsof -i :18888'
```

**Check logs:**
```bash
ssh norman@192.168.1.186 'sudo journalctl -u swarm-dashboard -n 50'
```

### No agent data showing

**Verify gateway is accessible:**
```bash
ssh norman@192.168.1.186 'curl http://localhost:8080/api/v1/swarm/agents'
```

**If gateway is not running:**
```bash
ssh norman@192.168.1.186
cd /home/norman/cortex-gateway
./cortex-gateway &
```

### SSE connection fails

**Browser error:** "EventSource failed"

**Solution:** The dashboard uses Server-Sent Events. Make sure:
1. Pink's firewall allows port 18888
2. Gateway is running on port 8080
3. Browser supports SSE (all modern browsers do)

### Agents show "no current task"

This is normal if agents are idle. Tasks are detected by:
1. Searching memory API for agent names
2. Matching recent memories (episodic type)
3. Displaying the most recent activity

**To test with mock data:**
```bash
curl -X POST http://localhost:8080/api/v1/memories/store \
  -H "Content-Type: application/json" \
  -d '{"content":"harold: Working on authentication module","type":"episodic","importance":0.7}'
```

---

## Development

### Local Testing

Run locally (on MacBook):
```bash
cd /Users/normanking/ServerProjectsMac/cortex-gateway-test
go run ./cmd/swarm-dashboard/main.go
```

Access at: http://localhost:18888

### Build for Different Platforms

**Linux ARM64 (Pink):**
```bash
GOOS=linux GOARCH=arm64 go build -o swarm-dashboard-linux-arm64 ./cmd/swarm-dashboard/
```

**Linux x86_64:**
```bash
GOOS=linux GOARCH=amd64 go build -o swarm-dashboard-linux-amd64 ./cmd/swarm-dashboard/
```

**macOS ARM64 (M1/M2):**
```bash
GOOS=darwin GOARCH=arm64 go build -o swarm-dashboard-macos-arm64 ./cmd/swarm-dashboard/
```

---

## API Integration

### Fetch Agent Data (Example)

```bash
curl http://192.168.1.186:18888/events
```

Returns SSE stream with JSON:
```json
data: {
  "Agents": [
    {
      "name": "harold",
      "ip": "192.168.1.128",
      "status": "up",
      "current_task": "Building WebSocket handler...",
      "reasoning": ["Started task P1-2", "Analyzing message queue"]
    }
  ],
  "LastUpdate": "2026-02-06T11:15:30Z",
  "UpdateInterval": 5
}
```

---

## Security Notes

### Port Exposure
- Dashboard runs on **port 18888** (accessible on local network)
- No authentication required (internal tool)
- Firewall: Only allow local network access (192.168.1.0/24)

### Data Privacy
- Dashboard only shows data from memory API
- No sensitive credentials displayed
- Agent reasoning may contain code snippets

### Recommended Firewall Rule (Pink)
```bash
sudo ufw allow from 192.168.1.0/24 to any port 18888
sudo ufw deny 18888
```

---

## Roadmap

### Planned Features
- [ ] **Agent CPU/Memory metrics** (if available from agents)
- [ ] **Code snippets preview** (syntax highlighting)
- [ ] **Task completion progress bars**
- [ ] **Agent communication graph** (who talks to who)
- [ ] **Historical charts** (agent uptime, task count over time)
- [ ] **Filtering** (show only busy agents, hide offline)
- [ ] **Search** (search activity timeline)
- [ ] **Dark/Light theme toggle**

### Possible Integrations
- [ ] Slack notifications (agent started/stopped)
- [ ] Prometheus metrics export
- [ ] WebSocket for even faster updates
- [ ] Mobile-responsive design

---

## Files

| File | Purpose |
|------|---------|
| `cmd/swarm-dashboard/main.go` | Dashboard Go source |
| `swarm-dashboard` | Compiled binary (macOS) |
| `swarm-dashboard-linux-arm64` | Compiled binary (Pink) |
| `swarm-dashboard.service` | systemd service file |
| `deploy-dashboard-pink.sh` | Deployment script |
| `swarm-dashboard.log` | Local test logs |
| `SWARM-DASHBOARD-README.md` | This file |

---

## Credits

**Created:** 2026-02-06
**Author:** Claude Code (Opus 4.5)
**Purpose:** Real-time monitoring for Cortex swarm coding agents
**License:** Internal tool for Cortex ecosystem

---

## Quick Reference

**Build:**
```bash
go build -o swarm-dashboard ./cmd/swarm-dashboard/
```

**Deploy:**
```bash
./deploy-dashboard-pink.sh
```

**Access:**
```
http://192.168.1.186:18888
```

**Logs:**
```bash
ssh norman@192.168.1.186 'sudo journalctl -u swarm-dashboard -f'
```

**Restart:**
```bash
ssh norman@192.168.1.186 'sudo systemctl restart swarm-dashboard'
```

---

🎉 **Enjoy real-time visibility into your coding swarm!**
