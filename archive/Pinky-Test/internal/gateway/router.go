// Package gateway routes messages between channels and the agent loop.
package gateway

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/normanking/pinky/internal/agent"
	"github.com/normanking/pinky/internal/channels"
	"github.com/normanking/pinky/internal/identity"
	"github.com/normanking/pinky/internal/permissions"
	"github.com/normanking/pinky/internal/persona"
	"github.com/normanking/pinky/internal/tools"
)

// Router routes messages from channels to the agent loop and back.
// It maintains session state and handles cross-channel user identity.
type Router struct {
	mu sync.RWMutex

	// Dependencies
	channels  map[string]channels.Channel
	identity  *identity.Service
	agent     *agent.Loop
	personas  *persona.Manager

	// Session management
	sessions map[string]*Session // keyed by user ID

	// Configuration
	defaultWorkingDir string

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// Session tracks an active user session across channels.
type Session struct {
	UserID        string
	User          *identity.User
	ActiveChannel string    // Currently active channel
	ChannelID     string    // Platform-specific channel/chat ID
	LastMessageAt time.Time
	CreatedAt     time.Time

	// Persona for this session
	Persona *persona.Persona

	// Working directory for shell operations
	WorkingDir string

	// Conversation state is managed by the agent loop
}

// Config configures the router.
type Config struct {
	Identity          *identity.Service
	Agent             *agent.Loop
	Personas          *persona.Manager
	DefaultWorkingDir string
}

// New creates a new message router.
func New(cfg Config) *Router {
	return &Router{
		channels:          make(map[string]channels.Channel),
		identity:          cfg.Identity,
		agent:             cfg.Agent,
		personas:          cfg.Personas,
		sessions:          make(map[string]*Session),
		defaultWorkingDir: cfg.DefaultWorkingDir,
	}
}

// RegisterChannel adds a channel to the router.
func (r *Router) RegisterChannel(ch channels.Channel) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.channels[ch.Name()] = ch
}

// GetChannel retrieves a channel by name.
func (r *Router) GetChannel(name string) (channels.Channel, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ch, ok := r.channels[name]
	return ch, ok
}

// Start begins routing messages from all enabled channels.
func (r *Router) Start(ctx context.Context) error {
	r.ctx, r.cancel = context.WithCancel(ctx)

	// Start all enabled channels
	for name, ch := range r.channels {
		if !ch.IsEnabled() {
			continue
		}

		if err := ch.Start(r.ctx); err != nil {
			return fmt.Errorf("failed to start channel %s: %w", name, err)
		}

		// Start message handler goroutine for this channel
		r.wg.Add(1)
		go r.handleChannel(ch)
	}

	return nil
}

// Stop gracefully shuts down the router.
func (r *Router) Stop() error {
	if r.cancel != nil {
		r.cancel()
	}

	// Wait for all handlers to finish
	r.wg.Wait()

	// Stop all channels
	for _, ch := range r.channels {
		if err := ch.Stop(); err != nil {
			return err
		}
	}

	return nil
}

// handleChannel processes messages from a single channel.
func (r *Router) handleChannel(ch channels.Channel) {
	defer r.wg.Done()

	for {
		select {
		case <-r.ctx.Done():
			return
		case msg, ok := <-ch.Incoming():
			if !ok {
				return
			}
			r.routeMessage(ch, msg)
		}
	}
}

// routeMessage processes an incoming message and routes it to the agent.
func (r *Router) routeMessage(ch channels.Channel, msg *channels.InboundMessage) {
	// 1. Resolve user identity
	user := r.identity.GetOrCreate(msg.ChannelName, msg.UserID, extractUsername(msg))

	// 2. Get or create session
	session := r.getOrCreateSession(user, msg.ChannelName, msg.ChannelID)

	// 3. Resolve persona for this user
	personaInstance := r.resolvePersona(user)
	session.Persona = personaInstance

	// 4. Set up approval handler for this channel
	r.agent.SetApprovalHandler(func(req *permissions.ApprovalRequest) (*permissions.ApprovalResponse, error) {
		return r.handleApproval(ch, session, req)
	})

	// 5. Set up tool output handler
	r.agent.SetToolCompleteHandler(func(name string, output *tools.ToolOutput) {
		r.sendToolOutput(ch, session, name, output)
	})

	// 6. Build agent request
	agentReq := &agent.Request{
		UserID:     user.ID,
		Content:    msg.Content,
		Channel:    msg.ChannelName,
		Persona:    personaInstance,
		WorkingDir: session.WorkingDir,
	}

	// 7. Process through agent loop
	resp, err := r.agent.Process(r.ctx, agentReq)
	if err != nil {
		r.sendError(ch, session, err)
		return
	}

	// 8. Send response back through channel
	r.sendResponse(ch, session, resp)
}

// getOrCreateSession retrieves or creates a session for a user.
func (r *Router) getOrCreateSession(user *identity.User, channelName, channelID string) *Session {
	r.mu.Lock()
	defer r.mu.Unlock()

	if session, ok := r.sessions[user.ID]; ok {
		// Update active channel
		session.ActiveChannel = channelName
		session.ChannelID = channelID
		session.LastMessageAt = time.Now()
		return session
	}

	// Create new session
	session := &Session{
		UserID:        user.ID,
		User:          user,
		ActiveChannel: channelName,
		ChannelID:     channelID,
		LastMessageAt: time.Now(),
		CreatedAt:     time.Now(),
		WorkingDir:    r.defaultWorkingDir,
	}

	r.sessions[user.ID] = session
	return session
}

// GetSession retrieves a session by user ID.
func (r *Router) GetSession(userID string) (*Session, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	session, ok := r.sessions[userID]
	return session, ok
}

// resolvePersona gets the persona for a user.
func (r *Router) resolvePersona(user *identity.User) *persona.Persona {
	if r.personas == nil {
		return nil
	}

	// Use user's preferred persona
	if user.Persona != "" {
		if p, err := r.personas.Get(user.Persona); err == nil {
			return p
		}
	}

	// Fall back to current/default persona
	if p, err := r.personas.Current(); err == nil {
		return p
	}

	return nil
}

// handleApproval requests user approval through the active channel.
func (r *Router) handleApproval(ch channels.Channel, session *Session, req *permissions.ApprovalRequest) (*permissions.ApprovalResponse, error) {
	// Send approval request to user
	approvalReq := &channels.ApprovalRequest{
		ID:         req.ID,
		Tool:       req.Tool,
		Command:    req.Command,
		RiskLevel:  channels.RiskLevel(req.RiskLevel),
		WorkingDir: req.WorkingDir,
		Reason:     req.Reason,
	}

	if err := ch.SendApprovalRequest(session.ChannelID, approvalReq); err != nil {
		return nil, fmt.Errorf("failed to send approval request: %w", err)
	}

	// Wait for response (this would typically be handled async via button callback)
	// For now, we'll need a mechanism to receive the response
	// This is a placeholder - real implementation would use a callback system
	return r.waitForApproval(session, req.ID)
}

// waitForApproval waits for user to respond to an approval request.
// In practice, this would be handled via button callbacks or message polling.
func (r *Router) waitForApproval(session *Session, requestID string) (*permissions.ApprovalResponse, error) {
	// This is a synchronous placeholder.
	// Real implementation would:
	// 1. Store pending approval request
	// 2. Wait for button click or message response
	// 3. Parse response and return

	// For TUI/WebUI, this might be handled differently (blocking prompt)
	// For Telegram/Discord, this would use inline buttons

	// Default: auto-deny after timeout
	select {
	case <-r.ctx.Done():
		return &permissions.ApprovalResponse{Approved: false}, r.ctx.Err()
	case <-time.After(5 * time.Minute):
		return &permissions.ApprovalResponse{Approved: false}, nil
	}
}

// sendToolOutput sends tool execution results to the user.
func (r *Router) sendToolOutput(ch channels.Channel, session *Session, toolName string, output *tools.ToolOutput) {
	if output == nil {
		return
	}

	toolOutput := &channels.ToolOutput{
		Tool:     toolName,
		Success:  output.Success,
		Output:   output.Output,
		Error:    output.Error,
		Duration: output.Duration,
	}

	// Non-blocking send - errors are logged but don't stop execution
	_ = ch.SendToolOutput(session.ChannelID, toolOutput)
}

// sendResponse sends the agent's response back to the user.
func (r *Router) sendResponse(ch channels.Channel, session *Session, resp *agent.Response) {
	msg := &channels.OutboundMessage{
		Content: resp.Content,
		Format:  channels.FormatMarkdown,
	}

	if err := ch.SendMessage(session.ChannelID, msg); err != nil {
		// Log error but don't propagate - message already processed
		fmt.Printf("failed to send response to %s: %v\n", session.ActiveChannel, err)
	}
}

// sendError sends an error message to the user.
func (r *Router) sendError(ch channels.Channel, session *Session, err error) {
	msg := &channels.OutboundMessage{
		Content: fmt.Sprintf("Sorry, I encountered an error: %v", err),
		Format:  channels.FormatPlain,
	}

	_ = ch.SendMessage(session.ChannelID, msg)
}

// extractUsername gets a display name from the message metadata.
func extractUsername(msg *channels.InboundMessage) string {
	if name, ok := msg.Metadata["username"]; ok {
		return name
	}
	if name, ok := msg.Metadata["display_name"]; ok {
		return name
	}
	return msg.UserID
}

// SessionStats returns statistics about active sessions.
func (r *Router) SessionStats() SessionStatistics {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := SessionStatistics{
		TotalSessions:   len(r.sessions),
		ByChannel:       make(map[string]int),
		ActiveLastHour:  0,
		ActiveLastDay:   0,
	}

	now := time.Now()
	hourAgo := now.Add(-1 * time.Hour)
	dayAgo := now.Add(-24 * time.Hour)

	for _, session := range r.sessions {
		stats.ByChannel[session.ActiveChannel]++
		if session.LastMessageAt.After(hourAgo) {
			stats.ActiveLastHour++
		}
		if session.LastMessageAt.After(dayAgo) {
			stats.ActiveLastDay++
		}
	}

	return stats
}

// SessionStatistics contains session metrics.
type SessionStatistics struct {
	TotalSessions  int
	ByChannel      map[string]int
	ActiveLastHour int
	ActiveLastDay  int
}

// CleanupStale removes sessions that haven't been active for the given duration.
func (r *Router) CleanupStale(maxAge time.Duration) int {
	r.mu.Lock()
	defer r.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	removed := 0

	for userID, session := range r.sessions {
		if session.LastMessageAt.Before(cutoff) {
			delete(r.sessions, userID)
			removed++
		}
	}

	return removed
}
