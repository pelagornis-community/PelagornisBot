package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/pelagornis/pelagornis-bot/internal/storage"
	"github.com/pelagornis/pelagornis-bot/internal/storage/sqlite"
)

func TestProjectCategorySettings(t *testing.T) {
	ctx := context.Background()
	db, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "bot.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = storage.Migrate(ctx, db.DB); err != nil {
		t.Fatal(err)
	}
	if err = db.EnsureGuild(ctx, "guild", "ko"); err != nil {
		t.Fatal(err)
	}
	if err = db.SetProjectCategory(ctx, "guild", "1091726089174392833"); err != nil {
		t.Fatal(err)
	}
	got, err := db.ProjectCategory(ctx, "guild")
	if err != nil || got != "1091726089174392833" {
		t.Fatalf("category = %q, %v", got, err)
	}
	if err = db.SetGitHubNotificationChannel(ctx, "guild", "security", "1565011226512203806"); err != nil {
		t.Fatal(err)
	}
	if err = db.SetGitHubNotificationChannel(ctx, "guild", "audit", "1565011279335129248"); err != nil {
		t.Fatal(err)
	}
	for kind, want := range map[string]string{"security": "1565011226512203806", "audit": "1565011279335129248"} {
		got, err = db.GitHubNotificationChannel(ctx, "guild", kind)
		if err != nil || got != want {
			t.Fatalf("%s channel = %q, %v", kind, got, err)
		}
	}
}
