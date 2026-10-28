package feeds

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/pelagornis/pelagornis-bot/internal/storage"
	"github.com/pelagornis/pelagornis-bot/internal/storage/sqlite"
)

func TestAddReleaseQueuesImmediatePoll(t *testing.T) {
	ctx := context.Background()
	db, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "bot.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = storage.Migrate(ctx, db.DB); err != nil {
		t.Fatal(err)
	}
	if err = db.EnsureGuild(ctx, "1", "ko"); err != nil {
		t.Fatal(err)
	}
	p := Poller{Store: db}
	if _, err = p.AddRelease(ctx, "1", "owner/repository", "2", 900); err != nil {
		t.Fatal(err)
	}
	var jobs int
	if err = db.DB.QueryRowContext(ctx, `select count(*) from jobs where kind='feed.poll' and status='pending'`).Scan(&jobs); err != nil {
		t.Fatal(err)
	}
	if jobs != 1 {
		t.Fatalf("expected immediate poll job, got %d", jobs)
	}
}

func TestAddReleaseRejectsDuplicateActiveRepository(t *testing.T) {
	ctx := context.Background()
	db, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "bot.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = storage.Migrate(ctx, db.DB); err != nil {
		t.Fatal(err)
	}
	if err = db.EnsureGuild(ctx, "1", "ko"); err != nil {
		t.Fatal(err)
	}
	p := Poller{Store: db}
	if _, err = p.AddRelease(ctx, "1", "owner/repository", "2", 900); err != nil {
		t.Fatal(err)
	}
	if _, err = p.AddRelease(ctx, "1", "owner/repository", "2", 900); err == nil {
		t.Fatal("expected duplicate release repository to be rejected")
	}
}

func TestEnqueueAllChecksRecentlyPolledSourcesOnStartup(t *testing.T) {
	ctx := context.Background()
	db, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "bot.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = storage.Migrate(ctx, db.DB); err != nil {
		t.Fatal(err)
	}
	if err = db.EnsureGuild(ctx, "1", "ko"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.DB.ExecContext(ctx, `insert into feed_sources(id,guild_id,kind,repo,channel_id,last_checked_at) values('source','1','github_release','owner/repository','2',current_timestamp)`); err != nil {
		t.Fatal(err)
	}
	if err = (Poller{Store: db}).EnqueueAll(ctx); err != nil {
		t.Fatal(err)
	}
	var jobs int
	if err = db.DB.QueryRowContext(ctx, `select count(*) from jobs where kind='feed.poll' and status='pending'`).Scan(&jobs); err != nil {
		t.Fatal(err)
	}
	if jobs != 1 {
		t.Fatalf("expected one startup catch-up job, got %d", jobs)
	}
}
