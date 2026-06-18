package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"sync"
	"time"
)

type Ad struct {
	ID           string `json:"id"`
	Text         string `json:"text"`
	URL          string `json:"url"`
	SpinnerText  string `json:"spinnerText"`
	EarnedMicros int64  `json:"earnedMicros"`
}

// Shown when the ad server is unreachable. Preview slots — they never earn.
var houseAds = []Ad{
	{ID: "house_uv", Text: "uv · fast Python packages and project installs", URL: "https://docs.astral.sh/uv/", SpinnerText: "uv · Python packages"},
	{ID: "house_ripgrep", Text: "ripgrep · recursive search that respects gitignore", URL: "https://github.com/BurntSushi/ripgrep", SpinnerText: "ripgrep · fast search"},
	{ID: "house_duckdb", Text: "DuckDB · in-process SQL for analytics and parquet", URL: "https://duckdb.org", SpinnerText: "DuckDB · local OLAP"},
}

var registerDeviceOnce sync.Once

// httpClient is shared across fetchAd / fetchAdsConcurrent so TCP connections
// and HTTP keep-alive are reused across the hot per-turn paths. The idle-conn
// pool is widened past the default of 2 so a concurrent batch reuses warm
// connections instead of opening a fresh socket per request.
var httpClient = &http.Client{
	Timeout: 800 * time.Millisecond,
	Transport: &http.Transport{
		MaxIdleConns:        16,
		MaxIdleConnsPerHost: 12,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 400 * time.Millisecond,
	},
}

func fetchAd(cfg *Config, cmd string) Ad {
	u := fmt.Sprintf("%s/api/serve?cmd=%s&d=%s", cfg.APIBase, url.QueryEscape(cmd), cfg.DeviceID)
	if resp, err := httpClient.Get(u); err == nil {
		defer resp.Body.Close()
		var ad Ad
		if json.NewDecoder(resp.Body).Decode(&ad) == nil && ad.ID != "" && ad.Text != "" {
			ad.ID = stripControlChars(ad.ID)
			ad.Text = stripControlChars(ad.Text)
			ad.URL = stripControlChars(ad.URL)
			ad.SpinnerText = stripControlChars(ad.SpinnerText)
			return ad
		}
	}
	return houseAds[rand.Intn(len(houseAds))]
}

// fetchAdsConcurrent fires count fetchAd calls in parallel against the shared
// httpClient and returns the results. Order is not meaningful; callers dedupe.
func fetchAdsConcurrent(cfg *Config, cmd string, count int) []Ad {
	ads := make([]Ad, count)
	var wg sync.WaitGroup
	wg.Add(count)
	for i := 0; i < count; i++ {
		i := i
		go func() {
			defer wg.Done()
			ads[i] = fetchAd(cfg, cmd)
		}()
	}
	wg.Wait()
	return ads
}

// Server-supplied strings end up inside OSC 8 escape sequences; strip C0, DEL,
// and C1 so an ad can never inject its own terminal control codes.
func stripControlChars(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if r < 0x20 || (r >= 0x7f && r <= 0x9f) {
			continue
		}
		out = append(out, r)
	}
	return string(out)
}

func registerDevice(cfg *Config) {
	if cfg.DeviceID == "" || cfg.DeviceSecret == "" {
		return
	}

	sum := sha256.Sum256([]byte(cfg.DeviceSecret))
	body, _ := json.Marshal(map[string]any{
		"deviceId":   cfg.DeviceID,
		"secretHash": hex.EncodeToString(sum[:]),
	})

	resp, err := httpClient.Post(cfg.APIBase+"/api/device/register", "application/json", bytes.NewReader(body))
	if err == nil {
		resp.Body.Close()
	}
}

// reportImpression posts the impression on the shared client (800ms timeout) so
// a slow or dead network can never freeze the shell prompt for seconds after a
// wrapped command has already finished.
func reportImpression(cfg *Config, ad Ad, cmd string, seconds int) {
	registerDeviceOnce.Do(func() {
		registerDevice(cfg)
	})

	body, _ := json.Marshal(map[string]any{
		"deviceId": cfg.DeviceID,
		"adId":     ad.ID,
		"cmd":      cmd,
		"seconds":  seconds,
		"kind":     "impression",
	})
	resp, err := httpClient.Post(cfg.APIBase+"/api/events", "application/json", bytes.NewReader(body))
	if err == nil {
		resp.Body.Close()
	}
}
