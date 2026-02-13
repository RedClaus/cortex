---
project: Cortex
component: Unknown
phase: Ideation
date_created: 2026-02-01T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:16:29.818645
---

# CortexBrain Container Framework — Design & Deployment Plan
**Version:** 0.1 | **Date:** 2026-02-01 | **Author:** Norman King + Albert
**Status:** PRIORITY #1 — Architectural Foundation
**Assigned:** Harold (Overseer) + Albert (Architect)

---

## Executive Summary

**Decision:** Containerize the entire Cortex ecosystem so that every agent, service, and brain can be:
- Deployed on any OS with a single command
- Backed up and restored anywhere
- Managed via API (not SSH)
- Auto-healed on crash
- Scaled by spinning up new containers

This is the **foundation** for everything — the swarm, CortexBrain, CortexAgent, CortexInfra, CortexHive, the Avatar, and all future projects.

---

## Design Principles

1. **OS-Agnostic** — Runs on Linux, macOS, Windows. Backup on one, restore on another.
2. **API-Managed** — Every agent's lifecycle (start/stop/restart/update/delete) via REST API.
3. **Self-Healing** — Crashed containers auto-restart. No human intervention for routine failures.
4. **Portable State** — All agent state (memory, config, workspace) in mounted volumes. Backup = tar the volume.
5. **Image = Truth** — The Docker image contains everything needed to run. No "missing binary" ever again.
6. **Service Discovery** — Agents find each other automatically. No hardcoded IPs/ports.
7. **GPU Passthrough** — Containers can access GPU when available (Pink's RTX 3090).
8. **Secure by Default** — Rootless containers, isolated networks, secret management.

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────────┐
│                     CONTROL PLANE (BotMaker or Custom)                   │
│                                                                          │
│   REST API: /api/bots, /api/stats, /health                             │
│   Dashboard: Web UI for monitoring + management                         │
│   Image Registry: Local registry for agent images                       │
│                                                                          │
└───────────────────────────────┬──────────────────────────────────────────┘
                                │ Docker API
                                │
        ┌───────────────────────┼───────────────────────┐
        │                       │                       │
   ┌────▼────┐            ┌────▼────┐            ┌────▼────┐
   │ NETWORK │            │ NETWORK │            │ NETWORK │
   │  BRAIN  │            │  SWARM  │            │ SERVICES│
   │         │            │         │            │         │
   │ cortex- │            │ harold  │            │ bridge  │
   │ brain   │            │ pink    │            │ redis   │
   │ observer│            │ red     │            │ registry│
   │ monitor │            │ skippy  │            │ monitor │
   │ avatar  │            │ kentaro │            │ ollama  │
   │         │            │ CCP×3   │            │         │
   └────┬────┘            └────┬────┘            └────┬────┘
        │                      │                      │
   ┌────▼────────────────────▼────────────────────────▼────┐
   │              SHARED VOLUMES                             │
   │                                                          │
   │  /data/agents/{name}/          ← per-agent workspace    │
   │    ├── workspace/              ← MEMORY.md, files       │
   │    ├── config/                 ← gateway config         │
   │    ├── sessions/               ← session logs           │
   │    └── secrets/                ← API keys (encrypted)   │
   │                                                          │
   │  /data/shared/                 ← cross-agent resources  │
   │    ├── knowledge/              ← shared knowledge base  │
   │    ├── models/                 ← Ollama model cache     │
   │    └── backups/                ← automated backups      │
   └──────────────────────────────────────────────────────────┘
```

---

## What Gets Containerized

### Core Services

| Service | Image | Port | GPU | Notes |
|---------|-------|------|-----|-------|
| **A2A Bridge** | `cortex/bridge:latest` | 18802 | No | The spine — runs first, always |
| **Ollama** | `ollama/ollama:latest` | 11434 | Yes | Shared model inference |
| **Redis** | `redis:alpine` | 6379 | No | Message queuing, caching |
| **Local Registry** | `registry:2` | 5000 | No | Private Docker image registry |

### Agent Containers

| Agent | Base Image | Volumes | Special Config |
|-------|-----------|---------|----------------|
| **Harold** (Overseer) | `cortex/agent:latest` | workspace, config, sessions | Bridge access, agent management API |
| **Albert** (Prime) | `cortex/agent:latest` | workspace, config, sessions, memory | All channels, full tool access |
| **Albert-Shadow** | `cortex/agent:latest` | synced workspace | Failover, read-only sync |
| **Pink** (Coder-001) | `cortex/agent-gpu:latest` | workspace, config | GPU access, RTX 3090 |
| **Red** (Coder-002) | `cortex/agent:latest` | workspace, config | Standard worker |
| **Skippy** (General) | `cortex/agent:latest` | workspace, config | Standard worker |
| **Kentaro** (Shadow) | `cortex/agent:latest` | workspace, config | Harold shadow |
| **CCP Nodes** (×3) | `cortex/agent:latest` | workspace, config | Consensus cluster |
| **Health Ring** (×3) | `cortex/agent:latest` | workspace, config | Doctor/Nurse/Paramedic |

### CortexBrain Services

| Service | Image | Port | GPU | Notes |
|---------|-------|------|-----|-------|
| **CortexBrain** | `cortex/brain:latest` | 18892 | Optional | Core cognitive engine |
| **Neural Bus Observer** | `cortex/observer:latest` | 8765 | No | WebSocket event server |
| **Thinking Monitor** | `cortex/monitor:latest` | 5173 | No | 3D brain dashboard |
| **Avatar** | `cortex/avatar:latest` | 8080 | Yes | Talking head (future) |

---

## Image Strategy

### Base Images

```dockerfile
# cortex/agent:latest — Standard OpenClaw agent
FROM node:24-slim
RUN npm install -g openclaw
COPY default-config/ /etc/openclaw/
HEALTHCHECK --interval=30s CMD curl -f http://localhost:${PORT}/health || exit 1
ENTRYPOINT ["openclaw", "gateway", "start"]

# cortex/agent-gpu:latest — GPU-enabled agent (for Pink)
FROM cortex/agent:latest
RUN apt-get update && apt-get install -y nvidia-container-toolkit
# Ollama client pre-installed

# cortex/brain:latest — CortexBrain Go binary
FROM golang:1.22-alpine AS builder
COPY . /build
RUN cd /build && go build -o /cortex-brain ./cmd/cortex-server
FROM alpine:3.19
COPY --from=builder /cortex-brain /usr/local/bin/
HEALTHCHECK --interval=30s CMD curl -f http://localhost:18892/health || exit 1
ENTRYPOINT ["cortex-brain"]
```

### Per-Agent Configuration

Each agent gets a `docker-compose.agent.yml` or is managed via BotMaker API:

```yaml
# Example: Harold
services:
  harold:
    image: cortex/agent:latest
    container_name: harold
    restart: always
    ports:
      - "18789:18789"
    volumes:
      - /data/agents/harold/workspace:/root/.openclaw/workspace
      - /data/agents/harold/config:/root/.openclaw/config
      - /data/agents/harold/sessions:/root/.openclaw/agents/main/sessions
    environment:
      - OPENCLAW_MODEL=anthropic/claude-sonnet-4-20250514
      - OPENCLAW_CHANNELS=telegram
      - AGENT_ROLE=overseer
    networks:
      - swarm
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:18789/health"]
      interval: 30s
      retries: 3
    depends_on:
      - bridge
      - ollama
```

---

## Volume / State Management

### Per-Agent Volumes

```
/data/agents/{agent-name}/
├── workspace/           ← MEMORY.md, SOUL.md, USER.md, project files
│   └── memory/          ← Daily logs, archives
├── config/              ← openclaw.yaml, .env
├── sessions/            ← Session transcripts (.jsonl)
├── skills/              ← Installed skills
└── secrets/             ← API keys (encrypted at rest)
```

### Backup Strategy

```bash
# Backup entire swarm (one command)
#!/bin/bash
BACKUP_DIR="/data/backups/$(date +%Y%m%d-%H%M%S)"
mkdir -p $BACKUP_DIR

# Stop agents gracefully
docker compose stop

# Tar each agent's state
for agent in /data/agents/*/; do
  name=$(basename $agent)
  tar czf "$BACKUP_DIR/$name.tar.gz" -C /data/agents "$name"
done

# Backup shared resources
tar czf "$BACKUP_DIR/shared.tar.gz" -C /data shared/

# Restart agents
docker compose start

echo "Backup complete: $BACKUP_DIR"
```

```bash
# Restore on ANY machine (any OS with Docker)
#!/bin/bash
BACKUP_DIR=$1

# Extract all agent states
for archive in $BACKUP_DIR/*.tar.gz; do
  tar xzf "$archive" -C /data/agents/
done

# Start everything
docker compose up -d

echo "Swarm restored from $BACKUP_DIR"
```

### Cross-OS Portability

| Backup on... | Restore on... | Works? |
|---|---|---|
| macOS (Harold's Mac) | Linux (Proxmox CT) | ✅ Yes — containers are OS-agnostic |
| Linux (Pink) | macOS (new Mac) | ✅ Yes |
| macOS | Windows (WSL2) | ✅ Yes |
| Any → Any | ✅ | Volume data is just files. Container images are portable. |

---

## Service Discovery

### Option A: Docker DNS (Simple)
- All containers on same Docker network
- Reference by container name: `http://bridge:18802`, `http://ollama:11434`
- No hardcoded IPs ever again

### Option B: Consul/mDNS (Advanced)
- Service registration on startup
- Health-checked discovery
- Cross-host discovery (for multi-machine swarm)

**Recommendation:** Start with Docker DNS. Migrate to Consul when we go multi-host.

---

## Network Architecture

```yaml
networks:
  swarm:        # Agent-to-agent + bridge communication
    driver: bridge
  brain:        # CortexBrain internal services
    driver: bridge  
  services:     # Infrastructure (Redis, Ollama, Registry)
    driver: bridge
  public:       # Exposed services (Monitor, Dashboard, Avatar)
    driver: bridge
```

Agents on `swarm` network can talk to each other and the bridge.
CortexBrain services on `brain` network are isolated.
Only `public` network services are exposed to host.

---

## GPU Strategy

```yaml
# Pink (RTX 3090) — GPU-enabled containers
services:
  ollama:
    image: ollama/ollama:latest
    deploy:
      resources:
        reservations:
          devices:
            - driver: nvidia
              count: 1
              capabilities: [gpu]
    volumes:
      - /data/shared/models:/root/.ollama

  pink:
    image: cortex/agent-gpu:latest
    deploy:
      resources:
        reservations:
          devices:
            - driver: nvidia
              count: 1
              capabilities: [gpu]
```

---

## Migration Plan

### Phase 1: Containerize Core Services (Week 1)
- [ ] Build `cortex/agent:latest` Docker image (OpenClaw + standard config)
- [ ] Containerize A2A Bridge (`cortex/bridge:latest`)
- [ ] Containerize Ollama with GPU passthrough
- [ ] Set up Docker Compose for core services
- [ ] Test: Bridge + Ollama + one agent container

### Phase 2: Migrate Swarm Agents (Week 2)
- [ ] Export each agent's workspace/config/sessions to volume structure
- [ ] Create per-agent Docker Compose entries
- [ ] Stand up Harold as first containerized agent
- [ ] Verify bridge communication works container-to-container
- [ ] Migrate Pink (with GPU), Red, Skippy, Kentaro
- [ ] Migrate CCP nodes and Health Ring

### Phase 3: Control Plane (Week 3)
- [ ] Deploy BotMaker (or custom control plane) 
- [ ] API management for all agents (start/stop/restart/logs)
- [ ] Web dashboard for monitoring
- [ ] Automated health checks + restart policies
- [ ] Alert integration (notify Albert/Norman on failures)

### Phase 4: CortexBrain Services (Week 4)
- [ ] Containerize CortexBrain binary
- [ ] Containerize Neural Bus Observer
- [ ] Containerize Thinking Monitor
- [ ] Docker Compose for full brain stack
- [ ] Test: Monitor → Observer → Brain pipeline in containers

### Phase 5: Backup & Portability (Week 5)
- [ ] Automated nightly backups
- [ ] Test: backup on Mac → restore on Linux
- [ ] Test: backup on Linux → restore on Mac
- [ ] Disaster recovery runbook
- [ ] CI/CD pipeline for image builds

### Phase 6: Future Projects Template (Ongoing)
- [ ] Document the container framework as the standard for all new projects
- [ ] Template Docker Compose for new agents
- [ ] Template Dockerfile for new services
- [ ] Standard volume layout for new projects
- [ ] Integration guide: how to add a new service to the swarm

---

## Harold's Role

Harold becomes the **container orchestrator** — not just a coordinator, but the active manager of the swarm via API:

| Harold Today | Harold Containerized |
|---|---|
| SSHs into machines | Calls Docker/BotMaker API |
| Checks if process is running | Reads container health status |
| Manually restarts agents | API call: restart container |
| Can't fix "binary missing" | Pulls latest image, redeploys |
| Blind to resource usage | Reads CPU/memory/network stats |
| Can't spin up new agents | Creates new container in seconds |

---

## Success Criteria

- [ ] `docker compose up -d` starts the entire swarm from scratch
- [ ] Any agent can be stopped, its container deleted, and restarted without data loss
- [ ] Full swarm backup fits in a single tar file
- [ ] Backup restores on a fresh Linux machine with only Docker installed
- [ ] Harold can manage all agents via API without SSH
- [ ] Crashed agents auto-restart within 30 seconds
- [ ] New agent deployment takes <2 minutes via API
- [ ] GPU workloads (Ollama, Avatar) work in containers on Pink

---

## Open Questions for Norman

1. **BotMaker vs. Custom vs. Portainer?** — BotMaker is OpenClaw-specific, Portainer is general-purpose Docker UI, or we build custom. Recommendation: Start with BotMaker, evaluate.
2. **Single host vs. multi-host?** — Do we keep agents distributed across machines, or consolidate to fewer hosts? Docker Compose = single host. Docker Swarm/K3s = multi-host.
3. **Where does the control plane run?** — Harold's Mac? A dedicated Proxmox CT? Pink?
4. **Image registry?** — Local registry on the network, or GitHub Container Registry (ghcr.io)?
5. **Timeline pressure?** — This is 5 weeks of work. Parallel with other projects or full focus?

---

*"The container is the universal deployment unit. Everything that runs should run in one. Everything that persists should be in a volume. Everything that can crash should auto-restart."*
