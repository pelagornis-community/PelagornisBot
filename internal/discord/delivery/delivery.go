package delivery

import (
	"bytes"
	"context"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Sender interface {
	ChannelMessageSendComplex(string, *discordgo.MessageSend, ...discordgo.RequestOption) (*discordgo.Message, error)
}
type Service struct {
	Discord    Sender
	FetchImage func(context.Context, string) (*discordgo.File, error)
}

func (s Service) Send(ctx context.Context, channel, content string) (string, error) {
	return s.SendWithImage(ctx, channel, content, "")
}

func (s Service) SendWithImage(ctx context.Context, channel, content, imageURL string) (string, error) {
	if channel == "" {
		return "", fmt.Errorf("channel required")
	}
	summary, previewURL := splitPreviewURL(content)
	title := "📰 Feed update"
	isRelease := false
	if strings.HasPrefix(summary, "GitHub release:") {
		isRelease = true
		lines := strings.SplitN(summary, "\n", 2)
		title = "🚀 " + strings.TrimSpace(strings.TrimPrefix(lines[0], "GitHub release:"))
		if len(lines) == 2 {
			summary = strings.TrimSpace(lines[1])
		} else {
			summary = "_No release notes were provided._"
		}
	}
	summary = limitText(summary, 2_800)
	parts := []discordgo.MessageComponent{discordgo.TextDisplay{Content: "# " + title + "\n" + summary}}
	files := []*discordgo.File{}
	if imageURL != "" {
		file, err := s.releaseImage(ctx, imageURL)
		if err != nil {
			// Preserve the complete release card: scheduler retry is preferable to
			// publishing a permanent notification with its image missing.
			return "", fmt.Errorf("download release image: %w", err)
		}
		files = append(files, file)
		parts = append(parts, discordgo.Separator{}, discordgo.MediaGallery{Items: []discordgo.MediaGalleryItem{{Media: discordgo.UnfurledMediaItem{URL: "attachment://release.png"}}}})
	}
	if previewURL != "" {
		label := "Open feed"
		if isRelease {
			label = "View release"
		}
		parts = append(parts, discordgo.Separator{}, discordgo.ActionsRow{Components: []discordgo.MessageComponent{discordgo.Button{Style: discordgo.LinkButton, URL: previewURL, Label: label, Emoji: &discordgo.ComponentEmoji{Name: "↗️"}}}})
	}
	m, e := s.Discord.ChannelMessageSendComplex(channel, &discordgo.MessageSend{
		Components:      []discordgo.MessageComponent{discordgo.Container{Components: parts}},
		Files:           files,
		Flags:           discordgo.MessageFlagsIsComponentsV2,
		AllowedMentions: &discordgo.MessageAllowedMentions{Parse: []discordgo.AllowedMentionType{}},
	})
	if e != nil {
		return "", e
	}
	return m.ID, nil
}

func (s Service) releaseImage(ctx context.Context, imageURL string) (*discordgo.File, error) {
	if s.FetchImage != nil {
		return s.FetchImage(ctx, imageURL)
	}
	u, err := url.Parse(imageURL)
	if err != nil || u.Scheme != "https" || u.Host != "opengraph.githubassets.com" {
		return nil, fmt.Errorf("untrusted GitHub image URL")
	}
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "PelagornisBot/1.0 (+https://github.com/pelagornis/pelagornis-bot)")
	req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/*,*/*;q=0.8")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub image HTTP %d", resp.StatusCode)
	}
	if !strings.HasPrefix(resp.Header.Get("Content-Type"), "image/") {
		return nil, fmt.Errorf("GitHub image has unexpected content type")
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("GitHub image is empty")
	}
	return &discordgo.File{Name: "release.png", Reader: bytes.NewReader(data)}, nil
}

func limitText(value string, max int) string {
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max-1]) + "…"
}

func splitPreviewURL(content string) (summary, previewURL string) {
	lines := strings.Split(strings.TrimSpace(content), "\n")
	if len(lines) > 1 {
		last := strings.TrimSpace(lines[len(lines)-1])
		if strings.HasPrefix(last, "https://") || strings.HasPrefix(last, "http://") {
			return strings.TrimSpace(strings.Join(lines[:len(lines)-1], "\n")), last
		}
	}
	return strings.TrimSpace(content), ""
}
