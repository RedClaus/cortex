// Package telegram provides a Telegram channel adapter for Pinky
package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/normanking/pinky/internal/channels"
	"github.com/normanking/pinky/internal/config"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Adapter implements the channels.Channel interface for Telegram
type Adapter struct {
	config   config.TelegramConfig
	bot      *tgbotapi.BotAPI
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
	ChatID    int64
	MessageID int
	CreatedAt time.Time
}

// approvalResponse represents a user's response to an approval request
type approvalResponse struct {
	RequestID string
	Approved  bool
	UserID    string
}

// New creates a new Telegram adapter
func New(cfg config.TelegramConfig, logger *slog.Logger) *Adapter {
	if logger == nil {
		logger = slog.Default()
	}

	return &Adapter{
		config:           cfg,
		logger:           logger.With("channel", "telegram"),
		incoming:         make(chan *channels.InboundMessage, 100),
		pendingApprovals: make(map[string]*pendingApproval),
		approvalChan:     make(chan *approvalResponse, 10),
	}
}

// Name returns the channel name
func (a *Adapter) Name() string {
	return "telegram"
}

// IsEnabled returns whether the channel is enabled
func (a *Adapter) IsEnabled() bool {
	return a.config.Enabled
}

// SupportsMedia returns true - Telegram supports file uploads
func (a *Adapter) SupportsMedia() bool {
	return true
}

// SupportsButtons returns true - Telegram has inline keyboard buttons
func (a *Adapter) SupportsButtons() bool {
	return true
}

// SupportsThreading returns true - Telegram has reply threading
func (a *Adapter) SupportsThreading() bool {
	return true
}

// Start initializes and starts the Telegram adapter
func (a *Adapter) Start(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.running {
		return nil
	}

	if a.config.Token == "" {
		return fmt.Errorf("telegram token not configured")
	}

	// Create bot API client
	bot, err := tgbotapi.NewBotAPI(a.config.Token)
	if err != nil {
		return fmt.Errorf("failed to create telegram bot: %w", err)
	}

	a.bot = bot
	a.logger.Info("telegram bot authorized", "username", bot.Self.UserName)

	// Create cancellable context
	ctx, cancel := context.WithCancel(ctx)
	a.cancel = cancel
	a.running = true

	// Start receiving updates
	go a.receiveUpdates(ctx)

	// Start approval timeout handler
	go a.handleApprovalTimeouts(ctx)

	return nil
}

// Stop stops the Telegram adapter
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
	a.logger.Info("telegram adapter stopped")
	return nil
}

// Incoming returns the channel for incoming messages
func (a *Adapter) Incoming() <-chan *channels.InboundMessage {
	return a.incoming
}

// SendMessage sends a message to a user
func (a *Adapter) SendMessage(userID string, msg *channels.OutboundMessage) error {
	chatID, err := parseChatID(userID)
	if err != nil {
		return err
	}

	text := msg.Content
	if msg.Format == channels.FormatCode {
		text = "```\n" + text + "\n```"
	}

	teleMsg := tgbotapi.NewMessage(chatID, text)
	if msg.Format == channels.FormatMarkdown || msg.Format == channels.FormatCode {
		teleMsg.ParseMode = tgbotapi.ModeMarkdownV2
		teleMsg.Text = escapeMarkdownV2(text)
	}

	// Add buttons if present
	if len(msg.Buttons) > 0 {
		var buttons []tgbotapi.InlineKeyboardButton
		for _, btn := range msg.Buttons {
			buttons = append(buttons, tgbotapi.NewInlineKeyboardButtonData(btn.Label, btn.ID))
		}
		teleMsg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(buttons...),
		)
	}

	// Reply threading
	if msg.ReplyTo != "" {
		msgID, _ := parseMessageID(msg.ReplyTo)
		teleMsg.ReplyToMessageID = msgID
	}

	_, err = a.bot.Send(teleMsg)
	return err
}

// SendApprovalRequest sends an approval request with approve/deny buttons
func (a *Adapter) SendApprovalRequest(userID string, req *channels.ApprovalRequest) error {
	chatID, err := parseChatID(userID)
	if err != nil {
		return err
	}

	// Format approval message
	text := formatApprovalMessage(req)

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeMarkdownV2

	// Add approve/deny buttons
	approveData := fmt.Sprintf("approve:%s", req.ID)
	denyData := fmt.Sprintf("deny:%s", req.ID)
	alwaysData := fmt.Sprintf("always:%s", req.ID)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Approve", approveData),
			tgbotapi.NewInlineKeyboardButtonData("❌ Deny", denyData),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Always Allow", alwaysData),
		),
	)
	msg.ReplyMarkup = keyboard

	sent, err := a.bot.Send(msg)
	if err != nil {
		return err
	}

	// Track pending approval
	a.approvalMu.Lock()
	a.pendingApprovals[req.ID] = &pendingApproval{
		Request:   req,
		UserID:    userID,
		ChatID:    chatID,
		MessageID: sent.MessageID,
		CreatedAt: time.Now(),
	}
	a.approvalMu.Unlock()

	return nil
}

// SendToolOutput sends the result of a tool execution
func (a *Adapter) SendToolOutput(userID string, output *channels.ToolOutput) error {
	chatID, err := parseChatID(userID)
	if err != nil {
		return err
	}

	var status string
	if output.Success {
		status = "✅"
	} else {
		status = "❌"
	}

	text := fmt.Sprintf("%s *%s* \\(%s\\)\n", status, escapeMarkdownV2(output.Tool), escapeMarkdownV2(output.Duration.String()))

	if output.Output != "" {
		// Truncate long output
		content := output.Output
		if len(content) > 1000 {
			content = content[:1000] + "... (truncated)"
		}
		text += "```\n" + escapeMarkdownV2(content) + "\n```"
	}

	if output.Error != "" {
		text += "\n*Error:* " + escapeMarkdownV2(output.Error)
	}

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeMarkdownV2

	_, err = a.bot.Send(msg)
	return err
}

// receiveUpdates processes incoming Telegram updates
func (a *Adapter) receiveUpdates(ctx context.Context) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := a.bot.GetUpdatesChan(u)

	for {
		select {
		case <-ctx.Done():
			return
		case update := <-updates:
			a.handleUpdate(update)
		}
	}
}

// handleUpdate processes a single Telegram update
func (a *Adapter) handleUpdate(update tgbotapi.Update) {
	// Handle callback queries (button presses)
	if update.CallbackQuery != nil {
		a.handleCallbackQuery(update.CallbackQuery)
		return
	}

	// Handle regular messages
	if update.Message != nil {
		a.handleMessage(update.Message)
		return
	}
}

// handleMessage processes an incoming message
func (a *Adapter) handleMessage(msg *tgbotapi.Message) {
	// Ignore non-text messages for now
	if msg.Text == "" {
		return
	}

	inbound := &channels.InboundMessage{
		ID:          fmt.Sprintf("%d", msg.MessageID),
		UserID:      fmt.Sprintf("%d", msg.Chat.ID),
		ChannelName: "telegram",
		ChannelID:   fmt.Sprintf("%d", msg.Chat.ID),
		Content:     msg.Text,
		Metadata: map[string]string{
			"username":   msg.From.UserName,
			"first_name": msg.From.FirstName,
			"last_name":  msg.From.LastName,
		},
		ReceivedAt: time.Unix(int64(msg.Date), 0),
	}

	// Handle reply threading
	if msg.ReplyToMessage != nil {
		inbound.ReplyTo = fmt.Sprintf("%d", msg.ReplyToMessage.MessageID)
	}

	// Handle media attachments
	if len(msg.Photo) > 0 {
		// Get largest photo
		photo := msg.Photo[len(msg.Photo)-1]
		inbound.Media = append(inbound.Media, channels.Media{
			Type: channels.MediaImage,
			URL:  photo.FileID,
		})
	}

	if msg.Document != nil {
		inbound.Media = append(inbound.Media, channels.Media{
			Type:     channels.MediaDocument,
			URL:      msg.Document.FileID,
			Filename: msg.Document.FileName,
			MimeType: msg.Document.MimeType,
		})
	}

	// Send to incoming channel
	select {
	case a.incoming <- inbound:
	default:
		a.logger.Warn("incoming message channel full, dropping message")
	}
}

// handleCallbackQuery processes button press callbacks
func (a *Adapter) handleCallbackQuery(query *tgbotapi.CallbackQuery) {
	data := query.Data

	// Parse callback data
	parts := strings.SplitN(data, ":", 2)
	if len(parts) != 2 {
		a.logger.Warn("invalid callback data", "data", data)
		return
	}

	action := parts[0]
	requestID := parts[1]

	// Look up pending approval
	a.approvalMu.RLock()
	pending, exists := a.pendingApprovals[requestID]
	a.approvalMu.RUnlock()

	if !exists {
		// Answer callback to remove loading state
		callback := tgbotapi.NewCallback(query.ID, "Request expired or already handled")
		a.bot.Request(callback)
		return
	}

	// Handle the action
	var approved bool
	var message string

	switch action {
	case "approve":
		approved = true
		message = "✅ Approved"
	case "deny":
		approved = false
		message = "❌ Denied"
	case "always":
		approved = true
		message = "✅ Always allowed"
		// TODO: Persist "always allow" preference
	default:
		a.logger.Warn("unknown callback action", "action", action)
		return
	}

	// Answer callback
	callback := tgbotapi.NewCallback(query.ID, message)
	a.bot.Request(callback)

	// Update the message to show result
	edit := tgbotapi.NewEditMessageText(
		pending.ChatID,
		pending.MessageID,
		fmt.Sprintf("%s\n\n*Result:* %s", escapeMarkdownV2(formatApprovalMessagePlain(pending.Request)), escapeMarkdownV2(message)),
	)
	edit.ParseMode = tgbotapi.ModeMarkdownV2
	a.bot.Send(edit)

	// Remove from pending
	a.approvalMu.Lock()
	delete(a.pendingApprovals, requestID)
	a.approvalMu.Unlock()

	// Send approval response
	select {
	case a.approvalChan <- &approvalResponse{
		RequestID: requestID,
		Approved:  approved,
		UserID:    pending.UserID,
	}:
	default:
		a.logger.Warn("approval channel full")
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
			edit := tgbotapi.NewEditMessageText(
				pending.ChatID,
				pending.MessageID,
				"⏰ Approval request expired",
			)
			a.bot.Send(edit)
			delete(a.pendingApprovals, id)
		}
	}
}

// Helper functions

func parseChatID(userID string) (int64, error) {
	var chatID int64
	_, err := fmt.Sscanf(userID, "%d", &chatID)
	return chatID, err
}

func parseMessageID(id string) (int, error) {
	var msgID int
	_, err := fmt.Sscanf(id, "%d", &msgID)
	return msgID, err
}

func formatApprovalMessage(req *channels.ApprovalRequest) string {
	risk := "🟢"
	switch req.RiskLevel {
	case "medium":
		risk = "🟡"
	case "high":
		risk = "🔴"
	}

	return fmt.Sprintf("*Pinky wants to execute:*\n\n%s `%s`\n\n*Tool:* %s %s\n*Directory:* %s\n*Reason:* %s",
		risk,
		escapeMarkdownV2(req.Command),
		escapeMarkdownV2(req.Tool),
		escapeMarkdownV2("("+string(req.RiskLevel)+" risk)"),
		escapeMarkdownV2(req.WorkingDir),
		escapeMarkdownV2(req.Reason),
	)
}

func formatApprovalMessagePlain(req *channels.ApprovalRequest) string {
	return fmt.Sprintf("Pinky wants to execute: %s\nTool: %s (%s risk)\nDirectory: %s",
		req.Command, req.Tool, req.RiskLevel, req.WorkingDir)
}

// escapeMarkdownV2 escapes special characters for Telegram MarkdownV2
func escapeMarkdownV2(text string) string {
	// Characters that need escaping in MarkdownV2
	chars := []string{"_", "*", "[", "]", "(", ")", "~", "`", ">", "#", "+", "-", "=", "|", "{", "}", ".", "!"}
	result := text
	for _, char := range chars {
		result = strings.ReplaceAll(result, char, "\\"+char)
	}
	return result
}

// GetApprovalChannel returns the channel for approval responses
func (a *Adapter) GetApprovalChannel() <-chan *approvalResponse {
	return a.approvalChan
}

// DismissApproval removes an approval dialog
func (a *Adapter) DismissApproval(userID string, requestID string) error {
	return nil
}

// SetApprovalCallback sets the callback for approval responses
func (a *Adapter) SetApprovalCallback(callback channels.ApprovalCallback) {
	// Telegram uses GetApprovalChannel instead
}

// SupportsEditing returns true if the channel supports message editing
func (a *Adapter) SupportsEditing() bool {
	return true
}
