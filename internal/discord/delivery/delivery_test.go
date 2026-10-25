package delivery

import (
	"github.com/bwmarrin/discordgo"
	"testing"
)

type fake struct{ got *discordgo.MessageSend }

func (f *fake) ChannelMessageSendComplex(_ string, m *discordgo.MessageSend, _ ...discordgo.RequestOption) (*discordgo.Message, error) {
	f.got = m
	return &discordgo.Message{ID: "m"}, nil
}
func TestMentionsDisabled(t *testing.T) {
	f := &fake{}
	id, e := (Service{f}).Send(t.Context(), "c", "@everyone")
	if e != nil || id != "m" || len(f.got.AllowedMentions.Parse) != 0 {
		t.Fatal("unsafe send")
	}
}
