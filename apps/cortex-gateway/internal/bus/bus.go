package bus

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/cortexhub/cortex-gateway/internal/messaging"
	"github.com/gorilla/websocket"
)

// Client represents a Neural Bus client that can use either
// WebSocket (legacy) or Redis Streams (new) as the backend
type Client struct {
	// WebSocket fields (legacy mode)
	conn *websocket.Conn

	// Redis fields (new mode)
	redisClient  *messaging.RedisClient
	processor    *messaging.PriorityProcessor
	agentName    string
	useRedis     bool
	stopCh       chan struct{}
}

// Event represents a neural bus event
type Event struct {
	EventType string                 `json:"event_type"`
	Payload   map[string]interface{} `json:"payload"`
	Timestamp string                 `json:"timestamp"`
	Source    string                 `json:"source,omitempty"`
}

// NewClient creates a new Neural Bus client
// The url parameter can be:
// - A WebSocket URL (ws:// or wss://) for legacy mode
// - "redis://host:port" for Redis Streams mode
// - Any other URL will try WebSocket first, then fall back
func NewClient(url string) (*Client, error) {
	// Check if it's a Redis URL
	if len(url) > 8 && url[:8] == "redis://" {
		redisAddr := url[8:]
		return NewClientWithRedis(redisAddr, "cortex-gateway")
	}

	// Fall back to WebSocket for backward compatibility
	return NewClientWithWebSocket(url)
}

// NewClientWithRedis creates a new client using Redis Streams
func NewClientWithRedis(redisAddr, agentName string) (*Client, error) {
	redisClient, err := messaging.NewRedisClient(messaging.RedisConfig{
		Addr:     redisAddr,
		Password: "",
		DB:       0,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Redis at %s: %w", redisAddr, err)
	}

	processor := messaging.NewPriorityProcessor(redisClient, agentName)

	return &Client{
		redisClient: redisClient,
		processor:   processor,
		agentName:   agentName,
		useRedis:    true,
		stopCh:      make(chan struct{}),
	}, nil
}

// NewClientWithWebSocket creates a legacy WebSocket client for backward compatibility
func NewClientWithWebSocket(url string) (*Client, error) {
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return nil, err
	}
	return &Client{
		conn:     conn,
		useRedis: false,
		stopCh:   make(chan struct{}),
	}, nil
}

// Publish publishes an event to the Neural Bus
func (c *Client) Publish(event Event) error {
	if c.useRedis {
		return c.publishRedis(event)
	}
	return c.publishWebSocket(event)
}

func (c *Client) publishWebSocket(event Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return c.conn.WriteMessage(websocket.TextMessage, data)
}

func (c *Client) publishRedis(event Event) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Map event type to priority
	priority := messaging.PriorityNormal
	switch event.EventType {
	case "critical", "error", "alert":
		priority = messaging.PriorityCritical
	case "high", "warning":
		priority = messaging.PriorityHigh
	case "low", "debug":
		priority = messaging.PriorityLow
	}

	payload := map[string]interface{}{
		"event_type": event.EventType,
		"payload":    event.Payload,
		"timestamp":  event.Timestamp,
		"source":     event.Source,
	}

	msg := messaging.NewTaskMessage(
		c.agentName,
		"", // Broadcast
		priority,
		messaging.TaskTypeMessage,
		payload,
	)

	stream := messaging.StreamName(priority, messaging.TaskTypeMessage)
	values := msg.ToRedisValues()

	_, err := c.redisClient.Publish(ctx, stream, values)
	return err
}

// Subscribe subscribes to Neural Bus events and returns a channel of events
func (c *Client) Subscribe() <-chan Event {
	ch := make(chan Event)

	if c.useRedis {
		go c.subscribeRedis(ch)
	} else {
		go c.subscribeWebSocket(ch)
	}

	return ch
}

func (c *Client) subscribeWebSocket(ch chan<- Event) {
	defer close(ch)
	for {
		select {
		case <-c.stopCh:
			return
		default:
			_, message, err := c.conn.ReadMessage()
			if err != nil {
				log.Println("read error:", err)
				return
			}
			var event Event
			if err := json.Unmarshal(message, &event); err != nil {
				log.Println("unmarshal error:", err)
				continue
			}
			ch <- event
		}
	}
}

func (c *Client) subscribeRedis(ch chan<- Event) {
	defer close(ch)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	taskChan := c.processor.Start(ctx)

	for {
		select {
		case <-c.stopCh:
			cancel()
			return
		case task, ok := <-taskChan:
			if !ok {
				return
			}

			event := Event{
				EventType: getEventTypeFromTask(task),
				Payload:   task.Payload,
				Timestamp: time.Unix(task.Created, 0).Format(time.RFC3339),
				Source:    task.From,
			}

			ch <- event
		}
	}
}

func getEventTypeFromTask(task *messaging.TaskMessage) string {
	if eventType, ok := task.Payload["event_type"].(string); ok {
		return eventType
	}
	return task.Type
}

// Close closes the Neural Bus client gracefully
func (c *Client) Close() error {
	close(c.stopCh)

	if c.useRedis && c.redisClient != nil {
		return c.redisClient.Close()
	}

	if c.conn != nil {
		return c.conn.Close()
	}

	return nil
}

// IsConnected checks if the client is connected
func (c *Client) IsConnected() bool {
	if c.useRedis && c.redisClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		return c.redisClient.IsConnected(ctx)
	}

	return c.conn != nil
}

// UsingRedis returns true if using Redis Streams backend
func (c *Client) UsingRedis() bool {
	return c.useRedis
}
