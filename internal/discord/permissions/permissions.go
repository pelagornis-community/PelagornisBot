package permissions

import "github.com/bwmarrin/discordgo"

func CanManage(i *discordgo.InteractionCreate) bool {
	if i.Member == nil {
		return false
	}
	return i.Member.Permissions&discordgo.PermissionManageGuild != 0 || i.Member.Permissions&discordgo.PermissionAdministrator != 0
}
func CanAssignRole(g *discordgo.Guild, roleID string, botTop int) bool {
	for _, r := range g.Roles {
		if r.ID == roleID {
			return !r.Managed && r.Position < botTop && r.Permissions&(discordgo.PermissionAdministrator|discordgo.PermissionManageGuild) == 0
		}
	}
	return false
}
