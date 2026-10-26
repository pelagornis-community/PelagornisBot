package gateway

import (
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/pelagornis/pelagornis-bot/internal/config"
	"github.com/pelagornis/pelagornis-bot/internal/discord/components"
)

func TestReviewMessageUsesSignedSubmissionScopedButtons(t *testing.T) {
	g := Gateway{cfg: config.Config{SessionSecret: "test secret"}}
	m := g.reviewMessage("guild", "submission", "applicant", map[string]string{"intro": "hello"})
	if m.AllowedMentions == nil || len(m.AllowedMentions.Parse) != 0 {
		t.Fatal("review message must not permit mentions")
	}
	row, ok := m.Components[0].(discordgo.ActionsRow)
	if !ok || len(row.Components) != 2 {
		t.Fatal("expected approve and reject buttons")
	}
	for _, component := range row.Components {
		button := component.(discordgo.Button)
		id, err := components.Parse("test secret", button.CustomID)
		if err != nil || id.Guild != "guild" || id.Resource != "submission" {
			t.Fatalf("button is not bound to its submission: %#v, %v", id, err)
		}
		if id.Action != "form_approve" && id.Action != "form_reject" {
			t.Fatalf("unexpected button action %q", id.Action)
		}
	}
}
