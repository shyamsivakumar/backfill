package main

import (
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"
)

func TestSpinnerVerbCappedSoClaudeStatusFits(t *testing.T) {
	// The bug in the screenshot: the server sends a long descriptive spinnerText,
	// and Claude appends "(5s · ↓ 42 tokens · thinking with high effort)" after it,
	// so the line overflows and gets cut. The verb should collapse to the tool name.
	long := Ad{ID: "tip_fd", SpinnerText: "fd: a simple, fast alternative to find with sensibl..."}
	if got := spinnerVerbForAd(long); got != "fd" {
		t.Errorf("verbose spinnerText should collapse to the tool name: got %q", got)
	}

	split := Ad{ID: "house_uv", Text: "uv — fast Python packages and project installs"}
	if got := spinnerVerbForAd(split); got != "uv" {
		t.Errorf("verb should split on the em dash: got %q", got)
	}

	// A short " · " house verb keeps its form (no description separator to split on).
	spinner := Ad{ID: "house_ripgrep", SpinnerText: "ripgrep · fast search"}
	if got := spinnerVerbForAd(spinner); got != "ripgrep · fast search" {
		t.Errorf("a short · verb should pass through unchanged: got %q", got)
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

func TestSpinnerAdBytesShortLabelForInlineAgents(t *testing.T) {
	// Factory/Codex inline injection must collapse a verbose description to the
	// tool name + the "ad · " disclosure, not the whole sentence.
	got := string(spinnerAdBytes(Ad{ID: "tip_fd", SpinnerText: "fd: a simple, fast alternative to find with sensibl..."}))
	if got != "ad · fd" {
		t.Errorf("inline agent verb should be the short label: got %q", got)
	}
}
