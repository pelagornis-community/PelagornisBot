package commands

import "github.com/bwmarrin/discordgo"

// Every management command opens an in-Discord dashboard. Inputs are collected
// through signed buttons, selectors, and modals rather than slash arguments.
func Definitions() []*discordgo.ApplicationCommand {
	staff := ptr(discordgo.PermissionManageGuild)
	return []*discordgo.ApplicationCommand{
		{Name: "setup", Description: "Open the server setup dashboard", DefaultMemberPermissions: staff},
		{Name: "help", Description: "Show PelagornisBot help"},
		{Name: "rule", Description: "Read the server rules"},
		{Name: "welcome", Description: "Publish the welcome panel", DefaultMemberPermissions: staff},
		{Name: "form", Description: "Open the form management dashboard", DefaultMemberPermissions: staff},
		{Name: "panel", Description: "Open the interactive panel dashboard", DefaultMemberPermissions: staff},
		{Name: "ticket", Description: "Open the support ticket dashboard"},
		{Name: "feeds", Description: "Open the feed management dashboard", DefaultMemberPermissions: staff},
		{Name: "release", Description: "Open the GitHub release dashboard", DefaultMemberPermissions: staff},
		{Name: "role", Description: "Open the self-service role dashboard", DefaultMemberPermissions: staff},
		{Name: "faq", Description: "Open the FAQ dashboard"},
		{Name: "stats", Description: "Show guild analytics", DefaultMemberPermissions: staff},
		{Name: "workflow", Description: "Open the workflow management dashboard", DefaultMemberPermissions: staff},
	}
}
func ptr(v int64) *int64 { return &v }
