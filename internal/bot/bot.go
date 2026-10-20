package bot

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/pelagornis/pelagornis-bot/internal/config"
	"github.com/pelagornis/pelagornis-bot/internal/promotion"
	"github.com/pelagornis/pelagornis-bot/internal/store"
	"github.com/robfig/cron/v3"
)

type Bot struct {
	session *discordgo.Session
	cfg     *config.Config
	store   *store.Store
	promo   *promotion.Service
	cron    *cron.Cron
}

func New(cfg *config.Config, st *store.Store) (*Bot, error) {
	token := os.Getenv("DISCORD_BOT_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("DISCORD_BOT_TOKEN is required")
	}

	intents := discordgo.IntentsGuilds |
		discordgo.IntentsGuildMembers |
		discordgo.IntentsGuildMessages |
		discordgo.IntentsMessageContent

	session, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, err
	}
	session.Identify.Intents = intents

	b := &Bot{
		session: session,
		cfg:     cfg,
		store:   st,
		promo:   promotion.New(session, cfg, st),
		cron:    cron.New(),
	}

	session.AddHandler(b.onReady)
	session.AddHandler(b.onGuildMemberAdd)
	session.AddHandler(b.onMessageCreate)
	session.AddHandler(b.onInteractionCreate)

	return b, nil
}

func (b *Bot) Start() error {
	if err := b.session.Open(); err != nil {
		if strings.Contains(err.Error(), "4014") {
			return fmt.Errorf(
				"Discord 4014 Disallowed intent(s): Developer Portal → 해당 앱 → Bot → "+
					"Privileged Gateway Intents에서 아래를 켠 뒤 저장하세요:\n"+
					"  • SERVER MEMBERS INTENT (환영 메시지)\n"+
					"  • MESSAGE CONTENT INTENT (첫 메시지·모더레이션·FAQ)\n"+
					"원본: %w", err,
			)
		}
		return err
	}

	if b.cfg.Promotion.Enabled {
		spec := promotion.CronSpec(b.cfg.Promotion.PollMinutes)
		_, err := b.cron.AddFunc(spec, b.promo.RunOnce)
		if err != nil {
			return fmt.Errorf("promotion cron: %w", err)
		}
		b.cron.Start()
		go b.promo.RunOnce()
	}

	log.Println("bot: connected")
	return nil
}

func (b *Bot) Stop() {
	if b.cron != nil {
		b.cron.Stop()
	}
	_ = b.session.Close()
}

func (b *Bot) onReady(s *discordgo.Session, r *discordgo.Ready) {
	log.Printf("bot: logged in as %s#%s", r.User.Username, r.User.Discriminator)
	if err := b.registerCommands(); err != nil {
		log.Printf("bot: register commands: %v", err)
	}
}

func (b *Bot) onGuildMemberAdd(s *discordgo.Session, m *discordgo.GuildMemberAdd) {
	if m.GuildID != b.cfg.GuildID || m.User == nil || m.User.Bot {
		return
	}
	chID := b.cfg.Onboarding.WelcomeChannelID
	if chID == "" {
		return
	}
	msg := strings.ReplaceAll(b.cfg.Onboarding.WelcomeTemplate, "{user}", m.User.Mention())
	if msg == "" {
		msg = fmt.Sprintf("환영합니다, %s! 규칙: /rules", m.User.Mention())
	}
	if b.cfg.Onboarding.WelcomeDM {
		ch, err := s.UserChannelCreate(m.User.ID)
		if err == nil {
			_, _ = s.ChannelMessageSend(ch.ID, msg)
		}
		return
	}
	_, _ = s.ChannelMessageSend(chID, msg)
}

func (b *Bot) onMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author == nil || m.Author.Bot || m.GuildID != b.cfg.GuildID {
		return
	}
	// TODO: security → moderation → faq (Phase 2)
	if b.cfg.Onboarding.FirstMessageReminder {
		seen, err := b.store.HasSeenFirstMessage(m.Author.ID, m.GuildID)
		if err == nil && !seen {
			_ = b.store.MarkFirstMessage(m.Author.ID, m.GuildID)
			tpl := b.cfg.Onboarding.FirstMessageTemplate
			if tpl != "" {
				_, _ = s.ChannelMessageSend(m.ChannelID, tpl)
			}
		}
	}
}

func (b *Bot) onInteractionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}
	switch i.ApplicationCommandData().Name {
	case "rules":
		b.handleRules(s, i)
	default:
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Unknown command",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
	}
}

func (b *Bot) handleRules(s *discordgo.Session, i *discordgo.InteractionCreate) {
	embeds := buildRulesEmbeds(b.cfg)
	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: embeds,
		},
	}); err != nil {
		log.Printf("rules: respond failed: %v", err)
	}
}

func (b *Bot) registerCommands() error {
	cmds := []*discordgo.ApplicationCommand{
		{Name: "rules", Description: "View server rules, enforcement & FAQ"},
	}
	appID := b.session.State.User.ID
	if appID == "" {
		return fmt.Errorf("bot user id not ready")
	}

	guildID := os.Getenv("DISCORD_GUILD_ID")
	if guildID != "" {
		_, err := b.session.ApplicationCommandBulkOverwrite(appID, guildID, cmds)
		return err
	}
	_, err := b.session.ApplicationCommandBulkOverwrite(appID, "", cmds)
	return err
}
