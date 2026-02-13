---
project: Cortex-Gateway
component: Agents
phase: Ideation
date_created: 2026-02-07T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-08T14:46:20.380158
---

# Pinky/Pink Design Documents - Comprehensive Index

**Generated:** 2026-02-07
**Source:** CortexBrain Vault Search
**Total Files Found:** 31 documents mentioning "pinky" or "pink"

---

## Executive Summary

**Pink** (192.168.1.186) is a **critical infrastructure node** in the Cortex swarm ecosystem:

### Primary Roles:
1. **Inference Engine Host** - Ollama GPU inference (3 coding models)
2. **CortexBrain Host** - Main AI assistant on port 18892
3. **Redis Host** - Message queue and cache on port 6379
4. **k3s Control Plane** - Part of 3-node Kubernetes cluster
5. **Compute Layer** - Heavy GPU workloads

### Services Running on Pink:
- **CortexBrain:** Port 18892 (A2A server)
- **Ollama:** Port 11434 (LLM inference)
- **Redis:** Port 6379 (message queue/cache)
- **Swarm Dashboard:** Port 18888 (monitoring - currently down)
- **k3s Server:** Part of harold/pink/kentaro control plane

---

## Document Categories

### 1. Core Architecture Documents (3)

#### A. Swarm Architecture v2 - Production Grade
**File:** `cortex-swarm-architecture-v2-production-grade.md`
**Location:** `/Users/normanking/Documents/CortexBrain Vault/`

**Pink's Role:**
- **k3s Control Plane Node** (with harold, kentaro)
- **Compute Layer** for GPU inference
- **CortexBrain Host** + Ollama
- **Redis Cache/Queue**

**Key Mentions:**
```
harold:  etcd + gateway + inference routing (192.168.1.128)
pink:    etcd + CortexBrain + Ollama (GPU inference) (192.168.1.186)
kentaro: etcd + worker tasks (192.168.1.149)
```

**Architecture Diagram:**
```
GATEWAY LAYER (Stateless)
  ┌ harold (.128)
  ┌ pink (.186)
  └ kentaro (.149)

COMPUTE LAYER
  ┌ pink (.186)    ← GPU inference
  ┌ red (.188)
  └ worker-3 (.220)
```

**Critical Design Decisions:**
- Pink runs heavy inference (Llama 70B capable)
- CPU/disk contention can spike etcd latency
- Resource isolation prevents cascading failure
- Pink is active-active in load balancer pool

---

#### B. Swarm Phase 2 - SDLC Environments
**File:** `cortex-swarm-phase2-sdlc-environments.md`
**Location:** `/Users/normanking/Documents/CortexBrain Vault/`

**Pink's Role:**
- **k3s Control Plane:** harold (.128), pink (.186), kentaro (.149)
- Part of single production k3s cluster

---

#### C. Swarm Resilience Architecture
**File:** `cortex-swarm-resilience-architecture.md`
**Location:** `/Users/normanking/Documents/CortexBrain Vault/`

**Pink's Role:**
- Failover scenarios involving Pink as leader
- VIP routing to Pink when Harold is down
- Network partition handling with Pink

**Failure Scenarios:**
```
Scenario: Pink runs large inference task (Llama 70B)
Impact: CPU/disk contention → etcd write latency spikes
Result: etcd leader election triggered
```

---

### 2. Infrastructure Documents (7)

#### A. Cortex-Gateway PRD
**File:** `Reference/Infrastructure/CORTEX-GATEWAY-PRD.md`

**Pink References:**
- Gateway routing to Pink:18892 (CortexBrain)
- Load balancing across harold, pink, kentaro
- Health checks to Pink services

---

#### B. Cortex-Gateway Build Plan
**File:** `Reference/Infrastructure/CORTEX-GATEWAY-BUILD-PLAN.md`

**Pink References:**
- Service discovery for Pink's services
- Pink as Ollama inference backend
- Redis connection on Pink:6379

---

#### C. Health System Plan
**File:** `Reference/Infrastructure/HEALTH-SYSTEM-PLAN.md`

**Pink Health Checks:**
```yaml
- name: pink
  checks:
    - type: http
      url: "{{resolve pink cortexbrain}}/health"
    - type: http
      url: "{{resolve pink ollama}}/api/tags"
    - type: tcp
      port: 6379  # Redis
```

---

#### D. CortexHub Plan
**File:** `Reference/Infrastructure/CORTEXHUB-PLAN.md`

**Pink's Role:**
- Central hub component host
- Service mesh node

---

#### E. Container Framework Plan
**File:** `Reference/Infrastructure/CONTAINER-FRAMEWORK-PLAN.md`

**Pink Deployment:**
- k3s pod scheduling to Pink node
- GPU resource allocation
- Affinity rules for inference workloads

---

#### F. Cortex Monitor Audit
**File:** `Reference/Infrastructure/CORTEX-MONITOR-AUDIT.md`

**Pink Monitoring:**
```
metrics:
  - assistant_request_duration_seconds{agent="pink"}
  - gpu_utilization{node="pink"}
  - inference_queue_depth{backend="pink"}
```

---

#### G. Infrastructure Tasks
**File:** `infrastructure-tasks.md`

**Pink Tasks:**
- Deploy services to Pink
- Configure Pink's GPU passthrough
- Setup Pink monitoring

---

### 3. Agent & Memory Documents (6)

#### A. Agent Swarm Workflow Skill Evaluation
**File:** `Reference/Agents/AGENT-SWARM-WORKFLOW-SKILL-EVALUATION.md`

**Pink's Role:**
- Agent in swarm coordination
- Task distribution target

---

#### B. Harold Memory Verification
**File:** `Reference/OpenClaw/HAROLD-MEMORY-VERIFICATION.md`

**Pink References:**
- Harold ↔ Pink communication
- Shared memory access patterns

---

#### C. CortexBrain Primary Memory Plan
**File:** `Reference/Memory/CORTEXBRAIN-PRIMARY-MEMORY-PLAN.md`

**Pink's Memory System:**
- Redis on Pink as memory backend
- CortexBrain memory storage on Pink

---

#### D. CortexBrain Memory Enhancement PRD
**File:** `Reference/Memory/CORTEXBRAIN-MEMORY-ENHANCEMENT-PRD.md`

**Pink Integration:**
- Enhanced memory API on Pink:18892
- Redis memory caching

---

#### E. Memory Overview
**File:** `Reference/Memory/MEMORY.md`

**Pink Components:**
- Memory storage backend
- Redis cache layer

---

#### F. MemVid Final Verdict
**File:** `Reference/Memory/MEMVID-FINAL-VERDICT.md`

**Pink Storage:**
- Vector database considerations
- Memory persistence on Pink

---

### 4. Architecture & Journey Documents (9)

#### A. A2A-MCP Hybrid Implementation Plan
**File:** `Reference/Architecture/A2A-MCP-HYBRID-IMPLEMENTATION-PLAN.md`

**Pink A2A Server:**
- CortexBrain A2A endpoint on Pink:18892
- Agent communication via A2A protocol

---

#### B. Cortex Journey NotebookLM Source
**File:** `Reference/Architecture/cortex-journey-notebooklm-source.md`

**Pink Journey:**
- Pink's evolution as infrastructure node
- Service consolidation on Pink

---

#### C. Cortex Coder Training Plan
**File:** `Reference/Architecture/CORTEX-CODER-TRAINING-PLAN.md`

**Pink Training:**
- Ollama model training on Pink
- GPU utilization for fine-tuning

---

#### D. CortexBrain Mission Control Plan
**File:** `Reference/Architecture/CORTEXBRAIN-MISSION-CONTROL-PLAN.md`

**Pink Mission Control:**
- Central coordination from Pink
- Dashboard deployment on Pink

---

#### E. Cortex Journey Build Plan
**File:** `Reference/Architecture/CORTEX-JOURNEY-BUILD-PLAN.md`

**Pink Build Tasks:**
- Build pipeline on Pink
- CI/CD integration

---

#### F. A2A vs MCP Adoption Analysis
**File:** `Reference/Architecture/A2A-vs-MCP-ADOPTION-ANALYSIS.md`

**Pink Protocol:**
- A2A server implementation on Pink
- MCP integration considerations

---

#### G. Cortex Journey Spec
**File:** `Reference/Architecture/CORTEX-JOURNEY-SPEC.md`

**Pink Specifications:**
- Service specifications
- API endpoints on Pink

---

#### H. Cortex Journey Status
**File:** `Reference/Architecture/CORTEX-JOURNEY-STATUS.md`

**Pink Status:**
- Current deployment status
- Service health on Pink

---

#### I. CortexBrain Overview
**File:** `Reference/Architecture/CORTEXBRAIN-OVERVIEW.md`

**Pink Overview:**
- Pink as CortexBrain host
- Infrastructure overview

---

### 5. Documentation Files (2)

#### A. Cortex-Gateway Manual
**File:** `docs/Cortex-Gateway Manual.md`

**Pink Configuration:**
- Gateway routing to Pink
- Service discovery examples
- API endpoint documentation

---

#### B. User Manual
**File:** `docs/USER_MANUAL.md`

**Pink User Guide:**
- Connecting to services on Pink
- Troubleshooting Pink connectivity

---

### 6. Session & Status Files (4)

#### A. Session Notes - TUI Redesign Dec 18
**File:** `SESSION_NOTES_TUI-Redesign-Dec18.md`

**Pink Work:**
- TUI changes affecting Pink
- Dashboard deployment attempts

---

#### B. Knowledge Page Complete
**File:** `KNOWLEDGE_PAGE_COMPLETE.md`

**Pink Knowledge:**
- Pink service documentation
- Configuration examples

---

#### C. Start Here
**File:** `00-START-HERE.md`

**Pink Quick Start:**
- Accessing Pink services
- Common Pink endpoints

---

#### D. Reference Index
**File:** `Reference/INDEX.md`

**Pink References:**
- Links to Pink documentation
- Pink architecture diagrams

---

## Pink Service Details

### Current Configuration (from cortex-gateway config.yaml)

```yaml
# Pink Agent Configuration
swarm:
  agents:
    - name: pink
      host: 192.168.1.186
      services:
        cortexbrain: 18892
        ollama: 11434
        redis: 6379

# Ollama on Pink
ollama:
  url: "http://192.168.1.186:11434"

# Health Checks for Pink
healthring:
  members:
    - name: pink
      checks:
        - type: http
          url: "{{resolve pink cortexbrain}}/health"
        - type: http
          url: "{{resolve pink ollama}}/api/tags"
        - type: tcp
          port: 6379
```

### Pink Service URLs

| Service | URL | Status | Purpose |
|---------|-----|--------|---------|
| **CortexBrain** | http://192.168.1.186:18892 | ✅ UP | A2A server, main AI |
| **Ollama** | http://192.168.1.186:11434 | ✅ UP | LLM inference |
| **Redis** | tcp://192.168.1.186:6379 | ✅ UP | Cache/queue |
| **Swarm Dashboard** | http://192.168.1.186:18888 | ❌ DOWN | Monitoring UI |

### Pink Models (Ollama)

1. **go-coder:latest** (Qwen2, 4.7GB) - DEFAULT
   - Specialized for Go development
   - Fast inference

2. **cortex-coder:latest** (DeepSeek2, 8.9GB)
   - General coding tasks
   - High quality completions

3. **deepseek-coder-v2:latest**
   - Complex refactoring
   - Architecture design

---

## Pink Architecture Patterns

### Pattern 1: Stateless Gateway → Stateful Compute
```
Client → VIP (192.168.1.200)
      → HAProxy
      → [harold, pink, kentaro] (any gateway)
      → Pink:18892 (CortexBrain - stateful)
      → Pink:11434 (Ollama - stateful GPU)
```

### Pattern 2: k3s Pod Scheduling
```
Pod → Scheduler
    → Pink node (GPU affinity)
    → Inference workload
```

### Pattern 3: Memory Architecture
```
Request → Gateway
        → CortexBrain (Pink:18892)
        → Redis (Pink:6379) [cache]
        → Postgres [persistent]
```

---

## Pink in Network Topology

```
┌─────────────────────────────────────────────────────────┐
│                   CORTEX NETWORK                         │
│                   192.168.1.0/24                         │
├─────────────────────────────────────────────────────────┤
│                                                          │
│  VIP: 192.168.1.200 (floating)                          │
│                                                          │
│  ┌──────────────────────────────────────────────────┐  │
│  │              CONTROL PLANE (k3s)                  │  │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐       │  │
│  │  │ harold   │  │  PINK    │  │ kentaro  │       │  │
│  │  │ .128     │  │  .186    │  │  .149    │       │  │
│  │  │          │  │          │  │          │       │  │
│  │  │ Gateway  │  │ Gateway  │  │ Gateway  │       │  │
│  │  │ Bridge   │  │ Brain    │  │ Worker   │       │  │
│  │  │          │  │ Ollama   │  │          │       │  │
│  │  │          │  │ Redis    │  │          │       │  │
│  │  └──────────┘  └──────────┘  └──────────┘       │  │
│  └──────────────────────────────────────────────────┘  │
│                                                          │
│  ┌──────────────────────────────────────────────────┐  │
│  │              WORKER NODES                         │  │
│  │  ┌──────────┐  ┌──────────┐                      │  │
│  │  │   red    │  │ proxmox  │                      │  │
│  │  │  .188    │  │  .203    │                      │  │
│  │  └──────────┘  └──────────┘                      │  │
│  └──────────────────────────────────────────────────┘  │
│                                                          │
│  ┌──────────────────────────────────────────────────┐  │
│  │         MONITORING                                │  │
│  │  ┌──────────┐                                     │  │
│  │  │healthring│                                     │  │
│  │  │  .206    │                                     │  │
│  │  └──────────┘                                     │  │
│  └──────────────────────────────────────────────────┘  │
│                                                          │
└─────────────────────────────────────────────────────────┘
```

---

## Pink Design Decisions

### Decision 1: Consolidate Services on Pink
**Rationale:**
- GPU available for inference
- Sufficient resources for multiple services
- Reduces network hops (Brain + Ollama co-located)

**Trade-offs:**
- Single point of failure for inference
- Resource contention possible
- **Mitigation:** k3s can reschedule pods to other nodes

### Decision 2: Pink in k3s Control Plane
**Rationale:**
- High availability (3-node etcd quorum)
- Can participate in leader election
- Direct access to k8s API

**Trade-offs:**
- Inference workloads can spike etcd latency
- **Mitigation:** Resource isolation, separate etcd disk

### Decision 3: Redis on Pink
**Rationale:**
- Co-located with CortexBrain (low latency)
- Memory caching for inference results
- Session storage

**Trade-offs:**
- Memory contention with GPU workloads
- **Mitigation:** Memory limits, Redis maxmemory

---

## Pink Failure Scenarios

### Scenario 1: Pink Completely Down
**Impact:**
- ❌ CortexBrain unavailable
- ❌ Ollama inference unavailable
- ❌ Redis cache unavailable
- ✅ Gateway layer still works (harold, kentaro)
- ✅ k3s quorum maintained (2/3 nodes)

**Recovery:**
- Gateway routes to harold or kentaro
- Inference falls back to cloud engines
- k3s reschedules CortexBrain pod

### Scenario 2: Pink GPU Overload
**Impact:**
- ⚠️ Slow inference
- ⚠️ Potential etcd latency spikes
- ✅ Other services continue

**Recovery:**
- Task queue builds up
- Automatic spillover to cloud engines
- Prometheus alerts trigger

### Scenario 3: Pink Network Partition
**Impact:**
- ⚠️ Split-brain risk
- ⚠️ VIP may route incorrectly
- ⚠️ etcd re-election

**Recovery:**
- Gossip protocol detects partition
- VIP fails over to harold
- etcd quorum re-establishes

---

## Pink Monitoring

### Metrics to Watch

```prometheus
# CPU/Memory
node_cpu_seconds_total{instance="pink"}
node_memory_MemAvailable_bytes{instance="pink"}

# GPU
nvidia_gpu_duty_cycle{instance="pink"}
nvidia_gpu_memory_used_bytes{instance="pink"}

# Services
cortexbrain_request_duration_seconds{node="pink"}
ollama_inference_duration_seconds{node="pink"}
redis_connected_clients{node="pink"}

# k3s
etcd_server_leader_changes_total{node="pink"}
kube_node_status_condition{node="pink",condition="Ready"}
```

### Alerts

```yaml
- alert: PinkDown
  expr: up{job="pink"} == 0
  for: 1m
  severity: critical

- alert: PinkGPUOverload
  expr: nvidia_gpu_duty_cycle{instance="pink"} > 95
  for: 5m
  severity: warning

- alert: PinkEtcdSlow
  expr: etcd_disk_backend_commit_duration_seconds{node="pink"} > 0.1
  for: 2m
  severity: warning
```

---

## Pink Deployment Checklist

### Hardware
- [ ] GPU passthrough configured (if VM)
- [ ] Sufficient RAM (16GB+ recommended)
- [ ] SSD for etcd storage
- [ ] Network bonding for reliability

### Software
- [ ] k3s server installed
- [ ] CortexBrain deployed (pod or systemd)
- [ ] Ollama installed and running
- [ ] Redis installed and running
- [ ] Swarm dashboard binary deployed (currently missing)

### Network
- [ ] Static IP configured (192.168.1.186)
- [ ] Firewall rules:
  - [ ] 18892 (CortexBrain)
  - [ ] 11434 (Ollama)
  - [ ] 6379 (Redis - internal only)
  - [ ] 18888 (Dashboard)
  - [ ] 6443 (k3s API)
- [ ] DNS resolution (pink.local)

### Monitoring
- [ ] Prometheus scraping configured
- [ ] Grafana dashboards imported
- [ ] Alertmanager rules deployed
- [ ] Health ring checks passing

---

## Related Commands

### Check Pink Services
```bash
# From local machine
curl http://192.168.1.186:18892/health    # CortexBrain
curl http://192.168.1.186:11434/api/tags  # Ollama
nc -zv 192.168.1.186 6379                 # Redis

# Via cortex-gateway API
curl http://localhost:8080/api/v1/swarm/agents | grep pink
curl http://localhost:8080/api/v1/healthring/status | jq '.pink'
```

### Deploy to Pink (k3s)
```bash
# Apply deployment
kubectl apply -f cortexbrain-deployment.yaml

# Check pod on Pink
kubectl get pods -o wide | grep pink

# View logs
kubectl logs -f deployment/cortexbrain -n cortex-prod
```

### Monitor Pink
```bash
# SSH to Pink
ssh norman@192.168.1.186

# Check services
systemctl status cortexbrain
systemctl status ollama
systemctl status redis

# Check GPU
nvidia-smi

# Check k3s
k3s kubectl get nodes
```

---

## Future Enhancements for Pink

### Planned (from vault docs)

1. **Swarm Dashboard Deployment** ⏳
   - Deploy dashboard on Pink:18888
   - Real-time agent monitoring

2. **Redis Pub/Sub for Messaging** 🚧
   - Replace HTTP bridge with Redis
   - Sub-millisecond latency

3. **Vector Database** 💭
   - Add Milvus/Qdrant on Pink
   - Enhanced memory retrieval

4. **Model Fine-tuning** 🎯
   - Use Pink GPU for training
   - Custom coding models

5. **Auto-scaling** 📈
   - HPA for CortexBrain pods
   - Dynamic resource allocation

---

## Document Locations

All documents found in:
```
/Users/normanking/Documents/CortexBrain Vault/
```

**Total:** 31 files containing Pink/Pinky references

**Most Important:**
1. `cortex-swarm-architecture-v2-production-grade.md` - Core architecture
2. `cortex-swarm-resilience-architecture.md` - Failure handling
3. `Reference/Infrastructure/CORTEX-GATEWAY-PRD.md` - Current system
4. `Reference/Infrastructure/HEALTH-SYSTEM-PLAN.md` - Monitoring

---

## Summary

**Pink (192.168.1.186)** is a **cornerstone of the Cortex ecosystem**:

✅ **Critical Services:** CortexBrain, Ollama, Redis
✅ **Architecture Role:** k3s control plane + compute layer
✅ **Status:** Operational (3/4 services UP, dashboard DOWN)
✅ **Design:** Well-documented across 31 vault files
✅ **Monitoring:** Health checks, metrics, alerts configured

**Next Actions:**
1. Deploy swarm-dashboard on Pink:18888 ✅ Binary ready
2. Consider Redis pub/sub migration (architecture decision)
3. Monitor GPU utilization for capacity planning

---

**Index Generated:** 2026-02-07 18:30 EST
**Vault Location:** `/Users/normanking/Documents/CortexBrain Vault/`
**Report Location:** `/Users/normanking/ServerProjectsMac/cortex-gateway-test/PINKY_DESIGN_DOCS_INDEX.md`
