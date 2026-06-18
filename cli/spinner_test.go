package main

import (
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"
)

func TestSpinnerVerbCappedSoClaudeStatusFits(t *testing.T) {
	// The bug in the screenshot: the server sends a long spinnerText with its own
	// trailing "...", and Claude appends "(5s · ↓ 42 tokens · …)" after it, so the
	// line overflows and gets cut.
	long := Ad{ID: "tip_fd", SpinnerText: "fd: a simple, fast alternative to find with sensibl..."}
	verb := spinnerVerbForAd(long)
	if n := len([]rune(verb)); n > maxSpinnerVerbCols {
		t.Errorf("verb too long for Claude's status to fit: %d runes %q", n, verb)
	}
	if strings.HasSuffix(verb, "...") {
		t.Errorf("server's trailing ellipsis not normalized: %q", verb)
	}

	split := Ad{ID: "house_uv", Text: "uv — fast Python packages and project installs"}
	if got := spinnerVerbForAd(split); got != "uv" {
		t.Errorf("verb should split on the em dash: got %q", got)
	}

	spinner := Ad{ID: "house_ripgrep", SpinnerText: "ripgrep · fast search"}
	if got := spinnerVerbForAd(spinner); got != "ripgrep · fast search" {
		t.Errorf("a short spinnerText should pass through unchanged: got %q", got)
	}

	// A spinnerText that is only an ellipsis caps to empty; fall through to Text
	// rather than emit a blank verb.
	punct := Ad{ID: "house_x", SpinnerText: "…", Text: "delta — a syntax-highlighting pager for git"}
	if got := spinnerVerbForAd(punct); got != "delta" {
		t.Errorf("empty capped spinnerText should fall through to Text: got %q", got)
	}

	// A lone trailing period is part of the word, not a server ellipsis.
	if got := capSpinnerVerb("etc."); got != "etc." {
		t.Errorf("lone trailing period should survive: got %q", got)
	}

	// Degenerate inputs must collapse to empty (so the caller falls through or
	// the batch skips them) without looping forever.
	for _, s := range []string{"", "   ", "…", "...", "… … …", "...…..."} {
		if got := capSpinnerVerb(s); got != "" {
			t.Errorf("degenerate input %q should cap to empty: got %q", s, got)
		}
	}

	// Wide (CJK) glyphs count as two columns, so the cap holds the verb to the
	// same visible width Claude's status budget assumes — not twice as wide.
	wide := strings.Repeat("本", 40)
	if got := runewidth.StringWidth(capSpinnerVerb(wide)); got > maxSpinnerVerbCols {
		t.Errorf("CJK verb exceeds the column cap: width %d", got)
	}
}
