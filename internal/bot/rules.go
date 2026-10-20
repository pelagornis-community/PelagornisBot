package bot

import (
	"fmt"
	"strings"
	"github.com/bwmarrin/discordgo"
	"github.com/pelagornis/pelagornis-bot/internal/config"
)

const (
	discordEmbedFieldMax  = 1024
	discordEmbedDescMax   = 4096
	discordTotalEmbedMax  = 6000
	defaultRulesColorHex  = "#212121"
)

// buildRulesEmbeds returns a single embed (one Discord message).
func buildRulesEmbeds(cfg *config.Config) []*discordgo.MessageEmbed {
	o := cfg.Onboarding
	re := o.RulesEmbed
	color := re.EmbedColor()

	desc := strings.TrimSpace(re.Description)
	if desc == "" {
		desc = "Welcome. **Joining this server means you agree to the rules below.**"
	}
	if quick := buildRulesQuickRef(cfg); quick != "" {
		desc += "\n\n**Quick note**\n" + quick
	}
	desc = truncate(desc, discordEmbedDescMax)

	var fields []*discordgo.MessageEmbedField
	for i, r := range o.Rules {
		body := strings.TrimSpace(r.Body)
		if body == "" {
			continue
		}
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   formatRuleFieldName(i, r),
			Value:  truncate(formatRuleBody(body), discordEmbedFieldMax),
			Inline: false,
		})
	}

	if len(fields) == 0 {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:  "Notice",
			Value: "Rules are not configured. Contact staff.",
		})
	}

	footerText := strings.TrimSpace(re.Footer)
	if footerText == "" {
		footerText = "Warn → Mute → Ban · Appeals: `/feedback`"
	}
	fields = append(fields, &discordgo.MessageEmbedField{
		Name:   "Enforcement",
		Value:  truncate(formatBlockText(footerText), discordEmbedFieldMax),
		Inline: false,
	})

	embed := &discordgo.MessageEmbed{
		Title:       rulesTitle(re.Title),
		Description: desc,
		Color:       color,
		Fields:      fields,
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Pelagornis · /rules",
		},
	}

	if re.ThumbnailURL != "" {
		embed.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: re.ThumbnailURL}
	}

	// Safety: stay under Discord's 6000-char cap for one message / one embed batch.
	if embedTextLen(embed) > discordTotalEmbedMax {
		embed = compactEmbed(embed)
	}

	return []*discordgo.MessageEmbed{embed}
}

func rulesTitle(title string) string {
	if strings.TrimSpace(title) != "" {
		return title
	}
	return "Server Rules"
}

func formatRuleFieldName(index int, r config.RuleEntry) string {
	n := index + 1
	title := strings.TrimSpace(r.Title)
	if r.Emoji != "" && title != "" {
		return fmt.Sprintf("`%02d` %s %s", n, r.Emoji, title)
	}
	if title != "" {
		return fmt.Sprintf("`%02d` %s", n, title)
	}
	return fmt.Sprintf("`%02d`", n)
}

func formatRuleBody(body string) string {
	lines := strings.Split(body, "\n")
	var out []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		line = strings.TrimPrefix(line, "•")
		line = strings.TrimPrefix(line, "▸")
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, "▸ "+line)
		}
	}
	return strings.Join(out, "\n")
}

func formatBlockText(text string) string {
	lines := strings.Split(text, "\n")
	var out []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "•") {
			line = "▸ " + strings.TrimSpace(strings.TrimPrefix(line, "•"))
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func buildRulesQuickRef(cfg *config.Config) string {
	var parts []string
	m := cfg.Moderation
	if m.NewUser.LinkRestrictHours > 0 {
		parts = append(parts, fmt.Sprintf("New members: links restricted %dh", m.NewUser.LinkRestrictHours))
	}
	if m.RateLimit.MaxMessages > 0 && m.RateLimit.WindowSeconds > 0 {
		parts = append(parts, fmt.Sprintf("Spam filter: %d msgs / %ds", m.RateLimit.MaxMessages, m.RateLimit.WindowSeconds))
	}
	if cfg.Feedback.ChannelID != "" && !isPlaceholder(cfg.Feedback.ChannelID) {
		parts = append(parts, "Reports: `/feedback`")
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " · ")
}

func compactEmbed(e *discordgo.MessageEmbed) *discordgo.MessageEmbed {
	for _, f := range e.Fields {
		if len(f.Value) > 400 {
			f.Value = truncate(f.Value, 400)
		}
	}
	if len(e.Description) > 800 {
		e.Description = truncate(e.Description, 800)
	}
	return e
}

func embedTextLen(e *discordgo.MessageEmbed) int {
	if e == nil {
		return 0
	}
	n := len(e.Title) + len(e.Description)
	if e.Footer != nil {
		n += len(e.Footer.Text)
	}
	for _, f := range e.Fields {
		n += len(f.Name) + len(f.Value)
	}
	return n
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func isPlaceholder(s string) bool {
	lower := strings.ToLower(s)
	return strings.Contains(lower, "xxxx") ||
		strings.Contains(lower, "your_") ||
		(strings.Contains(lower, "_id") && strings.Contains(lower, "channel"))
}
