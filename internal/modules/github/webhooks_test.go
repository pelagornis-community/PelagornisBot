package github

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/pelagornis/pelagornis-bot/internal/storage"
	"github.com/pelagornis/pelagornis-bot/internal/storage/sqlite"
)

type fakeDiscord struct {
	channels              []*discordgo.Channel
	creates, sends, edits int
}

func (d *fakeDiscord) ChannelMessageSendComplex(_ string, _ *discordgo.MessageSend, _ ...discordgo.RequestOption) (*discordgo.Message, error) {
	d.sends++
	return &discordgo.Message{ID: "message"}, nil
}
func (d *fakeDiscord) GuildChannels(_ string, _ ...discordgo.RequestOption) ([]*discordgo.Channel, error) {
	return d.channels, nil
}
func (d *fakeDiscord) GuildChannelCreateComplex(_ string, data discordgo.GuildChannelCreateData, _ ...discordgo.RequestOption) (*discordgo.Channel, error) {
	d.creates++
	ch := &discordgo.Channel{ID: "channel-" + string(rune(d.creates)), Name: data.Name, ParentID: data.ParentID}
	d.channels = append(d.channels, ch)
	return ch, nil
}
func (d *fakeDiscord) ChannelEdit(id string, data *discordgo.ChannelEdit, _ ...discordgo.RequestOption) (*discordgo.Channel, error) {
	d.edits++
	for _, ch := range d.channels {
		if ch.ID == id {
			ch.Name = data.Name
			return ch, nil
		}
	}
	return &discordgo.Channel{ID: id, Name: data.Name}, nil
}

func TestWebhookProjectConfigurationUsesMatchingGuildAndIsIdempotent(t *testing.T) {
	ctx := context.Background()
	db, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "bot.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = storage.Migrate(ctx, db.DB); err != nil {
		t.Fatal(err)
	}
	if err = db.EnsureGuild(ctx, "guild-a", "ko"); err != nil {
		t.Fatal(err)
	}
	if err = db.EnsureGuild(ctx, "guild-b", "ko"); err != nil {
		t.Fatal(err)
	}
	if err = db.SetGitHubOrganization(ctx, "guild-a", "pelagornis"); err != nil {
		t.Fatal(err)
	}
	if err = db.SetProjectCategory(ctx, "guild-b", "wrong-category"); err != nil {
		t.Fatal(err)
	}
	if err = db.SetRole(ctx, "guild-b", "staff", "wrong-staff"); err != nil {
		t.Fatal(err)
	}

	raw := []byte(`{"action":"created","repository":{"id":1,"name":"Test Release","full_name":"pelagornis/test-release","html_url":"https://github.com/pelagornis/test-release"},"sender":{"login":"maintainer"}}`)
	payload, _ := json.Marshal(map[string]string{"event": "repository", "payload": string(raw)})
	service := WebhookService{Store: db, Discord: &fakeDiscord{}}
	err = service.Handle(ctx, sqlite.Job{Kind: "github.webhook", GuildID: "guild-a", Payload: string(payload)})
	if err == nil || !strings.Contains(err.Error(), "project category is not configured for guild guild-a") {
		t.Fatalf("configuration error = %v", err)
	}

	if err = db.SetProjectCategory(ctx, "guild-a", "project-category"); err != nil {
		t.Fatal(err)
	}
	if err = db.SetRole(ctx, "guild-a", "staff", "staff-role"); err != nil {
		t.Fatal(err)
	}
	fd := &fakeDiscord{}
	service.Discord = fd
	if err = service.Handle(ctx, sqlite.Job{Kind: "github.webhook", GuildID: "guild-a", Payload: string(payload)}); err != nil {
		t.Fatal(err)
	}
	if fd.creates != 2 {
		t.Fatalf("created %d channels, want 2", fd.creates)
	}
	if err = service.Handle(ctx, sqlite.Job{Kind: "github.webhook", GuildID: "guild-a", Payload: string(payload)}); err != nil {
		t.Fatal(err)
	}
	if fd.creates != 2 {
		t.Fatalf("duplicate delivery created %d channels", fd.creates)
	}
	p, err := db.GitHubProject(ctx, "guild-a", "pelagornis/test-release")
	if err != nil || p.ChannelID == "" || p.CIChannelID == "" {
		t.Fatalf("project = %#v, %v", p, err)
	}
}

func TestWebhookRepositoryDeleteArchivesAndRecreateRestoresProjectChannels(t *testing.T) {
	ctx := context.Background()
	db, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "bot.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = storage.Migrate(ctx, db.DB); err != nil {
		t.Fatal(err)
	}
	if err = db.EnsureGuild(ctx, "guild", "ko"); err != nil {
		t.Fatal(err)
	}
	if err = db.SetProjectCategory(ctx, "guild", "project-category"); err != nil {
		t.Fatal(err)
	}
	if err = db.SetRole(ctx, "guild", "staff", "staff-role"); err != nil {
		t.Fatal(err)
	}
	if err = db.SaveGitHubProject(ctx, "guild", sqlite.GitHubProject{Repository: "pelagornis/test-release", ChannelID: "main", CIChannelID: "ci"}); err != nil {
		t.Fatal(err)
	}
	fd := &fakeDiscord{channels: []*discordgo.Channel{{ID: "main", Name: "test-release", ParentID: "project-category"}, {ID: "ci", Name: "test-release-ci", ParentID: "project-category"}}}
	service := WebhookService{Store: db, Discord: fd}
	deleteRaw := []byte(`{"action":"deleted","repository":{"id":1,"name":"Test Release","full_name":"pelagornis/test-release"},"sender":{"login":"maintainer"}}`)
	deletePayload, _ := json.Marshal(map[string]string{"event": "repository", "payload": string(deleteRaw)})
	if err = service.Handle(ctx, sqlite.Job{Kind: "github.webhook", GuildID: "guild", Payload: string(deletePayload)}); err != nil {
		t.Fatal(err)
	}
	if fd.edits != 2 || fd.channels[0].Name != "archived-test-release" || fd.channels[1].Name != "archived-test-release-ci" {
		t.Fatalf("archive edits=%d channels=%#v", fd.edits, fd.channels)
	}
	p, err := db.GitHubProject(ctx, "guild", "pelagornis/test-release")
	if err != nil || !p.Archived {
		t.Fatalf("archived project=%#v err=%v", p, err)
	}

	createRaw := []byte(`{"action":"created","repository":{"id":1,"name":"Test Release","full_name":"pelagornis/test-release"},"sender":{"login":"maintainer"}}`)
	createPayload, _ := json.Marshal(map[string]string{"event": "repository", "payload": string(createRaw)})
	if err = service.Handle(ctx, sqlite.Job{Kind: "github.webhook", GuildID: "guild", Payload: string(createPayload)}); err != nil {
		t.Fatal(err)
	}
	if fd.creates != 0 || fd.edits != 4 || fd.channels[0].Name != "test-release" || fd.channels[1].Name != "test-release-ci" {
		t.Fatalf("restore creates=%d edits=%d channels=%#v", fd.creates, fd.edits, fd.channels)
	}
	p, err = db.GitHubProject(ctx, "guild", "pelagornis/test-release")
	if err != nil || p.Archived {
		t.Fatalf("restored project=%#v err=%v", p, err)
	}
}
