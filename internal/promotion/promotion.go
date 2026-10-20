package promotion

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mmcdole/gofeed"
	"github.com/pelagornis/pelagornis-bot/internal/config"
	"github.com/pelagornis/pelagornis-bot/internal/store"
)

const (
	platformXRSS      = "x_rss"
	platformYouTubeRSS = "youtube_rss"
)

type Service struct {
	session *discordgo.Session
	cfg     *config.Config
	store   *store.Store
	parser  *gofeed.Parser
}

func New(session *discordgo.Session, cfg *config.Config, st *store.Store) *Service {
	return &Service{
		session: session,
		cfg:     cfg,
		store:   st,
		parser:  gofeed.NewParser(),
	}
}

func (s *Service) RunOnce() {
	if !s.cfg.Promotion.Enabled {
		return
	}
	channelID := s.cfg.Promotion.DiscordChannelID
	if s.cfg.Promotion.X.Enabled {
		url, err := s.cfg.Promotion.X.FeedURL()
		if err != nil {
			log.Printf("promotion: x: %v", err)
		} else {
			s.pollFeed(platformXRSS, url, channelID, "📢 New Tweet")
		}
	}
	if s.cfg.Promotion.YouTube.Enabled {
		url, err := s.cfg.Promotion.YouTube.FeedURL()
		if err != nil {
			log.Printf("promotion: youtube: %v", err)
		} else {
			s.pollFeed(platformYouTubeRSS, url, channelID, "🎬 New Video")
		}
	}
}

func (s *Service) pollFeed(platform, feedURL, channelID, titlePrefix string) {
	feed, err := s.parser.ParseURL(feedURL)
	if err != nil {
		log.Printf("promotion: fetch %s: %v", platform, err)
		if platform == platformYouTubeRSS && strings.Contains(err.Error(), "404") {
			log.Printf("promotion: youtube hint: channel_id must be UC... from https://www.youtube.com/channel/UC... — not @handle")
		}
		return
	}
	if len(feed.Items) == 0 {
		return
	}

	lastID, _ := s.store.GetPromotionCursor(platform, feedURL)
	var pending []*gofeed.Item
	for _, item := range feed.Items {
		id := itemID(item)
		if id == "" {
			continue
		}
		if lastID != "" && id == lastID {
			break
		}
		pending = append(pending, item)
	}
	if len(pending) == 0 {
		return
	}

	// Oldest first
	for i, j := 0, len(pending)-1; i < j; i, j = i+1, j-1 {
		pending[i], pending[j] = pending[j], pending[i]
	}

	for _, item := range pending {
		if err := s.postEmbed(channelID, titlePrefix, item); err != nil {
			log.Printf("promotion: post %s: %v", platform, err)
			continue
		}
		_ = s.store.SetPromotionCursor(platform, feedURL, itemID(item))
	}
}

func itemID(item *gofeed.Item) string {
	if item.GUID != "" {
		return item.GUID
	}
	return item.Link
}

func (s *Service) postEmbed(channelID, titlePrefix string, item *gofeed.Item) error {
	title := titlePrefix
	if item.Title != "" {
		title = titlePrefix + " — " + item.Title
	}
	desc := item.Description
	if desc == "" {
		desc = item.Content
	}
	if len(desc) > 300 {
		desc = desc[:297] + "..."
	}
	embed := &discordgo.MessageEmbed{
		Title:       title,
		Description: desc,
		URL:         item.Link,
		Footer: &discordgo.MessageEmbedFooter{
			Text: "PelagornisBot · auto",
		},
	}
	if item.PublishedParsed != nil {
		embed.Timestamp = item.PublishedParsed.Format(time.RFC3339)
	}
	if item.Image != nil && item.Image.URL != "" {
		embed.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: item.Image.URL}
	}
	_, err := s.session.ChannelMessageSendEmbed(channelID, embed)
	return err
}

func FeedKey(url string) string {
	return strings.TrimSpace(url)
}

func CronSpec(minutes int) string {
	if minutes < 1 {
		minutes = 30
	}
	return fmt.Sprintf("*/%d * * * *", minutes)
}
