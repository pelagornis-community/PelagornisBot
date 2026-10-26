package delivery

import (
	"context"
	"fmt"
	"github.com/bwmarrin/discordgo"
)

type Sender interface {
	ChannelMessageSendComplex(string, *discordgo.MessageSend, ...discordgo.RequestOption) (*discordgo.Message, error)
}
type Service struct{ Discord Sender }

func (s Service) Send(ctx context.Context, channel, content string) (string, error) {
	if channel == "" {
		return "", fmt.Errorf("channel required")
	}
	m, e := s.Discord.ChannelMessageSendComplex(channel, &discordgo.MessageSend{Embeds: []*discordgo.MessageEmbed{{Title: "📰 New update", Description: content, Color: 0x5865F2, Footer: &discordgo.MessageEmbedFooter{Text: "Pelagornis · Feed delivery"}}}, AllowedMentions: &discordgo.MessageAllowedMentions{Parse: []discordgo.AllowedMentionType{}}})
	if e != nil {
		return "", e
	}
	return m.ID, nil
}

var _ = context.Background
