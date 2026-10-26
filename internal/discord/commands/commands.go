package commands

import "github.com/bwmarrin/discordgo"

func Definitions() []*discordgo.ApplicationCommand {
	return []*discordgo.ApplicationCommand{
		{Name: "setup", Description: "Open the server setup dashboard", DefaultMemberPermissions: ptr(discordgo.PermissionManageGuild), Options: []*discordgo.ApplicationCommandOption{{Type: discordgo.ApplicationCommandOptionString, Name: "action", Description: "Optional legacy configuration action", Choices: choices("member-role", "pending-role", "review-channel")}, {Type: discordgo.ApplicationCommandOptionRole, Name: "role", Description: "Role for member or pending access"}, {Type: discordgo.ApplicationCommandOptionChannel, Name: "channel", Description: "Private staff review channel"}}},
		{Name: "help", Description: "Show PelagornisBot help"},
		{Name: "rule", Description: "Read the server rules"},
		{Name: "welcome", Description: "Publish the welcome panel", DefaultMemberPermissions: ptr(discordgo.PermissionManageGuild), Options: []*discordgo.ApplicationCommandOption{{Type: discordgo.ApplicationCommandOptionChannel, Name: "channel", Description: "Optional destination; defaults to this channel"}}},
		{Name: "form", Description: "Open the form management dashboard", DefaultMemberPermissions: ptr(discordgo.PermissionManageGuild), Options: []*discordgo.ApplicationCommandOption{{Type: discordgo.ApplicationCommandOptionString, Name: "action", Description: "Optional legacy form action", Choices: choices("create", "publish", "review-channel")}, {Type: discordgo.ApplicationCommandOptionString, Name: "id", Description: "Form ID"}, {Type: discordgo.ApplicationCommandOptionChannel, Name: "channel", Description: "Panel or staff-review channel"}}},
		{Name: "panel", Description: "Open the interactive panel dashboard", DefaultMemberPermissions: ptr(discordgo.PermissionManageGuild), Options: []*discordgo.ApplicationCommandOption{{Type: discordgo.ApplicationCommandOptionString, Name: "type", Description: "Optional legacy panel type", Choices: choices("ticket", "role")}, {Type: discordgo.ApplicationCommandOptionChannel, Name: "channel", Description: "Channel to publish into"}, {Type: discordgo.ApplicationCommandOptionRole, Name: "role", Description: "Role for a role panel"}}},
		{Name: "ticket", Description: "Open the support ticket dashboard", Options: []*discordgo.ApplicationCommandOption{{Type: discordgo.ApplicationCommandOptionString, Name: "action", Description: "Optional staff ticket action", Choices: choices("open", "close", "reopen", "claim", "assign")}, {Type: discordgo.ApplicationCommandOptionString, Name: "id", Description: "Ticket ID"}, {Type: discordgo.ApplicationCommandOptionUser, Name: "user", Description: "Staff member to assign"}}},
		{Name: "feeds", Description: "Open the feed management dashboard", DefaultMemberPermissions: ptr(discordgo.PermissionManageGuild), Options: []*discordgo.ApplicationCommandOption{{Type: discordgo.ApplicationCommandOptionString, Name: "action", Description: "Optional legacy feed action", Choices: choices("add", "list", "pause")}, {Type: discordgo.ApplicationCommandOptionString, Name: "url", Description: "Feed URL"}, {Type: discordgo.ApplicationCommandOptionChannel, Name: "channel", Description: "Destination channel"}, {Type: discordgo.ApplicationCommandOptionString, Name: "id", Description: "Feed source ID"}}},
		{Name: "release", Description: "Open the GitHub release dashboard", DefaultMemberPermissions: ptr(discordgo.PermissionManageGuild), Options: []*discordgo.ApplicationCommandOption{{Type: discordgo.ApplicationCommandOptionString, Name: "action", Description: "Optional legacy release action", Choices: choices("add", "list", "pause")}, {Type: discordgo.ApplicationCommandOptionString, Name: "repo", Description: "GitHub repository owner/name"}, {Type: discordgo.ApplicationCommandOptionChannel, Name: "channel", Description: "Destination channel"}, {Type: discordgo.ApplicationCommandOptionString, Name: "id", Description: "Release source ID"}}},
		{Name: "role", Description: "Open the self-service role dashboard", DefaultMemberPermissions: ptr(discordgo.PermissionManageGuild), Options: []*discordgo.ApplicationCommandOption{{Type: discordgo.ApplicationCommandOptionChannel, Name: "channel", Description: "Optional destination; defaults to this channel"}, {Type: discordgo.ApplicationCommandOptionRole, Name: "role", Description: "Optional role to publish"}}},
		{Name: "faq", Description: "Search the guild FAQ", Options: []*discordgo.ApplicationCommandOption{{Type: discordgo.ApplicationCommandOptionString, Name: "query", Description: "Question or keyword", Required: true}}},
		{Name: "stats", Description: "Show guild analytics", DefaultMemberPermissions: ptr(discordgo.PermissionManageGuild)},
		{Name: "workflow", Description: "Open the workflow management dashboard", DefaultMemberPermissions: ptr(discordgo.PermissionManageGuild), Options: []*discordgo.ApplicationCommandOption{{Type: discordgo.ApplicationCommandOptionString, Name: "action", Description: "Optional legacy workflow action", Choices: choices("add", "list", "pause")}, {Type: discordgo.ApplicationCommandOptionString, Name: "name", Description: "Workflow name"}, {Type: discordgo.ApplicationCommandOptionString, Name: "definition", Description: "Workflow JSON definition"}, {Type: discordgo.ApplicationCommandOptionString, Name: "id", Description: "Workflow ID"}}},
	}
}
func ptr(v int64) *int64 { return &v }
func choices(values ...string) []*discordgo.ApplicationCommandOptionChoice {
	items := make([]*discordgo.ApplicationCommandOptionChoice, 0, len(values))
	for _, value := range values {
		items = append(items, &discordgo.ApplicationCommandOptionChoice{Name: value, Value: value})
	}
	return items
}
