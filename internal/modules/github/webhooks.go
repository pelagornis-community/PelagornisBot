package github

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/pelagornis/pelagornis-bot/internal/discord/delivery"
	"github.com/pelagornis/pelagornis-bot/internal/storage/sqlite"
)

// Discord is intentionally narrow so webhook routing can be tested without a
// live Discord connection.
type Discord interface {
	delivery.Sender
	GuildChannelCreateComplex(string, discordgo.GuildChannelCreateData, ...discordgo.RequestOption) (*discordgo.Channel, error)
	GuildChannels(string, ...discordgo.RequestOption) ([]*discordgo.Channel, error)
}

type WebhookService struct {
	Store                *sqlite.Store
	Discord              Discord
	PublicReleaseChannel string
}

type webhookPayload struct {
	Action       string `json:"action"`
	Organization struct {
		Login string `json:"login"`
	} `json:"organization"`
	Repository struct {
		ID       int64  `json:"id"`
		Name     string `json:"name"`
		FullName string `json:"full_name"`
		HTMLURL  string `json:"html_url"`
	} `json:"repository"`
	Sender struct {
		Login string `json:"login"`
	} `json:"sender"`
	Release struct {
		Name       string `json:"name"`
		TagName    string `json:"tag_name"`
		Body       string `json:"body"`
		HTMLURL    string `json:"html_url"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
	} `json:"release"`
	PullRequest struct {
		Title   string `json:"title"`
		HTMLURL string `json:"html_url"`
		Merged  bool   `json:"merged"`
	} `json:"pull_request"`
	Issue struct {
		Title       string    `json:"title"`
		HTMLURL     string    `json:"html_url"`
		PullRequest *struct{} `json:"pull_request"`
	} `json:"issue"`
	Discussion struct {
		Title   string `json:"title"`
		HTMLURL string `json:"html_url"`
	} `json:"discussion"`
	Deployment struct {
		Environment string `json:"environment"`
		Ref         string `json:"ref"`
		SHA         string `json:"sha"`
	} `json:"deployment"`
	DeploymentStatus struct {
		State          string `json:"state"`
		EnvironmentURL string `json:"environment_url"`
		LogURL         string `json:"log_url"`
	} `json:"deployment_status"`
	WorkflowJob struct {
		Name       string `json:"name"`
		HTMLURL    string `json:"html_url"`
		Conclusion string `json:"conclusion"`
	} `json:"workflow_job"`
}

func (s WebhookService) Handle(ctx context.Context, j sqlite.Job) error {
	if j.Kind != "github.webhook" {
		return fmt.Errorf("unsupported job %q", j.Kind)
	}
	var queued struct {
		Event   string `json:"event"`
		Payload string `json:"payload"`
	}
	if err := json.Unmarshal([]byte(j.Payload), &queued); err != nil {
		return err
	}
	var p webhookPayload
	if err := json.Unmarshal([]byte(queued.Payload), &p); err != nil {
		return err
	}
	if p.Repository.FullName == "" {
		return nil
	}
	if queued.Event == "repository" && p.Action == "deleted" {
		audit, _ := s.Store.GitHubNotificationChannel(ctx, j.GuildID, "audit")
		if audit == "" {
			return nil
		}
		_, err := (delivery.Service{Discord: s.Discord}).Send(ctx, audit, "GitHub · ["+p.Repository.FullName+"] Repository deleted\nTriggered by @"+p.Sender.Login+"\n"+p.Repository.HTMLURL)
		return err
	}

	// A repository lifecycle event is the only event allowed to create rooms.
	// Other deliveries lazily provision too, which recovers from a missed create
	// delivery after GitHub retries have expired.
	project, err := s.ensureProject(ctx, j.GuildID, p.Repository)
	if err != nil {
		return err
	}
	main, ci := project.ChannelID, project.CIChannelID
	send := func(channel, title, body, url string) error {
		if channel == "" {
			return nil
		}
		content := "GitHub · [" + p.Repository.FullName + "] " + title + "\n" + strings.TrimSpace(body)
		if url != "" {
			content += "\n" + url
		}
		_, err := (delivery.Service{Discord: s.Discord}).Send(ctx, channel, content)
		return err
	}

	switch queued.Event {
	case "repository":
		if p.Action == "created" || p.Action == "archived" || p.Action == "unarchived" || p.Action == "renamed" || p.Action == "deleted" || p.Action == "transferred" {
			if p.Action == "archived" || p.Action == "unarchived" {
				project.Archived = p.Action == "archived"
				if err := s.Store.SaveGitHubProject(ctx, j.GuildID, project); err != nil {
					return err
				}
			}
			audit, _ := s.Store.GitHubNotificationChannel(ctx, j.GuildID, "audit")
			return send(audit, "Repository "+p.Action, "Triggered by @"+p.Sender.Login, p.Repository.HTMLURL)
		}
	case "release":
		if p.Action != "published" || p.Release.Draft || p.Release.Prerelease {
			return nil
		}
		name := p.Release.Name
		if name == "" {
			name = p.Release.TagName
		}
		content := "🚀 [" + p.Repository.FullName + "] · " + name + "\n" + strings.TrimSpace(p.Release.Body) + "\n" + p.Release.HTMLURL
		imageURL, _ := New().ReleaseImage(ctx, p.Release.HTMLURL)
		if s.PublicReleaseChannel != "" {
			if _, err := (delivery.Service{Discord: s.Discord}).SendWithImage(ctx, s.PublicReleaseChannel, content, imageURL); err != nil {
				return err
			}
		}
		if main != s.PublicReleaseChannel {
			if err := send(main, "Release published · "+name, p.Release.Body, p.Release.HTMLURL); err != nil {
				return err
			}
		}
		_, _ = s.Store.DB.ExecContext(ctx, `update feed_sources set last_checked_at=current_timestamp where guild_id=? and kind='github_release' and repo=?`, j.GuildID, p.Repository.FullName)
	case "deployment_status":
		state := strings.ToLower(p.DeploymentStatus.State)
		if !strings.EqualFold(p.Deployment.Environment, "production") || (state != "success" && state != "failure" && state != "error") {
			return nil
		}
		channel := main
		if state != "success" {
			channel = ci
		}
		return send(channel, "Production deployment · "+state, "`"+p.Deployment.Ref+"`", firstURL(p.DeploymentStatus.EnvironmentURL, p.DeploymentStatus.LogURL))
	case "workflow_job":
		conclusion := strings.ToLower(p.WorkflowJob.Conclusion)
		if p.Action != "completed" || (conclusion != "failure" && conclusion != "cancelled" && conclusion != "timed_out" && conclusion != "action_required") {
			return nil
		}
		return send(ci, "CI "+conclusion, p.WorkflowJob.Name, p.WorkflowJob.HTMLURL)
	case "pull_request":
		if p.Action == "opened" || p.Action == "ready_for_review" || (p.Action == "closed" && p.PullRequest.Merged) {
			return send(main, "Pull request · "+p.Action, p.PullRequest.Title, p.PullRequest.HTMLURL)
		}
	case "issues":
		if p.Issue.PullRequest != nil {
			return nil
		}
		if p.Action == "opened" || p.Action == "closed" || p.Action == "reopened" {
			return send(main, "Issue · "+p.Action, p.Issue.Title, p.Issue.HTMLURL)
		}
	case "discussion":
		if p.Action == "created" || p.Action == "answered" {
			return send(main, "Discussion · "+p.Action, p.Discussion.Title, p.Discussion.HTMLURL)
		}
	case "dependabot_alert", "code_scanning_alert", "secret_scanning_alert":
		security, _ := s.Store.GitHubNotificationChannel(ctx, j.GuildID, "security")
		return send(security, "Security event · "+queued.Event, "Action: "+p.Action, p.Repository.HTMLURL)
	}
	return nil
}

func (s WebhookService) ensureProject(ctx context.Context, guild string, repo struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	HTMLURL  string `json:"html_url"`
}) (sqlite.GitHubProject, error) {
	p, err := s.Store.GitHubProject(ctx, guild, repo.FullName)
	if err == nil {
		return p, nil
	}
	if err != sql.ErrNoRows {
		return p, err
	}
	category, err := s.Store.ProjectCategory(ctx, guild)
	if err != nil {
		return p, fmt.Errorf("project category is not configured")
	}
	staff, err := s.Store.Role(ctx, guild, "staff")
	if err != nil {
		return p, fmt.Errorf("staff role is not configured")
	}
	channels, err := s.Discord.GuildChannels(guild)
	if err != nil {
		return p, err
	}
	base := discordName(repo.Name)
	find := func(name string) string {
		for _, ch := range channels {
			if ch.ParentID == category && ch.Name == name {
				return ch.ID
			}
		}
		return ""
	}
	main, ci := find(base), find(base+"-ci")
	overwrites := []*discordgo.PermissionOverwrite{{ID: guild, Type: discordgo.PermissionOverwriteTypeRole, Deny: discordgo.PermissionViewChannel}, {ID: staff, Type: discordgo.PermissionOverwriteTypeRole, Allow: discordgo.PermissionViewChannel | discordgo.PermissionSendMessages | discordgo.PermissionReadMessageHistory}}
	if main == "" {
		ch, e := s.Discord.GuildChannelCreateComplex(guild, discordgo.GuildChannelCreateData{Name: base, Type: discordgo.ChannelTypeGuildText, ParentID: category, Topic: "GitHub project: " + repo.FullName, PermissionOverwrites: overwrites})
		if e != nil {
			return p, e
		}
		main = ch.ID
	}
	if ci == "" {
		ch, e := s.Discord.GuildChannelCreateComplex(guild, discordgo.GuildChannelCreateData{Name: base + "-ci", Type: discordgo.ChannelTypeGuildText, ParentID: category, Topic: "CI alerts: " + repo.FullName, PermissionOverwrites: overwrites})
		if e != nil {
			return p, e
		}
		ci = ch.ID
	}
	p = sqlite.GitHubProject{Repository: repo.FullName, ChannelID: main, CIChannelID: ci}
	if err := s.Store.SaveGitHubProject(ctx, guild, p); err != nil {
		return p, err
	}
	return p, nil
}

func discordName(value string) string {
	value = strings.ToLower(value)
	value = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return '-'
	}, value)
	value = strings.Trim(value, "-")
	if value == "" {
		return "github-project"
	}
	if len(value) > 90 {
		return value[:90]
	}
	return value
}
func firstURL(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
