package messaging

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisConfig holds configuration for Redis connection
type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

// RedisClient wraps go-redis with Cortex-specific operations
type RedisClient struct {
	rdb *redis.Client
	cfg RedisConfig
}

// Message represents a message from a Redis Stream
type Message struct {
	ID     string
	Stream string
	Values map[string]interface{}
}

// NewRedisClient creates a new Redis client with connection validation
func NewRedisClient(cfg RedisConfig) (*RedisClient, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}

	return &RedisClient{
		rdb: rdb,
		cfg: cfg,
	}, nil
}

// Ping checks if Redis is reachable
func (c *RedisClient) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

// Publish publishes a message to a Redis Stream using XADD
func (c *RedisClient) Publish(ctx context.Context, stream string, values map[string]interface{}) (string, error) {
	result, err := c.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: stream,
		Values: values,
	}).Result()

	if err != nil {
		return "", fmt.Errorf("xadd failed: %w", err)
	}

	return result, nil
}

// Subscribe subscribes to a stream using consumer groups with XREADGROUP
func (c *RedisClient) Subscribe(ctx context.Context, stream, group, consumer string) (<-chan Message, error) {
	// Create consumer group if not exists (ignore error if already exists)
	c.rdb.XGroupCreateMkStream(ctx, stream, group, "0")

	msgChan := make(chan Message, 100)

	go c.readLoop(ctx, stream, group, consumer, msgChan)

	return msgChan, nil
}

// readLoop continuously reads messages from the stream
func (c *RedisClient) readLoop(ctx context.Context, stream, group, consumer string, msgChan chan<- Message) {
	defer close(msgChan)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			// Read from stream with blocking
			results, err := c.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
				Group:    group,
				Consumer: consumer,
				Streams:  []string{stream, ">"},
				Count:    10,
				Block:    1000 * time.Millisecond, // 1 second block
			}).Result()

			if err != nil {
				if err == redis.Nil {
					continue // No messages
				}
				if ctx.Err() != nil {
					return // Context cancelled
				}
				// Other errors, log and keep trying
				fmt.Printf("Redis read error: %v\n", err)
				continue
			}

			// Process messages
			for _, result := range results {
				for _, msg := range result.Messages {
					msgChan <- Message{
						ID:     msg.ID,
						Stream: stream,
						Values: msg.Values,
					}

					// Acknowledge message
					c.rdb.XAck(ctx, stream, group, msg.ID)
				}
			}
		}
	}
}

// Close closes the Redis connection
func (c *RedisClient) Close() error {
	return c.rdb.Close()
}

// RawClient returns the underlying go-redis client for advanced operations
func (c *RedisClient) RawClient() *redis.Client {
	return c.rdb
}

// IsConnected checks if the client is connected to Redis
func (c *RedisClient) IsConnected(ctx context.Context) bool {
	return c.Ping(ctx) == nil
}

// WithRetry executes a function with retry logic
func (c *RedisClient) WithRetry(ctx context.Context, maxRetries int, fn func() error) error {
	var err error
	for i := 0; i < maxRetries; i++ {
		if err = fn(); err == nil {
			return nil
		}
		if i < maxRetries-1 {
			time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
		}
	}
	return fmt.Errorf("failed after %d retries: %w", maxRetries, err)
}
