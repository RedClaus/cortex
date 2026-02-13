// Package backend provides client implementations for connecting to A2A-compliant agents.
//
// The A2A (Agent-to-Agent) protocol enables communication between AI agents using
// JSON-RPC 2.0 over HTTP. This package wraps the official a2a-go SDK to provide
// a convenient interface for Salamander's TUI framework, including BubbleTea
// integration for reactive UI updates.
//
// Key features:
//   - Automatic agent card discovery
//   - Streaming and non-streaming message sending
//   - Task lifecycle management (create, monitor, cancel)
//   - Artifact extraction from responses
//   - BubbleTea commands for seamless UI integration
package backend

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/normanking/salamander/pkg/schema"

	"github.com/a2aproject/a2a-go/a2a"
	"github.com/a2aproject/a2a-go/a2aclient"
	"github.com/a2aproject/a2a-go/a2aclient/agentcard"
)

// ═══════════════════════════════════════════════════════════════════════════════
// ERRORS
// ═══════════════════════════════════════════════════════════════════════════════

var (
	// ErrNotConnected is returned when attempting operations without a connection.
	ErrNotConnected = errors.New("not connected to A2A agent")

	// ErrNoActiveTask is returned when trying to cancel without an active task.
	ErrNoActiveTask = errors.New("no active task to cancel")

	// ErrTaskFailed is returned when a task fails.
	ErrTaskFailed = errors.New("task failed")

	// ErrTaskCancelled is returned when a task is cancelled.
	ErrTaskCancelled = errors.New("task cancelled")
)

// ═══════════════════════════════════════════════════════════════════════════════
// TYPES
// ═══════════════════════════════════════════════════════════════════════════════

// Client wraps the A2A SDK client with additional functionality for Salamander.
type Client struct {
	url       string
	client    *a2aclient.Client
	streaming bool
	timeout   time.Duration

	// Authentication
	authToken  string
	authScheme string

	// Current task state
	mu            sync.RWMutex
	currentTaskID a2a.TaskID
	cancelFunc    context.CancelFunc
	connected     bool

	// Cached agent card
	agentCard *a2a.AgentCard

	// HTTP client for agent card resolution
	httpClient *http.Client

	// Agent card resolver
	resolver *agentcard.Resolver
}

// TaskResult represents the outcome of a completed task.
type TaskResult struct {
	TaskID    string
	Status    string // "completed", "failed", "cancelled"
	Response  string
	Artifacts []Artifact
	Error     error
}

// Artifact represents a piece of data returned by the agent.
type Artifact struct {
	Name        string
	Description string
	Type        string // "text", "data", "file"
	MimeType    string
	Data        interface{}
}

// StreamUpdate represents a real-time update during task execution.
type StreamUpdate struct {
	Type     string    // "status", "content", "artifact", "done", "error"
	Status   string    // Current task status when Type is "status"
	Content  string    // Text content when Type is "content"
	Artifact *Artifact // Artifact data when Type is "artifact"
	Error    error     // Error when Type is "error"
}

// ═══════════════════════════════════════════════════════════════════════════════
// BUBBLETEA MESSAGES
// ═══════════════════════════════════════════════════════════════════════════════

// ConnectedMsg is sent when the client successfully connects to an agent.
type ConnectedMsg struct {
	AgentCard *a2a.AgentCard
}

// DisconnectedMsg is sent when the client disconnects or loses connection.
type DisconnectedMsg struct {
	Error error
}

// TaskStartedMsg is sent when a new task begins.
type TaskStartedMsg struct {
	TaskID string
}

// TaskUpdateMsg is sent for streaming updates during task execution.
type TaskUpdateMsg struct {
	Update StreamUpdate
}

// TaskCompletedMsg is sent when a task finishes (success, failure, or cancellation).
type TaskCompletedMsg struct {
	Result TaskResult
}

// ═══════════════════════════════════════════════════════════════════════════════
// CLIENT CREATION
// ═══════════════════════════════════════════════════════════════════════════════

// NewClient creates a new A2A client from the given configuration.
func NewClient(cfg schema.BackendConfig) (*Client, error) {
	if cfg.URL == "" {
		return nil, errors.New("backend URL is required")
	}

	// Set default timeout
	timeout := time.Duration(cfg.Timeout) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	// Create HTTP client for agent card resolution
	httpClient := &http.Client{
		Timeout: timeout,
	}

	// Create agent card resolver
	resolver := agentcard.NewResolver(httpClient)

	return &Client{
		url:        cfg.URL,
		streaming:  cfg.Streaming,
		timeout:    timeout,
		authToken:  cfg.AuthToken,
		authScheme: cfg.AuthScheme,
		httpClient: httpClient,
		resolver:   resolver,
	}, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// CONNECTION MANAGEMENT
// ═══════════════════════════════════════════════════════════════════════════════

// FetchAgentCard retrieves the agent's capabilities and metadata.
func (c *Client) FetchAgentCard() (*a2a.AgentCard, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	// Build resolve options for authentication
	var opts []agentcard.ResolveOption
	if c.authToken != "" {
		scheme := c.authScheme
		if scheme == "" {
			scheme = "Bearer"
		}
		opts = append(opts, agentcard.WithRequestHeader("Authorization", fmt.Sprintf("%s %s", scheme, c.authToken)))
	}

	card, err := c.resolver.Resolve(ctx, c.url, opts...)
	if err != nil {
		return nil, fmt.Errorf("fetch agent card: %w", err)
	}

	// Create the A2A client from the card
	var factoryOpts []a2aclient.FactoryOption

	// Add authentication interceptor if configured
	if c.authToken != "" {
		credStore := a2aclient.NewInMemoryCredentialsStore()
		// Store the credential for the default session
		credStore.Set("default", "bearer", a2aclient.AuthCredential(c.authToken))
		factoryOpts = append(factoryOpts, a2aclient.WithInterceptors(&a2aclient.AuthInterceptor{
			Service: credStore,
		}))
	}

	a2aClient, err := a2aclient.NewFromCard(ctx, card, factoryOpts...)
	if err != nil {
		return nil, fmt.Errorf("create client from card: %w", err)
	}

	c.mu.Lock()
	c.agentCard = card
	c.client = a2aClient
	c.connected = true
	c.mu.Unlock()

	return card, nil
}

// IsConnected returns true if the client has successfully connected to an agent.
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// AgentCard returns the cached agent card, or nil if not connected.
func (c *Client) AgentCard() *a2a.AgentCard {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.agentCard
}

// Close cleans up the client and cancels any active tasks.
func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cancelFunc != nil {
		c.cancelFunc()
		c.cancelFunc = nil
	}

	if c.client != nil {
		_ = c.client.Destroy()
		c.client = nil
	}

	c.connected = false
	c.currentTaskID = ""
	c.agentCard = nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// MESSAGE SENDING
// ═══════════════════════════════════════════════════════════════════════════════

// SendMessage sends a message and waits for the complete response (non-streaming).
func (c *Client) SendMessage(ctx context.Context, message string) (*TaskResult, error) {
	if !c.IsConnected() {
		return nil, ErrNotConnected
	}

	// Create cancellable context
	ctx, cancel := context.WithCancel(ctx)
	c.mu.Lock()
	c.cancelFunc = cancel
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		c.cancelFunc = nil
		c.mu.Unlock()
	}()

	// Build the message
	msg := a2a.NewMessage(a2a.MessageRoleUser, a2a.TextPart{Text: message})

	// Build the request params
	params := &a2a.MessageSendParams{
		Message: msg,
	}

	// Send the message
	resp, err := c.client.SendMessage(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("send message: %w", err)
	}

	// Process response
	return c.processResponse(resp)
}

// SendMessageStream sends a message with streaming updates.
func (c *Client) SendMessageStream(ctx context.Context, message string, onUpdate func(StreamUpdate)) error {
	if !c.IsConnected() {
		return ErrNotConnected
	}

	// Create cancellable context
	ctx, cancel := context.WithCancel(ctx)
	c.mu.Lock()
	c.cancelFunc = cancel
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		c.cancelFunc = nil
		c.currentTaskID = ""
		c.mu.Unlock()
	}()

	// Build the message
	msg := a2a.NewMessage(a2a.MessageRoleUser, a2a.TextPart{Text: message})

	// Build the request params
	params := &a2a.MessageSendParams{
		Message: msg,
	}

	// Send with streaming using the iterator
	for event, err := range c.client.SendStreamingMessage(ctx, params) {
		if err != nil {
			onUpdate(StreamUpdate{Type: "error", Error: err})
			return fmt.Errorf("streaming error: %w", err)
		}

		update := c.processStreamEvent(event)
		onUpdate(update)

		if update.Type == "error" || update.Type == "done" {
			break
		}
	}

	return nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// TASK MANAGEMENT
// ═══════════════════════════════════════════════════════════════════════════════

// CancelTask cancels the currently running task.
func (c *Client) CancelTask() error {
	c.mu.Lock()
	taskID := c.currentTaskID
	cancelFunc := c.cancelFunc
	c.mu.Unlock()

	if taskID == "" && cancelFunc == nil {
		return ErrNoActiveTask
	}

	// Cancel the context first
	if cancelFunc != nil {
		cancelFunc()
	}

	// If we have a task ID, also send cancel request to server
	if taskID != "" && c.client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		params := &a2a.TaskIDParams{
			ID: taskID,
		}

		_, err := c.client.CancelTask(ctx, params)
		if err != nil {
			// Log but don't fail - context cancellation is the primary mechanism
			return fmt.Errorf("cancel task on server: %w", err)
		}
	}

	return nil
}

// GetTask retrieves the current status of a task by ID.
func (c *Client) GetTask(taskID string) (*a2a.Task, error) {
	if !c.IsConnected() {
		return nil, ErrNotConnected
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	params := &a2a.TaskQueryParams{
		ID: a2a.TaskID(taskID),
	}

	task, err := c.client.GetTask(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("get task: %w", err)
	}

	return task, nil
}

// CurrentTaskID returns the ID of the currently running task, if any.
func (c *Client) CurrentTaskID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return string(c.currentTaskID)
}

// ═══════════════════════════════════════════════════════════════════════════════
// BUBBLETEA COMMANDS
// ═══════════════════════════════════════════════════════════════════════════════

// Connect returns a BubbleTea command that connects to the agent.
func (c *Client) Connect() tea.Cmd {
	return func() tea.Msg {
		card, err := c.FetchAgentCard()
		if err != nil {
			return DisconnectedMsg{Error: err}
		}
		return ConnectedMsg{AgentCard: card}
	}
}

// Send returns a BubbleTea command that sends a message to the agent.
// If streaming is enabled, it will send streaming updates via TaskUpdateMsg.
func (c *Client) Send(message string) tea.Cmd {
	return func() tea.Msg {
		if c.streaming {
			return c.sendStreaming(message)
		}
		return c.sendNonStreaming(message)
	}
}

// sendNonStreaming sends a message without streaming.
func (c *Client) sendNonStreaming(message string) tea.Msg {
	ctx := context.Background()
	result, err := c.SendMessage(ctx, message)
	if err != nil {
		return TaskCompletedMsg{
			Result: TaskResult{
				Status: "failed",
				Error:  err,
			},
		}
	}
	return TaskCompletedMsg{Result: *result}
}

// sendStreaming sends a message with streaming. Returns the final message.
func (c *Client) sendStreaming(message string) tea.Msg {
	ctx := context.Background()

	var finalResult TaskResult
	finalResult.Status = "completed"

	err := c.SendMessageStream(ctx, message, func(update StreamUpdate) {
		// Accumulate the response
		switch update.Type {
		case "content":
			finalResult.Response += update.Content
		case "artifact":
			if update.Artifact != nil {
				finalResult.Artifacts = append(finalResult.Artifacts, *update.Artifact)
			}
		case "error":
			finalResult.Status = "failed"
			finalResult.Error = update.Error
		}
	})

	if err != nil {
		finalResult.Status = "failed"
		finalResult.Error = err
	}

	return TaskCompletedMsg{Result: finalResult}
}

// SendWithUpdates returns a BubbleTea command that sends a message and provides
// streaming updates through a channel. Use this when you need real-time updates.
func (c *Client) SendWithUpdates(message string, updates chan<- tea.Msg) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		// Send task started message
		updates <- TaskStartedMsg{TaskID: c.CurrentTaskID()}

		var finalResult TaskResult
		finalResult.Status = "completed"

		err := c.SendMessageStream(ctx, message, func(update StreamUpdate) {
			updates <- TaskUpdateMsg{Update: update}

			switch update.Type {
			case "content":
				finalResult.Response += update.Content
			case "artifact":
				if update.Artifact != nil {
					finalResult.Artifacts = append(finalResult.Artifacts, *update.Artifact)
				}
			case "error":
				finalResult.Status = "failed"
				finalResult.Error = update.Error
			}
		})

		if err != nil {
			finalResult.Status = "failed"
			finalResult.Error = err
		}

		return TaskCompletedMsg{Result: finalResult}
	}
}

// Cancel returns a BubbleTea command that cancels the current task.
func (c *Client) Cancel() tea.Cmd {
	return func() tea.Msg {
		err := c.CancelTask()
		if err != nil {
			return TaskCompletedMsg{
				Result: TaskResult{
					Status: "failed",
					Error:  err,
				},
			}
		}
		return TaskCompletedMsg{
			Result: TaskResult{
				Status: "cancelled",
				Error:  ErrTaskCancelled,
			},
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// INTERNAL HELPERS
// ═══════════════════════════════════════════════════════════════════════════════

// processResponse converts the SDK response to a TaskResult.
func (c *Client) processResponse(resp a2a.SendMessageResult) (*TaskResult, error) {
	result := &TaskResult{}

	if resp == nil {
		return nil, errors.New("nil response from agent")
	}

	switch r := resp.(type) {
	case *a2a.Task:
		result.TaskID = string(r.ID)
		result.Status = mapTaskStatus(r.Status.State)

		// Store task ID
		c.mu.Lock()
		c.currentTaskID = r.ID
		c.mu.Unlock()

		// Check for failure
		if r.Status.State == a2a.TaskStateFailed {
			errMsg := "task failed"
			if r.Status.Message != nil {
				// Extract text from message parts
				for _, part := range r.Status.Message.Parts {
					if tp, ok := part.(a2a.TextPart); ok {
						errMsg = tp.Text
						break
					}
				}
			}
			result.Error = errors.New(errMsg)
			return result, nil
		}

		// Extract response from artifacts
		result.Response, result.Artifacts = extractArtifacts(r.Artifacts)

	case *a2a.Message:
		// Direct message response
		result.Status = "completed"
		for _, part := range r.Parts {
			if tp, ok := part.(a2a.TextPart); ok {
				result.Response += tp.Text
			}
		}
	}

	return result, nil
}

// processStreamEvent converts a streaming event to a StreamUpdate.
func (c *Client) processStreamEvent(event a2a.Event) StreamUpdate {
	update := StreamUpdate{}

	switch e := event.(type) {
	case *a2a.TaskStatusUpdateEvent:
		update.Type = "status"
		update.Status = mapTaskStatus(e.Status.State)

		// Store task ID
		c.mu.Lock()
		c.currentTaskID = e.TaskID
		c.mu.Unlock()

		// Check if task is done
		if e.Final {
			if e.Status.State == a2a.TaskStateFailed {
				update.Type = "error"
				errMsg := "task failed"
				if e.Status.Message != nil {
					for _, part := range e.Status.Message.Parts {
						if tp, ok := part.(a2a.TextPart); ok {
							errMsg = tp.Text
							break
						}
					}
				}
				update.Error = errors.New(errMsg)
			} else if e.Status.State == a2a.TaskStateCompleted {
				update.Type = "done"
			} else if e.Status.State == a2a.TaskStateCanceled {
				update.Type = "done"
				update.Error = ErrTaskCancelled
			}
		}

	case *a2a.TaskArtifactUpdateEvent:
		if e.Artifact != nil {
			artifact := parseArtifact(e.Artifact)
			if artifact != nil {
				if artifact.Type == "text" {
					update.Type = "content"
					if text, ok := artifact.Data.(string); ok {
						update.Content = text
					}
				} else {
					update.Type = "artifact"
					update.Artifact = artifact
				}
			}
		}

	case *a2a.Task:
		update.Type = "status"
		update.Status = mapTaskStatus(e.Status.State)

		c.mu.Lock()
		c.currentTaskID = e.ID
		c.mu.Unlock()

		if e.Status.State.Terminal() {
			if e.Status.State == a2a.TaskStateFailed {
				update.Type = "error"
				update.Error = ErrTaskFailed
			} else {
				update.Type = "done"
			}
		}

	case *a2a.Message:
		update.Type = "content"
		for _, part := range e.Parts {
			if tp, ok := part.(a2a.TextPart); ok {
				update.Content += tp.Text
			}
		}
	}

	return update
}

// mapTaskStatus maps protocol task state to a string status.
func mapTaskStatus(state a2a.TaskState) string {
	switch state {
	case a2a.TaskStateSubmitted:
		return "submitted"
	case a2a.TaskStateWorking:
		return "working"
	case a2a.TaskStateInputRequired:
		return "input-required"
	case a2a.TaskStateCompleted:
		return "completed"
	case a2a.TaskStateFailed:
		return "failed"
	case a2a.TaskStateCanceled:
		return "cancelled"
	case a2a.TaskStateAuthRequired:
		return "auth-required"
	case a2a.TaskStateRejected:
		return "rejected"
	default:
		return "unknown"
	}
}

// extractArtifacts processes artifacts and returns the text response and structured artifacts.
func extractArtifacts(artifacts []*a2a.Artifact) (string, []Artifact) {
	var textParts []string
	var result []Artifact

	for _, a := range artifacts {
		artifact := parseArtifact(a)
		if artifact == nil {
			continue
		}

		if artifact.Type == "text" {
			if text, ok := artifact.Data.(string); ok {
				textParts = append(textParts, text)
			}
		} else {
			result = append(result, *artifact)
		}
	}

	response := ""
	for i, part := range textParts {
		if i > 0 {
			response += "\n"
		}
		response += part
	}

	return response, result
}

// parseArtifact converts a protocol artifact to our Artifact type.
func parseArtifact(a *a2a.Artifact) *Artifact {
	if a == nil {
		return nil
	}

	artifact := &Artifact{
		Name:        a.Name,
		Description: a.Description,
	}

	// Process parts
	for _, part := range a.Parts {
		switch p := part.(type) {
		case a2a.TextPart:
			artifact.Type = "text"
			artifact.MimeType = "text/plain"
			artifact.Data = p.Text

		case a2a.DataPart:
			artifact.Type = "data"
			artifact.MimeType = "application/json"
			artifact.Data = p.Data

		case a2a.FilePart:
			artifact.Type = "file"
			switch f := p.File.(type) {
			case a2a.FileBytes:
				artifact.MimeType = f.MimeType
				artifact.Data = map[string]interface{}{
					"name":  f.Name,
					"bytes": f.Bytes,
				}
			case a2a.FileURI:
				artifact.MimeType = f.MimeType
				artifact.Data = map[string]interface{}{
					"name": f.Name,
					"uri":  f.URI,
				}
			}
		}
	}

	return artifact
}

// ═══════════════════════════════════════════════════════════════════════════════
// CONFIGURATION HELPERS
// ═══════════════════════════════════════════════════════════════════════════════

// URL returns the configured agent URL.
func (c *Client) URL() string {
	return c.url
}

// IsStreaming returns true if streaming mode is enabled.
func (c *Client) IsStreaming() bool {
	return c.streaming
}

// SetStreaming enables or disables streaming mode.
func (c *Client) SetStreaming(enabled bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.streaming = enabled
}

// Timeout returns the configured timeout duration.
func (c *Client) Timeout() time.Duration {
	return c.timeout
}

// SetTimeout updates the timeout duration.
func (c *Client) SetTimeout(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.timeout = d
}
