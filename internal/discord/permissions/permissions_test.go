package permissions

import (
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestCanManageRequiresGuildManagementPermission(t *testing.T) {
	if CanManage(&discordgo.InteractionCreate{Interaction: &discordgo.Interaction{Member: &discordgo.Member{Permissions: discordgo.PermissionViewChannel}}}) {
		t.Fatal("ordinary member must not review applications")
	}
	if !CanManage(&discordgo.InteractionCreate{Interaction: &discordgo.Interaction{Member: &discordgo.Member{Permissions: discordgo.PermissionManageGuild}}}) {
		t.Fatal("Manage Server must authorize reviews")
	}
}
