package delivery

import (
	"context"
	"github.com/bwmarrin/discordgo"
	"strings"
	"testing"
)

type fake struct{ got *discordgo.MessageSend }

func (f *fake) ChannelMessageSendComplex(_ string, m *discordgo.MessageSend, _ ...discordgo.RequestOption) (*discordgo.Message, error) {
	f.got = m
	return &discordgo.Message{ID: "m"}, nil
}
func TestMentionsDisabled(t *testing.T) {
	f := &fake{}
	id, e := (Service{Discord: f}).Send(t.Context(), "c", "@everyone")
	if e != nil || id != "m" || len(f.got.AllowedMentions.Parse) != 0 {
		t.Fatal("unsafe send")
	}
}

func TestGitHubReleaseUsesV2WithCanonicalImage(t *testing.T) {
	f := &fake{}
	_, e := (Service{Discord: f, FetchImage: func(_ context.Context, _ string) (*discordgo.File, error) {
		return &discordgo.File{Name: "release.png", Reader: strings.NewReader("image")}, nil
	}}).SendWithImage(t.Context(), "c", "GitHub release: [pelagornis/test-release] · 1.0.3\n## Changes\n- Faster startup\nhttps://github.com/pelagornis/test-release/releases/tag/1.0.3", "https://opengraph.githubassets.com/hash/pelagornis/test-release/releases/tag/1.0.3")
	if e != nil {
		t.Fatal(e)
	}
	if f.got.Flags&discordgo.MessageFlagsIsComponentsV2 == 0 || len(f.got.Components) != 1 {
		t.Fatal("expected Components V2 release card")
	}
	container, ok := f.got.Components[0].(discordgo.Container)
	if !ok || len(container.Components) != 5 {
		t.Fatal("expected title, canonical image, and link button")
	}
	header, ok := container.Components[0].(discordgo.TextDisplay)
	if !ok || header.Content != "# 🚀 [pelagornis/test-release] · 1.0.3\n## Changes\n- Faster startup" {
		t.Fatalf("release package and notes were not rendered: %#v", container.Components[0])
	}
	gallery, ok := container.Components[2].(discordgo.MediaGallery)
	if !ok || gallery.Items[0].Media.URL == "" {
		t.Fatal("expected canonical GitHub Open Graph image")
	}
	row, ok := container.Components[4].(discordgo.ActionsRow)
	if !ok || row.Components[0].(discordgo.Button).URL != "https://github.com/pelagornis/test-release/releases/tag/1.0.3" {
		t.Fatal("expected release link button")
	}
	if len(f.got.Files) != 1 || f.got.Files[0].Name != "release.png" {
		t.Fatal("expected release image attachment")
	}
}
