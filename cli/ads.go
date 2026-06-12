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
	ID   string `json:"id"`
	Text string `json:"text"`
	URL  string `json:"url"`
}

// Shown when the ad server is unreachable. Preview slots — they never earn.
var houseAds = []Ad{
	{ID: "house_advertise", Text: "Your ad here — reach data engineers while their pipelines run", URL: "/advertise"},
	{ID: "house_earn", Text: "This footer pays you. Sign in at backfill to start accruing", URL: "/"},
	{ID: "house_duckdb", Text: "House pick: DuckDB — in-process OLAP that eats parquet for breakfast", URL: "/r-ext/duckdb"},
}

var registerDeviceOnce sync.Once

func fetchAd(cfg *Config, cmd string) Ad {
	client := &http.Client{Timeout: 800 * time.Millisecond}
	u := fmt.Sprintf("%s/api/serve?cmd=%s&d=%s", cfg.APIBase, url.QueryEscape(cmd), cfg.DeviceID)
	if resp, err := client.Get(u); err == nil {
		defer resp.Body.Close()
		var ad Ad
		if json.NewDecoder(resp.Body).Decode(&ad) == nil && ad.ID != "" && ad.Text != "" {
			ad.ID = stripControlChars(ad.ID)
			ad.Text = stripControlChars(ad.Text)
			ad.URL = stripControlChars(ad.URL)
			return ad
		}
	}
	return houseAds[rand.Intn(len(houseAds))]
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

	client := &http.Client{Timeout: 800 * time.Millisecond}
	resp, err := client.Post(cfg.APIBase+"/api/device/register", "application/json", bytes.NewReader(body))
	if err == nil {
		resp.Body.Close()
	}
}

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
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Post(cfg.APIBase+"/api/events", "application/json", bytes.NewReader(body))
	if err == nil {
		resp.Body.Close()
	}
}
