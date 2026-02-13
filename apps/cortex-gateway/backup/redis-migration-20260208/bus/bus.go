package bus

import (
	"encoding/json"
	"log"

	"github.com/gorilla/websocket"
)

// Client represents a Neural Bus WebSocket client
type Client struct {
	conn *websocket.Conn
}

// Event represents a neural bus event
type Event struct {
	EventType string                 `json:"event_type"`
	Payload   map[string]interface{} `json:"payload"`
	Timestamp string                 `json:"timestamp"`
}

// NewClient creates a new Neural Bus client
func NewClient(url string) (*Client, error) {
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return nil, err
	}
	return &Client{conn: conn}, nil
}

// Publish publishes an event to the Neural Bus
func (c *Client) Publish(event Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return c.conn.WriteMessage(websocket.TextMessage, data)
}

// Subscribe subscribes to Neural Bus events and returns a channel of events
func (c *Client) Subscribe() <-chan Event {
	ch := make(chan Event)
	go func() {
		defer close(ch)
		for {
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
	}()
	return ch
}

// Close closes the WebSocket connection gracefully
func (c *Client) Close() error {
	return c.conn.Close()
}
