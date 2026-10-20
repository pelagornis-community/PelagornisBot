package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	GuildID    string         `yaml:"guild_id"`
	Roles      Roles          `yaml:"roles"`
	Onboarding Onboarding     `yaml:"onboarding"`
	Moderation Moderation     `yaml:"moderation"`
	Promotion  Promotion      `yaml:"promotion"`
	Announcement Announcement `yaml:"announcement"`
	FAQ        FAQ            `yaml:"faq"`
	Feedback   Feedback       `yaml:"feedback"`
	Security   Security       `yaml:"security"`
}

type Roles struct {
	Mod   []string `yaml:"mod"`
	Admin []string `yaml:"admin"`
}

type Onboarding struct {
	WelcomeChannelID       string      `yaml:"welcome_channel_id"`
	WelcomeDM              bool        `yaml:"welcome_dm"`
	WelcomeTemplate        string      `yaml:"welcome_template"`
	FirstMessageReminder   bool        `yaml:"first_message_reminder"`
	FirstMessageTemplate   string      `yaml:"first_message_template"`
	RulesEmbed             RulesEmbed  `yaml:"rules_embed"`
	Rules                  []RuleEntry `yaml:"rules"`
}

// RulesEmbed customizes the /rules Discord embed (title, intro, footer, color).
type RulesEmbed struct {
	Title        string `yaml:"title"`
	Description  string `yaml:"description"`
	Footer       string `yaml:"footer"`
	Color        int    `yaml:"color"`      // decimal, e.g. 2171169 = #212121
	ColorHex     string `yaml:"color_hex"`  // RefineUI Primary: #212121 Black, #4D4D4D Dark Gray, …
	ThumbnailURL string `yaml:"thumbnail_url"`
}

// EmbedColor returns Discord embed sidebar color (RefineUI gray default #212121).
func (re RulesEmbed) EmbedColor() int {
	if re.ColorHex != "" {
		if c, err := ParseHexColor(re.ColorHex); err == nil {
			return c
		}
	}
	if re.Color > 0 {
		return re.Color
	}
	c, _ := ParseHexColor("#212121")
	return c
}

type RuleEntry struct {
	Title   string `yaml:"title"`
	Body    string `yaml:"body"`
	Emoji   string `yaml:"emoji"`
}

type Moderation struct {
	BannedWords    []string       `yaml:"banned_words"`
	BannedPatterns []string       `yaml:"banned_patterns"`
	RateLimit      RateLimit      `yaml:"rate_limit"`
	NewUser        NewUserPolicy  `yaml:"new_user"`
	LogChannelID   string         `yaml:"log_channel_id"`
}

type RateLimit struct {
	WindowSeconds   int    `yaml:"window_seconds"`
	MaxMessages     int    `yaml:"max_messages"`
	Action          string `yaml:"action"`
	TimeoutMinutes  int    `yaml:"timeout_minutes"`
}

type NewUserPolicy struct {
	LinkRestrictHours int      `yaml:"link_restrict_hours"`
	ExemptRoleIDs     []string `yaml:"exempt_role_ids"`
}

type Promotion struct {
	Enabled          bool          `yaml:"enabled"`
	DiscordChannelID string        `yaml:"discord_channel_id"`
	PollMinutes      int           `yaml:"poll_minutes"`
	X                PromotionFeed `yaml:"x"`
	YouTube          YouTubeFeed   `yaml:"youtube"`
}

type PromotionFeed struct {
	Enabled    bool   `yaml:"enabled"`
	RSSFeedURL string `yaml:"rss_feed_url"`
}

type YouTubeFeed struct {
	Enabled    bool   `yaml:"enabled"`
	RSSFeedURL string `yaml:"rss_feed_url"`
	UseAPI     bool   `yaml:"use_api"`
	ChannelID  string `yaml:"channel_id"`
}

type Announcement struct {
	DefaultChannelID   string `yaml:"default_channel_id"`
	SubscriberRoleID   string `yaml:"subscriber_role_id"`
	AllowMentionHere   bool   `yaml:"allow_mention_here"`
}

type FAQ struct {
	Enabled          bool       `yaml:"enabled"`
	CooldownSeconds  int        `yaml:"cooldown_seconds"`
	ChannelsOnly     []string   `yaml:"channels_only"`
	Rules            []FAQRule  `yaml:"rules"`
}

type FAQRule struct {
	Keywords []string `yaml:"keywords"`
	Response string   `yaml:"response"`
}

type Feedback struct {
	ChannelID      string `yaml:"channel_id"`
	AllowAnonymous bool   `yaml:"allow_anonymous"`
	StoreDB        bool   `yaml:"store_db"`
}

type Security struct {
	Enabled          bool            `yaml:"enabled"`
	Action           string          `yaml:"action"`
	Patterns         []SecurityPattern `yaml:"patterns"`
	NotifyModChannel bool            `yaml:"notify_mod_channel"`
}

type SecurityPattern struct {
	Name  string `yaml:"name"`
	Regex string `yaml:"regex"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) validate() error {
	if c.GuildID == "" {
		return fmt.Errorf("guild_id is required")
	}
	if c.Promotion.Enabled {
		if c.Promotion.DiscordChannelID == "" {
			return fmt.Errorf("promotion.discord_channel_id is required when promotion is enabled")
		}
		if c.Promotion.PollMinutes < 1 {
			c.Promotion.PollMinutes = 30
		}
		if c.Promotion.X.Enabled {
			if _, err := c.Promotion.X.FeedURL(); err != nil {
				return err
			}
		}
		if c.Promotion.YouTube.Enabled {
			if c.Promotion.YouTube.UseAPI {
				return fmt.Errorf("youtube use_api is not supported in lite; use channel_id or rss_feed_url")
			}
			if _, err := c.Promotion.YouTube.FeedURL(); err != nil {
				return err
			}
		}
	}
	return nil
}

// FeedURL returns the X RSS feed URL or an error if still a template placeholder.
func (x PromotionFeed) FeedURL() (string, error) {
	url := strings.TrimSpace(x.RSSFeedURL)
	if url == "" {
		return "", fmt.Errorf("promotion.x: set rss_feed_url or set x.enabled: false")
	}
	if strings.Contains(url, "your_handle") {
		return "", fmt.Errorf("promotion.x: replace your_handle in rss_feed_url with your X username, or set x.enabled: false")
	}
	return url, nil
}

// FeedURL builds YouTube Atom RSS from channel_id, or uses rss_feed_url.
func (y YouTubeFeed) FeedURL() (string, error) {
	ch := strings.TrimSpace(y.ChannelID)
	if ch != "" && !isPlaceholder(ch) {
		return "https://www.youtube.com/feeds/videos.xml?channel_id=" + ch, nil
	}
	url := strings.TrimSpace(y.RSSFeedURL)
	if url == "" || isPlaceholder(url) {
		return "", fmt.Errorf(
			"promotion.youtube: set channel_id to UC... (from YouTube channel URL), " +
				"or a valid rss_feed_url — example UCxxxxxxxx is a placeholder and returns 404",
		)
	}
	return url, nil
}

func isPlaceholder(s string) bool {
	lower := strings.ToLower(s)
	return strings.Contains(lower, "xxxx") ||
		strings.Contains(lower, "your_") ||
		strings.Contains(lower, "example")
}

func DefaultPath() string {
	if p := os.Getenv("CONFIG_PATH"); p != "" {
		return p
	}
	return "config/config.yaml"
}
