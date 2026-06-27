package main

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"time"
)

type config struct {
	addr                  string
	appOrigin             string
	traqBaseURL           string
	oauthClientID         string
	oauthRedirectURL      string
	oauthScope            string
	traqBotAccessToken    string
	syncInterval          time.Duration
	viewerPollInterval    time.Duration
	viewerChannelsPerTick int
}

func loadConfig() config {
	return config{
		addr:                  envString("SERVER_ADDR", ":8080"),
		appOrigin:             envString("APP_ORIGIN", "http://localhost:5173"),
		traqBaseURL:           strings.TrimRight(envString("TRAQ_BASE_URL", "https://q.trap.jp"), "/"),
		oauthClientID:         os.Getenv("TRAQ_CLIENT_ID"),
		oauthRedirectURL:      envString("TRAQ_REDIRECT_URL", "http://localhost:5173/oauth/callback"),
		oauthScope:            envString("OAUTH_SCOPE", "read"),
		traqBotAccessToken:    os.Getenv("TRAQ_BOT_ACCESS_TOKEN"),
		syncInterval:          envDuration("SYNC_INTERVAL", 30*time.Second),
		viewerPollInterval:    envDuration("VIEWER_POLL_INTERVAL", 20*time.Second),
		viewerChannelsPerTick: envInt("VIEWER_POLL_CHANNELS", 40),
	}
}

func envString(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	value, err := time.ParseDuration(raw)
	if err != nil || value <= 0 {
		traqLogWarn("invalid config %s=%q; using %s", key, raw, fallback)
		return fallback
	}
	return value
}

func envInt(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		traqLogWarn("invalid config %s=%q; using %d", key, raw, fallback)
		return fallback
	}
	return value
}

func loadDotEnv(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key != "" {
			_ = os.Setenv(key, value)
		}
	}
}
