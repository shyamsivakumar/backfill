package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
)

type Config struct {
	DeviceID string `json:"device_id"`
	Enabled  bool   `json:"enabled"`
	APIBase  string `json:"api_base"`
}

func configPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".backfill", "config.json")
}

func loadConfig() *Config {
	cfg := &Config{Enabled: true, APIBase: defaultAPIBase}
	if b, err := os.ReadFile(configPath()); err == nil {
		json.Unmarshal(b, cfg)
	}
	cfg.DeviceID = stripControlChars(cfg.DeviceID)
	cfg.APIBase = stripControlChars(cfg.APIBase)

	deviceOverride := ""
	if v := os.Getenv("BACKFILL_DEVICE"); v != "" {
		v = stripControlChars(v)
		if v != "" && len(v) <= 64 {
			deviceOverride = v
		}
	}

	if !validAPIBase(cfg.APIBase) {
		cfg.APIBase = defaultAPIBase
	}
	if cfg.DeviceID == "" && deviceOverride == "" {
		b := make([]byte, 8)
		rand.Read(b)
		cfg.DeviceID = "dev_" + hex.EncodeToString(b)
		saveConfig(cfg)
	}
	if deviceOverride != "" {
		cfg.DeviceID = deviceOverride
	}
	if v := os.Getenv("BACKFILL_API"); v != "" {
		v = stripControlChars(v)
		if validAPIBase(v) {
			cfg.APIBase = v
		}
	}
	return cfg
}

func validAPIBase(s string) bool {
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

func saveConfig(cfg *Config) {
	p := configPath()
	os.MkdirAll(filepath.Dir(p), 0o755)
	b, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(p, b, 0o600)
}
