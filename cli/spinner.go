package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/mattn/go-runewidth"
	"golang.org/x/term"
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

// fallbackSpinnerVerbCols is the verb width used when the terminal size can't be
// read (a hook with no tty and no cached width). Conservative so a narrow pane
// never overflows; spinnerVerbCols widens it whenever the real width is known.
const fallbackSpinnerVerbCols = 24

// spinnerStatusReserve is the width Claude keeps to the right of the verb: its own
// leading glyph, our type marker, and the appended status, e.g.
// "(123s · ↓ 9.9k tokens · thinking with high effort)". The verb budget is the
// terminal width minus this, so a wide window shows full repo names and only a
// narrow one truncates.
const spinnerStatusReserve = 55

// spinnerVerbCols is the column budget for the verb label, derived from the real
// terminal width when it can be read. Claude appends its status after the verb, so
// a verb sized to the full width would push that status off-screen; reserve room
// for it and let the verb fill whatever remains. Falls back to a fixed width when
// the size is unknown.
func spinnerVerbCols() int {
	cols := detectTermCols()
	if cols <= 0 {
		return fallbackSpinnerVerbCols
	}
	budget := cols - spinnerStatusReserve
	if budget < 12 {
		budget = 12
	}
	return budget
}

// spinnerContentSuffix returns a natural-language source for bare-name content
// ("repo" → "repo on GitHub") so the verb ends on a complete word. A bare name
// otherwise sits right before Claude's trailing "…", which reads it as a chopped
// word. Ads and tips already end on a "· descriptor", so they get nothing.
func spinnerContentSuffix(id string) string {
	switch {
	case strings.HasPrefix(id, "gh_"):
		return " on GitHub"
	case strings.HasPrefix(id, "hn_"):
		return " on HN"
	}
	return ""
}

func spinnerVerbForAd(ad Ad, maxCols int) string {
	raw := spinnerLabel(stripControlChars(ad.SpinnerText))
	if capSpinnerVerb(raw, maxCols) == "" {
		raw = spinnerLabel(stripControlChars(ad.Text))
	}
	label := capSpinnerVerb(raw, maxCols)
	if label == "" {
		return ""
	}
	// A single bare token (no descriptor) gets a natural source appended; the name
	// truncates to make room, the source word always stays whole.
	if suffix := spinnerContentSuffix(ad.ID); suffix != "" && !strings.ContainsAny(label, " ·") {
		nameCols := maxCols - runewidth.StringWidth(suffix)
		if nameCols < 4 {
			nameCols = 4
		}
		label = capSpinnerVerb(raw, nameCols) + suffix
	}
	return spinnerTypeMarker(ad.ID) + label
}

// spinnerTypeMarker returns a small colored diamond that signals the content type
// in Claude's spinner — blue for free content (trending repo / HN / tip), orange
// for a paid ad. Claude renders verbs through Ink, which never honors ANSI, so a
// colored emoji is the only way to carry color into that surface: the glyph is
// colored by the font, not by an escape code. The small diamonds (🔹🔸) are the
// only compact colored glyphs Unicode offers — there is no green or other-color
// small diamond — so the palette is the two that matter: ad vs not-ad. Gated to
// macOS, where color emoji render; elsewhere it would be a tofu box.
func spinnerTypeMarker(id string) string {
	if runtime.GOOS != "darwin" {
		return ""
	}
	switch {
	case id == "" || id == "earnings":
		return ""
	case strings.HasPrefix(id, "gh_"), strings.HasPrefix(id, "hn_"), strings.HasPrefix(id, "tip_"):
		return "\U0001F539 " // small blue diamond — free content
	default:
		return "\U0001F538 " // small orange diamond — paid ad
	}
}

// spinnerLabel trims an ad down to its lead label — the tool or repo name before
// the first description separator — so the verb stays short. The server ships full
// sentences ("fd: a simple, fast alternative …") meant for the wrapped-command
// line; the spinner only needs the name, the way Claude's native verbs are a
// single word.
func spinnerLabel(s string) string {
	for _, sep := range []string{" — ", " - ", ": "} {
		if before, _, ok := strings.Cut(s, sep); ok && strings.TrimSpace(before) != "" {
			s = before
		}
	}
	s = strings.TrimSpace(s)
	// A trending entry is "owner/repo": show the repo name (the recognizable,
	// shorter half) rather than truncating the owner mid-word into "owner/re…".
	if !strings.ContainsAny(s, " \t") && strings.Contains(s, "/") {
		if i := strings.LastIndexByte(s, '/'); i >= 0 && i+1 < len(s) {
			s = s[i+1:]
		}
	}
	return s
}

// capSpinnerVerb drops a trailing ellipsis the server already appended (but not a
// lone period, so "etc." survives), then truncates to maxCols columns with a
// single "…" so the verb plus Claude's status fit on one line.
func capSpinnerVerb(s string, maxCols int) string {
	s = strings.TrimSpace(s)
	for {
		t := strings.TrimRight(strings.TrimSuffix(strings.TrimSuffix(s, "…"), "..."), " ")
		if t == s {
			break
		}
		s = t
	}
	return runewidth.Truncate(s, maxCols, "…")
}

// detectTermCols returns the controlling terminal's column count, trying the
// standard streams first, then /dev/tty (so it works from a hook whose stdout is a
// pipe), then the width cached by the last wrapped command. Returns 0 if unknown.
func detectTermCols() int {
	for _, f := range []*os.File{os.Stdout, os.Stderr, os.Stdin} {
		if c, _, err := term.GetSize(int(f.Fd())); err == nil && c > 0 {
			return c
		}
	}
	if tty, err := os.Open("/dev/tty"); err == nil {
		c, _, err := term.GetSize(int(tty.Fd()))
		tty.Close()
		if err == nil && c > 0 {
			return c
		}
	}
	return readCachedTermCols()
}

func termColsCachePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".backfill", "term-cols")
}

// cacheTermCols records the terminal width from a context that has a real tty (a
// wrapped command), so the tty-less spinner hook can still size verbs to the pane.
func cacheTermCols(cols int) {
	if cols <= 0 {
		return
	}
	_ = os.WriteFile(termColsCachePath(), []byte(strconv.Itoa(cols)), 0o600)
}

func readCachedTermCols() int {
	b, err := os.ReadFile(termColsCachePath())
	if err != nil {
		return 0
	}
	c, _ := strconv.Atoi(strings.TrimSpace(string(b)))
	return c
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

	maxCols := spinnerVerbCols()
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
		v := spinnerVerbForAd(ad, maxCols)
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
