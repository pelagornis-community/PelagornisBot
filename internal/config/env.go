package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	DiscordToken, ApplicationID, DevGuildID, SQLitePath, HTTPListen, LogLevel, DefaultLocale, SessionSecret, EncryptionKey, GitHubWebhookSecret string
	Dashboard, GitHubWebhook                                                                                                                    bool
	FormRetention                                                                                                                               time.Duration
}

func Load() (Config, error) {
	if e := loadDotEnv(".env"); e != nil {
		return Config{}, fmt.Errorf("load .env: %w", e)
	}
	c := Config{DiscordToken: os.Getenv("DISCORD_BOT_TOKEN"), ApplicationID: os.Getenv("DISCORD_APPLICATION_ID"), DevGuildID: os.Getenv("DISCORD_DEV_GUILD_ID"), SQLitePath: value("SQLITE_PATH", "data/pelagornisbot.db"), HTTPListen: value("HTTP_LISTEN", "127.0.0.1:8080"), LogLevel: value("LOG_LEVEL", "info"), DefaultLocale: value("DEFAULT_LOCALE", "ko"), SessionSecret: os.Getenv("SESSION_SECRET"), EncryptionKey: os.Getenv("ENCRYPTION_KEY"), GitHubWebhookSecret: os.Getenv("GITHUB_WEBHOOK_SECRET"), Dashboard: boolv("ENABLE_DASHBOARD"), GitHubWebhook: boolv("ENABLE_GITHUB_WEBHOOK"), FormRetention: 90 * 24 * time.Hour}
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

// loadDotEnv supplies local development defaults without overriding injected
// environment variables, which remain authoritative in containers and CI.
func loadDotEnv(path string) error {
	f, e := os.Open(path)
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, raw, ok := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			return fmt.Errorf("invalid assignment %q", line)
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		value := strings.TrimSpace(raw)
		if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
			value, e = strconv.Unquote(value)
			if e != nil {
				return fmt.Errorf("%s: %w", key, e)
			}
		} else if len(value) >= 2 && value[0] == '\'' && value[len(value)-1] == '\'' {
			value = value[1 : len(value)-1]
		}
		if e := os.Setenv(key, value); e != nil {
			return e
		}
	}
	return scanner.Err()
}
func value(k, d string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return d
}
func boolv(k string) bool { v, _ := strconv.ParseBool(os.Getenv(k)); return v }
