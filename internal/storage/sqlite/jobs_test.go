package sqlite_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/pelagornis/pelagornis-bot/internal/storage"
	"github.com/pelagornis/pelagornis-bot/internal/storage/sqlite"
)

func newJobStore(t *testing.T) (*sqlite.Store, context.Context) {
	t.Helper()
	ctx := context.Background()
	db, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "bot.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	if err = storage.Migrate(ctx, db.DB); err != nil {
		t.Fatal(err)
	}
	if err = db.EnsureGuild(ctx, "guild", "ko"); err != nil {
		t.Fatal(err)
	}
	return db, ctx
}

func claimOne(t *testing.T, db *sqlite.Store, ctx context.Context) sqlite.Job {
	t.Helper()
	jobs, err := db.ClaimJobs(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 {
		t.Fatalf("claimed %d jobs, want 1", len(jobs))
	}
	return jobs[0]
}

func TestFinishJobSchedulesRetryWithNonNullAvailability(t *testing.T) {
	db, ctx := newJobStore(t)
	if err := db.InsertJob(ctx, "guild", "github.webhook", `{}`, "retry-job"); err != nil {
		t.Fatal(err)
	}
	j := claimOne(t, db, ctx)
	if err := db.FinishJob(ctx, j.ID, errors.New("missing configuration")); err != nil {
		t.Fatal(err)
	}
	var status, available string
	var lease any
	if err := db.DB.QueryRowContext(ctx, `select status,available_at,lease_until from jobs where id=?`, j.ID).Scan(&status, &available, &lease); err != nil {
		t.Fatal(err)
	}
	if status != "pending" || available == "" || lease != nil {
		t.Fatalf("status=%q available=%q lease=%v", status, available, lease)
	}
}

func TestFinishJobMarksSuccessDoneAndRetryLimitDead(t *testing.T) {
	db, ctx := newJobStore(t)
	if err := db.InsertJob(ctx, "guild", "github.webhook", `{}`, "success-job"); err != nil {
		t.Fatal(err)
	}
	j := claimOne(t, db, ctx)
	if err := db.FinishJob(ctx, j.ID, nil); err != nil {
		t.Fatal(err)
	}
	var status string
	if err := db.DB.QueryRowContext(ctx, `select status from jobs where id=?`, j.ID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "done" {
		t.Fatalf("success status=%q", status)
	}

	if err := db.InsertJob(ctx, "guild", "github.webhook", `{}`, "dead-job"); err != nil {
		t.Fatal(err)
	}
	dead := claimOne(t, db, ctx)
	// attempts is incremented by ClaimJobs; set the final attempt explicitly so
	// this test verifies the retry ceiling without waiting for backoff windows.
	if _, err := db.DB.ExecContext(ctx, `update jobs set attempts=5 where id=?`, dead.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.FinishJob(ctx, dead.ID, errors.New("permanent failure")); err != nil {
		t.Fatal(err)
	}
	var available string
	if err := db.DB.QueryRowContext(ctx, `select status,available_at from jobs where id=?`, dead.ID).Scan(&status, &available); err != nil {
		t.Fatal(err)
	}
	if status != "dead" || available == "" {
		t.Fatalf("dead status=%q available=%q", status, available)
	}
}

func TestClaimJobsRecoversExpiredRunningLease(t *testing.T) {
	db, ctx := newJobStore(t)
	if err := db.InsertJob(ctx, "guild", "github.webhook", `{}`, "lease-job"); err != nil {
		t.Fatal(err)
	}
	j := claimOne(t, db, ctx)
	if _, err := db.DB.ExecContext(ctx, `update jobs set lease_until=datetime('now','-1 second') where id=?`, j.ID); err != nil {
		t.Fatal(err)
	}
	recovered := claimOne(t, db, ctx)
	if recovered.ID != j.ID || recovered.Attempts != 2 {
		t.Fatalf("recovered %#v, first %#v", recovered, j)
	}
}
