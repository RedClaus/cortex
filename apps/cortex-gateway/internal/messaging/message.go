package messaging

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// Priority levels for task routing
const (
	PriorityCritical = "critical"
	PriorityHigh     = "high"
	PriorityNormal   = "normal"
	PriorityLow      = "low"
)

// Task types for different kinds of work
const (
	TaskTypeCoding    = "coding"
	TaskTypeReview    = "review"
	TaskTypeDeploy    = "deploy"
	TaskTypeHeartbeat = "heartbeat"
	TaskTypeMessage   = "message"
	TaskTypeQuery     = "query"
)

// Consumer group names
const (
	ConsumerGroupAgents  = "agents"
	ConsumerGroupWorkers = "workers"
	ConsumerGroupHarold  = "harold"
)

// Stream names
const (
	StreamTasksCritical = "cortex:tasks:critical"
	StreamTasksHigh     = "cortex:tasks:high"
	StreamTasksNormal   = "cortex:tasks:normal"
	StreamTasksLow      = "cortex:tasks:low"
	StreamHeartbeats    = "cortex:heartbeats"
	StreamDLQ           = "cortex:tasks:dlq"
)

// TaskMessage represents a task sent between agents via Redis Streams
type TaskMessage struct {
	ID       string                 `json:"id"`
	From     string                 `json:"from"`
	To       string                 `json:"to"`
	Priority string                 `json:"priority"`
	Type     string                 `json:"type"`
	Payload  map[string]interface{} `json:"payload"`
	Created  int64                  `json:"created"`
}

// NewTaskMessage creates a new task message with generated ID and timestamp
func NewTaskMessage(from, to, priority, taskType string, payload map[string]interface{}) TaskMessage {
	return TaskMessage{
		ID:       generateMessageID(),
		From:     from,
		To:       to,
		Priority: priority,
		Type:     taskType,
		Payload:  payload,
		Created:  time.Now().Unix(),
	}
}

// Marshal converts TaskMessage to JSON bytes
func (m TaskMessage) Marshal() ([]byte, error) {
	return json.Marshal(m)
}

// ToRedisValues converts TaskMessage to Redis stream values map
func (m TaskMessage) ToRedisValues() map[string]interface{} {
	payloadJSON, _ := json.Marshal(m.Payload)

	return map[string]interface{}{
		"id":       m.ID,
		"from":     m.From,
		"to":       m.To,
		"priority": m.Priority,
		"type":     m.Type,
		"payload":  string(payloadJSON),
		"created":  strconv.FormatInt(m.Created, 10),
	}
}

// TaskMessageFromRedisValues creates TaskMessage from Redis stream values
func TaskMessageFromRedisValues(values map[string]interface{}) (*TaskMessage, error) {
	msg := &TaskMessage{}

	// Extract string fields
	if v, ok := values["id"].(string); ok {
		msg.ID = v
	}
	if v, ok := values["from"].(string); ok {
		msg.From = v
	}
	if v, ok := values["to"].(string); ok {
		msg.To = v
	}
	if v, ok := values["priority"].(string); ok {
		msg.Priority = v
	}
	if v, ok := values["type"].(string); ok {
		msg.Type = v
	}

	// Parse payload JSON
	if v, ok := values["payload"].(string); ok {
		var payload map[string]interface{}
		if err := json.Unmarshal([]byte(v), &payload); err != nil {
			return nil, fmt.Errorf("failed to unmarshal payload: %w", err)
		}
		msg.Payload = payload
	}

	// Parse created timestamp
	if v, ok := values["created"].(string); ok {
		created, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse created: %w", err)
		}
		msg.Created = created
	}

	return msg, nil
}

// StreamName returns the Redis stream name for a given priority and task type
func StreamName(priority, taskType string) string {
	// Priority-based streams
	switch priority {
	case PriorityCritical:
		return StreamTasksCritical
	case PriorityHigh:
		return StreamTasksHigh
	case PriorityNormal:
		return StreamTasksNormal
	case PriorityLow:
		return StreamTasksLow
	default:
		return StreamTasksNormal
	}
}

// HeartbeatStreamName returns the stream name for heartbeats
func HeartbeatStreamName() string {
	return StreamHeartbeats
}

// DeadLetterStreamName returns the stream name for failed messages
func DeadLetterStreamName() string {
	return StreamDLQ
}

// AgentMessageStreamName returns the stream name for direct agent messages
func AgentMessageStreamName(agentName string) string {
	return fmt.Sprintf("cortex:messages:%s", agentName)
}

// HeartbeatMessage represents an agent heartbeat
type HeartbeatMessage struct {
	Agent     string                 `json:"agent"`
	Status    string                 `json:"status"`
	Timestamp int64                  `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// ToRedisValues converts HeartbeatMessage to Redis stream values
func (h HeartbeatMessage) ToRedisValues() map[string]interface{} {
	metadataJSON, _ := json.Marshal(h.Metadata)
	return map[string]interface{}{
		"agent":     h.Agent,
		"status":    h.Status,
		"timestamp": strconv.FormatInt(h.Timestamp, 10),
		"metadata":  string(metadataJSON),
	}
}

// HeartbeatFromRedisValues creates HeartbeatMessage from Redis values
func HeartbeatFromRedisValues(values map[string]interface{}) (*HeartbeatMessage, error) {
	hb := &HeartbeatMessage{}

	if v, ok := values["agent"].(string); ok {
		hb.Agent = v
	}
	if v, ok := values["status"].(string); ok {
		hb.Status = v
	}
	if v, ok := values["timestamp"].(string); ok {
		ts, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, err
		}
		hb.Timestamp = ts
	}
	if v, ok := values["metadata"].(string); ok {
		json.Unmarshal([]byte(v), &hb.Metadata)
	}

	return hb, nil
}

var messageIDCounter uint64

func generateMessageID() string {
	messageIDCounter++
	return fmt.Sprintf("msg_%d_%d", time.Now().UnixNano(), messageIDCounter)
}
