package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

type statuslineCache struct {
	Ad        Ad    `json:"ad"`
	FetchedAt int64 `json:"fetched_at"`
}

func statuslineCachePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".backfill", "statusline-ad.json")
}

func cmdStatusline() {
	io.Copy(io.Discard, os.Stdin)

	cfg := loadConfig()
	if !cfg.Enabled {
		return
	}

	now := time.Now().Unix()
	cache, ok := readStatuslineCache()
	if !ok || now-cache.FetchedAt > 60 {
		if ok {
			elapsed := int(now - cache.FetchedAt)
			if elapsed >= 5 {
				if elapsed > 90 {
					elapsed = 90
				}
				reportImpressionFast(cfg, cache.Ad, "claude-code", elapsed)
			}
		}
		cache = statuslineCache{Ad: fetchAd(cfg, "claude-code"), FetchedAt: now}
		writeStatuslineCache(cache)
	}

	printStatuslineAd(cfg, cache.Ad)
}

func readStatuslineCache() (statuslineCache, bool) {
	var cache statuslineCache
	b, err := os.ReadFile(statuslineCachePath())
	if err != nil {
		return cache, false
	}
	if json.Unmarshal(b, &cache) != nil || cache.Ad.ID == "" || cache.Ad.Text == "" || cache.FetchedAt == 0 {
		return cache, false
	}
	cache.Ad.ID = stripControlChars(cache.Ad.ID)
	cache.Ad.Text = stripControlChars(cache.Ad.Text)
	cache.Ad.URL = stripControlChars(cache.Ad.URL)
	return cache, true
}

func writeStatuslineCache(cache statuslineCache) {
	p := statuslineCachePath()
	os.MkdirAll(filepath.Dir(p), 0o755)
	b, _ := json.Marshal(cache)
	os.WriteFile(p, b, 0o600)
}

func printStatuslineAd(cfg *Config, ad Ad) {
	adID := stripControlChars(ad.ID)
	text := stripControlChars(ad.Text)
	href := fmt.Sprintf("%s/r/%s?d=%s", cfg.APIBase, url.PathEscape(adID), url.QueryEscape(cfg.DeviceID))
	fmt.Printf("\x1b[2mad\x1b[0m \x1b]8;;%s\x1b\\\x1b[33m%s\x1b[0m\x1b]8;;\x1b\\\n", href, text)
}

func reportImpressionFast(cfg *Config, ad Ad, cmd string, seconds int) {
	body, _ := json.Marshal(map[string]any{
		"deviceId": cfg.DeviceID,
		"adId":     ad.ID,
		"cmd":      cmd,
		"seconds":  seconds,
		"kind":     "impression",
	})
	client := &http.Client{Timeout: 800 * time.Millisecond}
	resp, err := client.Post(cfg.APIBase+"/api/events", "application/json", bytes.NewReader(body))
	if err == nil {
		resp.Body.Close()
	}
}
