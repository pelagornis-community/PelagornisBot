package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/pelagornis/pelagornis-bot/internal/config"
	"github.com/pelagornis/pelagornis-bot/internal/discord/commands"
	"github.com/pelagornis/pelagornis-bot/internal/discord/components"
	"github.com/pelagornis/pelagornis-bot/internal/discord/permissions"
	"github.com/pelagornis/pelagornis-bot/internal/modules/faq"
	"github.com/pelagornis/pelagornis-bot/internal/modules/feeds"
	"github.com/pelagornis/pelagornis-bot/internal/modules/forms"
	"github.com/pelagornis/pelagornis-bot/internal/modules/tickets"
	"github.com/pelagornis/pelagornis-bot/internal/storage/sqlite"
	"github.com/pelagornis/pelagornis-bot/internal/workflow"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type Gateway struct {
	session *discordgo.Session
	cfg     config.Config
	db      *sqlite.Store
	forms   *forms.Service
	feeds   feeds.Poller
	faq     *faq.Service
	tickets *tickets.Service
	once    sync.Once
}

func New(c config.Config, d *sqlite.Store) (*Gateway, error) {
	s, e := discordgo.New("Bot " + c.DiscordToken)
	if e != nil {
		return nil, e
	}
	s.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMembers
	s.AddHandler(func(_ *discordgo.Session, r *discordgo.Ready) { slog.Info("discord ready", "user", r.User.String()) })
	g := &Gateway{session: s, cfg: c, db: d, forms: forms.New(d.DB), feeds: feeds.Poller{Store: d, Client: feeds.New()}, faq: faq.New(d.DB), tickets: tickets.New(d.DB)}
	s.AddHandler(g.interaction)
	s.AddHandler(g.memberJoin)
	return g, nil
}
func (g *Gateway) Open() error {
	if e := g.session.Open(); e != nil {
		return e
	}
	return g.register()
}
func (g *Gateway) Close() error                { return g.session.Close() }
func (g *Gateway) Session() *discordgo.Session { return g.session }
func (g *Gateway) register() error {
	var guild string = g.cfg.DevGuildID
	existing, err := g.session.ApplicationCommands(g.cfg.ApplicationID, guild)
	if err != nil {
		return fmt.Errorf("list commands: %w", err)
	}
	byName := make(map[string]*discordgo.ApplicationCommand, len(existing))
	for _, command := range existing {
		byName[command.Name] = command
	}
	for _, c := range commands.Definitions() {
		if old := byName[c.Name]; old != nil {
			if _, err = g.session.ApplicationCommandEdit(g.cfg.ApplicationID, guild, old.ID, c); err != nil {
				return fmt.Errorf("update %s: %w", c.Name, err)
			}
			continue
		}
		if _, err = g.session.ApplicationCommandCreate(g.cfg.ApplicationID, guild, c); err != nil {
			return fmt.Errorf("register %s: %w", c.Name, err)
		}
	}
	return nil
}
func (g *Gateway) interaction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.GuildID == "" {
		g.reply(s, i, "This command is available in a server only.", true)
		return
	}
	if e := g.db.EnsureGuild(context.Background(), i.GuildID, g.cfg.DefaultLocale); e != nil {
		g.reply(s, i, "Configuration storage is temporarily unavailable.", true)
		return
	}
	if i.Type == discordgo.InteractionMessageComponent {
		g.component(s, i)
		return
	}
	if i.Type == discordgo.InteractionModalSubmit {
		g.modal(s, i)
		return
	}
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}
	name := i.ApplicationCommandData().Name
	if name != "help" && name != "rule" && name != "ticket" && name != "faq" && !permissions.CanManage(i) {
		g.reply(s, i, "Manage Server permission is required.", true)
		return
	}
	switch name {
	case "feeds":
		o := i.ApplicationCommandData().Options
		action := ""
		url := ""
		channel := ""
		id := ""
		for _, v := range o {
			switch v.Name {
			case "action":
				action = v.StringValue()
			case "url":
				url = v.StringValue()
			case "channel":
				channel = v.ChannelValue(s).ID
			case "id":
				id = v.StringValue()
			}
		}
		if action == "" {
			g.dashboard(s, i, "feeds")
			return
		}
		switch action {
		case "add":
			fid, e := g.feeds.Add(context.Background(), i.GuildID, url, channel, 900)
			if e != nil {
				g.reply(s, i, "Feed was not added: "+e.Error(), true)
				return
			}
			g.reply(s, i, "Feed added: `"+fid+"`. Its first poll establishes a baseline.", true)
		case "list":
			all, e := g.feeds.List(context.Background(), i.GuildID)
			if e != nil {
				g.reply(s, i, "Could not list feeds.", true)
				return
			}
			text := "No feeds configured."
			if len(all) > 0 {
				text = "Configured feeds:\n"
				for _, f := range all {
					text += "`" + f.ID + "` (" + f.Kind + ") → " + f.ChannelID + "\n"
				}
			}
			g.reply(s, i, text, true)
		case "pause":
			if e := g.feeds.Pause(context.Background(), i.GuildID, id); e != nil {
				g.reply(s, i, "Feed was not paused: "+e.Error(), true)
				return
			}
			g.reply(s, i, "Feed paused.", true)
		default:
			g.reply(s, i, "Use action: add, list, or pause.", true)
		}
	case "form":
		g.formCommand(s, i)
	case "setup":
		g.setupCommand(s, i)
	case "help":
		g.replyEmbeds(s, i, g.helpEmbeds(), true)
	case "rule":
		g.replyV2(s, i, g.ruleComponents(), false)
	case "welcome":
		g.welcomeCommand(s, i)
	case "ticket":
		g.ticketCommand(s, i)
	case "panel":
		g.panelCommand(s, i)
	case "role":
		g.roleCommand(s, i)
	case "release":
		g.releaseCommand(s, i)
	case "stats":
		g.statsCommand(s, i)
	case "workflow":
		g.workflowCommand(s, i)
	case "faq":
		q := ""
		for _, o := range i.ApplicationCommandData().Options {
			if o.Name == "query" {
				q = o.StringValue()
			}
		}
		rows, e := g.faq.Search(context.Background(), i.GuildID, q)
		if e != nil {
			g.reply(s, i, "FAQ search failed.", true)
			return
		}
		if len(rows) == 0 {
			g.reply(s, i, "No FAQ entry found.", true)
			return
		}
		g.reply(s, i, rows[0], true)
	default:
		g.reply(s, i, "Unknown command. Run /help for available commands.", true)
	}
}

func (g *Gateway) helpEmbeds() []*discordgo.MessageEmbed {
	return []*discordgo.MessageEmbed{
		{Title: "✦ PelagornisBot Guide", Description: "Use the commands below to get started. Commands marked **Staff** require the **Manage Server** permission.", Color: 0x5865F2, Fields: []*discordgo.MessageEmbedField{
			{Name: "📖 Everyone", Value: "`/rule` — Read server rules\n`/faq query:<keyword>` — Search published FAQ entries\n`/ticket action:open` — Open a private support ticket", Inline: false},
			{Name: "🧭 First-time staff setup", Value: "`/setup action:member-role role:@Member`\n`/setup action:pending-role role:@Pending`\n`/setup action:review-channel channel:#staff-review`\n`/welcome channel:#welcome`", Inline: false},
			{Name: "📝 Applications · panels", Value: "`/form action:create` → copy the returned ID\n`/form action:publish id:<form-id> channel:#apply`\n`/panel type:ticket channel:#support`\n`/role channel:#roles role:@Role`", Inline: false},
		}, Footer: &discordgo.MessageEmbedFooter{Text: "PelagornisBot · Select values from Discord's command menu to avoid typos."}},
		{Title: "Staff tools", Color: 0x57F287, Fields: []*discordgo.MessageEmbedField{
			{Name: "🎫 Tickets", Value: "`/ticket action:close id:<ticket-id>`\n`/ticket action:reopen id:<ticket-id>`\n`/ticket action:claim id:<ticket-id>`\n`/ticket action:assign id:<ticket-id> user:@staff`", Inline: false},
			{Name: "📰 Feeds · releases", Value: "`/feeds action:add url:<rss-url> channel:#news`\n`/feeds action:list` · `/feeds action:pause id:<source-id>`\n`/release action:add repo:owner/repo channel:#releases`\n`/release action:list` · `/release action:pause id:<source-id>`", Inline: false},
			{Name: "⚙️ Operations", Value: "`/stats` — Guild activity totals\n`/workflow action:list` — List workflows\n`/workflow action:add name:<name> definition:<json>`\n`/workflow action:pause id:<workflow-id>`", Inline: false},
		}, Footer: &discordgo.MessageEmbedFooter{Text: "Source IDs are returned by the matching list command."}},
	}
}

func (g *Gateway) dashboard(s *discordgo.Session, i *discordgo.InteractionCreate, kind string) {
	title, description := "✦ PelagornisBot", "Choose an action below."
	buttons := []discordgo.MessageComponent{}
	button := func(action, label, emoji string, style discordgo.ButtonStyle) {
		buttons = append(buttons, discordgo.Button{CustomID: components.Sign(g.cfg.SessionSecret, components.ID{Version: "v1", Action: action, Guild: i.GuildID}), Label: label, Emoji: &discordgo.ComponentEmoji{Name: emoji}, Style: style})
	}
	switch kind {
	case "setup":
		title, description = "⚙️ Server setup", "Configure onboarding roles and the private staff review channel."
		button("setup_member_pick", "Member role", "👤", discordgo.PrimaryButton)
		button("setup_pending_pick", "Pending role", "⏳", discordgo.SecondaryButton)
		button("setup_review_pick", "Review channel", "🔒", discordgo.SecondaryButton)
	case "form":
		title, description = "📝 Form center", "Create an application form or configure where staff reviews are delivered."
		button("form_create", "Create application", "➕", discordgo.PrimaryButton)
		button("setup_review_pick", "Review channel", "🔒", discordgo.SecondaryButton)
	case "ticket":
		title, description = "🎫 Support center", "Open a private support ticket. Staff management remains available from the ticket dashboard commands."
		button("ticket_open", "Open ticket", "🎫", discordgo.PrimaryButton)
	case "feeds":
		title, description = "📰 Feed center", "Add an RSS or Atom feed to post into this channel."
		button("feed_add_modal", "Add RSS feed", "➕", discordgo.PrimaryButton)
	case "release":
		title, description = "🚀 Release center", "Subscribe this channel to GitHub releases."
		button("release_add_modal", "Add GitHub release", "➕", discordgo.PrimaryButton)
	case "role":
		title, description = "✨ Role panel", "Choose a role to publish a self-service role button in this channel."
		button("role_pick", "Choose role", "✨", discordgo.PrimaryButton)
	case "panel":
		title, description = "📌 Panel center", "Publish a ticket panel or choose a role for a self-service panel."
		button("panel_ticket", "Ticket panel", "🎫", discordgo.PrimaryButton)
		button("role_pick", "Role panel", "✨", discordgo.SecondaryButton)
	case "workflow":
		title, description = "⚡ Workflow center", "Create a typed workflow from a name and JSON definition."
		button("workflow_add_modal", "Create workflow", "➕", discordgo.PrimaryButton)
	}
	g.replyEmbedComponents(s, i, &discordgo.MessageEmbed{Title: title, Description: description, Color: 0x5865F2, Footer: &discordgo.MessageEmbedFooter{Text: "PelagornisBot · Changes require administrator permission."}}, []discordgo.MessageComponent{discordgo.ActionsRow{Components: buttons}}, true)
}

func (g *Gateway) ruleComponents() []discordgo.MessageComponent {
	divider := true
	large := discordgo.SeparatorSpacingSizeLarge
	container := func(_ int, text ...string) discordgo.MessageComponent {
		parts := []discordgo.MessageComponent{}
		for n, value := range text {
			if n > 0 {
				parts = append(parts, discordgo.Separator{Divider: &divider, Spacing: &large})
			}
			parts = append(parts, discordgo.TextDisplay{Content: value})
		}
		return discordgo.Container{Components: parts}
	}
	return []discordgo.MessageComponent{
		container(0x5865F2, "# 📜 Server Rules\nWelcome to **Pelagornis**. Be respectful, stay on-topic, and keep the community safe.\n\n**Joining this server means you agree to these rules.**\n\n> ✨ New members: links restricted for **24 hours** · Spam filter: **5 messages / 10 seconds**"),
		container(0x57F287, "## `01` ⭐ Respect\n• No harassment, hate speech, slurs, or dogpiling\n• Stop if someone is uncomfortable; jokes are not an excuse\n• Ask staff for help instead of public flame wars · 13+ only", "## `02` 🤖 AI & APIs\n• Never post API keys, tokens, or webhook secrets\n• No jailbreaks, leaked prompts, impersonation, or abusive use", "## `03` 💬 Channels & Support\n• Use the right channel; read pins and topics first\n• Include steps and relevant logs in support requests\n• No unsolicited staff DMs or mass pings"),
		container(0xED4245, "## `04` 📢 Spam & Promotion\n• No spam, repost floods, raids, brigading, or unsolicited promotion", "## `05` 🔐 Content & Privacy\n• No NSFW outside marked channels, doxxing, phishing, or scam links\n• Follow [Discord Community Guidelines](https://discord.com/guidelines)", "## `06` 👮 Staff & enforcement\n• Follow moderator instructions; appeal through `/feedback` or mod DM\n• Reminder → Warning → Mute → Kick → Ban\n• **Zero tolerance:** CSAM, raids, API/key leaks, severe harassment\n\n-# Pelagornis · /rule"),
	}
}

func (g *Gateway) welcomeCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	channelID := ""
	for _, option := range i.ApplicationCommandData().Options {
		if option.Name == "channel" {
			if channel := option.ChannelValue(s); channel != nil {
				channelID = channel.ID
			}
		}
	}
	if channelID == "" {
		channelID = i.ChannelID
	}
	rulesURL := "https://discord.com/channels/" + i.GuildID + "/1563951289027666122"
	ticket := components.Sign(g.cfg.SessionSecret, components.ID{Version: "v1", Action: "ticket_open", Guild: i.GuildID})
	message := &discordgo.MessageSend{Components: []discordgo.MessageComponent{discordgo.Container{Components: []discordgo.MessageComponent{discordgo.TextDisplay{Content: "# ✦ Welcome to Pelagornis\nA calm place to learn, build, and help one another.\n\n## 📌 Get started\n1. Read the server rules\n2. Complete an application if one is posted\n3. Use the right channel and check its pins\n\n## 🛟 Need help?\nOpen a private ticket and the staff team will assist you.\n\n## 🔒 Safety first\nNever share passwords, API keys, tokens, or recovery codes.\n\n-# Pelagornis · Welcome aboard"}}}, discordgo.ActionsRow{Components: []discordgo.MessageComponent{
		discordgo.Button{URL: rulesURL, Label: "Read the rules", Emoji: &discordgo.ComponentEmoji{Name: "📜"}, Style: discordgo.LinkButton},
		discordgo.Button{CustomID: ticket, Label: "Get support", Emoji: &discordgo.ComponentEmoji{Name: "🛟"}, Style: discordgo.SecondaryButton},
	}}}, Flags: discordgo.MessageFlagsIsComponentsV2, AllowedMentions: &discordgo.MessageAllowedMentions{Parse: []discordgo.AllowedMentionType{}}}
	if file, err := openWelcomeHero(); err == nil {
		message.Files = []*discordgo.File{file}
		container := message.Components[0].(discordgo.Container)
		container.Components = append([]discordgo.MessageComponent{discordgo.MediaGallery{Items: []discordgo.MediaGalleryItem{{Media: discordgo.UnfurledMediaItem{URL: "attachment://hero.png"}}}}}, container.Components...)
		message.Components[0] = container
	} else if !os.IsNotExist(err) {
		slog.Warn("welcome hero unavailable; publishing without image", "error", err)
	}
	if _, err := s.ChannelMessageSendComplex(channelID, message); err != nil {
		g.reply(s, i, "Welcome panel could not be published.", true)
		return
	}
	g.reply(s, i, "Welcome panel published in <#"+channelID+">.", true)
}
func openWelcomeHero() (*discordgo.File, error) {
	for _, path := range []string{"assets/hero.png", "/assets/hero.png"} {
		file, err := os.Open(path)
		if err == nil {
			return &discordgo.File{Name: filepath.Base(path), ContentType: "image/png", Reader: file}, nil
		}
		if !os.IsNotExist(err) {
			return nil, err
		}
	}
	return nil, os.ErrNotExist
}
func (g *Gateway) setupCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	action, roleID, channelID := "", "", ""
	for _, option := range i.ApplicationCommandData().Options {
		switch option.Name {
		case "action":
			action = option.StringValue()
		case "role":
			if role := option.RoleValue(s, i.GuildID); role != nil {
				roleID = role.ID
			}
		case "channel":
			if channel := option.ChannelValue(s); channel != nil {
				channelID = channel.ID
			}
		}
	}
	if action == "" {
		g.dashboard(s, i, "setup")
		return
	}
	switch action {
	case "member-role", "pending-role":
		if roleID == "" {
			g.reply(s, i, "Choose a role for this setup action.", true)
			return
		}
		kind := strings.TrimSuffix(action, "-role")
		if e := g.db.SetRole(context.Background(), i.GuildID, kind, roleID); e != nil {
			g.reply(s, i, "Could not save the role configuration.", true)
			return
		}
		g.reply(s, i, kind+" role configured.", true)
	case "review-channel":
		if channelID == "" {
			g.reply(s, i, "Choose a private staff review channel.", true)
			return
		}
		if e := g.forms.SetReviewChannel(context.Background(), i.GuildID, channelID); e != nil {
			g.reply(s, i, "Could not save the review channel.", true)
			return
		}
		g.reply(s, i, "Staff review channel configured: <#"+channelID+">.", true)
	default:
		g.reply(s, i, "Use member-role, pending-role, or review-channel.", true)
	}
}
func (g *Gateway) ticketCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	action, id, userID := "", "", ""
	for _, o := range i.ApplicationCommandData().Options {
		if o.Name == "action" {
			action = o.StringValue()
		}
		if o.Name == "id" {
			id = o.StringValue()
		}
		if o.Name == "user" {
			if u := o.UserValue(s); u != nil {
				userID = u.ID
			}
		}
	}
	if action == "" {
		g.dashboard(s, i, "ticket")
		return
	}
	if action == "close" || action == "reopen" || action == "claim" || action == "assign" {
		if !permissions.CanManage(i) {
			g.reply(s, i, "Manage Server permission is required.", true)
			return
		}
		var e error
		switch action {
		case "close":
			e = g.tickets.Close(context.Background(), i.GuildID, id, i.Member.User.ID)
		case "reopen":
			e = g.tickets.Reopen(context.Background(), i.GuildID, id)
		case "claim":
			e = g.tickets.Assign(context.Background(), i.GuildID, id, i.Member.User.ID)
		case "assign":
			if userID == "" {
				g.reply(s, i, "Choose a staff member to assign.", true)
				return
			}
			e = g.tickets.Assign(context.Background(), i.GuildID, id, userID)
		}
		if e != nil {
			g.reply(s, i, "Ticket "+action+" failed: "+e.Error(), true)
			return
		}
		g.reply(s, i, "Ticket "+action+" completed.", true)
		return
	}
	if action != "open" {
		g.reply(s, i, "Use open, close, reopen, claim, or assign.", true)
		return
	}
	g.openTicket(s, i, i.Member.User.ID)
}
func (g *Gateway) openTicket(s *discordgo.Session, i *discordgo.InteractionCreate, userID string) {
	ch, e := s.GuildChannelCreateComplex(i.GuildID, discordgo.GuildChannelCreateData{Name: "ticket-" + userID, Type: discordgo.ChannelTypeGuildText, PermissionOverwrites: []*discordgo.PermissionOverwrite{{ID: i.GuildID, Type: discordgo.PermissionOverwriteTypeRole, Deny: discordgo.PermissionViewChannel}, {ID: userID, Type: discordgo.PermissionOverwriteTypeMember, Allow: discordgo.PermissionViewChannel | discordgo.PermissionSendMessages}}})
	if e != nil {
		g.reply(s, i, "Ticket channel could not be created.", true)
		return
	}
	ticket, e := g.tickets.Create(context.Background(), i.GuildID, userID, ch.ID)
	if e != nil {
		_, _ = s.ChannelDelete(ch.ID)
		g.reply(s, i, "Ticket storage failed; channel removed.", true)
		return
	}
	_ = g.db.IncrementMetric(context.Background(), i.GuildID, "ticket.opened", "")
	g.reply(s, i, "Ticket opened: <#"+ch.ID+"> ("+ticket+")", true)
}

func (g *Gateway) panelCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	kind, channelID, roleID := "", "", ""
	for _, o := range i.ApplicationCommandData().Options {
		switch o.Name {
		case "type":
			kind = o.StringValue()
		case "channel":
			if c := o.ChannelValue(s); c != nil {
				channelID = c.ID
			}
		case "role":
			if r := o.RoleValue(s, i.GuildID); r != nil {
				roleID = r.ID
			}
		}
	}
	if kind == "" {
		g.dashboard(s, i, "panel")
		return
	}
	if kind == "ticket" {
		g.publishTicketPanel(s, i, channelID)
		return
	}
	if kind == "role" {
		g.publishRolePanel(s, i, channelID, roleID)
		return
	}
	g.reply(s, i, "Panel type must be ticket or role.", true)
}
func (g *Gateway) publishTicketPanel(s *discordgo.Session, i *discordgo.InteractionCreate, channelID string) {
	custom := components.Sign(g.cfg.SessionSecret, components.ID{Version: "v1", Action: "ticket_open", Guild: i.GuildID})
	_, err := s.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{Embeds: []*discordgo.MessageEmbed{{Title: "🎫 Support desk", Description: "Need help? Create a private ticket and the staff team can assist you.", Color: 0x5865F2, Footer: &discordgo.MessageEmbedFooter{Text: "Pelagornis · Support"}}}, Components: []discordgo.MessageComponent{discordgo.ActionsRow{Components: []discordgo.MessageComponent{discordgo.Button{CustomID: custom, Label: "Open ticket", Emoji: &discordgo.ComponentEmoji{Name: "🎫"}, Style: discordgo.PrimaryButton}}}}, AllowedMentions: &discordgo.MessageAllowedMentions{Parse: []discordgo.AllowedMentionType{}}})
	if err != nil {
		g.reply(s, i, "Ticket panel could not be published.", true)
		return
	}
	g.reply(s, i, "Ticket panel published in <#"+channelID+">.", true)
}
func (g *Gateway) roleCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	channelID, roleID := "", ""
	for _, o := range i.ApplicationCommandData().Options {
		if o.Name == "channel" {
			if c := o.ChannelValue(s); c != nil {
				channelID = c.ID
			}
		}
		if o.Name == "role" {
			if r := o.RoleValue(s, i.GuildID); r != nil {
				roleID = r.ID
			}
		}
	}
	if roleID == "" {
		g.dashboard(s, i, "role")
		return
	}
	g.publishRolePanel(s, i, channelID, roleID)
}
func (g *Gateway) publishRolePanel(s *discordgo.Session, i *discordgo.InteractionCreate, channelID, roleID string) {
	if channelID == "" || roleID == "" {
		g.reply(s, i, "Choose a channel and role.", true)
		return
	}
	if err := g.ensureAssignableRole(s, i.GuildID, roleID); err != nil {
		g.reply(s, i, "That role cannot be managed: "+err.Error(), true)
		return
	}
	custom := components.Sign(g.cfg.SessionSecret, components.ID{Version: "v1", Action: "role_toggle", Guild: i.GuildID, Resource: roleID})
	_, err := s.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{Embeds: []*discordgo.MessageEmbed{{Title: "✨ Choose your role", Description: "Use the button below to add or remove <@&" + roleID + ">.", Color: 0xFEE75C, Footer: &discordgo.MessageEmbedFooter{Text: "Pelagornis · Self-service roles"}}}, Components: []discordgo.MessageComponent{discordgo.ActionsRow{Components: []discordgo.MessageComponent{discordgo.Button{CustomID: custom, Label: "Toggle role", Emoji: &discordgo.ComponentEmoji{Name: "✨"}, Style: discordgo.SecondaryButton}}}}, AllowedMentions: &discordgo.MessageAllowedMentions{Roles: []string{roleID}}})
	if err != nil {
		g.reply(s, i, "Role panel could not be published.", true)
		return
	}
	g.reply(s, i, "Role panel published in <#"+channelID+">.", true)
}
func (g *Gateway) releaseCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	action, repo, channelID, id := "", "", "", ""
	for _, o := range i.ApplicationCommandData().Options {
		switch o.Name {
		case "action":
			action = o.StringValue()
		case "repo":
			repo = o.StringValue()
		case "channel":
			if c := o.ChannelValue(s); c != nil {
				channelID = c.ID
			}
		case "id":
			id = o.StringValue()
		}
	}
	if action == "" {
		g.dashboard(s, i, "release")
		return
	}
	switch action {
	case "add":
		if repo == "" || channelID == "" {
			g.reply(s, i, "Provide repository (owner/name) and destination channel.", true)
			return
		}
		fid, err := g.feeds.AddRelease(context.Background(), i.GuildID, repo, channelID, 900)
		if err != nil {
			g.reply(s, i, "Release source was not added: "+err.Error(), true)
			return
		}
		g.reply(s, i, "GitHub release source added: `"+fid+"`. An initial release check is queued now.", true)
	case "list":
		all, err := g.feeds.List(context.Background(), i.GuildID)
		if err != nil {
			g.reply(s, i, "Could not list release sources.", true)
			return
		}
		out := "No GitHub release sources configured."
		for _, f := range all {
			if f.Kind == "github_release" {
				if out == "No GitHub release sources configured." {
					out = "GitHub release sources:\n"
				}
				out += "`" + f.ID + "` — " + f.Repo + " → <#" + f.ChannelID + ">\n"
			}
		}
		g.reply(s, i, out, true)
	case "pause":
		if err := g.feeds.Pause(context.Background(), i.GuildID, id); err != nil {
			g.reply(s, i, "Release source was not paused: "+err.Error(), true)
			return
		}
		g.reply(s, i, "Release source paused.", true)
	default:
		g.reply(s, i, "Use add, list, or pause.", true)
	}
}
func (g *Gateway) statsCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx := context.Background()
	var pending, approved, openTickets, deliveries int
	if err := g.db.DB.QueryRowContext(ctx, `select count(*) from form_submissions where guild_id=? and status='pending'`, i.GuildID).Scan(&pending); err != nil {
		g.reply(s, i, "Statistics are temporarily unavailable.", true)
		return
	}
	_ = g.db.DB.QueryRowContext(ctx, `select count(*) from form_submissions where guild_id=? and status='approved'`, i.GuildID).Scan(&approved)
	_ = g.db.DB.QueryRowContext(ctx, `select count(*) from tickets where guild_id=? and status in ('open','claimed')`, i.GuildID).Scan(&openTickets)
	_ = g.db.DB.QueryRowContext(ctx, `select count(*) from feed_deliveries where guild_id=? and status='posted'`, i.GuildID).Scan(&deliveries)
	g.replyEmbeds(s, i, []*discordgo.MessageEmbed{{Title: "📊 Guild statistics", Description: "Current totals recorded by PelagornisBot for this server.", Color: 0x5865F2, Fields: []*discordgo.MessageEmbedField{
		{Name: "📝 Forms", Value: fmt.Sprintf("Pending review: **%d**\nApproved: **%d**", pending, approved), Inline: true},
		{Name: "🎫 Support", Value: fmt.Sprintf("Active tickets: **%d**", openTickets), Inline: true},
		{Name: "📰 Delivery", Value: fmt.Sprintf("Posted feed items: **%d**", deliveries), Inline: true},
	}, Footer: &discordgo.MessageEmbedFooter{Text: "Counts are scoped to this server."}}}, true)
}
func (g *Gateway) workflowCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	action, name, raw, id := "", "", "", ""
	for _, o := range i.ApplicationCommandData().Options {
		switch o.Name {
		case "action":
			action = o.StringValue()
		case "name":
			name = o.StringValue()
		case "definition":
			raw = o.StringValue()
		case "id":
			id = o.StringValue()
		}
	}
	if action == "" {
		g.dashboard(s, i, "workflow")
		return
	}
	ctx := context.Background()
	switch action {
	case "add":
		var def workflow.Definition
		if err := json.Unmarshal([]byte(raw), &def); err != nil || def.Validate() != nil {
			g.reply(s, i, "Definition must be valid JSON with trigger and 1-10 allowed actions.", true)
			return
		}
		if name == "" {
			g.reply(s, i, "Provide a workflow name.", true)
			return
		}
		var newID string
		err := g.db.DB.QueryRowContext(ctx, `insert into workflows(id,guild_id,name,definition_json) values(lower(hex(randomblob(16))),?,?,?) returning id`, i.GuildID, name, raw).Scan(&newID)
		if err != nil {
			g.reply(s, i, "Workflow could not be saved.", true)
			return
		}
		g.reply(s, i, "Workflow added: `"+newID+"`.", true)
	case "list":
		rows, err := g.db.DB.QueryContext(ctx, `select id,name,enabled from workflows where guild_id=? order by name`, i.GuildID)
		if err != nil {
			g.reply(s, i, "Could not list workflows.", true)
			return
		}
		defer rows.Close()
		out := "No workflows configured."
		for rows.Next() {
			var wid, wn string
			var enabled int
			if rows.Scan(&wid, &wn, &enabled) == nil {
				if out == "No workflows configured." {
					out = "Workflows:\n"
				}
				state := "paused"
				if enabled == 1 {
					state = "active"
				}
				out += "`" + wid + "` — " + wn + " (" + state + ")\n"
			}
		}
		g.reply(s, i, out, true)
	case "pause":
		result, err := g.db.DB.ExecContext(ctx, `update workflows set enabled=0 where id=? and guild_id=? and enabled=1`, id, i.GuildID)
		if err != nil {
			g.reply(s, i, "Workflow could not be paused.", true)
			return
		}
		n, _ := result.RowsAffected()
		if n != 1 {
			g.reply(s, i, "Workflow is not active in this guild.", true)
			return
		}
		g.reply(s, i, "Workflow paused.", true)
	default:
		g.reply(s, i, "Use add, list, or pause.", true)
	}
}
func (g *Gateway) formCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	action, id, channel := "", "", ""
	for _, o := range i.ApplicationCommandData().Options {
		switch o.Name {
		case "action":
			action = o.StringValue()
		case "id":
			id = o.StringValue()
		case "channel":
			if c := o.ChannelValue(s); c != nil {
				channel = c.ID
			}
		}
	}
	if action == "" {
		g.dashboard(s, i, "form")
		return
	}
	switch action {
	case "review-channel":
		if channel == "" {
			g.reply(s, i, "Choose a private staff review channel.", true)
			return
		}
		if e := g.forms.SetReviewChannel(context.Background(), i.GuildID, channel); e != nil {
			g.reply(s, i, "Could not save the review channel.", true)
			return
		}
		g.reply(s, i, "Staff review channel configured: <#"+channel+">.", true)
	case "create":
		f, e := g.forms.Create(context.Background(), i.GuildID, "application", forms.Schema{Title: "Community application", Fields: []forms.Field{{ID: "nickname", Label: "Nickname", Type: "short_text", Required: true, Max: 100}, {ID: "intro", Label: "Introduce yourself", Type: "paragraph", Required: true, Max: 1000}}})
		if e != nil {
			g.reply(s, i, "Could not create form: "+e.Error(), true)
			return
		}
		g.reply(s, i, "Draft created: `"+f.ID+"`.", true)
	case "publish":
		if e := g.forms.Publish(context.Background(), i.GuildID, id); e != nil {
			g.reply(s, i, "Could not publish: "+e.Error(), true)
			return
		}
		if channel != "" {
			custom := components.Sign(g.cfg.SessionSecret, components.ID{Version: "v1", Action: "form_start", Guild: i.GuildID, Resource: id})
			_, e := s.ChannelMessageSendComplex(channel, &discordgo.MessageSend{Embeds: []*discordgo.MessageEmbed{{Title: "📝 Community application", Description: "Ready to join? Complete the short application below. Your answers are sent only to the staff review channel.", Color: 0x5865F2, Footer: &discordgo.MessageEmbedFooter{Text: "Pelagornis · Applications"}}}, Components: []discordgo.MessageComponent{discordgo.ActionsRow{Components: []discordgo.MessageComponent{discordgo.Button{CustomID: custom, Label: "Start application", Emoji: &discordgo.ComponentEmoji{Name: "📝"}, Style: discordgo.PrimaryButton}}}}, AllowedMentions: &discordgo.MessageAllowedMentions{Parse: []discordgo.AllowedMentionType{}}})
			if e != nil {
				g.reply(s, i, "Form published but panel failed: "+e.Error(), true)
				return
			}
		}
		g.reply(s, i, "Form published.", true)
	default:
		g.reply(s, i, "Use create, publish, or review-channel.", true)
	}
}
func (g *Gateway) approveRoles(s *discordgo.Session, guild, user string) error {
	member, e := g.db.Role(context.Background(), guild, "member")
	if e != nil {
		return fmt.Errorf("member role is not configured")
	}
	pending, _ := g.db.Role(context.Background(), guild, "pending")
	if err := g.ensureAssignableRole(s, guild, member); err != nil {
		return fmt.Errorf("member role is unsafe or above bot: %w", err)
	}
	if pending == "" {
		return fmt.Errorf("pending role is not configured")
	}
	if err := g.ensureAssignableRole(s, guild, pending); err != nil {
		return fmt.Errorf("pending role is unsafe or above bot: %w", err)
	}
	if e = s.GuildMemberRoleAdd(guild, user, member); e != nil {
		return e
	}
	if e = s.GuildMemberRoleRemove(guild, user, pending); e != nil {
		return e
	}
	return nil
}
func (g *Gateway) ensureAssignableRole(s *discordgo.Session, guild, roleID string) error {
	roles, err := s.GuildRoles(guild)
	if err != nil {
		return err
	}
	me, err := s.User("@me")
	if err != nil {
		return err
	}
	bot, err := s.GuildMember(guild, me.ID)
	if err != nil {
		return err
	}
	top := 0
	for _, role := range roles {
		for _, have := range bot.Roles {
			if role.ID == have && role.Position > top {
				top = role.Position
			}
		}
	}
	if !permissions.CanAssignRole(&discordgo.Guild{Roles: roles}, roleID, top) {
		return fmt.Errorf("role is managed, @everyone, or not below the bot")
	}
	return nil
}
func (g *Gateway) component(s *discordgo.Session, i *discordgo.InteractionCreate) {
	id, e := components.Parse(g.cfg.SessionSecret, i.MessageComponentData().CustomID)
	if e != nil || id.Guild != i.GuildID || (id.User != "" && (i.Member == nil || id.User != i.Member.User.ID)) {
		g.reply(s, i, "This panel action is invalid or expired.", true)
		return
	}
	if (strings.HasPrefix(id.Action, "setup_") || strings.HasPrefix(id.Action, "feed_") || strings.HasPrefix(id.Action, "release_") || strings.HasPrefix(id.Action, "workflow_") || id.Action == "form_create" || id.Action == "role_pick" || id.Action == "role_publish_set" || id.Action == "panel_ticket") && !permissions.CanManage(i) {
		g.reply(s, i, "Manage Server permission is required.", true)
		return
	}
	if id.Action == "form_approve" || id.Action == "form_reject" {
		g.reviewComponent(s, i, id)
		return
	}
	if id.Action == "form_create" {
		f, err := g.forms.Create(context.Background(), i.GuildID, "application", forms.Schema{Title: "Community application", Fields: []forms.Field{{ID: "nickname", Label: "Nickname", Type: "short_text", Required: true, Max: 100}, {ID: "intro", Label: "Introduce yourself", Type: "paragraph", Required: true, Max: 1000}}})
		if err != nil {
			g.reply(s, i, "Could not create form: "+err.Error(), true)
			return
		}
		g.reply(s, i, "Draft created: `"+f.ID+"`. Open `/form` again and use the legacy publish option until the publish picker is added.", true)
		return
	}
	if id.Action == "setup_member_pick" || id.Action == "setup_pending_pick" || id.Action == "role_pick" {
		setAction := "setup_member_set"
		label := "Choose the member role"
		if id.Action == "setup_pending_pick" {
			setAction = "setup_pending_set"
			label = "Choose the pending role"
		}
		if id.Action == "role_pick" {
			setAction = "role_publish_set"
			label = "Choose a role for the panel"
		}
		custom := components.Sign(g.cfg.SessionSecret, components.ID{Version: "v1", Action: setAction, Guild: i.GuildID})
		g.replyEmbedComponents(s, i, &discordgo.MessageEmbed{Title: "✨ Role selection", Description: label, Color: 0x5865F2}, []discordgo.MessageComponent{discordgo.ActionsRow{Components: []discordgo.MessageComponent{discordgo.SelectMenu{MenuType: discordgo.RoleSelectMenu, CustomID: custom, Placeholder: "Select a role", MinValues: ptrInt(1), MaxValues: 1}}}}, true)
		return
	}
	if id.Action == "setup_review_pick" {
		custom := components.Sign(g.cfg.SessionSecret, components.ID{Version: "v1", Action: "setup_review_set", Guild: i.GuildID})
		g.replyEmbedComponents(s, i, &discordgo.MessageEmbed{Title: "🔒 Review channel", Description: "Choose the private staff review channel.", Color: 0x5865F2}, []discordgo.MessageComponent{discordgo.ActionsRow{Components: []discordgo.MessageComponent{discordgo.SelectMenu{MenuType: discordgo.ChannelSelectMenu, CustomID: custom, Placeholder: "Select a channel", MinValues: ptrInt(1), MaxValues: 1, ChannelTypes: []discordgo.ChannelType{discordgo.ChannelTypeGuildText}}}}}, true)
		return
	}
	if id.Action == "setup_member_set" || id.Action == "setup_pending_set" || id.Action == "setup_review_set" || id.Action == "role_publish_set" {
		values := i.MessageComponentData().Values
		if len(values) != 1 {
			g.reply(s, i, "Select exactly one item.", true)
			return
		}
		value := values[0]
		if id.Action == "setup_review_set" {
			if err := g.forms.SetReviewChannel(context.Background(), i.GuildID, value); err != nil {
				g.reply(s, i, "Could not save the review channel.", true)
			} else {
				g.reply(s, i, "Staff review channel configured: <#"+value+">.", true)
			}
			return
		}
		if id.Action == "role_publish_set" {
			g.publishRolePanel(s, i, i.ChannelID, value)
			return
		}
		kind := "member"
		if id.Action == "setup_pending_set" {
			kind = "pending"
		}
		if err := g.db.SetRole(context.Background(), i.GuildID, kind, value); err != nil {
			g.reply(s, i, "Could not save the role configuration.", true)
		} else {
			g.reply(s, i, kind+" role configured.", true)
		}
		return
	}
	if id.Action == "panel_ticket" {
		g.publishTicketPanel(s, i, i.ChannelID)
		return
	}
	if id.Action == "feed_add_modal" || id.Action == "release_add_modal" || id.Action == "workflow_add_modal" {
		action := strings.TrimSuffix(id.Action, "_modal") + "_submit"
		custom := components.Sign(g.cfg.SessionSecret, components.ID{Version: "v1", Action: action, Guild: i.GuildID, User: i.Member.User.ID})
		inputs := []discordgo.MessageComponent{discordgo.ActionsRow{Components: []discordgo.MessageComponent{discordgo.TextInput{CustomID: "value", Label: "RSS / Atom URL", Style: discordgo.TextInputShort, Required: true, MaxLength: 500}}}}
		title := "Add RSS feed"
		if id.Action == "release_add_modal" {
			title = "Add GitHub release"
			inputs = []discordgo.MessageComponent{discordgo.ActionsRow{Components: []discordgo.MessageComponent{discordgo.TextInput{CustomID: "value", Label: "Repository (owner/name)", Style: discordgo.TextInputShort, Required: true, MaxLength: 200}}}}
		}
		if id.Action == "workflow_add_modal" {
			title = "Create workflow"
			inputs = []discordgo.MessageComponent{discordgo.ActionsRow{Components: []discordgo.MessageComponent{discordgo.TextInput{CustomID: "name", Label: "Workflow name", Style: discordgo.TextInputShort, Required: true, MaxLength: 100}}}, discordgo.ActionsRow{Components: []discordgo.MessageComponent{discordgo.TextInput{CustomID: "definition", Label: "Definition JSON", Style: discordgo.TextInputParagraph, Required: true, MaxLength: 4000}}}}
		}
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{Type: discordgo.InteractionResponseModal, Data: &discordgo.InteractionResponseData{CustomID: custom, Title: title, Components: inputs}})
		return
	}
	if id.Action == "ticket_open" {
		if i.Member == nil || i.Member.User == nil {
			g.reply(s, i, "This action requires a server member.", true)
			return
		}
		g.openTicket(s, i, i.Member.User.ID)
		return
	}
	if id.Action == "rules_show" {
		g.replyV2(s, i, g.ruleComponents(), true)
		return
	}
	if id.Action == "role_toggle" {
		g.toggleRole(s, i, id.Resource)
		return
	}
	if id.Action != "form_start" {
		g.reply(s, i, "This panel action is invalid or expired.", true)
		return
	}
	f, e := g.forms.Published(context.Background(), id.Guild, id.Resource)
	if e != nil {
		g.reply(s, i, "That form is unavailable.", true)
		return
	}
	rows := []discordgo.MessageComponent{}
	for _, field := range f.Schema.Fields {
		if field.Type != "short_text" && field.Type != "paragraph" {
			g.reply(s, i, "This form requires a staged component flow.", true)
			return
		}
		style := discordgo.TextInputShort
		if field.Type == "paragraph" {
			style = discordgo.TextInputParagraph
		}
		rows = append(rows, discordgo.ActionsRow{Components: []discordgo.MessageComponent{discordgo.TextInput{CustomID: field.ID, Label: field.Label, Style: style, Required: field.Required, MaxLength: field.Max}}})
	}
	modal := components.Sign(g.cfg.SessionSecret, components.ID{Version: "v1", Action: "form_submit", Guild: id.Guild, Resource: id.Resource, User: id.User})
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{Type: discordgo.InteractionResponseModal, Data: &discordgo.InteractionResponseData{CustomID: modal, Title: f.Schema.Title, Components: rows}})
}
func (g *Gateway) toggleRole(s *discordgo.Session, i *discordgo.InteractionCreate, roleID string) {
	if i.Member == nil || i.Member.User == nil {
		g.reply(s, i, "This action requires a server member.", true)
		return
	}
	if err := g.ensureAssignableRole(s, i.GuildID, roleID); err != nil {
		g.reply(s, i, "This role is no longer safe for the bot to manage.", true)
		return
	}
	has := false
	for _, r := range i.Member.Roles {
		if r == roleID {
			has = true
			break
		}
	}
	var err error
	if has {
		err = s.GuildMemberRoleRemove(i.GuildID, i.Member.User.ID, roleID)
	} else {
		err = s.GuildMemberRoleAdd(i.GuildID, i.Member.User.ID, roleID)
	}
	if err != nil {
		g.reply(s, i, "Role change failed.", true)
		return
	}
	if has {
		g.reply(s, i, "Role removed.", true)
	} else {
		g.reply(s, i, "Role added.", true)
	}
}
func (g *Gateway) modal(s *discordgo.Session, i *discordgo.InteractionCreate) {
	id, e := components.Parse(g.cfg.SessionSecret, i.ModalSubmitData().CustomID)
	if e != nil || id.Guild != i.GuildID || i.Member == nil || id.User != i.Member.User.ID {
		g.reply(s, i, "This form is invalid or expired.", true)
		return
	}
	if id.Action == "form_reject_submit" {
		g.rejectModal(s, i, id)
		return
	}
	if id.Action == "feed_add_submit" || id.Action == "release_add_submit" || id.Action == "workflow_add_submit" {
		if !permissions.CanManage(i) {
			g.reply(s, i, "Manage Server permission is required.", true)
			return
		}
		values := map[string]string{}
		for _, row := range i.ModalSubmitData().Components {
			if ar, ok := row.(*discordgo.ActionsRow); ok {
				for _, part := range ar.Components {
					if input, ok := part.(*discordgo.TextInput); ok {
						values[input.CustomID] = strings.TrimSpace(input.Value)
					}
				}
			}
		}
		if id.Action == "feed_add_submit" {
			fid, err := g.feeds.Add(context.Background(), i.GuildID, values["value"], i.ChannelID, 900)
			if err != nil {
				g.reply(s, i, "Feed was not added: "+err.Error(), true)
			} else {
				g.reply(s, i, "Feed added: `"+fid+"`.", true)
			}
			return
		}
		if id.Action == "release_add_submit" {
			fid, err := g.feeds.AddRelease(context.Background(), i.GuildID, values["value"], i.ChannelID, 900)
			if err != nil {
				g.reply(s, i, "Release source was not added: "+err.Error(), true)
			} else {
				g.reply(s, i, "GitHub release source added: `"+fid+"`. An initial release check is queued now.", true)
			}
			return
		}
		var def workflow.Definition
		if err := json.Unmarshal([]byte(values["definition"]), &def); err != nil || def.Validate() != nil || values["name"] == "" {
			g.reply(s, i, "Definition must be valid JSON with trigger and 1-10 allowed actions.", true)
			return
		}
		var newID string
		err := g.db.DB.QueryRowContext(context.Background(), `insert into workflows(id,guild_id,name,definition_json) values(lower(hex(randomblob(16))),?,?,?) returning id`, i.GuildID, values["name"], values["definition"]).Scan(&newID)
		if err != nil {
			g.reply(s, i, "Workflow could not be saved.", true)
		} else {
			g.reply(s, i, "Workflow added: `"+newID+"`.", true)
		}
		return
	}
	if id.Action != "form_submit" {
		g.reply(s, i, "This form is invalid or expired.", true)
		return
	}
	f, e := g.forms.Published(context.Background(), id.Guild, id.Resource)
	if e != nil {
		g.reply(s, i, "That form is unavailable.", true)
		return
	}
	answers := map[string]string{}
	for _, row := range i.ModalSubmitData().Components {
		if ar, ok := row.(*discordgo.ActionsRow); ok {
			for _, part := range ar.Components {
				if in, ok := part.(*discordgo.TextInput); ok {
					answers[in.CustomID] = in.Value
				}
			}
		}
	}
	reviewChannel, e := g.forms.ReviewChannel(context.Background(), i.GuildID)
	if e != nil {
		g.reply(s, i, "Staff review is not configured for this server.", true)
		return
	}
	submission, e := g.forms.Submit(context.Background(), f, i.Member.User.ID, answers)
	if e != nil {
		g.reply(s, i, "Please correct the form: "+e.Error(), true)
		return
	}
	message, e := s.ChannelMessageSendComplex(reviewChannel, g.reviewMessage(i.GuildID, submission, i.Member.User.ID, answers))
	if e != nil {
		g.reply(s, i, "Submission saved, but staff review delivery failed. Reference: "+submission, true)
		return
	}
	if e := g.forms.SetReviewMessage(context.Background(), i.GuildID, submission, message.ID); e != nil {
		slog.Warn("form review message was not recorded", "submission", submission, "error", e)
	}
	g.reply(s, i, "Submitted for private staff review. Reference: "+submission, true)
}

func (g *Gateway) reviewMessage(guild, submission, user string, answers map[string]string) *discordgo.MessageSend {
	fields := make([]*discordgo.MessageEmbedField, 0, len(answers))
	for key, value := range answers {
		value = strings.TrimSpace(value)
		if len(value) > 1024 {
			value = value[:1021] + "..."
		}
		fields = append(fields, &discordgo.MessageEmbedField{Name: key, Value: value, Inline: false})
	}
	approve := components.Sign(g.cfg.SessionSecret, components.ID{Version: "v1", Action: "form_approve", Guild: guild, Resource: submission})
	reject := components.Sign(g.cfg.SessionSecret, components.ID{Version: "v1", Action: "form_reject", Guild: guild, Resource: submission})
	return &discordgo.MessageSend{
		Embed: &discordgo.MessageEmbed{Title: "Application awaiting review", Description: "Applicant: <@" + user + ">\nSubmission: `" + submission + "`", Fields: fields},
		Components: []discordgo.MessageComponent{discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.Button{CustomID: approve, Label: "Approve", Style: discordgo.SuccessButton},
			discordgo.Button{CustomID: reject, Label: "Reject", Style: discordgo.DangerButton},
		}}},
		AllowedMentions: &discordgo.MessageAllowedMentions{Parse: []discordgo.AllowedMentionType{}},
	}
}

func (g *Gateway) reviewComponent(s *discordgo.Session, i *discordgo.InteractionCreate, id components.ID) {
	if !permissions.CanManage(i) {
		g.reply(s, i, "Manage Server permission is required to review applications.", true)
		return
	}
	if id.Action == "form_reject" {
		modal := components.Sign(g.cfg.SessionSecret, components.ID{Version: "v1", Action: "form_reject_submit", Guild: id.Guild, Resource: id.Resource, User: i.Member.User.ID})
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{Type: discordgo.InteractionResponseModal, Data: &discordgo.InteractionResponseData{CustomID: modal, Title: "Reject application", Components: []discordgo.MessageComponent{discordgo.ActionsRow{Components: []discordgo.MessageComponent{discordgo.TextInput{CustomID: "reason", Label: "Reason", Style: discordgo.TextInputParagraph, Required: true, MaxLength: 500}}}}}})
		return
	}
	g.completeReview(s, i, id.Resource, "approved", "")
}

func (g *Gateway) rejectModal(s *discordgo.Session, i *discordgo.InteractionCreate, id components.ID) {
	if !permissions.CanManage(i) {
		g.reply(s, i, "Manage Server permission is required to review applications.", true)
		return
	}
	reason := ""
	for _, row := range i.ModalSubmitData().Components {
		if ar, ok := row.(*discordgo.ActionsRow); ok {
			for _, part := range ar.Components {
				if input, ok := part.(*discordgo.TextInput); ok && input.CustomID == "reason" {
					reason = strings.TrimSpace(input.Value)
				}
			}
		}
	}
	g.completeReview(s, i, id.Resource, "rejected", reason)
}

func (g *Gateway) completeReview(s *discordgo.Session, i *discordgo.InteractionCreate, submissionID, status, reason string) {
	ctx := context.Background()
	submission, e := g.forms.Pending(ctx, i.GuildID, submissionID)
	if e != nil {
		g.reply(s, i, "This submission has already been reviewed or is unavailable.", true)
		return
	}
	if status == "approved" {
		if e := g.approveRoles(s, i.GuildID, submission.UserID); e != nil {
			g.reply(s, i, "Roles were not changed: "+e.Error(), true)
			return
		}
	}
	updated, e := g.forms.Review(ctx, i.GuildID, submissionID, i.Member.User.ID, status, reason)
	if e != nil {
		g.reply(s, i, "Review failed: "+e.Error(), true)
		return
	}
	if !updated {
		g.reply(s, i, "This submission was already reviewed by another administrator.", true)
		return
	}
	_ = g.db.Audit(ctx, i.GuildID, i.Member.User.ID, "form."+status, submissionID)
	g.notifyFormDecision(s, submission.UserID, status, reason)
	g.reply(s, i, "Submission "+status+".", true)
}

func (g *Gateway) notifyFormDecision(s *discordgo.Session, user, status, reason string) {
	channel, e := s.UserChannelCreate(user)
	if e != nil {
		slog.Warn("could not open form decision DM", "user", user, "error", e)
		return
	}
	message := "Your community application was " + status + "."
	if status == "rejected" {
		message += " Reason: " + reason
	}
	color := 0x57F287
	if status == "rejected" {
		color = 0xED4245
	}
	if _, e := s.ChannelMessageSendComplex(channel.ID, &discordgo.MessageSend{Embeds: []*discordgo.MessageEmbed{{Title: "📝 Application update", Description: message, Color: color, Footer: &discordgo.MessageEmbedFooter{Text: "Pelagornis · Applications"}}}, AllowedMentions: &discordgo.MessageAllowedMentions{Parse: []discordgo.AllowedMentionType{}}}); e != nil {
		slog.Warn("could not send form decision DM", "user", user, "error", e)
	}
}
func (g *Gateway) reply(s *discordgo.Session, i *discordgo.InteractionCreate, text string, ephemeral bool) {
	flags := discordgo.MessageFlags(0)
	if ephemeral {
		flags = discordgo.MessageFlagsEphemeral
	}
	title, color := responseStyle(text)
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{Type: discordgo.InteractionResponseChannelMessageWithSource, Data: &discordgo.InteractionResponseData{Components: v2Container(color, "## "+title+"\n"+text+"\n\n-# PelagornisBot"), Flags: flags | discordgo.MessageFlagsIsComponentsV2, AllowedMentions: &discordgo.MessageAllowedMentions{Parse: []discordgo.AllowedMentionType{}}}})
}
func responseStyle(text string) (string, int) {
	lower := strings.ToLower(text)
	for _, marker := range []string{"failed", "could not", "required", "unavailable", "invalid", "not active", "not open", "not closed", "not configured", "choose ", "provide ", "use ", "denied", "unsafe", "temporarily unavailable"} {
		if strings.Contains(lower, marker) {
			return "⚠️ Action needed", 0xED4245
		}
	}
	if strings.Contains(lower, "configured") || strings.Contains(lower, "list") || strings.Contains(lower, "statistics") {
		return "✦ PelagornisBot", 0x5865F2
	}
	return "✅ Complete", 0x57F287
}
func (g *Gateway) replyEmbeds(s *discordgo.Session, i *discordgo.InteractionCreate, embeds []*discordgo.MessageEmbed, ephemeral bool) {
	parts := []discordgo.MessageComponent{}
	for _, e := range embeds {
		body := "# " + e.Title + "\n" + e.Description
		for _, f := range e.Fields {
			body += "\n\n## " + f.Name + "\n" + f.Value
		}
		if e.Footer != nil {
			body += "\n\n-# " + e.Footer.Text
		}
		parts = append(parts, v2Container(e.Color, body)...)
	}
	g.replyV2(s, i, parts, ephemeral)
}
func v2Container(_ int, text string) []discordgo.MessageComponent {
	return []discordgo.MessageComponent{discordgo.Container{Components: []discordgo.MessageComponent{discordgo.TextDisplay{Content: text}}}}
}
func (g *Gateway) replyV2(s *discordgo.Session, i *discordgo.InteractionCreate, components []discordgo.MessageComponent, ephemeral bool) {
	flags := discordgo.MessageFlagsIsComponentsV2
	if ephemeral {
		flags |= discordgo.MessageFlagsEphemeral
	}
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{Type: discordgo.InteractionResponseChannelMessageWithSource, Data: &discordgo.InteractionResponseData{Components: components, Flags: flags, AllowedMentions: &discordgo.MessageAllowedMentions{Parse: []discordgo.AllowedMentionType{}}}})
}
func (g *Gateway) replyEmbedComponents(s *discordgo.Session, i *discordgo.InteractionCreate, embed *discordgo.MessageEmbed, components []discordgo.MessageComponent, ephemeral bool) {
	flags := discordgo.MessageFlags(0)
	if ephemeral {
		flags = discordgo.MessageFlagsEphemeral
	}
	text := "# " + embed.Title + "\n" + embed.Description
	for _, field := range embed.Fields {
		text += "\n\n## " + field.Name + "\n" + field.Value
	}
	if embed.Footer != nil {
		text += "\n\n-# " + embed.Footer.Text
	}
	all := []discordgo.MessageComponent{discordgo.Container{Components: []discordgo.MessageComponent{discordgo.TextDisplay{Content: text}}}}
	all = append(all, components...)
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{Type: discordgo.InteractionResponseChannelMessageWithSource, Data: &discordgo.InteractionResponseData{Components: all, Flags: flags | discordgo.MessageFlagsIsComponentsV2, AllowedMentions: &discordgo.MessageAllowedMentions{Parse: []discordgo.AllowedMentionType{}}}})
}
func ptrInt(v int) *int { return &v }
func (g *Gateway) memberJoin(s *discordgo.Session, m *discordgo.GuildMemberAdd) {
	if m.User.Bot {
		return
	}
	_ = g.db.EnsureGuild(context.Background(), m.GuildID, g.cfg.DefaultLocale)
	pending, err := g.db.Role(context.Background(), m.GuildID, "pending")
	if err != nil {
		return
	}
	roles, err := s.GuildRoles(m.GuildID)
	if err != nil {
		return
	}
	me, err := s.User("@me")
	if err != nil {
		return
	}
	bot, err := s.GuildMember(m.GuildID, me.ID)
	if err != nil {
		return
	}
	top := 0
	for _, r := range roles {
		for _, have := range bot.Roles {
			if r.ID == have && r.Position > top {
				top = r.Position
			}
		}
	}
	if !permissions.CanAssignRole(&discordgo.Guild{Roles: roles}, pending, top) {
		slog.Warn("pending role cannot be assigned safely", "guild", m.GuildID)
		return
	}
	if err := s.GuildMemberRoleAdd(m.GuildID, m.User.ID, pending); err != nil {
		slog.Warn("pending role assignment failed", "guild", m.GuildID, "error", err)
		return
	}
	_ = g.db.Audit(context.Background(), m.GuildID, "system", "onboarding.pending_role", m.User.ID)
}
