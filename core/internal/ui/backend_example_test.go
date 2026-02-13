package ui_test

import (
	"errors"
	"time"

	"github.com/normanking/cortex/internal/ui"
)

// ExampleBackend demonstrates how to implement the Backend interface.
// This is a mock implementation for testing and documentation purposes.
type ExampleBackend struct {
	streamChan chan ui.StreamChunk
	models     []ui.ModelInfo
	sessions   []ui.SessionInfo
}

// NewExampleBackend creates a new example backend for testing.
func NewExampleBackend() *ExampleBackend {
	return &ExampleBackend{
		streamChan: make(chan ui.StreamChunk, 10),
		models: []ui.ModelInfo{
			{
				ID:           "gpt-4",
				Name:         "GPT-4",
				Provider:     "openai",
				IsLocal:      false,
				Capabilities: []string{"chat", "vision"},
				MaxTokens:    8192,
			},
			{
				ID:           "llama2",
				Name:         "Llama 2",
				Provider:     "ollama",
				IsLocal:      true,
				Capabilities: []string{"chat"},
				MaxTokens:    4096,
			},
		},
		sessions: []ui.SessionInfo{
			{
				ID:        "session-1",
				Name:      "Morning Chat",
				CreatedAt: time.Now().Add(-2 * time.Hour),
				UpdatedAt: time.Now().Add(-1 * time.Hour),
				Messages:  15,
				Model:     "gpt-4",
				Tags:      []string{"work", "important"},
			},
		},
	}
}

// SendMessage implements Backend.SendMessage
func (b *ExampleBackend) SendMessage(content string) (<-chan ui.StreamChunk, error) {
	if content == "" {
		return nil, errors.New("message cannot be empty")
	}

	// Start a goroutine to simulate streaming response
	go func() {
		// Simulate streaming "Hello, world!" one word at a time
		words := []string{"Hello", ", ", "world", "!"}
		for _, word := range words {
			b.streamChan <- ui.StreamChunk{
				Content: word,
				Done:    false,
				Error:   nil,
				Metadata: map[string]interface{}{
					"model": "gpt-4",
				},
			}
			time.Sleep(100 * time.Millisecond)
		}

		// Send final chunk
		b.streamChan <- ui.StreamChunk{
			Done: true,
		}
	}()

	return b.streamChan, nil
}

// StreamChannel implements Backend.StreamChannel
func (b *ExampleBackend) StreamChannel() <-chan ui.StreamChunk {
	return b.streamChan
}

// CancelStream implements Backend.CancelStream
func (b *ExampleBackend) CancelStream() error {
	// Close the channel to signal cancellation
	close(b.streamChan)
	return nil
}

// GetModels implements Backend.GetModels
func (b *ExampleBackend) GetModels() ([]ui.ModelInfo, error) {
	return b.models, nil
}

// GetSessions implements Backend.GetSessions
func (b *ExampleBackend) GetSessions() ([]ui.SessionInfo, error) {
	return b.sessions, nil
}

// Example_backendUsage demonstrates how to use the Backend interface.
func Example_backendUsage() {
	// Create backend
	backend := NewExampleBackend()

	// Send a message
	streamCh, err := backend.SendMessage("Hello, AI!")
	if err != nil {
		panic(err)
	}

	// Read streaming response
	for chunk := range streamCh {
		if chunk.Error != nil {
			// Handle error
			break
		}

		if chunk.Done {
			// Streaming complete
			break
		}

		// Process chunk content
		_ = chunk.Content
	}

	// Fetch available models
	models, _ := backend.GetModels()
	_ = models

	// Fetch conversation sessions
	sessions, _ := backend.GetSessions()
	_ = sessions
}

// Example_messageLifecycle demonstrates the message state lifecycle.
func Example_messageLifecycle() {
	// Create a user message
	userMsg := ui.NewUserMessage("What is the weather today?")
	_ = userMsg.ID        // Unique identifier
	_ = userMsg.Role      // RoleUser
	_ = userMsg.State     // MessageComplete (user messages are immediately complete)
	_ = userMsg.Timestamp // When created

	// Create an assistant message (starts in pending state)
	assistantMsg := ui.NewAssistantMessage()
	_ = assistantMsg.State // MessagePending

	// Simulate streaming response
	assistantMsg.AppendContent("The weather ")
	_ = assistantMsg.State // MessageStreaming (automatically changed)

	assistantMsg.AppendContent("is sunny ")
	assistantMsg.AppendContent("and warm.")

	// Mark complete when streaming finishes
	assistantMsg.MarkComplete()
	_ = assistantMsg.State      // MessageComplete
	_ = assistantMsg.Duration() // Time from creation to completion

	// Create a system message
	sysMsg := ui.NewSystemMessage("Connection established")
	_ = sysMsg.Role  // RoleSystem
	_ = sysMsg.State // MessageComplete
}
