package messaging

import (
	"context"
	"log"
	"time"
)

// HeartbeatManager handles sending and receiving heartbeats via Redis Streams
type HeartbeatManager struct {
	client    *RedisClient
	agentName string
	stopCh    chan struct{}
}

// NewHeartbeatManager creates a new heartbeat manager
func NewHeartbeatManager(client *RedisClient, agentName string) *HeartbeatManager {
	return &HeartbeatManager{
		client:    client,
		agentName: agentName,
		stopCh:    make(chan struct{}),
	}
}

// StartHeartbeatLoop starts sending periodic heartbeats
func (h *HeartbeatManager) StartHeartbeatLoop(ctx context.Context, interval time.Duration, status string, metadata map[string]interface{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Send initial heartbeat
	h.SendHeartbeat(ctx, status, metadata)

	for {
		select {
		case <-ctx.Done():
			log.Printf("Heartbeat loop stopping for agent %s", h.agentName)
			return
		case <-h.stopCh:
			log.Printf("Heartbeat loop stopped for agent %s", h.agentName)
			return
		case <-ticker.C:
			if err := h.SendHeartbeat(ctx, status, metadata); err != nil {
				log.Printf("Failed to send heartbeat: %v", err)
			}
		}
	}
}

// Stop stops the heartbeat loop
func (h *HeartbeatManager) Stop() {
	close(h.stopCh)
}

// SendHeartbeat sends a single heartbeat to Redis
func (h *HeartbeatManager) SendHeartbeat(ctx context.Context, status string, metadata map[string]interface{}) error {
	hb := HeartbeatMessage{
		Agent:     h.agentName,
		Status:    status,
		Timestamp: time.Now().Unix(),
		Metadata:  metadata,
	}

	stream := HeartbeatStreamName()
	values := hb.ToRedisValues()

	_, err := h.client.Publish(ctx, stream, values)
	if err != nil {
		return err
	}

	return nil
}

// SubscribeToHeartbeats subscribes to heartbeats from all agents
func (h *HeartbeatManager) SubscribeToHeartbeats(ctx context.Context) (<-chan *HeartbeatMessage, error) {
	msgChan := make(chan *HeartbeatMessage, 100)

	stream := HeartbeatStreamName()
	group := ConsumerGroupAgents
	consumer := h.agentName + "-hb-consumer"

	redisChan, err := h.client.Subscribe(ctx, stream, group, consumer)
	if err != nil {
		return nil, err
	}

	go func() {
		defer close(msgChan)
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-redisChan:
				if !ok {
					return
				}

				hb, err := HeartbeatFromRedisValues(msg.Values)
				if err != nil {
					log.Printf("Failed to parse heartbeat: %v", err)
					continue
				}

				msgChan <- hb
			}
		}
	}()

	return msgChan, nil
}

// GetAgentHealth returns the last known status of an agent
// This is a simplified version - in production you'd want to track this in memory
func (h *HeartbeatManager) GetAgentHealth(ctx context.Context, agentName string) (*HeartbeatMessage, error) {
	// Read recent heartbeats from the stream
	rdb := h.client.RawClient()

	results, err := rdb.XRevRangeN(ctx, HeartbeatStreamName(), "+", "-", 100).Result()
	if err != nil {
		return nil, err
	}

	// Find the most recent heartbeat for the specified agent
	for _, msg := range results {
		if agent, ok := msg.Values["agent"].(string); ok && agent == agentName {
			return HeartbeatFromRedisValues(msg.Values)
		}
	}

	return nil, nil // Agent not found
}
