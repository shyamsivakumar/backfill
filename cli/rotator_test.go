package main

import (
	"os"
	"strings"
	"testing"
	"time"

	"golang.org/x/term"
)

func TestSlotLabelDistinguishesContentFromAds(t *testing.T) {
	cases := map[string]string{
		"gh_owner_repo": "trending",
		"hn_12345":      "hn",
		"tip_ripgrep":   "tip",
		"house_uv":      "ad",
		"camp_abc":      "ad",
	}
	for id, want := range cases {
		if got, _ := slotLabel(id); got != want {
			t.Errorf("slotLabel(%q) = %q, want %q", id, got, want)
		}
	}
}

func TestComposeLineDistinguishesContentByColorNotWord(t *testing.T) {
	line := composeLine("⠙ dbt 1/3", 10*time.Second,
		Ad{ID: "gh_foo_bar", Text: "foo/bar · a genuinely useful repo"}, "https://x/r/gh_foo_bar")
	// Content carries no "trending"/word label and is NOT prefixed as an ad; it
	// is distinguished by its cyan color (\x1b[36m) instead.
	if strings.Contains(line, "ad · ") {
		t.Errorf("content mislabeled as an ad: %q", line)
	}
	if strings.Contains(line, "trending") {
		t.Errorf("content still carries a 'trending' word label: %q", line)
	}
	if !strings.Contains(line, "\x1b[36m") {
		t.Errorf("content not shown in its distinct color: %q", line)
	}
	if !strings.Contains(line, "~$") {
		t.Errorf("live earnings meter missing from line: %q", line)
	}

	adLine := composeLine("⠙ dbt 1/3", time.Second, Ad{ID: "house_uv", Text: "uv · fast Python"}, "https://x")
	if !strings.Contains(adLine, "ad · ") {
		t.Errorf("paid ad missing its 'ad ·' disclosure: %q", adLine)
	}
}

func TestComposeLineTruncatesWideGlyphsByColumn(t *testing.T) {
	// A CJK ad (two columns per rune) must not overflow the line: the slot text's
	// display width has to stay within the budget, not just its rune count.
	cols := 80
	if c, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && c > 0 {
		cols = c
	}
	wide := strings.Repeat("本", 200)
	line := composeLine("⠙ dbt 1/3", time.Second, Ad{ID: "house_x", Text: wide}, "https://x")
	if w := visibleLen(line); w > cols {
		t.Errorf("composed line overflows terminal width %d: got %d columns", cols, w)
	}
}

func TestMeterAdvancesOverTime(t *testing.T) {
	if meterText(0) == meterText(30*time.Second) {
		t.Error("earnings meter does not advance with elapsed time")
	}
	if !strings.HasPrefix(meterText(time.Second), "~$") {
		t.Errorf("meter not formatted as an estimate: %q", meterText(time.Second))
	}
}
