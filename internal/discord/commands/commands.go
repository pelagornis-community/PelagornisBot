package commands

import "github.com/bwmarrin/discordgo"

func Definitions() []*discordgo.ApplicationCommand {
	return []*discordgo.ApplicationCommand{
		{Name: "setup", Description: "Configure CommunityOS for this server", DefaultMemberPermissions: ptr(discordgo.PermissionManageGuild)},
		{Name: "help", Description: "Show CommunityOS help"},
		{Name: "form", Description: "Manage forms", DefaultMemberPermissions: ptr(discordgo.PermissionManageGuild), Options: []*discordgo.ApplicationCommandOption{{Type: discordgo.ApplicationCommandOptionString, Name: "action", Description: "create, publish, review, disable, or list", Required: true}, {Type: discordgo.ApplicationCommandOptionString, Name: "id", Description: "Form or submission ID"}, {Type: discordgo.ApplicationCommandOptionString, Name: "decision", Description: "approve or reject"}, {Type: discordgo.ApplicationCommandOptionChannel, Name: "channel", Description: "Panel destination"}}},
		{Name: "panel", Description: "Publish an interactive panel", DefaultMemberPermissions: ptr(discordgo.PermissionManageGuild)},
		{Name: "ticket", Description: "Open or close a support ticket", Options: []*discordgo.ApplicationCommandOption{{Type: discordgo.ApplicationCommandOptionString, Name: "action", Description: "open or close", Required: true}, {Type: discordgo.ApplicationCommandOptionString, Name: "id", Description: "Ticket ID to close"}}},
		{Name: "feeds", Description: "Manage RSS and Atom feeds", DefaultMemberPermissions: ptr(discordgo.PermissionManageGuild), Options: []*discordgo.ApplicationCommandOption{{Type: discordgo.ApplicationCommandOptionString, Name: "action", Description: "add, list, or pause", Required: true}, {Type: discordgo.ApplicationCommandOptionString, Name: "url", Description: "Feed URL"}, {Type: discordgo.ApplicationCommandOptionChannel, Name: "channel", Description: "Destination channel"}, {Type: discordgo.ApplicationCommandOptionString, Name: "id", Description: "Feed source ID"}}},
		{Name: "release", Description: "Manage GitHub release sources", DefaultMemberPermissions: ptr(discordgo.PermissionManageGuild)},
		{Name: "role", Description: "Manage self-service role panels", DefaultMemberPermissions: ptr(discordgo.PermissionManageGuild)},
		{Name: "faq", Description: "Search the guild FAQ", Options: []*discordgo.ApplicationCommandOption{{Type: discordgo.ApplicationCommandOptionString, Name: "query", Description: "Question or keyword", Required: true}}},
		{Name: "stats", Description: "Show guild analytics", DefaultMemberPermissions: ptr(discordgo.PermissionManageGuild)},
		{Name: "workflow", Description: "Manage typed workflows", DefaultMemberPermissions: ptr(discordgo.PermissionManageGuild)},
	}
}
func ptr(v int64) *int64 { return &v }
