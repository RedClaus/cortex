package messaging

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestClient creates a Redis client for testing
// Requires Redis server running at 192.168.1.186:6379
func setupTestClient(t *testing.T) *RedisClient {
	cfg := RedisConfig{
		Addr:     "192.168.1.186:6379",
		Password: "",
		DB:       0,
	}
	client, err := NewRedisClient(cfg)
	if err != nil {
		t.Skipf("Redis not available: %v", err)
	}
	return client
}

func TestRedisClient_Connection(t *testing.T) {
	client := setupTestClient(t)
	defer client.Close()

	ctx := context.Background()
	err := client.Ping(ctx)
	assert.NoError(t, err)
}

func TestRedisClient_PublishAndSubscribe(t *testing.T) {
	client := setupTestClient(t)
	defer client.Close()

	ctx := context.Background()
	stream := "test:messages:" + t.Name()
	group := "test-group"
	consumer := "test-consumer"

	// Cleanup after test
	defer client.RawClient().Del(ctx, stream)

	// Subscribe
	msgChan, err := client.Subscribe(ctx, stream, group, consumer)
	require.NoError(t, err)

	// Give subscription time to set up
	time.Sleep(100 * time.Millisecond)

	// Publish a message
	testData := map[string]interface{}{
		"test": "data",
		"num":  42,
	}
	msgID, err := client.Publish(ctx, stream, testData)
	require.NoError(t, err)
	assert.NotEmpty(t, msgID)

	// Receive message
	select {
	case msg := <-msgChan:
		assert.NotEmpty(t, msg.ID)
		assert.Equal(t, stream, msg.Stream)
		assert.Equal(t, "data", msg.Values["test"])
		assert.Equal(t, "42", msg.Values["num"]) // Redis stores as string
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for message")
	}
}

func TestTaskMessage_Marshal(t *testing.T) {
	msg := TaskMessage{
		ID:       "task-001",
		From:     "harold",
		To:       "pink",
		Priority: PriorityHigh,
		Type:     TaskTypeCoding,
		Payload: map[string]interface{}{
			"description": "Fix auth bug",
			"deadline":    "2026-02-06T12:00:00Z",
		},
		Created: time.Now().Unix(),
	}

	data, err := msg.Marshal()
	require.NoError(t, err)

	var unmarshaled TaskMessage
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, msg.ID, unmarshaled.ID)
	assert.Equal(t, msg.From, unmarshaled.From)
	assert.Equal(t, msg.To, unmarshaled.To)
	assert.Equal(t, msg.Priority, unmarshaled.Priority)
}

func TestTaskMessage_ToRedisValues(t *testing.T) {
	msg := TaskMessage{
		ID:       "task-001",
		From:     "harold",
		To:       "pink",
		Priority: PriorityHigh,
		Type:     TaskTypeCoding,
		Payload: map[string]interface{}{
			"description": "Fix auth bug",
		},
		Created: 1704556800,
	}

	values := msg.ToRedisValues()

	assert.Equal(t, "task-001", values["id"])
	assert.Equal(t, "harold", values["from"])
	assert.Equal(t, "pink", values["to"])
	assert.Equal(t, "high", values["priority"])
	assert.Equal(t, "coding", values["type"])
	assert.NotEmpty(t, values["payload"])
	assert.Equal(t, "1704556800", values["created"])
}

func TestTaskMessage_FromRedisValues(t *testing.T) {
	payload, _ := json.Marshal(map[string]interface{}{
		"description": "Fix auth bug",
	})

	values := map[string]interface{}{
		"id":       "task-001",
		"from":     "harold",
		"to":       "pink",
		"priority": "high",
		"type":     "coding",
		"payload":  string(payload),
		"created":  "1704556800",
	}

	msg, err := TaskMessageFromRedisValues(values)
	require.NoError(t, err)

	assert.Equal(t, "task-001", msg.ID)
	assert.Equal(t, "harold", msg.From)
	assert.Equal(t, "pink", msg.To)
	assert.Equal(t, PriorityHigh, msg.Priority)
	assert.Equal(t, TaskTypeCoding, msg.Type)
	assert.Equal(t, "Fix auth bug", msg.Payload["description"])
}

func TestStreamName(t *testing.T) {
	tests := []struct {
		priority string
		taskType string
		expected string
	}{
		{PriorityCritical, TaskTypeCoding, StreamTasksCritical},
		{PriorityHigh, TaskTypeCoding, StreamTasksHigh},
		{PriorityNormal, TaskTypeCoding, StreamTasksNormal},
		{PriorityLow, TaskTypeCoding, StreamTasksLow},
	}

	for _, tt := range tests {
		result := StreamName(tt.priority, tt.taskType)
		assert.Equal(t, tt.expected, result)
	}
}

func TestPriorityProcessor(t *testing.T) {
	client := setupTestClient(t)
	defer client.Close()

	ctx := context.Background()
	agentName := "test-agent-" + t.Name()

	// Publish tasks with different priorities
	tasks := []TaskMessage{
		{ID: "low-1", Priority: PriorityLow, Type: TaskTypeCoding, Created: time.Now().Unix(), To: agentName},
		{ID: "critical-1", Priority: PriorityCritical, Type: TaskTypeCoding, Created: time.Now().Unix(), To: agentName},
		{ID: "normal-1", Priority: PriorityNormal, Type: TaskTypeCoding, Created: time.Now().Unix(), To: agentName},
		{ID: "high-1", Priority: PriorityHigh, Type: TaskTypeCoding, Created: time.Now().Unix(), To: agentName},
	}

	for _, task := range tasks {
		stream := StreamName(task.Priority, task.Type)
		_, err := client.Publish(ctx, stream, task.ToRedisValues())
		require.NoError(t, err)
	}

	// Process with priority
	processor := NewPriorityProcessor(client, agentName)
	taskChan := processor.Start(ctx)

	// Should receive in priority order: critical first
	select {
	case task := <-taskChan:
		assert.Equal(t, "critical-1", task.ID)
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for critical task")
	}

	// Cleanup
	for _, task := range tasks {
		stream := StreamName(task.Priority, task.Type)
		client.RawClient().Del(ctx, stream)
	}
}

func TestHeartbeatManager(t *testing.T) {
	client := setupTestClient(t)
	defer client.Close()

	ctx := context.Background()
	agentName := "test-agent-" + t.Name()

	hbMgr := NewHeartbeatManager(client, agentName)

	// Send heartbeat
	err := hbMgr.SendHeartbeat(ctx, "healthy", map[string]interface{}{
		"cpu":    45,
		"memory": 60,
	})
	require.NoError(t, err)

	// Verify heartbeat in stream
	rdb := client.RawClient()
	results, err := rdb.XRevRangeN(ctx, HeartbeatStreamName(), "+", "-", 10).Result()
	require.NoError(t, err)

	found := false
	for _, msg := range results {
		if agent, ok := msg.Values["agent"].(string); ok && agent == agentName {
			found = true
			assert.Equal(t, "healthy", msg.Values["status"])
			break
		}
	}
	assert.True(t, found, "Heartbeat not found in stream")
}

func TestDeadLetterQueue(t *testing.T) {
	client := setupTestClient(t)
	defer client.Close()

	dlq := NewDeadLetterQueue(client)
	ctx := context.Background()

	// Send to DLQ
	failedTask := TaskMessage{
		ID:       "failed-001",
		From:     "harold",
		To:       "pink",
		Priority: PriorityHigh,
		Type:     TaskTypeCoding,
		Payload:  map[string]interface{}{"test": "data"},
		Created:  time.Now().Unix(),
	}

	err := dlq.SendToDeadLetter(ctx, failedTask, "processing timeout", 3)
	require.NoError(t, err)

	// Get dead letters
	letters, err := dlq.GetDeadLetters(ctx, 10)
	require.NoError(t, err)

	// Find our letter
	found := false
	for _, letter := range letters {
		if letter.OriginalTask.ID == "failed-001" {
			found = true
			assert.Equal(t, "processing timeout", letter.Error)
			assert.Equal(t, 3, letter.RetryCount)
			break
		}
	}
	assert.True(t, found, "Dead letter not found")

	// Cleanup
	client.RawClient().Del(ctx, DeadLetterStreamName())
}

func TestRedisClient_WithRetry(t *testing.T) {
	client := setupTestClient(t)
	defer client.Close()

	ctx := context.Background()

	// Test successful retry
	callCount := 0
	err := client.WithRetry(ctx, 3, func() error {
		callCount++
		if callCount < 2 {
			return redis.Nil // Simulate temporary failure
		}
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 2, callCount)
}
