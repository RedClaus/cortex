package discord

import (
	"context"

	"github.com/bwmarrin/discordgo"
	"github.com/cortexhub/cortex-gateway/internal/channel"
)

type DiscordAdapter struct {
	token    string
	session  *discordgo.Session
	incoming chan *channel.Message
}

func NewDiscordAdapter(token string) *DiscordAdapter {
	return &DiscordAdapter{
		token:    token,
		incoming: make(chan *channel.Message, 100),
	}
}

func (d *DiscordAdapter) Name() string {
	return "discord"
}

func (d *DiscordAdapter) IsEnabled() bool {
	return d.token != ""
}

func (d *DiscordAdapter) Start(ctx context.Context) error {
	session, err := discordgo.New("Bot " + d.token)
	if err != nil {
		return err
	}
	d.session = session

	session.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		// Ignore bot messages
		if m.Author.Bot {
			return
		}

		// Only respond in DMs or when mentioned
		if m.GuildID != "" && !d.isMentioned(s.State.User.ID, m.Mentions) {
			return
		}

		msg := &channel.Message{
			ID:       m.ID,
			Channel:  "discord",
			UserID:   m.Author.ID,
			Content:  m.Content,
			Metadata: map[string]string{
				"guild_id":    m.GuildID,
				"channel_id":  m.ChannelID,
				"author_id":   m.Author.ID,
				"author_name": m.Author.Username,
			},
			Timestamp: int64(m.Timestamp.Unix()),
		}
		d.incoming <- msg
	})

	err = session.Open()
	if err != nil {
		return err
	}

	go func() {
		<-ctx.Done()
		session.Close()
	}()

	return nil
}

func (d *DiscordAdapter) Stop() error {
	if d.session != nil {
		d.session.Close()
	}
	close(d.incoming)
	return nil
}

func (d *DiscordAdapter) SendMessage(userID string, resp *channel.Response) error {
	channel, err := d.session.UserChannelCreate(userID)
	if err != nil {
		return err
	}

	_, err = d.session.ChannelMessageSend(channel.ID, resp.Content)
	return err
}

func (d *DiscordAdapter) Incoming() <-chan *channel.Message {
	return d.incoming
}

func (d *DiscordAdapter) isMentioned(botID string, mentions []*discordgo.User) bool {
	for _, mention := range mentions {
		if mention.ID == botID {
			return true
		}
	}
	return false
}