---
project: Cortex-Gateway
component: Agents
phase: Ideation
date_created: 2026-02-10T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-08T12:13:22.267783
---

# Redis Streams Migration Documentation

## Overview

This document describes the migration from HTTP A2A Bridge to Redis Streams for agent messaging in the Cortex-Gateway and CortexBrain systems.

## Architecture Changes

### Before (HTTP Bridge)
```
┌─────────────┐     HTTP POST     ┌─────────────┐
│   Harold    │ ────────────────> │ A2A Bridge  │
│  (Agent)    │                   │  :18802     │
└─────────────┘                   └─────────────┘
                                        │
                                        │ HTTP
                                        ▼
                                  ┌─────────────┐
                                  │   Agents    │
                                  └─────────────┘
```

### After (Redis Streams)
```
┌─────────────┐     XADD          ┌─────────────┐
│   Harold    │ ────────────────> │   Redis     │
│  (Agent)    │                   │  Streams    │
└─────────────┘                   └─────────────┘
                                        │
                                        │ XREADGROUP
                                        ▼
                                  ┌─────────────┐
                                  │   Agents    │
                                  └─────────────┘
```

## Key Improvements

1. **Latency**: Reduced from ~30s (HTTP polling) to <10ms (Redis Streams)
2. **Reliability**: Zero message loss with persistent streams
3. **Scalability**: Consumer groups enable load balancing
4. **Priority**: Native priority queues (critical > high > normal > low)
5. **Observability**: Built-in stream introspection and replay

## Redis Streams Structure

### Streams
- `cortex:tasks:critical` - Highest priority tasks (immediate processing)
- `cortex:tasks:high` - High priority tasks
- `cortex:tasks:normal` - Normal priority tasks
- `cortex:tasks:low` - Background tasks
- `cortex:heartbeats` - Agent health and status
- `cortex:tasks:dlq` - Dead Letter Queue for failed tasks
- `cortex:messages:{agent_name}` - Direct messages to specific agents

### Consumer Groups
- `agents` - General agent consumers
- `workers` - Worker node consumers
- `harold` - Harold primary consumer

## Message Format

### Task Message
```json
{
  "id": "msg_1234567890_1",
  "from": "harold",
  "to": "pink",
  "priority": "high",
  "type": "coding",
  "payload": {
    "description": "Implement feature X",
    "deadline": "2026-02-10T12:00:00Z"
  },
  "created": 1704556800
}
```

### Heartbeat Message
```json
{
  "agent": "pink",
  "status": "healthy",
  "timestamp": 1704556800,
  "metadata": {
    "cpu": 45,
    "memory": 60,
    "active_tasks": 2
  }
}
```

## Configuration

### config.yaml
```yaml
messaging:
  enabled: true
  redis_addr: "192.168.1.186:6379"
  redis_password: ""
  redis_db: 0
  use_redis: true
  heartbeat_interval: "30s"
```

### Environment Variables
- `GATEWAY_MESSAGING_ENABLED` - Enable Redis messaging
- `GATEWAY_REDIS_ADDR` - Redis server address
- `GATEWAY_USE_REDIS` - Use Redis instead of HTTP bridge

## Code Examples

### Publishing a Task
```go
bridge, _ := bridge.NewRedisBridge(bridge.RedisBridgeConfig{
    RedisAddr: "192.168.1.186:6379",
    AgentName: "harold",
})

taskID, err := bridge.PublishTask(ctx, bridge.TaskRequest{
    To:       "pink",
    Priority: messaging.PriorityHigh,
    Type:     messaging.TaskTypeCoding,
    Payload: map[string]interface{}{
        "description": "Implement feature X",
    },
})
```

### Subscribing to Tasks
```go
taskChan, _ := bridge.SubscribeTasks(ctx)

for task := range taskChan {
    fmt.Printf("Received task %s from %s\n", task.ID, task.From)
    // Process task...
}
```

### Sending Heartbeats
```go
hbMgr := messaging.NewHeartbeatManager(redisClient, "pink")
go hbMgr.StartHeartbeatLoop(ctx, 30*time.Second, "healthy", metadata)
```

## Migration Path

### Phase 1: Dual Mode (Current)
- Both HTTP bridge and Redis Streams run in parallel
- Configurable via `use_redis` flag
- Default: Redis enabled if available

### Phase 2: Redis Primary (Planned)
- Redis becomes the default
- HTTP bridge available as fallback
- Monitoring for issues

### Phase 3: HTTP Deprecated (Future)
- HTTP bridge disabled by default
- Redis Streams only
- Remove HTTP bridge code

## Monitoring

### Redis CLI Commands
```bash
# View stream length
redis-cli -h 192.168.1.186 XLEN cortex:tasks:high

# View pending messages
redis-cli -h 192.168.1.186 XPENDING cortex:tasks:high agents

# View consumer groups
redis-cli -h 192.168.1.186 XINFO GROUPS cortex:tasks:high

# View stream info
redis-cli -h 192.168.1.186 XINFO STREAM cortex:tasks:high

# Read recent messages
redis-cli -h 192.168.1.186 XREVRANGE cortex:tasks:high + - COUNT 10
```

### Health Checks
The gateway exposes health information at `/health` endpoint:
```json
{
  "status": "healthy",
  "redis_connected": true,
  "streams": {
    "cortex:tasks:critical": 10,
    "cortex:tasks:high": 45,
    "cortex:tasks:normal": 120,
    "cortex:tasks:low": 5
  }
}
```

## Error Handling

### Connection Failures
- Automatic retry with exponential backoff
- Fallback to HTTP bridge if configured
- Circuit breaker pattern for resilience

### Message Processing Failures
- Failed messages go to DLQ (`cortex:tasks:dlq`)
- Retry mechanism with configurable attempts
- Manual replay via API

### Dead Letter Queue
```go
dlq := messaging.NewDeadLetterQueue(redisClient)

// Get failed messages
letters, _ := dlq.GetDeadLetters(ctx, 10)

// Retry a specific message
_ = dlq.RetryDeadLetter(ctx, "1234567890-0")
```

## Testing

### Unit Tests
```bash
cd /Users/normanking/ServerProjectsMac/cortex-gateway-test
go test ./internal/messaging -v
```

### Integration Tests
```bash
go test ./internal/messaging -v -run Integration
```

### Bridge Tests
```bash
go test ./internal/bridge -v
```

## Troubleshooting

### Agent Not Receiving Messages
1. Check Redis connection: `redis-cli -h 192.168.1.186 PING`
2. Verify consumer group: `XINFO GROUPS cortex:tasks:high`
3. Check agent name matches subscription

### High Memory Usage
1. Trim old messages: `XTRIM cortex:tasks:normal MAXLEN 1000`
2. Set max length on publish
3. Monitor stream lengths

### Messages Not Being Acknowledged
1. Check pending messages: `XPENDING cortex:tasks:high agents`
2. View consumer info: `XINFO CONSUMERS cortex:tasks:high agents`
3. Claim stale messages if consumer died

## Files Changed

### New Files
- `internal/messaging/redis_client.go` - Redis client wrapper
- `internal/messaging/message.go` - Message structures
- `internal/messaging/priority_processor.go` - Priority queue processing
- `internal/messaging/heartbeat.go` - Heartbeat management
- `internal/messaging/dlq.go` - Dead Letter Queue
- `internal/bridge/redis_bridge.go` - Redis-based bridge

### Modified Files
- `internal/config/config.go` - Added messaging config
- `config.yaml` - Updated with messaging section
- `internal/bus/bus.go` - Updated to support Redis
- `internal/server/server.go` - Updated to use interface
- `cmd/cortex-gateway/main.go` - Updated initialization
- `go.mod` - Added go-redis dependency

## Rollback Procedure

If issues arise, revert to HTTP bridge:

1. Update config:
```yaml
messaging:
  enabled: false
  use_redis: false
```

2. Restart gateway

3. Monitor logs for bridge connectivity

## References

- [Redis Streams Documentation](https://redis.io/docs/data-types/streams/)
- [Go-Redis Client](https://github.com/redis/go-redis)
- [CortexBrain Bus Documentation](/Users/normanking/ServerProjectsMac/CortexBrain/internal/bus/)
