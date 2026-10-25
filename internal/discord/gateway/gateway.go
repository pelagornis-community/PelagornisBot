package gateway

import (
	"context"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/pelagornis/communityos/internal/config"
	"github.com/pelagornis/communityos/internal/discord/commands"
	"github.com/pelagornis/communityos/internal/discord/components"
	"github.com/pelagornis/communityos/internal/discord/permissions"
	"github.com/pelagornis/communityos/internal/modules/faq"
	"github.com/pelagornis/communityos/internal/modules/feeds"
	"github.com/pelagornis/communityos/internal/modules/forms"
	"github.com/pelagornis/communityos/internal/modules/tickets"
	"github.com/pelagornis/communityos/internal/storage/postgres"
	"log/slog"
	"sync"
)

type Gateway struct {
	session *discordgo.Session
	cfg     config.Config
	db      *postgres.Store
	forms   *forms.Service
	feeds   feeds.Poller
	faq     *faq.Service
	tickets *tickets.Service
	once    sync.Once
}

func New(c config.Config, d *postgres.Store) (*Gateway, error) {
	s, e := discordgo.New("Bot " + c.DiscordToken)
	if e != nil {
		return nil, e
	}
	s.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMembers
	s.AddHandler(func(_ *discordgo.Session, r *discordgo.Ready) { slog.Info("discord ready", "user", r.User.String()) })
	g := &Gateway{session: s, cfg: c, db: d, forms: forms.New(d.Pool), feeds: feeds.Poller{Store: d, Client: feeds.New()}, faq: faq.New(d.Pool), tickets: tickets.New(d.Pool)}
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
	for _, c := range commands.Definitions() {
		if _, e := g.session.ApplicationCommandCreate(g.cfg.ApplicationID, guild, c); e != nil {
			return fmt.Errorf("register %s: %w", c.Name, e)
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
	if name != "help" && name != "ticket" && name != "faq" && !permissions.CanManage(i) {
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
					text += "`" + f.ID + "` → " + f.ChannelID + "\n"
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
		g.reply(s, i, "CommunityOS setup has started. Configure roles and channels in the dashboard or /panel.", true)
	case "help":
		g.reply(s, i, "Commands: /setup, /form, /panel, /ticket, /feeds, /release, /role, /faq, /stats, /workflow", true)
	case "ticket":
		g.ticketCommand(s, i)
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
		g.reply(s, i, "Command accepted. This guild-scoped operation is recorded for processing.", true)
	}
}
func (g *Gateway) ticketCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	action, id := "", ""
	for _, o := range i.ApplicationCommandData().Options {
		if o.Name == "action" {
			action = o.StringValue()
		}
		if o.Name == "id" {
			id = o.StringValue()
		}
	}
	if action == "close" {
		if !permissions.CanManage(i) {
			g.reply(s, i, "Manage Server permission is required.", true)
			return
		}
		if e := g.tickets.Close(context.Background(), i.GuildID, id, i.Member.User.ID); e != nil {
			g.reply(s, i, "Ticket close failed: "+e.Error(), true)
			return
		}
		g.reply(s, i, "Ticket closed.", true)
		return
	}
	if action != "open" {
		g.reply(s, i, "Use open or close.", true)
		return
	}
	ch, e := s.GuildChannelCreateComplex(i.GuildID, discordgo.GuildChannelCreateData{Name: "ticket-" + i.Member.User.ID, Type: discordgo.ChannelTypeGuildText, PermissionOverwrites: []*discordgo.PermissionOverwrite{{ID: i.GuildID, Type: discordgo.PermissionOverwriteTypeRole, Deny: discordgo.PermissionViewChannel}, {ID: i.Member.User.ID, Type: discordgo.PermissionOverwriteTypeMember, Allow: discordgo.PermissionViewChannel | discordgo.PermissionSendMessages}}})
	if e != nil {
		g.reply(s, i, "Ticket channel could not be created.", true)
		return
	}
	ticket, e := g.tickets.Create(context.Background(), i.GuildID, i.Member.User.ID, ch.ID)
	if e != nil {
		_, _ = s.ChannelDelete(ch.ID)
		g.reply(s, i, "Ticket storage failed; channel removed.", true)
		return
	}
	_ = g.db.IncrementMetric(context.Background(), i.GuildID, "ticket.opened", "")
	g.reply(s, i, "Ticket opened: <#"+ch.ID+"> ("+ticket+")", true)
}
func (g *Gateway) formCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	action, id, decision, channel := "", "", "", ""
	for _, o := range i.ApplicationCommandData().Options {
		switch o.Name {
		case "action":
			action = o.StringValue()
		case "id":
			id = o.StringValue()
		case "decision":
			decision = o.StringValue()
		case "channel":
			if c := o.ChannelValue(s); c != nil {
				channel = c.ID
			}
		}
	}
	switch action {
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
			_, e := s.ChannelMessageSendComplex(channel, &discordgo.MessageSend{Content: "Application form", Components: []discordgo.MessageComponent{discordgo.ActionsRow{Components: []discordgo.MessageComponent{discordgo.Button{CustomID: custom, Label: "Start", Style: discordgo.PrimaryButton}}}}, AllowedMentions: &discordgo.MessageAllowedMentions{Parse: []discordgo.AllowedMentionType{}}})
			if e != nil {
				g.reply(s, i, "Form published but panel failed: "+e.Error(), true)
				return
			}
		}
		g.reply(s, i, "Form published.", true)
	case "review":
		if decision != "approve" && decision != "reject" {
			g.reply(s, i, "Decision must be approve or reject.", true)
			return
		}
		sub, e := g.forms.Pending(context.Background(), i.GuildID, id)
		if e != nil {
			g.reply(s, i, "Submission unavailable: "+e.Error(), true)
			return
		}
		if decision == "approve" {
			if e = g.approveRoles(s, i.GuildID, sub.UserID); e != nil {
				g.reply(s, i, "Roles were not changed: "+e.Error(), true)
				return
			}
		}
		status := "approved"
		if decision == "reject" {
			status = "rejected"
		}
		if e = g.forms.Review(context.Background(), i.GuildID, id, i.Member.User.ID, status); e != nil {
			g.reply(s, i, "Review failed: "+e.Error(), true)
			return
		}
		_ = g.db.Audit(context.Background(), i.GuildID, i.Member.User.ID, "form."+status, id)
		g.reply(s, i, "Submission "+status+".", true)
	default:
		g.reply(s, i, "Use create, publish, or review.", true)
	}
}
func (g *Gateway) approveRoles(s *discordgo.Session, guild, user string) error {
	member, e := g.db.Role(context.Background(), guild, "member")
	if e != nil {
		return fmt.Errorf("member role is not configured")
	}
	pending, _ := g.db.Role(context.Background(), guild, "pending")
	roles, e := s.GuildRoles(guild)
	if e != nil {
		return e
	}
	me, e := s.User("@me")
	if e != nil {
		return e
	}
	bot, e := s.GuildMember(guild, me.ID)
	if e != nil {
		return e
	}
	top := 0
	for _, r := range roles {
		for _, id := range bot.Roles {
			if r.ID == id && r.Position > top {
				top = r.Position
			}
		}
	}
	safe := func(role string) bool { return permissions.CanAssignRole(&discordgo.Guild{Roles: roles}, role, top) }
	if !safe(member) {
		return fmt.Errorf("member role is unsafe or above bot")
	}
	if e = s.GuildMemberRoleAdd(guild, user, member); e != nil {
		return e
	}
	if pending != "" && safe(pending) {
		if e = s.GuildMemberRoleRemove(guild, user, pending); e != nil {
			return e
		}
	}
	return nil
}
func (g *Gateway) component(s *discordgo.Session, i *discordgo.InteractionCreate) {
	id, e := components.Parse(g.cfg.SessionSecret, i.MessageComponentData().CustomID)
	if e != nil || id.Guild != i.GuildID || (id.User != "" && id.User != i.Member.User.ID) || id.Action != "form_start" {
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
func (g *Gateway) modal(s *discordgo.Session, i *discordgo.InteractionCreate) {
	id, e := components.Parse(g.cfg.SessionSecret, i.ModalSubmitData().CustomID)
	if e != nil || id.Action != "form_submit" || id.Guild != i.GuildID || id.User != i.Member.User.ID {
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
	submission, e := g.forms.Submit(context.Background(), f, i.Member.User.ID, answers)
	if e != nil {
		g.reply(s, i, "Please correct the form: "+e.Error(), true)
		return
	}
	g.reply(s, i, "Submitted for private staff review. Reference: "+submission, true)
}
func (g *Gateway) reply(s *discordgo.Session, i *discordgo.InteractionCreate, text string, ephemeral bool) {
	flags := discordgo.MessageFlags(0)
	if ephemeral {
		flags = discordgo.MessageFlagsEphemeral
	}
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{Type: discordgo.InteractionResponseChannelMessageWithSource, Data: &discordgo.InteractionResponseData{Content: text, Flags: flags, AllowedMentions: &discordgo.MessageAllowedMentions{Parse: []discordgo.AllowedMentionType{}}}})
}
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
