package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
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
		if elapsed >= 5 {
			if elapsed > 90 {
				elapsed = 90
			}
			reportImpressionFast(cfg, cache.Ad, "claude-code", elapsed)
		}
	}

	ad := fetchAd(cfg, "claude-code")
	ad.ID = stripControlChars(ad.ID)
	ad.Text = stripControlChars(ad.Text)
	ad.URL = stripControlChars(ad.URL)

	writeStatuslineCache(statuslineCache{Ad: ad, FetchedAt: now})
	setSpinnerVerb([]string{ad.Text})
}

func writeClaudeSettingsAtomic(settings map[string]any) error {
	p := claudeSettingsPath()
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
