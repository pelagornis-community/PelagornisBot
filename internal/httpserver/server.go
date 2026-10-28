package httpserver

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"github.com/pelagornis/pelagornis-bot/internal/config"
	"github.com/pelagornis/pelagornis-bot/internal/storage/sqlite"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

type Server struct {
	s   *http.Server
	cfg config.Config
	db  *sqlite.Store
}

func New(c config.Config, d *sqlite.Store) *Server {
	x := &Server{cfg: c, db: d}
	r := chi.NewRouter()
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200); _, _ = w.Write([]byte("ok\n")) })
	r.Get("/metrics", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("pelagornisbot_up 1\n")) })
	r.Post("/webhooks/github", x.github)
	x.s = &http.Server{Addr: c.HTTPListen, Handler: r, ReadHeaderTimeout: 5e9, MaxHeaderBytes: 8192}
	return x
}
func (x *Server) Run(ctx context.Context) {
	go func() { <-ctx.Done(); _ = x.s.Shutdown(context.Background()) }()
	_ = x.s.ListenAndServe()
}
func (x *Server) Close(c context.Context) error { return x.s.Shutdown(c) }
func (x *Server) github(w http.ResponseWriter, r *http.Request) {
	if !x.cfg.GitHubWebhook {
		http.NotFound(w, r)
		return
	}
	b, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 2<<20))
	if e != nil {
		http.Error(w, "too large", 413)
		return
	}
	sig := strings.TrimPrefix(r.Header.Get("X-Hub-Signature-256"), "sha256=")
	m := hmac.New(sha256.New, []byte(x.cfg.GitHubWebhookSecret))
	_, _ = m.Write(b)
	if sig == "" || !hmac.Equal([]byte(sig), []byte(hex.EncodeToString(m.Sum(nil)))) {
		http.Error(w, "invalid signature", 401)
		return
	}
	event := r.Header.Get("X-GitHub-Event")
	deliveryID := r.Header.Get("X-GitHub-Delivery")
	if event == "" || deliveryID == "" {
		http.Error(w, "missing GitHub delivery headers", http.StatusBadRequest)
		return
	}
	if event == "ping" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var envelope struct {
		Organization struct {
			Login string `json:"login"`
		} `json:"organization"`
		Repository struct {
			Owner struct {
				Login string `json:"login"`
			} `json:"owner"`
		} `json:"repository"`
	}
	if e = json.Unmarshal(b, &envelope); e != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	organization := envelope.Organization.Login
	if organization == "" {
		organization = envelope.Repository.Owner.Login
	}
	if organization == "" {
		http.Error(w, "missing organization", http.StatusUnprocessableEntity)
		return
	}
	guild, e := x.db.GuildForGitHubOrganization(r.Context(), organization)
	if e != nil {
		// Do not expose configured organization names to an arbitrary caller.
		w.WriteHeader(http.StatusAccepted)
		return
	}
	queued, e := x.db.QueueGitHubWebhook(r.Context(), guild, deliveryID, event, b)
	if e != nil {
		slog.Error("queue GitHub webhook", "event", event, "delivery", deliveryID, "error", e)
		http.Error(w, "queue unavailable", http.StatusServiceUnavailable)
		return
	}
	if !queued {
		slog.Info("ignored duplicate GitHub webhook", "event", event, "delivery", deliveryID)
	}
	w.WriteHeader(202)
}
