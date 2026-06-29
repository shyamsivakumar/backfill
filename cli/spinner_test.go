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
	if v := capSpinnerVerb(spinnerLabel(stripControlChars(a.SpinnerText)), fallbackSpinnerVerbCols); v != "" {
		return v
	}
	return capSpinnerVerb(spinnerLabel(stripControlChars(a.Text)), fallbackSpinnerVerbCols)
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
	// labelFor exercises spinnerLabel, the short single-token form still used for
	// inline agents (Codex/Factory). The Claude spinner now keeps the full
	// description; that wiring is covered by TestSpinnerVerbFull.
	for _, c := range cases {
		if got := labelFor(c.ad); got != c.want {
			t.Errorf("label for %+v = %q, want %q", c.ad, got, c.want)
		}
	}
}

func TestSpinnerVerbFull(t *testing.T) {
	m := spinnerTypeMarker
	// A trending repo shows "repo · description" — owner dropped, description kept —
	// sourced from the rich Text, not the bare SpinnerText slug.
	if got := spinnerVerbForAd(Ad{ID: "gh_tf", Text: "tensorflow/tensorflow · An Open Source ML Framework", SpinnerText: "tensorflow/tensorflow"}, 60); got != m("gh_tf")+"tensorflow · An Open Source ML Framework" {
		t.Errorf("trending verb = %q", got)
	}
	// An ad uses its full Text description, not the shortened SpinnerText.
	if got := spinnerVerbForAd(Ad{ID: "house_uv", Text: "uv · fast Python packages and project installs", SpinnerText: "uv · Python packages"}, 60); got != m("house_uv")+"uv · fast Python packages and project installs" {
		t.Errorf("ad verb should use full Text: got %q", got)
	}
	// A bare slug with no description falls back to the natural source.
	if got := spinnerVerbForAd(Ad{ID: "gh_bat", Text: "sharkdp/bat"}, 50); got != m("gh_bat")+"bat on GitHub" {
		t.Errorf("bare verb = %q, want %q", got, "bat on GitHub")
	}
	// A narrow budget truncates the description tail and never exceeds the budget.
	got := spinnerVerbForAd(Ad{ID: "gh_tf", Text: "tensorflow/tensorflow · An Open Source ML Framework"}, fallbackSpinnerVerbCols)
	if w := runewidth.StringWidth(strings.TrimPrefix(got, m("gh_tf"))); w > fallbackSpinnerVerbCols {
		t.Errorf("verb width %d exceeds budget %d: %q", w, fallbackSpinnerVerbCols, got)
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("overflow should end in an ellipsis: got %q", got)
	}
}

func TestCapSpinnerVerb(t *testing.T) {
	// A lone trailing period is part of the word, not a server ellipsis.
	if got := capSpinnerVerb("etc.", fallbackSpinnerVerbCols); got != "etc." {
		t.Errorf("lone trailing period should survive: got %q", got)
	}
	// Degenerate inputs collapse to empty without looping forever.
	for _, s := range []string{"", "   ", "…", "...", "… … …", "...…..."} {
		if got := capSpinnerVerb(s, fallbackSpinnerVerbCols); got != "" {
			t.Errorf("degenerate input %q should cap to empty: got %q", s, got)
		}
	}
	// Wide (CJK) glyphs count as two columns, so the cap holds visible width.
	if got := runewidth.StringWidth(capSpinnerVerb(strings.Repeat("本", 40), fallbackSpinnerVerbCols)); got > fallbackSpinnerVerbCols {
		t.Errorf("CJK verb exceeds the column cap: width %d", got)
	}
	// A wider budget lets a long repo name through without truncation.
	if got := capSpinnerVerb("defending-code-reference-harness", 50); got != "defending-code-reference-harness" {
		t.Errorf("wide budget should not truncate: got %q", got)
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
		"gh_repo":  "\U0001F539 ", // small blue diamond — free content
		"hn_123":   "\U0001F539 ",
		"tip_uv":   "\U0001F539 ", // small blue diamond — free content (tips too)
		"camp_abc": "\U0001F538 ", // small orange diamond — paid ad
		"house_uv": "\U0001F538 ",
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
	if !strings.HasPrefix(got, "ad · fd") {
		t.Errorf("inline agent verb should be the short label: got %q", got)
	}
}
