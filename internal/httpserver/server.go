package httpserver

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"github.com/go-chi/chi/v5"
	"github.com/pelagornis/pelagornis-bot/internal/config"
	"github.com/pelagornis/pelagornis-bot/internal/storage/sqlite"
	"io"
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
	b, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
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
	w.WriteHeader(202)
}
