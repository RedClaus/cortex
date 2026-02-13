// Package slack provides a Slack channel adapter for Pinky
package slack

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/normanking/pinky/internal/channels"
	"github.com/normanking/pinky/internal/config"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

// Adapter implements the channels.Channel interface for Slack
type Adapter struct {
	config   config.SlackConfig
	client   *slack.Client
	socket   *socketmode.Client
	logger   *slog.Logger
	incoming chan *channels.InboundMessage

	// Approval handling
	pendingApprovals map[string]*pendingApproval
	approvalMu       sync.RWMutex
	approvalChan     chan *approvalResponse

	// State
	running bool
	mu      sync.RWMutex
	cancel  context.CancelFunc
}

// pendingApproval tracks an approval request waiting for user response
type pendingApproval struct {
	Request   *channels.ApprovalRequest
	UserID    string
	ChannelID string
	MessageTS string
	CreatedAt time.Time
}

// approvalResponse represents a user's response to an approval request
type approvalResponse struct {
	RequestID string
	Approved  bool
	UserID    string
}

// New creates a new Slack adapter
func New(cfg config.SlackConfig, logger *slog.Logger) *Adapter {
	if logger == nil {
		logger = slog.Default()
	}

	return &Adapter{
		config:           cfg,
		logger:           logger.With("channel", "slack"),
		incoming:         make(chan *channels.InboundMessage, 100),
		pendingApprovals: make(map[string]*pendingApproval),
		approvalChan:     make(chan *approvalResponse, 10),
	}
}

// Name returns the channel name
func (a *Adapter) Name() string {
	return "slack"
}

// IsEnabled returns whether the channel is enabled
func (a *Adapter) IsEnabled() bool {
	return a.config.Enabled
}

// SupportsMedia returns true - Slack supports file uploads
func (a *Adapter) SupportsMedia() bool {
	return true
}

// SupportsButtons returns true - Slack has rich interactive components
func (a *Adapter) SupportsButtons() bool {
	return true
}

// SupportsThreading returns true - Slack has thread support
func (a *Adapter) SupportsThreading() bool {
	return true
}

// Start initializes and starts the Slack adapter
func (a *Adapter) Start(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.running {
		return nil
	}

	if a.config.Token == "" {
		return fmt.Errorf("slack bot token is required")
	}
	if a.config.AppToken == "" {
		return fmt.Errorf("slack app token is required for socket mode")
	}

	// Create Slack client with bot token
	a.client = slack.New(
		a.config.Token,
		slack.OptionAppLevelToken(a.config.AppToken),
	)

	// Create Socket Mode client for real-time events
	a.socket = socketmode.New(
		a.client,
		socketmode.OptionDebug(false),
	)

	// Create cancellable context
	ctx, cancel := context.WithCancel(ctx)
	a.cancel = cancel
	a.running = true

	// Start event handler
	go a.handleEvents(ctx)

	// Start Socket Mode connection
	go func() {
		if err := a.socket.Run(); err != nil {
			a.logger.Error("Socket Mode error", "error", err)
		}
	}()

	a.logger.Info("Slack adapter started")
	return nil
}

// Stop gracefully shuts down the Slack adapter
func (a *Adapter) Stop() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.running {
		return nil
	}

	if a.cancel != nil {
		a.cancel()
	}

	a.running = false
	close(a.incoming)
	a.logger.Info("Slack adapter stopped")
	return nil
}

// Incoming returns the channel for receiving messages
func (a *Adapter) Incoming() <-chan *channels.InboundMessage {
	return a.incoming
}

// SendMessage sends a message to a Slack user or channel
func (a *Adapter) SendMessage(userID string, msg *channels.OutboundMessage) error {
	if a.client == nil {
		return fmt.Errorf("slack client not initialized")
	}

	// Build message options
	opts := []slack.MsgOption{
		slack.MsgOptionText(a.formatContent(msg.Content, msg.Format), false),
	}

	// Add threading support
	if msg.ReplyTo != "" {
		opts = append(opts, slack.MsgOptionTS(msg.ReplyTo))
	}

	// Add buttons if present
	if len(msg.Buttons) > 0 {
		opts = append(opts, slack.MsgOptionAttachments(a.buildButtonAttachment(msg.Buttons, "")))
	}

	_, _, err := a.client.PostMessage(userID, opts...)
	if err != nil {
		a.logger.Error("Failed to send message", "error", err, "user", userID)
		return fmt.Errorf("failed to send slack message: %w", err)
	}

	return nil
}

// SendApprovalRequest sends a tool approval request with interactive buttons
func (a *Adapter) SendApprovalRequest(userID string, req *channels.ApprovalRequest) error {
	if a.client == nil {
		return fmt.Errorf("slack client not initialized")
	}

	// Build the approval message with rich formatting
	blocks := a.buildApprovalBlocks(req)

	// Send the message
	_, ts, err := a.client.PostMessage(
		userID,
		slack.MsgOptionBlocks(blocks...),
	)
	if err != nil {
		a.logger.Error("Failed to send approval request", "error", err, "request_id", req.ID)
		return fmt.Errorf("failed to send approval request: %w", err)
	}

	// Track the pending approval
	a.approvalMu.Lock()
	a.pendingApprovals[req.ID] = &pendingApproval{
		Request:   req,
		UserID:    userID,
		ChannelID: userID, // For DMs, channel ID is the user ID
		MessageTS: ts,
		CreatedAt: time.Now(),
	}
	a.approvalMu.Unlock()

	a.logger.Debug("Approval request sent", "request_id", req.ID, "user", userID)
	return nil
}

// SendToolOutput sends tool execution results
func (a *Adapter) SendToolOutput(userID string, output *channels.ToolOutput) error {
	if a.client == nil {
		return fmt.Errorf("slack client not initialized")
	}

	// Build the output message
	blocks := a.buildToolOutputBlocks(output)

	_, _, err := a.client.PostMessage(
		userID,
		slack.MsgOptionBlocks(blocks...),
	)
	if err != nil {
		a.logger.Error("Failed to send tool output", "error", err, "tool", output.Tool)
		return fmt.Errorf("failed to send tool output: %w", err)
	}

	return nil
}

// handleEvents processes incoming Socket Mode events
func (a *Adapter) handleEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case evt := <-a.socket.Events:
			switch evt.Type {
			case socketmode.EventTypeEventsAPI:
				a.handleEventsAPI(evt)
			case socketmode.EventTypeInteractive:
				a.handleInteraction(evt)
			case socketmode.EventTypeSlashCommand:
				a.handleSlashCommand(evt)
			case socketmode.EventTypeConnecting:
				a.logger.Debug("Connecting to Slack...")
			case socketmode.EventTypeConnected:
				a.logger.Info("Connected to Slack")
			case socketmode.EventTypeConnectionError:
				a.logger.Error("Slack connection error")
			}
		}
	}
}

// handleEventsAPI processes Events API payloads
func (a *Adapter) handleEventsAPI(evt socketmode.Event) {
	eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
	if !ok {
		return
	}

	a.socket.Ack(*evt.Request)

	switch eventsAPIEvent.Type {
	case slackevents.CallbackEvent:
		a.handleCallbackEvent(eventsAPIEvent)
	}
}

// handleCallbackEvent processes callback events (messages, mentions, etc.)
func (a *Adapter) handleCallbackEvent(event slackevents.EventsAPIEvent) {
	switch ev := event.InnerEvent.Data.(type) {
	case *slackevents.MessageEvent:
		// Ignore bot messages and message changes
		if ev.BotID != "" || ev.SubType != "" {
			return
		}

		msg := &channels.InboundMessage{
			ID:          ev.TimeStamp,
			UserID:      ev.User,
			ChannelName: "slack",
			ChannelID:   ev.Channel,
			Content:     ev.Text,
			ReplyTo:     ev.ThreadTimeStamp,
			ReceivedAt:  time.Now(),
			Metadata: map[string]string{
				"team": event.TeamID,
			},
		}

		select {
		case a.incoming <- msg:
		default:
			a.logger.Warn("Incoming message channel full, dropping message")
		}

	case *slackevents.AppMentionEvent:
		// Handle @mentions of the bot
		msg := &channels.InboundMessage{
			ID:          ev.TimeStamp,
			UserID:      ev.User,
			ChannelName: "slack",
			ChannelID:   ev.Channel,
			Content:     ev.Text,
			ReplyTo:     ev.ThreadTimeStamp,
			ReceivedAt:  time.Now(),
			Metadata: map[string]string{
				"type": "mention",
			},
		}

		select {
		case a.incoming <- msg:
		default:
			a.logger.Warn("Incoming message channel full, dropping message")
		}
	}
}

// handleInteraction processes interactive component callbacks (button clicks)
func (a *Adapter) handleInteraction(evt socketmode.Event) {
	callback, ok := evt.Data.(slack.InteractionCallback)
	if !ok {
		return
	}

	// Acknowledge the interaction immediately
	a.socket.Ack(*evt.Request)

	switch callback.Type {
	case slack.InteractionTypeBlockActions:
		a.handleBlockActions(callback)
	}
}

// handleBlockActions processes block action interactions (button clicks)
func (a *Adapter) handleBlockActions(callback slack.InteractionCallback) {
	for _, action := range callback.ActionCallback.BlockActions {
		// Parse the action ID to determine what was clicked
		// Format: pinky_approve_<request_id> or pinky_deny_<request_id>
		parts := strings.Split(action.ActionID, "_")
		if len(parts) < 3 || parts[0] != "pinky" {
			continue
		}

		actionType := parts[1]
		requestID := strings.Join(parts[2:], "_")

		a.approvalMu.RLock()
		_, exists := a.pendingApprovals[requestID]
		a.approvalMu.RUnlock()

		if !exists {
			a.logger.Warn("Received action for unknown approval", "request_id", requestID)
			a.updateApprovalMessage(callback.Channel.ID, callback.Message.Timestamp,
				"This approval request has expired or was already handled.")
			continue
		}

		approved := actionType == "approve"

		// Update the message to show the decision
		statusText := "Denied"
		statusEmoji := ":x:"
		if approved {
			statusText = "Approved"
			statusEmoji = ":white_check_mark:"
		}

		a.updateApprovalMessage(
			callback.Channel.ID,
			callback.Message.Timestamp,
			fmt.Sprintf("%s *%s* by <@%s>", statusEmoji, statusText, callback.User.ID),
		)

		// Remove from pending
		a.approvalMu.Lock()
		delete(a.pendingApprovals, requestID)
		a.approvalMu.Unlock()

		// Send the response
		select {
		case a.approvalChan <- &approvalResponse{
			RequestID: requestID,
			Approved:  approved,
			UserID:    callback.User.ID,
		}:
		default:
			a.logger.Error("Approval response channel full")
		}

		a.logger.Info("Approval action received",
			"request_id", requestID,
			"approved", approved,
			"user", callback.User.ID,
		)
	}
}

// handleSlashCommand processes slash commands
func (a *Adapter) handleSlashCommand(evt socketmode.Event) {
	cmd, ok := evt.Data.(slack.SlashCommand)
	if !ok {
		return
	}

	a.socket.Ack(*evt.Request)

	// Convert slash command to a message
	msg := &channels.InboundMessage{
		ID:          fmt.Sprintf("cmd_%d", time.Now().UnixNano()),
		UserID:      cmd.UserID,
		ChannelName: "slack",
		ChannelID:   cmd.ChannelID,
		Content:     fmt.Sprintf("/%s %s", cmd.Command, cmd.Text),
		ReceivedAt:  time.Now(),
		Metadata: map[string]string{
			"type":    "slash_command",
			"command": cmd.Command,
		},
	}

	select {
	case a.incoming <- msg:
	default:
		a.logger.Warn("Incoming message channel full, dropping slash command")
	}
}

// buildApprovalBlocks creates Slack Block Kit blocks for an approval request
func (a *Adapter) buildApprovalBlocks(req *channels.ApprovalRequest) []slack.Block {
	// Header section
	headerText := slack.NewTextBlockObject("mrkdwn",
		":warning: *Tool Approval Required*", false, false)
	headerSection := slack.NewSectionBlock(headerText, nil, nil)

	// Tool info section
	riskEmoji := a.riskEmoji(string(req.RiskLevel))
	infoText := slack.NewTextBlockObject("mrkdwn",
		fmt.Sprintf("*Tool:* `%s` %s\n*Risk Level:* %s\n*Working Directory:* `%s`",
			req.Tool, riskEmoji, req.RiskLevel, req.WorkingDir),
		false, false)
	infoSection := slack.NewSectionBlock(infoText, nil, nil)

	// Command section
	commandText := slack.NewTextBlockObject("mrkdwn",
		fmt.Sprintf("```%s```", req.Command), false, false)
	commandSection := slack.NewSectionBlock(commandText, nil, nil)

	// Reason section (if provided)
	var reasonSection *slack.SectionBlock
	if req.Reason != "" {
		reasonText := slack.NewTextBlockObject("mrkdwn",
			fmt.Sprintf("*Reason:* %s", req.Reason), false, false)
		reasonSection = slack.NewSectionBlock(reasonText, nil, nil)
	}

	// Action buttons
	approveBtn := slack.NewButtonBlockElement(
		fmt.Sprintf("pinky_approve_%s", req.ID),
		"approve",
		slack.NewTextBlockObject("plain_text", "Approve", true, false),
	)
	approveBtn.Style = slack.StylePrimary

	denyBtn := slack.NewButtonBlockElement(
		fmt.Sprintf("pinky_deny_%s", req.ID),
		"deny",
		slack.NewTextBlockObject("plain_text", "Deny", true, false),
	)
	denyBtn.Style = slack.StyleDanger

	actionsBlock := slack.NewActionBlock(
		fmt.Sprintf("pinky_actions_%s", req.ID),
		approveBtn,
		denyBtn,
	)

	// Assemble blocks
	blocks := []slack.Block{
		headerSection,
		slack.NewDividerBlock(),
		infoSection,
		commandSection,
	}

	if reasonSection != nil {
		blocks = append(blocks, reasonSection)
	}

	blocks = append(blocks, slack.NewDividerBlock(), actionsBlock)

	return blocks
}

// buildToolOutputBlocks creates Slack Block Kit blocks for tool output
func (a *Adapter) buildToolOutputBlocks(output *channels.ToolOutput) []slack.Block {
	// Status emoji and text
	statusEmoji := ":white_check_mark:"
	statusText := "Success"
	if !output.Success {
		statusEmoji = ":x:"
		statusText = "Failed"
	}

	// Header
	headerText := slack.NewTextBlockObject("mrkdwn",
		fmt.Sprintf("%s *Tool Execution: %s*", statusEmoji, statusText),
		false, false)
	headerSection := slack.NewSectionBlock(headerText, nil, nil)

	// Tool info
	infoText := slack.NewTextBlockObject("mrkdwn",
		fmt.Sprintf("*Tool:* `%s`\n*Duration:* %s", output.Tool, output.Duration.Round(time.Millisecond)),
		false, false)
	infoSection := slack.NewSectionBlock(infoText, nil, nil)

	blocks := []slack.Block{
		headerSection,
		infoSection,
	}

	// Output or error
	if output.Output != "" {
		outputText := output.Output
		// Truncate long output
		if len(outputText) > 2900 {
			outputText = outputText[:2900] + "\n... (truncated)"
		}

		outputBlock := slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("```%s```", outputText), false, false),
			nil, nil)
		blocks = append(blocks, outputBlock)
	}

	if output.Error != "" {
		errorBlock := slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", fmt.Sprintf(":warning: *Error:*\n```%s```", output.Error), false, false),
			nil, nil)
		blocks = append(blocks, errorBlock)
	}

	return blocks
}

// buildButtonAttachment creates an attachment with buttons
func (a *Adapter) buildButtonAttachment(buttons []channels.Button, callbackID string) slack.Attachment {
	var actions []slack.AttachmentAction

	for _, btn := range buttons {
		action := slack.AttachmentAction{
			Name:  btn.ID,
			Text:  btn.Label,
			Type:  "button",
			Value: btn.ID,
		}

		switch btn.Style {
		case channels.ButtonPrimary:
			action.Style = "primary"
		case channels.ButtonDanger:
			action.Style = "danger"
		default:
			action.Style = "default"
		}

		actions = append(actions, action)
	}

	return slack.Attachment{
		CallbackID: callbackID,
		Actions:    actions,
	}
}

// updateApprovalMessage updates an approval message after user action
func (a *Adapter) updateApprovalMessage(channelID, messageTS, text string) {
	_, _, _, err := a.client.UpdateMessage(
		channelID,
		messageTS,
		slack.MsgOptionText(text, false),
		slack.MsgOptionAttachments(), // Remove buttons
	)
	if err != nil {
		a.logger.Error("Failed to update approval message", "error", err)
	}
}

// formatContent formats message content based on format type
func (a *Adapter) formatContent(content string, format channels.MessageFormat) string {
	switch format {
	case channels.FormatCode:
		return fmt.Sprintf("```%s```", content)
	case channels.FormatMarkdown:
		// Slack uses mrkdwn which is similar but not identical to Markdown
		// Basic conversion: **bold** -> *bold*, _italic_ stays same
		content = strings.ReplaceAll(content, "**", "*")
		return content
	default:
		return content
	}
}

// riskEmoji returns an emoji for the risk level
func (a *Adapter) riskEmoji(riskLevel string) string {
	switch strings.ToLower(riskLevel) {
	case "high":
		return ":rotating_light:"
	case "medium":
		return ":warning:"
	case "low":
		return ":information_source:"
	default:
		return ""
	}
}

// GetApprovalResponse waits for and returns an approval response
// This is used by the agent loop to get user approval decisions
func (a *Adapter) GetApprovalResponse(ctx context.Context, requestID string) (bool, error) {
	for {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case resp := <-a.approvalChan:
			if resp.RequestID == requestID {
				return resp.Approved, nil
			}
			// Not our request, put it back (this is a simple approach)
			// In production, consider using a map of channels per request
			go func() {
				select {
				case a.approvalChan <- resp:
				case <-time.After(time.Second):
				}
			}()
		}
	}
}
