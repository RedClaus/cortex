package telegram

import (
	"context"
	"strconv"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/cortexhub/cortex-gateway/internal/channel"
)

type TelegramAdapter struct {
	bot      *tgbotapi.BotAPI
	token    string
	incoming chan *channel.Message
}

func NewTelegramAdapter(token string) *TelegramAdapter {
	return &TelegramAdapter{
		token:    token,
		incoming: make(chan *channel.Message, 100),
	}
}

func (t *TelegramAdapter) Name() string {
	return "telegram"
}

func (t *TelegramAdapter) IsEnabled() bool {
	return t.token != ""
}

func (t *TelegramAdapter) Start(ctx context.Context) error {
	bot, err := tgbotapi.NewBotAPI(t.token)
	if err != nil {
		return err
	}
	t.bot = bot
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := t.bot.GetUpdatesChan(u)
	go func() {
		for update := range updates {
			if update.Message != nil {
				msg := &channel.Message{
					ID:       strconv.Itoa(update.Message.MessageID),
					Channel:  "telegram",
					UserID:   strconv.Itoa(int(update.Message.Chat.ID)),
					Content:  update.Message.Text,
					Metadata: map[string]string{"from_id": strconv.Itoa(int(update.Message.From.ID))},
					Timestamp: int64(update.Message.Date),
				}
				t.incoming <- msg
			}
		}
	}()
	return nil
}

func (t *TelegramAdapter) Stop() error {
	close(t.incoming)
	return nil
}

func (t *TelegramAdapter) SendMessage(userID string, resp *channel.Response) error {
	chatID, _ := strconv.ParseInt(userID, 10, 64)
	reply := tgbotapi.NewMessage(chatID, resp.Content)
	_, err := t.bot.Send(reply)
	return err
}

func (t *TelegramAdapter) Incoming() <-chan *channel.Message {
	return t.incoming
}
