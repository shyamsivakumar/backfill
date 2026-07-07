package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"os"
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

type deviceRegistrationPayload struct {
	DeviceID   string `json:"deviceId"`
	SecretHash string `json:"secretHash"`
}

type impressionPayload struct {
	DeviceID string `json:"deviceId"`
	AdID     string `json:"adId"`
	Cmd      string `json:"cmd"`
	Seconds  int    `json:"seconds"`
	Kind     string `json:"kind"`
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

func debugLogf(format string, args ...any) {
	if os.Getenv("BACKFILL_DEBUG") == "" {
		return
	}
	fmt.Fprintf(os.Stderr, "bf debug: "+format+"\n", args...)
}

func safeHTTPError(err error) string {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		if urlErr.Err != nil {
			return fmt.Sprintf("%s: %T", urlErr.Op, urlErr.Err)
		}
		return urlErr.Op
	}
	return fmt.Sprintf("%T", err)
}

func fallbackAd() Ad {
	return houseAds[rand.Intn(len(houseAds))]
}

func fetchAd(cfg *Config, cmd string) Ad {
	u := fmt.Sprintf("%s/api/serve?cmd=%s&d=%s", cfg.APIBase, url.QueryEscape(cmd), cfg.DeviceID)
	resp, err := httpClient.Get(u)
	if err != nil {
		debugLogf("ad serve failed for cmd %q: %s", cmd, safeHTTPError(err))
		return fallbackAd()
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		debugLogf("ad serve returned HTTP %d for cmd %q", resp.StatusCode, cmd)
		return fallbackAd()
	}

	var ad Ad
	if err := json.NewDecoder(resp.Body).Decode(&ad); err != nil {
		debugLogf("ad serve decode failed for cmd %q: %v", cmd, err)
		return fallbackAd()
	}
	if ad.ID == "" || ad.Text == "" {
		debugLogf("ad serve returned incomplete ad for cmd %q (id=%t text=%t)", cmd, ad.ID != "", ad.Text != "")
		return fallbackAd()
	}

	ad.ID = stripControlChars(ad.ID)
	ad.Text = stripControlChars(ad.Text)
	ad.URL = stripControlChars(ad.URL)
	ad.SpinnerText = stripControlChars(ad.SpinnerText)
	return ad
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
		debugLogf("device registration skipped: missing device id or secret")
		return
	}

	sum := sha256.Sum256([]byte(cfg.DeviceSecret))
	body, _ := json.Marshal(deviceRegistrationPayload{
		DeviceID:   cfg.DeviceID,
		SecretHash: hex.EncodeToString(sum[:]),
	})

	resp, err := httpClient.Post(cfg.APIBase+"/api/device/register", "application/json", bytes.NewReader(body))
	if err != nil {
		debugLogf("device registration failed: %s", safeHTTPError(err))
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		debugLogf("device registration returned HTTP %d", resp.StatusCode)
	}
}

func impressionBody(cfg *Config, ad Ad, cmd string, seconds int) *bytes.Reader {
	body, _ := json.Marshal(impressionPayload{
		DeviceID: cfg.DeviceID,
		AdID:     ad.ID,
		Cmd:      cmd,
		Seconds:  seconds,
		Kind:     "impression",
	})
	return bytes.NewReader(body)
}

func postImpression(cfg *Config, ad Ad, cmd string, seconds int) {
	resp, err := httpClient.Post(cfg.APIBase+"/api/events", "application/json", impressionBody(cfg, ad, cmd, seconds))
	if err != nil {
		debugLogf("impression report failed for cmd %q: %s", cmd, safeHTTPError(err))
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		debugLogf("impression report returned HTTP %d for cmd %q", resp.StatusCode, cmd)
	}
}

// reportImpression posts the impression on the shared client (800ms timeout) so
// a slow or dead network can never freeze the shell prompt for seconds after a
// wrapped command has already finished.
func reportImpression(cfg *Config, ad Ad, cmd string, seconds int) {
	registerDeviceOnce.Do(func() {
		registerDevice(cfg)
	})

	postImpression(cfg, ad, cmd, seconds)
}
