// Package discord provides a Discord channel adapter for Pinky
package discord

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/normanking/pinky/internal/channels"
	"github.com/normanking/pinky/internal/config"
)

// Adapter implements the channels.Channel interface for Discord
type Adapter struct {
	config   config.DiscordConfig
	session  *discordgo.Session
	logger   *slog.Logger
	incoming chan *channels.InboundMessage

	// Approval handling
	pendingApprovals map[string]*pendingApproval
	approvalMu       sync.RWMutex
	approvalCallback channels.ApprovalCallback
	callbackMu       sync.RWMutex

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
	MessageID string
	CreatedAt time.Time
}

// New creates a new Discord adapter
func New(cfg config.DiscordConfig, logger *slog.Logger) *Adapter {
	if logger == nil {
		logger = slog.Default()
	}

	return &Adapter{
		config:           cfg,
		logger:           logger.With("channel", "discord"),
		incoming:         make(chan *channels.InboundMessage, 100),
		pendingApprovals: make(map[string]*pendingApproval),
	}
}

// Name returns the channel name
func (a *Adapter) Name() string {
	return "discord"
}

// IsEnabled returns whether the channel is enabled
func (a *Adapter) IsEnabled() bool {
	return a.config.Enabled
}

// SupportsMedia returns true - Discord supports file uploads
func (a *Adapter) SupportsMedia() bool {
	return true
}

// SupportsButtons returns true - Discord has message components (buttons)
func (a *Adapter) SupportsButtons() bool {
	return true
}

// SupportsThreading returns true - Discord has thread support
func (a *Adapter) SupportsThreading() bool {
	return true
}

// SupportsEditing returns true - Discord supports message editing
func (a *Adapter) SupportsEditing() bool {
	return true
}

// Start initializes and starts the Discord adapter
func (a *Adapter) Start(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.running {
		return nil
	}

	if a.config.Token == "" {
		return fmt.Errorf("discord token not configured")
	}

	// Create Discord session
	session, err := discordgo.New("Bot " + a.config.Token)
	if err != nil {
		return fmt.Errorf("failed to create discord session: %w", err)
	}

	// Set intents for message content and guild messages
	session.Identify.Intents = discordgo.IntentsGuildMessages |
		discordgo.IntentsDirectMessages |
		discordgo.IntentsMessageContent

	// Register event handlers
	session.AddHandler(a.handleMessageCreate)
	session.AddHandler(a.handleInteractionCreate)

	// Open connection
	if err := session.Open(); err != nil {
		return fmt.Errorf("failed to open discord connection: %w", err)
	}

	a.session = session
	a.logger.Info("discord bot connected", "username", session.State.User.Username)

	// Create cancellable context
	ctx, cancel := context.WithCancel(ctx)
	a.cancel = cancel
	a.running = true

	// Start approval timeout handler
	go a.handleApprovalTimeouts(ctx)

	return nil
}

// Stop stops the Discord adapter
func (a *Adapter) Stop() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.running {
		return nil
	}

	if a.cancel != nil {
		a.cancel()
	}

	if a.session != nil {
		a.session.Close()
	}

	a.running = false
	a.logger.Info("discord adapter stopped")
	return nil
}

// Incoming returns the channel for incoming messages
func (a *Adapter) Incoming() <-chan *channels.InboundMessage {
	return a.incoming
}

// SendMessage sends a message to a user/channel
func (a *Adapter) SendMessage(userID string, msg *channels.OutboundMessage) error {
	channelID := userID // In Discord, we use channel ID directly

	content := msg.Content
	if msg.Format == channels.FormatCode {
		content = "```\n" + content + "\n```"
	}

	// Build message send data
	messageSend := &discordgo.MessageSend{
		Content: content,
	}

	// Add buttons if present
	if len(msg.Buttons) > 0 {
		var buttons []discordgo.MessageComponent
		for _, btn := range msg.Buttons {
			style := discordgo.PrimaryButton
			switch btn.Style {
			case channels.ButtonSecondary:
				style = discordgo.SecondaryButton
			case channels.ButtonDanger:
				style = discordgo.DangerButton
			}

			buttons = append(buttons, discordgo.Button{
				Label:    btn.Label,
				Style:    style,
				CustomID: btn.ID,
			})
		}

		messageSend.Components = []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: buttons,
			},
		}
	}

	// Reply threading (reference)
	if msg.ReplyTo != "" {
		messageSend.Reference = &discordgo.MessageReference{
			MessageID: msg.ReplyTo,
		}
	}

	_, err := a.session.ChannelMessageSendComplex(channelID, messageSend)
	return err
}

// SendApprovalRequest sends an approval request with approve/deny buttons
func (a *Adapter) SendApprovalRequest(userID string, req *channels.ApprovalRequest) error {
	channelID := userID // In Discord context, userID is typically the channel ID

	// Format approval message as embed
	embed := formatApprovalEmbed(req)

	// Create approve/deny buttons
	approveID := fmt.Sprintf("approve:%s", req.ID)
	denyID := fmt.Sprintf("deny:%s", req.ID)
	alwaysID := fmt.Sprintf("always:%s", req.ID)

	components := []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Label:    "✅ Approve",
					Style:    discordgo.SuccessButton,
					CustomID: approveID,
				},
				discordgo.Button{
					Label:    "❌ Deny",
					Style:    discordgo.DangerButton,
					CustomID: denyID,
				},
			},
		},
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Label:    "✅ Always Allow",
					Style:    discordgo.SecondaryButton,
					CustomID: alwaysID,
				},
			},
		},
	}

	sent, err := a.session.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
		Embeds:     []*discordgo.MessageEmbed{embed},
		Components: components,
	})
	if err != nil {
		return err
	}

	// Track pending approval
	a.approvalMu.Lock()
	a.pendingApprovals[req.ID] = &pendingApproval{
		Request:   req,
		UserID:    userID,
		ChannelID: channelID,
		MessageID: sent.ID,
		CreatedAt: time.Now(),
	}
	a.approvalMu.Unlock()

	return nil
}

// SendToolOutput sends the result of a tool execution
func (a *Adapter) SendToolOutput(userID string, output *channels.ToolOutput) error {
	channelID := userID

	var color int
	var status string
	if output.Success {
		color = 0x00FF00 // Green
		status = "✅ Success"
	} else {
		color = 0xFF0000 // Red
		status = "❌ Failed"
	}

	// Truncate long output
	content := output.Output
	if len(content) > 1000 {
		content = content[:1000] + "... (truncated)"
	}

	embed := &discordgo.MessageEmbed{
		Title: fmt.Sprintf("%s - %s", output.Tool, status),
		Color: color,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:  "Duration",
				Value: output.Duration.String(),
			},
		},
	}

	if content != "" {
		embed.Description = "```\n" + content + "\n```"
	}

	if output.Error != "" {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:  "Error",
			Value: output.Error,
		})
	}

	_, err := a.session.ChannelMessageSendEmbed(channelID, embed)
	return err
}

// SetApprovalCallback sets the callback for approval responses
func (a *Adapter) SetApprovalCallback(callback channels.ApprovalCallback) {
	a.callbackMu.Lock()
	defer a.callbackMu.Unlock()
	a.approvalCallback = callback
}

// DismissApproval removes an approval dialog
func (a *Adapter) DismissApproval(userID string, requestID string) error {
	a.approvalMu.Lock()
	pending, exists := a.pendingApprovals[requestID]
	if exists {
		delete(a.pendingApprovals, requestID)
	}
	a.approvalMu.Unlock()

	if !exists {
		return nil
	}

	// Edit message to show dismissed
	_, err := a.session.ChannelMessageEditComplex(&discordgo.MessageEdit{
		Channel:    pending.ChannelID,
		ID:         pending.MessageID,
		Content:    stringPtr("⏹️ Approval request dismissed"),
		Components: &[]discordgo.MessageComponent{}, // Remove buttons
	})

	return err
}

// handleMessageCreate processes incoming Discord messages
func (a *Adapter) handleMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore messages from the bot itself
	if m.Author.ID == s.State.User.ID {
		return
	}

	// Ignore empty messages
	if m.Content == "" && len(m.Attachments) == 0 {
		return
	}

	inbound := &channels.InboundMessage{
		ID:          m.ID,
		UserID:      m.ChannelID, // Use channel ID as the "user" context
		ChannelName: "discord",
		ChannelID:   m.ChannelID,
		Content:     m.Content,
		Metadata: map[string]string{
			"author_id":       m.Author.ID,
			"author_username": m.Author.Username,
			"guild_id":        m.GuildID,
		},
		ReceivedAt: m.Timestamp,
	}

	// Handle reply threading
	if m.ReferencedMessage != nil {
		inbound.ReplyTo = m.ReferencedMessage.ID
	}

	// Handle media attachments
	for _, attachment := range m.Attachments {
		mediaType := channels.MediaDocument
		if strings.HasPrefix(attachment.ContentType, "image/") {
			mediaType = channels.MediaImage
		} else if strings.HasPrefix(attachment.ContentType, "video/") {
			mediaType = channels.MediaVideo
		} else if strings.HasPrefix(attachment.ContentType, "audio/") {
			mediaType = channels.MediaAudio
		}

		inbound.Media = append(inbound.Media, channels.Media{
			Type:     mediaType,
			URL:      attachment.URL,
			Filename: attachment.Filename,
			MimeType: attachment.ContentType,
		})
	}

	// Send to incoming channel
	select {
	case a.incoming <- inbound:
	default:
		a.logger.Warn("incoming message channel full, dropping message")
	}
}

// handleInteractionCreate processes button clicks and other interactions
func (a *Adapter) handleInteractionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Only handle button interactions
	if i.Type != discordgo.InteractionMessageComponent {
		return
	}

	data := i.MessageComponentData()
	customID := data.CustomID

	// Parse callback data
	parts := strings.SplitN(customID, ":", 2)
	if len(parts) != 2 {
		a.logger.Warn("invalid interaction custom ID", "custom_id", customID)
		return
	}

	action := parts[0]
	requestID := parts[1]

	// Look up pending approval
	a.approvalMu.RLock()
	pending, exists := a.pendingApprovals[requestID]
	a.approvalMu.RUnlock()

	if !exists {
		// Respond that request is expired
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Request expired or already handled",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	// Handle the action
	var approvalAction channels.ApprovalAction
	var message string

	switch action {
	case "approve":
		approvalAction = channels.ActionApprove
		message = "✅ Approved"
	case "deny":
		approvalAction = channels.ActionDeny
		message = "❌ Denied"
	case "always":
		approvalAction = channels.ActionAlwaysAllow
		message = "✅ Always allowed"
	default:
		a.logger.Warn("unknown interaction action", "action", action)
		return
	}

	// Acknowledge the interaction
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Embeds:     formatResultEmbed(pending.Request, message),
			Components: []discordgo.MessageComponent{}, // Remove buttons
		},
	})

	// Remove from pending
	a.approvalMu.Lock()
	delete(a.pendingApprovals, requestID)
	a.approvalMu.Unlock()

	// Invoke callback
	a.callbackMu.RLock()
	callback := a.approvalCallback
	a.callbackMu.RUnlock()

	if callback != nil {
		callback(requestID, approvalAction, "")
	}
}

// handleApprovalTimeouts removes stale approval requests
func (a *Adapter) handleApprovalTimeouts(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.cleanupExpiredApprovals()
		}
	}
}

// cleanupExpiredApprovals removes approval requests older than 5 minutes
func (a *Adapter) cleanupExpiredApprovals() {
	a.approvalMu.Lock()
	defer a.approvalMu.Unlock()

	cutoff := time.Now().Add(-5 * time.Minute)
	for id, pending := range a.pendingApprovals {
		if pending.CreatedAt.Before(cutoff) {
			// Update message to show timeout
			a.session.ChannelMessageEditComplex(&discordgo.MessageEdit{
				Channel:    pending.ChannelID,
				ID:         pending.MessageID,
				Content:    stringPtr("⏰ Approval request expired"),
				Components: &[]discordgo.MessageComponent{}, // Remove buttons
			})
			delete(a.pendingApprovals, id)
		}
	}
}

// Helper functions

func formatApprovalEmbed(req *channels.ApprovalRequest) *discordgo.MessageEmbed {
	var color int
	var riskEmoji string
	switch req.RiskLevel {
	case channels.RiskLow:
		color = 0x00FF00 // Green
		riskEmoji = "🟢"
	case channels.RiskMedium:
		color = 0xFFFF00 // Yellow
		riskEmoji = "🟡"
	case channels.RiskHigh:
		color = 0xFF0000 // Red
		riskEmoji = "🔴"
	default:
		color = 0x808080 // Gray
		riskEmoji = "⚪"
	}

	embed := &discordgo.MessageEmbed{
		Title:       "🔐 Approval Required",
		Description: fmt.Sprintf("```\n%s\n```", req.Command),
		Color:       color,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Tool",
				Value:  req.Tool,
				Inline: true,
			},
			{
				Name:   "Risk Level",
				Value:  fmt.Sprintf("%s %s", riskEmoji, string(req.RiskLevel)),
				Inline: true,
			},
		},
	}

	if req.WorkingDir != "" {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:  "Directory",
			Value: req.WorkingDir,
		})
	}

	if req.Reason != "" {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:  "Reason",
			Value: req.Reason,
		})
	}

	return embed
}

func formatResultEmbed(req *channels.ApprovalRequest, result string) []*discordgo.MessageEmbed {
	return []*discordgo.MessageEmbed{
		{
			Title:       "🔐 Approval Request",
			Description: fmt.Sprintf("```\n%s\n```\n\n**Result:** %s", req.Command, result),
			Color:       0x808080, // Gray for completed
			Fields: []*discordgo.MessageEmbedField{
				{
					Name:   "Tool",
					Value:  req.Tool,
					Inline: true,
				},
			},
		},
	}
}

func stringPtr(s string) *string {
	return &s
}
