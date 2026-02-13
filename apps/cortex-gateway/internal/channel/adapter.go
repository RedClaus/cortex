package channel

import "context"

// Message represents a message from a channel
type Message struct {
	ID        string
	Channel   string
	UserID    string
	Content   string
	Metadata  map[string]string
	Timestamp int64
}

// Response represents a response to send back to a channel
type Response struct {
	Content  string
	Metadata map[string]string
}

// InboundMessage represents an inbound message
type InboundMessage = Message

// OutboundMessage represents an outbound message
type OutboundMessage = Response

// ChannelAdapter is the interface for channel adapters
type ChannelAdapter interface {
	// Start starts the channel adapter
	Start(ctx context.Context) error

	// Stop stops the channel adapter
	Stop() error

	// SendMessage sends a message to the channel
	SendMessage(userID string, resp *Response) error

	// Incoming returns a channel of incoming messages
	Incoming() <-chan *Message

	// Name returns the name of the channel adapter
	Name() string

	// IsEnabled returns whether the channel is enabled
	IsEnabled() bool
}
