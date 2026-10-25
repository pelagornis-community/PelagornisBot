package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	DiscordToken, ApplicationID, DevGuildID, DatabaseURL, HTTPListen, LogLevel, DefaultLocale, SessionSecret, EncryptionKey, GitHubWebhookSecret string
	Dashboard, GitHubWebhook                                                                                                                     bool
	FormRetention                                                                                                                                time.Duration
	Workers                                                                                                                                      int
}

func Load() (Config, error) {
	c := Config{DiscordToken: os.Getenv("DISCORD_BOT_TOKEN"), ApplicationID: os.Getenv("DISCORD_APPLICATION_ID"), DevGuildID: os.Getenv("DISCORD_DEV_GUILD_ID"), DatabaseURL: os.Getenv("DATABASE_URL"), HTTPListen: value("HTTP_LISTEN", "127.0.0.1:8080"), LogLevel: value("LOG_LEVEL", "info"), DefaultLocale: value("DEFAULT_LOCALE", "ko"), SessionSecret: os.Getenv("SESSION_SECRET"), EncryptionKey: os.Getenv("ENCRYPTION_KEY"), GitHubWebhookSecret: os.Getenv("GITHUB_WEBHOOK_SECRET"), Dashboard: boolv("ENABLE_DASHBOARD"), GitHubWebhook: boolv("ENABLE_GITHUB_WEBHOOK"), FormRetention: 90 * 24 * time.Hour, Workers: intv("WORKER_CONCURRENCY", 3)}
	if c.DatabaseURL == "" {
		return c, fmt.Errorf("DATABASE_URL is required")
	}
	if c.DiscordToken == "" {
		return c, fmt.Errorf("DISCORD_BOT_TOKEN is required")
	}
	if c.ApplicationID == "" {
		return c, fmt.Errorf("DISCORD_APPLICATION_ID is required")
	}
	if c.Dashboard && len(c.SessionSecret) < 32 {
		return c, fmt.Errorf("SESSION_SECRET must be at least 32 bytes when dashboard is enabled")
	}
	if c.GitHubWebhook && c.GitHubWebhookSecret == "" {
		return c, fmt.Errorf("GITHUB_WEBHOOK_SECRET is required when webhook is enabled")
	}
	return c, nil
}
func value(k, d string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return d
}
func boolv(k string) bool { v, _ := strconv.ParseBool(os.Getenv(k)); return v }
func intv(k string, d int) int {
	v, e := strconv.Atoi(os.Getenv(k))
	if e != nil || v < 1 {
		return d
	}
	return v
}
