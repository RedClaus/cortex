---
project: Cortex-Gateway
component: Docs
phase: Production
date_created: 2026-02-02T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:21:11.263624
---

# Deploying Cortex-Gateway

## Prerequisites
- Docker Engine 24+ and Docker Compose v2
- Ollama running on host (for local inference)

## Quick Start
```bash
docker compose up -d
```

## First Run (Onboarding)
If no config.yaml exists, cortex-gateway enters onboarding mode:
```bash
# Check onboarding status
curl http://localhost:18800/api/v1/onboarding/status

# Import existing swarm config
curl -X POST http://localhost:18800/api/v1/onboarding/import \
  -H "Content-Type: application/json" \
  -d @my-swarm-config.json
```

## With Existing Config
```bash
cp your-config.yaml config.yaml
docker compose up -d
```

## GPU Access (Ollama on Host)
Ollama runs on the host, not inside Docker. The compose file uses
`host.docker.internal` to reach it (works on macOS/Windows automatically).
On Linux, the `extra_hosts` directive handles this.

## Verify
```bash
# Health check
curl http://localhost:18800/health

# List detected inference engines
curl http://localhost:18800/api/v1/inference/engines

# List swarm agents
curl http://localhost:18800/api/v1/swarm/agents

# Health ring status
curl http://localhost:18800/api/v1/healthring/status
```

## Backup
```bash
# Stop services
docker compose stop

# Backup volumes + config
docker run --rm -v cortex-gateway_gateway-data:/data -v $(pwd):/backup alpine \
  tar czf /backup/gateway-backup.tar.gz -C /data .
docker run --rm -v cortex-gateway_cortexbrain-data:/data -v $(pwd):/backup alpine \
  tar czf /backup/cortexbrain-backup.tar.gz -C /data .
cp config.yaml config.yaml.backup

# Restart
docker compose up -d
```

## Restore
```bash
docker run --rm -v cortex-gateway_gateway-data:/data -v $(pwd):/backup alpine \
  tar xzf /backup/gateway-backup.tar.gz -C /data
docker run --rm -v cortex-gateway_cortexbrain-data:/data -v $(pwd):/backup alpine \
  tar xzf /backup/cortexbrain-backup.tar.gz -C /data
docker compose up -d
```

## Environment Variables
| Variable | Default | Description |
|----------|---------|-------------|
| GATEWAY_PORT | 18800 | Gateway API port |
| CORTEXBRAIN_URL | http://cortexbrain:18892 | CortexBrain endpoint |
| OLLAMA_URL | http://host.docker.internal:11434 | Ollama endpoint |
| TELEGRAM_TOKEN | - | Telegram bot token |
| DISCORD_TOKEN | - | Discord bot token |
