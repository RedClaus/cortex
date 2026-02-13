package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// DeadLetterQueue handles failed tasks for later inspection and retry
type DeadLetterQueue struct {
	client *RedisClient
}

// DeadLetter represents a task that failed processing
type DeadLetter struct {
	DLQID        string
	OriginalTask TaskMessage
	Error        string
	RetryCount   int
	DeadAt       int64
}

// NewDeadLetterQueue creates a new DLQ handler
func NewDeadLetterQueue(client *RedisClient) *DeadLetterQueue {
	return &DeadLetterQueue{client: client}
}

// SendToDeadLetter sends a failed task to the DLQ
func (d *DeadLetterQueue) SendToDeadLetter(ctx context.Context, task TaskMessage, errorMsg string, retryCount int) error {
	stream := DeadLetterStreamName()

	payloadJSON, _ := json.Marshal(task.Payload)
	values := map[string]interface{}{
		"original_id":       task.ID,
		"original_from":     task.From,
		"original_to":       task.To,
		"original_priority": task.Priority,
		"original_type":     task.Type,
		"original_payload":  string(payloadJSON),
		"original_created":  strconv.FormatInt(task.Created, 10),
		"error":             errorMsg,
		"retry_count":       strconv.Itoa(retryCount),
		"dead_at":           strconv.FormatInt(time.Now().Unix(), 10),
	}

	_, err := d.client.Publish(ctx, stream, values)
	return err
}

// GetDeadLetters retrieves dead letters from the DLQ
func (d *DeadLetterQueue) GetDeadLetters(ctx context.Context, count int) ([]DeadLetter, error) {
	stream := DeadLetterStreamName()
	rdb := d.client.RawClient()

	results, err := rdb.XRevRangeN(ctx, stream, "+", "-", int64(count)).Result()
	if err == redis.Nil {
		return []DeadLetter{}, nil
	}
	if err != nil {
		return nil, err
	}

	var letters []DeadLetter
	for _, msg := range results {
		letter := d.parseDeadLetter(msg)
		letters = append(letters, letter)
	}

	return letters, nil
}

// RetryDeadLetter retries a dead letter by republishing to the original stream
func (d *DeadLetterQueue) RetryDeadLetter(ctx context.Context, dlqID string) error {
	stream := DeadLetterStreamName()
	rdb := d.client.RawClient()

	// Get the message
	results, err := rdb.XRange(ctx, stream, dlqID, dlqID).Result()
	if err != nil {
		return fmt.Errorf("failed to get DLQ message: %w", err)
	}
	if len(results) == 0 {
		return fmt.Errorf("DLQ message not found: %s", dlqID)
	}

	msg := results[0]
	letter := d.parseDeadLetter(msg)

	// Republish to original stream
	targetStream := StreamName(letter.OriginalTask.Priority, letter.OriginalTask.Type)
	_, err = d.client.Publish(ctx, targetStream, letter.OriginalTask.ToRedisValues())
	if err != nil {
		return fmt.Errorf("failed to republish: %w", err)
	}

	// Remove from DLQ
	rdb.XDel(ctx, stream, dlqID)

	return nil
}

// DeleteDeadLetter removes a message from the DLQ
func (d *DeadLetterQueue) DeleteDeadLetter(ctx context.Context, dlqID string) error {
	stream := DeadLetterStreamName()
	rdb := d.client.RawClient()
	return rdb.XDel(ctx, stream, dlqID).Err()
}

// parseDeadLetter parses a Redis message into a DeadLetter struct
func (d *DeadLetterQueue) parseDeadLetter(msg redis.XMessage) DeadLetter {
	letter := DeadLetter{
		DLQID: msg.ID,
	}

	// Parse original task
	task := TaskMessage{}
	if v, ok := msg.Values["original_id"].(string); ok {
		task.ID = v
	}
	if v, ok := msg.Values["original_from"].(string); ok {
		task.From = v
	}
	if v, ok := msg.Values["original_to"].(string); ok {
		task.To = v
	}
	if v, ok := msg.Values["original_priority"].(string); ok {
		task.Priority = v
	}
	if v, ok := msg.Values["original_type"].(string); ok {
		task.Type = v
	}
	if v, ok := msg.Values["original_payload"].(string); ok {
		var payload map[string]interface{}
		json.Unmarshal([]byte(v), &payload)
		task.Payload = payload
	}
	if v, ok := msg.Values["original_created"].(string); ok {
		created, _ := strconv.ParseInt(v, 10, 64)
		task.Created = created
	}

	letter.OriginalTask = task

	// Parse error details
	if v, ok := msg.Values["error"].(string); ok {
		letter.Error = v
	}
	if v, ok := msg.Values["retry_count"].(string); ok {
		count, _ := strconv.Atoi(v)
		letter.RetryCount = count
	}
	if v, ok := msg.Values["dead_at"].(string); ok {
		deadAt, _ := strconv.ParseInt(v, 10, 64)
		letter.DeadAt = deadAt
	}

	return letter
}

// GetDLQCount returns the number of messages in the DLQ
func (d *DeadLetterQueue) GetDLQCount(ctx context.Context) (int64, error) {
	rdb := d.client.RawClient()
	return rdb.XLen(ctx, DeadLetterStreamName()).Result()
}
