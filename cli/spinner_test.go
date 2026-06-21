package main

import (
	"runtime"
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"
)

// labelFor reproduces the label half of spinnerVerbForAd (without the type marker)
// so label assertions stay independent of the platform-gated emoji prefix.
func labelFor(a Ad) string {
	if v := capSpinnerVerb(spinnerLabel(stripControlChars(a.SpinnerText))); v != "" {
		return v
	}
	return capSpinnerVerb(spinnerLabel(stripControlChars(a.Text)))
}

func TestSpinnerLabelStaysShort(t *testing.T) {
	cases := []struct {
		ad   Ad
		want string
	}{
		// Verbose server description collapses to the tool name (the screenshot bug).
		{Ad{ID: "tip_fd", SpinnerText: "fd: a simple, fast alternative to find with sensibl..."}, "fd"},
		{Ad{ID: "house_uv", Text: "uv — fast Python packages and project installs"}, "uv"},
		// A short " · " house verb has no description separator, so it is kept whole.
		{Ad{ID: "house_ripgrep", SpinnerText: "ripgrep · fast search"}, "ripgrep · fast search"},
		// A long trending "owner/repo" shows the repo name, not a mid-word truncation.
		{Ad{ID: "gh_ebook", Text: "EbookFoundation/free-programming-books"}, "free-programming-books"},
		// An ellipsis-only spinnerText caps to empty and falls through to Text.
		{Ad{ID: "house_x", SpinnerText: "…", Text: "delta — a syntax-highlighting pager for git"}, "delta"},
	}
	for _, c := range cases {
		if got := labelFor(c.ad); got != c.want {
			t.Errorf("label for %+v = %q, want %q", c.ad, got, c.want)
		}
		// The full verb is the type marker plus the label.
		if got := spinnerVerbForAd(c.ad); got != spinnerTypeMarker(c.ad.ID)+c.want {
			t.Errorf("spinnerVerbForAd(%+v) = %q, want marker+%q", c.ad, got, c.want)
		}
	}
}

func TestCapSpinnerVerb(t *testing.T) {
	// A lone trailing period is part of the word, not a server ellipsis.
	if got := capSpinnerVerb("etc."); got != "etc." {
		t.Errorf("lone trailing period should survive: got %q", got)
	}
	// Degenerate inputs collapse to empty without looping forever.
	for _, s := range []string{"", "   ", "…", "...", "… … …", "...…..."} {
		if got := capSpinnerVerb(s); got != "" {
			t.Errorf("degenerate input %q should cap to empty: got %q", s, got)
		}
	}
	// Wide (CJK) glyphs count as two columns, so the cap holds visible width.
	if got := runewidth.StringWidth(capSpinnerVerb(strings.Repeat("本", 40))); got > maxSpinnerVerbCols {
		t.Errorf("CJK verb exceeds the column cap: width %d", got)
	}
}

func TestSpinnerTypeMarkerColorsByType(t *testing.T) {
	if runtime.GOOS != "darwin" {
		// Off macOS the marker is suppressed so verbs never render as tofu boxes.
		for _, id := range []string{"gh_x", "tip_y", "camp_z", "house_w"} {
			if m := spinnerTypeMarker(id); m != "" {
				t.Errorf("non-darwin marker for %q should be empty, got %q", id, m)
			}
		}
		return
	}
	cases := map[string]string{
		"gh_repo":  "\U0001F535 ", // blue — trending
		"hn_123":   "\U0001F535 ",
		"tip_uv":   "\U0001F7E2 ", // green — tip
		"camp_abc": "\U0001F7E0 ", // orange — paid ad
		"house_uv": "\U0001F7E0 ",
	}
	for id, want := range cases {
		if got := spinnerTypeMarker(id); got != want {
			t.Errorf("spinnerTypeMarker(%q) = %q, want %q", id, got, want)
		}
	}
	// Empty / earnings entries carry no marker.
	for _, id := range []string{"", "earnings"} {
		if got := spinnerTypeMarker(id); got != "" {
			t.Errorf("spinnerTypeMarker(%q) should be empty, got %q", id, got)
		}
	}
}

func TestSpinnerAdBytesShortLabelForInlineAgents(t *testing.T) {
	// Factory/Codex inline injection collapses a verbose description to the tool
	// name + the "ad · " disclosure, not the whole sentence.
	got := string(spinnerAdBytes(Ad{ID: "tip_fd", SpinnerText: "fd: a simple, fast alternative to find with sensibl..."}))
	if got != "ad · fd" {
		t.Errorf("inline agent verb should be the short label: got %q", got)
	}
}
