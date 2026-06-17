package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func setSpinnerVerb(verbs []string) error {
	settings, _, _, err := readClaudeSettings()
	if err != nil {
		return err
	}

	clean := make([]string, 0, len(verbs))
	for _, verb := range verbs {
		verb = stripControlChars(verb)
		if verb != "" {
			clean = append(clean, verb)
		}
	}
	if len(clean) == 0 {
		clean = []string{""}
	}

	settings["spinnerVerbs"] = map[string]any{
		"mode":  "replace",
		"verbs": clean,
	}
	return writeClaudeSettingsAtomic(settings)
}

func removeSpinnerVerb() error {
	settings, _, exists, err := readClaudeSettings()
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}

	if backupSettings, ok := readClaudeSettingsBackup(); ok {
		if restored, hasSpinner := backupSettings["spinnerVerbs"]; hasSpinner {
			settings["spinnerVerbs"] = restored
			return writeClaudeSettingsAtomic(settings)
		}
	}

	delete(settings, "spinnerVerbs")
	return writeClaudeSettingsAtomic(settings)
}

func spinnerVerbForAd(ad Ad) string {
	if strings.TrimSpace(ad.SpinnerText) != "" {
		return strings.TrimSpace(stripControlChars(ad.SpinnerText))
	}

	text := stripControlChars(ad.Text)
	if before, _, ok := strings.Cut(text, " — "); ok {
		return strings.TrimSpace(before)
	}
	if before, _, ok := strings.Cut(text, " - "); ok {
		return strings.TrimSpace(before)
	}

	runes := []rune(text)
	if len(runes) > 40 {
		runes = runes[:40]
	}
	return strings.TrimSpace(string(runes))
}

// fetchSpinnerBatch collects up to n distinct spinner verbs for Claude Code by
// calling fetchAd repeatedly, mixing whatever the server returns: ads, GitHub /
// HN content items, etc. Claude Code cycles through the verb list on its own, so
// a batch rotates during a session where a single verb would sit frozen. The
// first billable ad (ID not gh_/hn_) is returned as primary so the next refresh
// can bill one impression; when lifetime earnings exist, an earnings verb is
// appended once.
func fetchSpinnerBatch(cfg *Config, n int) (verbs []string, primary Ad, earned int64) {
	if n <= 0 {
		n = 10
	}

	// Fetch n+2 candidates concurrently (one round-trip): enough variety to
	// dedupe into a rotating batch without a 2*n burst on every hook.
	ads := fetchAdsConcurrent(cfg, "claude-code", n+2)

	verbs = make([]string, 0, n)
	seen := make(map[string]struct{}, n)
	for _, ad := range ads {
		if ad.EarnedMicros > earned {
			earned = ad.EarnedMicros
		}
		if primary.ID == "" && !strings.HasPrefix(ad.ID, "gh_") && !strings.HasPrefix(ad.ID, "hn_") {
			primary = ad
		}
		if len(verbs) >= n {
			continue
		}
		v := spinnerVerbForAd(ad)
		if v == "" {
			continue
		}
		if _, dup := seen[v]; dup {
			continue
		}
		seen[v] = struct{}{}
		verbs = append(verbs, v)
	}

	if earned > 0 {
		ev := fmt.Sprintf("$%.2f earned · backfill", float64(earned)/1e6)
		if _, dup := seen[ev]; !dup {
			verbs = append(verbs, ev)
		}
	}
	return verbs, primary, earned
}

func cmdSpinnerRefresh() {
	io.Copy(io.Discard, os.Stdin)

	cfg := loadConfig()
	if !cfg.Enabled {
		return
	}

	unlock, ok := acquireStatuslineRefreshLock()
	if !ok {
		return
	}
	defer unlock()

	now := time.Now().Unix()
	if cache, ok := readStatuslineCache(); ok {
		elapsed := int(now - cache.FetchedAt)
		if elapsed >= 5 &&
			cache.Ad.ID != "" &&
			!strings.HasPrefix(cache.Ad.ID, "gh_") &&
			!strings.HasPrefix(cache.Ad.ID, "hn_") {
			if elapsed > 90 {
				elapsed = 90
			}
			reportImpressionFast(cfg, cache.Ad, "claude-code", elapsed)
		}
	}

	verbs, primary, _ := fetchSpinnerBatch(cfg, 10)
	if len(verbs) == 0 {
		return
	}
	if err := setSpinnerVerb(verbs); err != nil {
		return
	}
	writeStatuslineCache(statuslineCache{Ad: primary, FetchedAt: now})
}

func writeClaudeSettingsAtomic(settings map[string]any) error {
	return writeJSONSettingsAtomic(claudeSettingsPath(), settings)
}

func writeJSONSettingsAtomic(p string, settings map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}

	b, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')

	f, err := os.CreateTemp(filepath.Dir(p), ".settings.json.*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)

	if _, err := f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err := f.Chmod(0o600); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	return os.Rename(tmp, p)
}
