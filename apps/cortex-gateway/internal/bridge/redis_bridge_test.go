package bridge

import (
	"context"
	"testing"
	"time"

	"github.com/cortexhub/cortex-gateway/internal/messaging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test using Redis server at 192.168.1.186:6379
func setupTestRedisBridge(t *testing.T) *RedisBridge {
	cfg := RedisBridgeConfig{
		RedisAddr: "192.168.1.186:6379",
		AgentName: "test-agent-" + t.Name(),
	}
	bridge, err := NewRedisBridge(cfg)
	if err != nil {
		t.Skipf("Redis not available: %v", err)
	}
	return bridge
}

func TestRedisBridge_Connection(t *testing.T) {
	bridge := setupTestRedisBridge(t)
	defer bridge.Close()

	ctx := context.Background()
	assert.True(t, bridge.IsConnected(ctx))
}

func TestRedisBridge_PublishTask(t *testing.T) {
	bridge := setupTestRedisBridge(t)
	defer bridge.Close()

	ctx := context.Background()

	taskID, err := bridge.PublishTask(ctx, TaskRequest{
		To:       "pink",
		Priority: messaging.PriorityHigh,
		Type:     messaging.TaskTypeCoding,
		Payload: map[string]interface{}{
			"description": "Test task",
		},
	})

	require.NoError(t, err)
	assert.NotEmpty(t, taskID)
}

func TestRedisBridge_SubscribeTasks(t *testing.T) {
	receiver := setupTestRedisBridge(t)
	defer receiver.Close()

	// Create publisher with different agent name
	publisherCfg := RedisBridgeConfig{
		RedisAddr: "192.168.1.186:6379",
		AgentName: "test-publisher",
	}
	publisher, err := NewRedisBridge(publisherCfg)
	if err != nil {
		t.Skipf("Redis not available: %v", err)
	}
	defer publisher.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Subscribe to tasks
	taskChan, err := receiver.SubscribeTasks(ctx)
	require.NoError(t, err)

	// Give subscription time to set up
	time.Sleep(200 * time.Millisecond)

	// Publish a task to the receiver
	receiverAgentName := "test-agent-" + t.Name()
	_, err = publisher.PublishTask(ctx, TaskRequest{
		To:       receiverAgentName,
		Priority: messaging.PriorityHigh,
		Type:     messaging.TaskTypeCoding,
		Payload: map[string]interface{}{
			"description": "Test subscription",
		},
	})
	require.NoError(t, err)

	// Receive the task
	select {
	case task := <-taskChan:
		assert.Equal(t, receiverAgentName, task.To)
		assert.Equal(t, "Test subscription", task.Payload["description"])
	case <-ctx.Done():
		t.Fatal("timeout waiting for task")
	}
}

func TestRedisBridge_SendHeartbeat(t *testing.T) {
	bridge := setupTestRedisBridge(t)
	defer bridge.Close()

	ctx := context.Background()

	err := bridge.SendHeartbeat(ctx, "busy", map[string]interface{}{
		"task":   "current task",
		"load":   0.75,
	})

	assert.NoError(t, err)
}

func TestRedisBridge_SendMessage(t *testing.T) {
	bridge := setupTestRedisBridge(t)
	defer bridge.Close()

	ctx := context.Background()

	err := bridge.SendMessage(ctx, "harold", "pink", "chat", "Hello Pink!")
	assert.NoError(t, err)
}

func TestRedisBridge_HeartbeatSubscription(t *testing.T) {
	// Create two bridges
	sender := setupTestRedisBridge(t)
	defer sender.Close()

	receiver := setupTestRedisBridge(t)
	defer receiver.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Subscribe to heartbeats
	hbChan, err := receiver.SubscribeToHeartbeats(ctx)
	require.NoError(t, err)

	// Give subscription time to set up
	time.Sleep(200 * time.Millisecond)

	// Send heartbeat from sender
	err = sender.SendHeartbeat(ctx, "healthy", map[string]interface{}{
		"test": "data",
	})
	require.NoError(t, err)

	// Receive heartbeat
	select {
	case hb := <-hbChan:
		assert.Equal(t, sender.agentName, hb.Agent)
		assert.Equal(t, "healthy", hb.Status)
	case <-ctx.Done():
		t.Fatal("timeout waiting for heartbeat")
	}
}

func TestRedisBridge_PriorityOrdering(t *testing.T) {
	receiver := setupTestRedisBridge(t)
	defer receiver.Close()

	publisherCfg := RedisBridgeConfig{
		RedisAddr: "192.168.1.186:6379",
		AgentName: "test-priority-publisher",
	}
	publisher, err := NewRedisBridge(publisherCfg)
	if err != nil {
		t.Skipf("Redis not available: %v", err)
	}
	defer publisher.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Subscribe
	taskChan, err := receiver.SubscribeTasks(ctx)
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)

	receiverAgentName := "test-agent-" + t.Name()

	// Send tasks in reverse priority order
	_, _ = publisher.PublishTask(ctx, TaskRequest{
		To:       receiverAgentName,
		Priority: messaging.PriorityLow,
		Type:     messaging.TaskTypeCoding,
		Payload:  map[string]interface{}{"order": "4"},
	})
	_, _ = publisher.PublishTask(ctx, TaskRequest{
		To:       receiverAgentName,
		Priority: messaging.PriorityNormal,
		Type:     messaging.TaskTypeCoding,
		Payload:  map[string]interface{}{"order": "3"},
	})
	_, _ = publisher.PublishTask(ctx, TaskRequest{
		To:       receiverAgentName,
		Priority: messaging.PriorityHigh,
		Type:     messaging.TaskTypeCoding,
		Payload:  map[string]interface{}{"order": "2"},
	})
	_, _ = publisher.PublishTask(ctx, TaskRequest{
		To:       receiverAgentName,
		Priority: messaging.PriorityCritical,
		Type:     messaging.TaskTypeCoding,
		Payload:  map[string]interface{}{"order": "1"},
	})

	// Should receive critical first
	select {
	case task := <-taskChan:
		assert.Equal(t, "1", task.Payload["order"])
	case <-ctx.Done():
		t.Fatal("timeout waiting for critical task")
	}

	// Then high
	select {
	case task := <-taskChan:
		assert.Equal(t, "2", task.Payload["order"])
	case <-ctx.Done():
		t.Fatal("timeout waiting for high task")
	}

	// Then normal
	select {
	case task := <-taskChan:
		assert.Equal(t, "3", task.Payload["order"])
	case <-ctx.Done():
		t.Fatal("timeout waiting for normal task")
	}

	// Then low
	select {
	case task := <-taskChan:
		assert.Equal(t, "4", task.Payload["order"])
	case <-ctx.Done():
		t.Fatal("timeout waiting for low task")
	}
}
