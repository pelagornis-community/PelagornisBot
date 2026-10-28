package httpserver

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/pelagornis/pelagornis-bot/internal/config"
	"github.com/pelagornis/pelagornis-bot/internal/storage"
	"github.com/pelagornis/pelagornis-bot/internal/storage/sqlite"
)

func TestGitHubWebhookQueuesSignedDeliveryOnce(t *testing.T) {
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
	if err = db.SetGitHubOrganization(ctx, "guild", "pelagornis"); err != nil {
		t.Fatal(err)
	}
	x := New(config.Config{GitHubWebhook: true, GitHubWebhookSecret: "webhook-secret"}, db)
	body := []byte(`{"organization":{"login":"pelagornis"},"repository":{"full_name":"pelagornis/test-release"}}`)
	request := func() *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/webhooks/github", bytes.NewReader(body))
		r.Header.Set("X-GitHub-Event", "repository")
		r.Header.Set("X-GitHub-Delivery", "delivery-1")
		m := hmac.New(sha256.New, []byte("webhook-secret"))
		_, _ = m.Write(body)
		r.Header.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(m.Sum(nil)))
		w := httptest.NewRecorder()
		x.github(w, r)
		return w
	}
	if got := request().Code; got != http.StatusAccepted {
		t.Fatalf("first delivery status = %d", got)
	}
	if got := request().Code; got != http.StatusAccepted {
		t.Fatalf("duplicate delivery status = %d", got)
	}
	var jobs, deliveries int
	if err = db.DB.QueryRow(`select count(*) from jobs where kind='github.webhook'`).Scan(&jobs); err != nil {
		t.Fatal(err)
	}
	if err = db.DB.QueryRow(`select count(*) from webhook_deliveries where provider='github'`).Scan(&deliveries); err != nil {
		t.Fatal(err)
	}
	if jobs != 1 || deliveries != 1 {
		t.Fatalf("jobs=%d deliveries=%d, want one each", jobs, deliveries)
	}
}

func TestGitHubWebhookRejectsInvalidSignature(t *testing.T) {
	x := New(config.Config{GitHubWebhook: true, GitHubWebhookSecret: "webhook-secret"}, &sqlite.Store{})
	r := httptest.NewRequest(http.MethodPost, "/webhooks/github", bytes.NewBufferString(`{}`))
	r.Header.Set("X-Hub-Signature-256", "sha256=wrong")
	w := httptest.NewRecorder()
	x.github(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", w.Code)
	}
}
