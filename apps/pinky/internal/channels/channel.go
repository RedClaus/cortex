// Package channels provides unified messaging channel interfaces for Pinky.
//
// The Channel interface is the core abstraction for all messaging channels
// (TUI, WebUI, Telegram, Discord, Slack). It provides:
//   - Unified messaging with media and formatting support
//   - Approval workflow with interactive buttons
//   - Async callback handling for button responses
//   - Capability discovery for channel-specific features
package channels

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Common errors
var (
	ErrChannelNotFound   = errors.New("channel not found")
	ErrApprovalTimeout   = errors.New("approval request timed out")
	ErrApprovalCancelled = errors.New("approval request cancelled")
	ErrChannelDisabled   = errors.New("channel is disabled")
)

// ApprovalAction represents the possible actions a user can take on an approval request.
// These are used as button identifiers across all channels.
type ApprovalAction string

const (
	ActionApprove     ApprovalAction = "approve"      // Approve this execution only
	ActionDeny        ApprovalAction = "deny"         // Deny this execution
	ActionAlwaysAllow ApprovalAction = "always_allow" // Approve and remember for this tool
	ActionAllowDir    ApprovalAction = "allow_dir"    // Approve and remember for this directory
	ActionEdit        ApprovalAction = "edit"         // Allow user to modify the command
	ActionCancel      ApprovalAction = "cancel"       // Cancel the pending request
)

// ApprovalCallback is called when a user responds to an approval request.
// It receives the request ID and the user's chosen action.
// If action is ActionEdit, modifiedCommand contains the user's edited command.
type ApprovalCallback func(requestID string, action ApprovalAction, modifiedCommand string)

// Channel is the interface all messaging channels must implement.
// It provides unified messaging, approval workflows, and capability discovery.
type Channel interface {
	// Lifecycle
	Name() string
	Start(ctx context.Context) error
	Stop() error
	IsEnabled() bool

	// Messaging
	SendMessage(userID string, msg *OutboundMessage) error
	Incoming() <-chan *InboundMessage

	// Approval workflow - the core of Pinky's permission system
	// SendApprovalRequest displays an approval dialog with action buttons.
	// The channel should display the request and allow the user to click
	// one of the approval buttons. When clicked, the channel calls the
	// registered ApprovalCallback with the request ID and chosen action.
	SendApprovalRequest(userID string, req *ApprovalRequest) error

	// SetApprovalCallback registers the handler for approval button clicks.
	// This must be called before the channel starts receiving approval responses.
	SetApprovalCallback(callback ApprovalCallback)

	// DismissApproval removes an approval dialog (e.g., on timeout or cancel)
	DismissApproval(userID string, requestID string) error

	// Tool output display
	SendToolOutput(userID string, output *ToolOutput) error

	// Capabilities - channels report what features they support
	SupportsMedia() bool
	SupportsButtons() bool
	SupportsThreading() bool
	SupportsEditing() bool // Can the user edit messages/commands inline?
}

// InboundMessage represents an incoming message from any channel
type InboundMessage struct {
	ID          string
	UserID      string
	ChannelName string // "telegram", "discord", "slack", "tui", "webui"
	ChannelID   string // Platform-specific channel/chat ID
	Content     string
	Media       []Media
	ReplyTo     string // Threading support
	Metadata    map[string]string
	ReceivedAt  time.Time
}

// OutboundMessage represents a message to send
type OutboundMessage struct {
	Content string
	Media   []Media
	Buttons []Button
	ReplyTo string
	Format  MessageFormat
}

// Media represents attached media (images, files, audio)
type Media struct {
	Type     MediaType
	URL      string
	Data     []byte
	Filename string
	MimeType string
}

// MediaType categorizes media
type MediaType string

const (
	MediaImage    MediaType = "image"
	MediaAudio    MediaType = "audio"
	MediaVideo    MediaType = "video"
	MediaDocument MediaType = "document"
)

// Button represents an interactive button
type Button struct {
	ID    string
	Label string
	Style ButtonStyle
}

// ButtonStyle defines button appearance
type ButtonStyle string

const (
	ButtonPrimary   ButtonStyle = "primary"
	ButtonSecondary ButtonStyle = "secondary"
	ButtonDanger    ButtonStyle = "danger"
)

// MessageFormat defines how to format the message
type MessageFormat string

const (
	FormatPlain    MessageFormat = "plain"
	FormatMarkdown MessageFormat = "markdown"
	FormatCode     MessageFormat = "code"
)

// ApprovalRequest asks user to approve a tool execution.
// This is the unified approval request used across all channels.
type ApprovalRequest struct {
	// ID uniquely identifies this approval request for callbacks
	ID string

	// UserID identifies who needs to approve this request
	UserID string

	// Tool is the name of the tool to be executed (e.g., "shell", "files")
	Tool string

	// Command is the specific command or operation to execute
	Command string

	// Args contains the full tool arguments (for display/editing)
	Args map[string]any

	// WorkingDir is where the command will execute
	WorkingDir string

	// RiskLevel indicates how dangerous this operation is
	RiskLevel RiskLevel

	// Reason explains why the tool wants to execute this command
	Reason string

	// CreatedAt is when the request was created
	CreatedAt time.Time

	// ExpiresAt is when the request will timeout (optional)
	ExpiresAt time.Time

	// Options controls which approval buttons to show
	Options ApprovalOptions
}

// RiskLevel indicates how dangerous a tool execution is
type RiskLevel string

const (
	RiskLow    RiskLevel = "low"    // Read-only operations, safe commands
	RiskMedium RiskLevel = "medium" // File modifications, network access
	RiskHigh   RiskLevel = "high"   // System changes, destructive operations
)

// ApprovalOptions controls which actions are available in the approval dialog
type ApprovalOptions struct {
	// ShowAlwaysAllow enables the "Always allow this tool" button
	ShowAlwaysAllow bool

	// ShowAllowDir enables the "Allow in this directory" button
	ShowAllowDir bool

	// ShowEdit enables the "Edit command" button (if channel supports it)
	ShowEdit bool

	// Timeout is how long to wait for a response (0 = use default)
	Timeout time.Duration

	// Message is an optional custom message to display with the request
	Message string
}

// ApprovalResponse represents the user's response to an approval request.
// This is used internally by channel implementations to communicate back.
type ApprovalResponse struct {
	// RequestID links this response to the original request
	RequestID string

	// Action is what the user chose
	Action ApprovalAction

	// ModifiedCommand is set if Action is ActionEdit
	ModifiedCommand string

	// RespondedAt is when the user responded
	RespondedAt time.Time
}

// ToolOutput shows the result of a tool execution
type ToolOutput struct {
	Tool     string
	Success  bool
	Output   string
	Error    string
	Duration time.Duration
}

// Router manages message routing across channels.
// It provides centralized handling of approval callbacks and message routing.
type Router struct {
	mu               sync.RWMutex
	channels         map[string]Channel
	approvalCallback ApprovalCallback
	incoming         chan *InboundMessage
	done             chan struct{}
}

// NewRouter creates a new channel router
func NewRouter() *Router {
	return &Router{
		channels: make(map[string]Channel),
		incoming: make(chan *InboundMessage, 100),
		done:     make(chan struct{}),
	}
}

// Register adds a channel to the router and wires up the approval callback
func (r *Router) Register(ch Channel) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.channels[ch.Name()] = ch

	// Wire up the approval callback if we have one
	if r.approvalCallback != nil {
		ch.SetApprovalCallback(r.approvalCallback)
	}
}

// Get retrieves a channel by name
func (r *Router) Get(name string) (Channel, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ch, ok := r.channels[name]
	return ch, ok
}

// All returns all registered channels
func (r *Router) All() []Channel {
	r.mu.RLock()
	defer r.mu.RUnlock()

	channels := make([]Channel, 0, len(r.channels))
	for _, ch := range r.channels {
		channels = append(channels, ch)
	}
	return channels
}

// SetApprovalCallback sets the callback for all channels.
// This propagates to all registered channels.
func (r *Router) SetApprovalCallback(callback ApprovalCallback) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.approvalCallback = callback
	for _, ch := range r.channels {
		ch.SetApprovalCallback(callback)
	}
}

// Incoming returns a unified channel for messages from all channels
func (r *Router) Incoming() <-chan *InboundMessage {
	return r.incoming
}

// StartAll starts all enabled channels and begins message aggregation
func (r *Router) StartAll(ctx context.Context) error {
	r.mu.RLock()
	channels := make([]Channel, 0, len(r.channels))
	for _, ch := range r.channels {
		if ch.IsEnabled() {
			channels = append(channels, ch)
		}
	}
	r.mu.RUnlock()

	// Start each channel
	for _, ch := range channels {
		if err := ch.Start(ctx); err != nil {
			return err
		}
	}

	// Start message aggregation goroutines
	for _, ch := range channels {
		go r.aggregateMessages(ctx, ch)
	}

	return nil
}

// aggregateMessages forwards messages from a channel to the unified incoming channel
func (r *Router) aggregateMessages(ctx context.Context, ch Channel) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-r.done:
			return
		case msg, ok := <-ch.Incoming():
			if !ok {
				return
			}
			select {
			case r.incoming <- msg:
			case <-ctx.Done():
				return
			case <-r.done:
				return
			}
		}
	}
}

// StopAll stops all channels
func (r *Router) StopAll() error {
	close(r.done)

	r.mu.RLock()
	defer r.mu.RUnlock()

	var lastErr error
	for _, ch := range r.channels {
		if err := ch.Stop(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// SendToChannel sends a message to a specific channel
func (r *Router) SendToChannel(channelName, userID string, msg *OutboundMessage) error {
	ch, ok := r.Get(channelName)
	if !ok {
		return ErrChannelNotFound
	}
	return ch.SendMessage(userID, msg)
}

// SendApprovalToChannel sends an approval request to a specific channel
func (r *Router) SendApprovalToChannel(channelName, userID string, req *ApprovalRequest) error {
	ch, ok := r.Get(channelName)
	if !ok {
		return ErrChannelNotFound
	}
	return ch.SendApprovalRequest(userID, req)
}

// BroadcastMessage sends a message to all enabled channels for a user
func (r *Router) BroadcastMessage(userID string, msg *OutboundMessage) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var lastErr error
	for _, ch := range r.channels {
		if ch.IsEnabled() {
			if err := ch.SendMessage(userID, msg); err != nil {
				lastErr = err
			}
		}
	}
	return lastErr
}

// -----------------------------------------------------------------------------
// Helper functions for building approval UI
// -----------------------------------------------------------------------------

// ApprovalButtons returns the buttons to display for an approval request.
// This is a helper for channel implementations to render approval dialogs.
func ApprovalButtons(req *ApprovalRequest, supportsEditing bool) []Button {
	buttons := []Button{
		{
			ID:    string(ActionApprove),
			Label: "Approve",
			Style: ButtonPrimary,
		},
		{
			ID:    string(ActionDeny),
			Label: "Deny",
			Style: ButtonDanger,
		},
	}

	if req.Options.ShowAlwaysAllow {
		buttons = append(buttons, Button{
			ID:    string(ActionAlwaysAllow),
			Label: "Always Allow",
			Style: ButtonSecondary,
		})
	}

	if req.Options.ShowAllowDir && req.WorkingDir != "" {
		buttons = append(buttons, Button{
			ID:    string(ActionAllowDir),
			Label: "Allow in Dir",
			Style: ButtonSecondary,
		})
	}

	if req.Options.ShowEdit && supportsEditing {
		buttons = append(buttons, Button{
			ID:    string(ActionEdit),
			Label: "Edit",
			Style: ButtonSecondary,
		})
	}

	return buttons
}

// FormatApprovalMessage creates a formatted message for an approval request.
// This is a helper for channel implementations.
func FormatApprovalMessage(req *ApprovalRequest) string {
	msg := "🔐 **Approval Required**\n\n"
	msg += "**Tool:** " + req.Tool + "\n"
	msg += "**Command:** `" + req.Command + "`\n"

	if req.WorkingDir != "" {
		msg += "**Directory:** " + req.WorkingDir + "\n"
	}

	msg += "**Risk Level:** " + riskEmoji(req.RiskLevel) + " " + string(req.RiskLevel) + "\n"

	if req.Reason != "" {
		msg += "\n**Reason:** " + req.Reason + "\n"
	}

	if req.Options.Message != "" {
		msg += "\n" + req.Options.Message + "\n"
	}

	return msg
}

// riskEmoji returns an emoji for the risk level
func riskEmoji(level RiskLevel) string {
	switch level {
	case RiskLow:
		return "🟢"
	case RiskMedium:
		return "🟡"
	case RiskHigh:
		return "🔴"
	default:
		return "⚪"
	}
}

// -----------------------------------------------------------------------------
// BaseChannel provides a partial implementation for common channel functionality
// -----------------------------------------------------------------------------

// BaseChannel provides common functionality that can be embedded by channel implementations.
// It handles approval callback registration and capability defaults.
type BaseChannel struct {
	name             string
	enabled          bool
	approvalCallback ApprovalCallback
	callbackMu       sync.RWMutex
	incoming         chan *InboundMessage
}

// NewBaseChannel creates a new base channel
func NewBaseChannel(name string, enabled bool) *BaseChannel {
	return &BaseChannel{
		name:     name,
		enabled:  enabled,
		incoming: make(chan *InboundMessage, 100),
	}
}

// Name returns the channel name
func (b *BaseChannel) Name() string {
	return b.name
}

// IsEnabled returns whether the channel is enabled
func (b *BaseChannel) IsEnabled() bool {
	return b.enabled
}

// SetEnabled enables or disables the channel
func (b *BaseChannel) SetEnabled(enabled bool) {
	b.enabled = enabled
}

// Incoming returns the channel for incoming messages
func (b *BaseChannel) Incoming() <-chan *InboundMessage {
	return b.incoming
}

// EnqueueMessage adds a message to the incoming queue
func (b *BaseChannel) EnqueueMessage(msg *InboundMessage) {
	select {
	case b.incoming <- msg:
	default:
		// Channel full, drop message (or could log warning)
	}
}

// SetApprovalCallback sets the approval callback
func (b *BaseChannel) SetApprovalCallback(callback ApprovalCallback) {
	b.callbackMu.Lock()
	defer b.callbackMu.Unlock()
	b.approvalCallback = callback
}

// InvokeApprovalCallback calls the approval callback if set
func (b *BaseChannel) InvokeApprovalCallback(requestID string, action ApprovalAction, modifiedCommand string) {
	b.callbackMu.RLock()
	callback := b.approvalCallback
	b.callbackMu.RUnlock()

	if callback != nil {
		callback(requestID, action, modifiedCommand)
	}
}

// SupportsMedia returns false by default
func (b *BaseChannel) SupportsMedia() bool {
	return false
}

// SupportsButtons returns false by default
func (b *BaseChannel) SupportsButtons() bool {
	return false
}

// SupportsThreading returns false by default
func (b *BaseChannel) SupportsThreading() bool {
	return false
}

// SupportsEditing returns false by default
func (b *BaseChannel) SupportsEditing() bool {
	return false
}

// Close closes the incoming channel
func (b *BaseChannel) Close() {
	close(b.incoming)
}

// -----------------------------------------------------------------------------
// NewApprovalRequest is a helper to create approval requests with sensible defaults
// -----------------------------------------------------------------------------

// NewApprovalRequest creates a new approval request with defaults
func NewApprovalRequest(id, userID, tool, command string) *ApprovalRequest {
	return &ApprovalRequest{
		ID:        id,
		UserID:    userID,
		Tool:      tool,
		Command:   command,
		RiskLevel: RiskMedium,
		CreatedAt: time.Now(),
		Options: ApprovalOptions{
			ShowAlwaysAllow: true,
			ShowAllowDir:    true,
			ShowEdit:        false,
		},
	}
}

// WithRiskLevel sets the risk level
func (r *ApprovalRequest) WithRiskLevel(level RiskLevel) *ApprovalRequest {
	r.RiskLevel = level
	return r
}

// WithWorkingDir sets the working directory
func (r *ApprovalRequest) WithWorkingDir(dir string) *ApprovalRequest {
	r.WorkingDir = dir
	return r
}

// WithReason sets the reason for the request
func (r *ApprovalRequest) WithReason(reason string) *ApprovalRequest {
	r.Reason = reason
	return r
}

// WithArgs sets the tool arguments
func (r *ApprovalRequest) WithArgs(args map[string]any) *ApprovalRequest {
	r.Args = args
	return r
}

// WithTimeout sets when the request expires
func (r *ApprovalRequest) WithTimeout(d time.Duration) *ApprovalRequest {
	r.ExpiresAt = time.Now().Add(d)
	r.Options.Timeout = d
	return r
}

// WithMessage sets a custom message
func (r *ApprovalRequest) WithMessage(msg string) *ApprovalRequest {
	r.Options.Message = msg
	return r
}
