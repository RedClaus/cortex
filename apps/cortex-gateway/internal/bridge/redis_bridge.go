package bridge

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/cortexhub/cortex-gateway/internal/messaging"
)

// RedisBridgeConfig configures the Redis bridge
type RedisBridgeConfig struct {
	RedisAddr string
	AgentName string
}

// TaskRequest represents a request to publish a task
type TaskRequest struct {
	To       string
	Priority string
	Type     string
	Payload  map[string]interface{}
}

// RedisBridge implements the A2A Bridge using Redis Streams
type RedisBridge struct {
	client       *messaging.RedisClient
	heartbeatMgr *messaging.HeartbeatManager
	agentName    string
	stopCh       chan struct{}
}

// NewRedisBridge creates a new Redis-based bridge client
func NewRedisBridge(cfg RedisBridgeConfig) (*RedisBridge, error) {
	redisClient, err := messaging.NewRedisClient(messaging.RedisConfig{
		Addr:     cfg.RedisAddr,
		Password: "",
		DB:       0,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create redis client: %w", err)
	}

	heartbeatMgr := messaging.NewHeartbeatManager(redisClient, cfg.AgentName)

	return &RedisBridge{
		client:       redisClient,
		heartbeatMgr: heartbeatMgr,
		agentName:    cfg.AgentName,
		stopCh:       make(chan struct{}),
	}, nil
}

// Start starts the bridge integration
func (b *RedisBridge) Start(ctx context.Context) error {
	// Start heartbeat loop
	go b.heartbeatMgr.StartHeartbeatLoop(ctx, 30*time.Second, "healthy", map[string]interface{}{
		"type":    "gateway",
		"version": "1.0.0",
	})

	log.Printf("Redis bridge started for agent %s", b.agentName)
	return nil
}

// Stop stops the bridge integration
func (b *RedisBridge) Stop() error {
	close(b.stopCh)
	b.heartbeatMgr.Stop()
	return b.client.Close()
}

// PublishTask publishes a task to the appropriate priority stream
func (b *RedisBridge) PublishTask(ctx context.Context, req TaskRequest) (string, error) {
	msg := messaging.NewTaskMessage(
		b.agentName,
		req.To,
		req.Priority,
		req.Type,
		req.Payload,
	)

	stream := messaging.StreamName(req.Priority, req.Type)
	values := msg.ToRedisValues()

	_, err := b.client.Publish(ctx, stream, values)
	if err != nil {
		return "", fmt.Errorf("failed to publish task: %w", err)
	}

	log.Printf("Published task %s to stream %s", msg.ID, stream)
	return msg.ID, nil
}

// SubscribeTasks subscribes to tasks for this agent across all priority streams
func (b *RedisBridge) SubscribeTasks(ctx context.Context) (<-chan *messaging.TaskMessage, error) {
	processor := messaging.NewPriorityProcessor(b.client, b.agentName)
	return processor.Start(ctx), nil
}

// SendMessage sends a direct message to another agent
func (b *RedisBridge) SendMessage(ctx context.Context, from, to, msgType, content string) error {
	// For direct messages, we use the agent's dedicated stream
	stream := messaging.AgentMessageStreamName(to)

	payload := map[string]interface{}{
		"content": content,
		"msgType": msgType,
	}

	msg := messaging.NewTaskMessage(from, to, messaging.PriorityNormal, messaging.TaskTypeMessage, payload)
	values := msg.ToRedisValues()

	_, err := b.client.Publish(ctx, stream, values)
	return err
}

// SendHeartbeat sends a single heartbeat
func (b *RedisBridge) SendHeartbeat(ctx context.Context, status string, metadata map[string]interface{}) error {
	return b.heartbeatMgr.SendHeartbeat(ctx, status, metadata)
}

// SubscribeToHeartbeats subscribes to heartbeats from all agents
func (b *RedisBridge) SubscribeToHeartbeats(ctx context.Context) (<-chan *messaging.HeartbeatMessage, error) {
	return b.heartbeatMgr.SubscribeToHeartbeats(ctx)
}

// IsConnected checks if the Redis connection is active
func (b *RedisBridge) IsConnected(ctx context.Context) bool {
	return b.client.IsConnected(ctx)
}

// GetRedisClient returns the underlying Redis client for advanced operations
func (b *RedisBridge) GetRedisClient() *messaging.RedisClient {
	return b.client
}
