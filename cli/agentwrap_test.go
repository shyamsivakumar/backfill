package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"
)

func TestSpinnerRewriterFixtures(t *testing.T) {
	const ad = "ad · fd"
	fixtures := []struct {
		name       string
		input      string
		want       string
		wantAds    int
		wantActive bool
	}{
		{
			name:       "codex working with ansi",
			input:      "\x1b[32mWorking\x1b[0m \x1b[2m(Esc to interrupt)\x1b[0m",
			want:       "\x1b[32m" + ad + "\x1b[0m \x1b[2m(Esc to interrupt)\x1b[0m",
			wantAds:    1,
			wantActive: true,
		},
		{
			name:       "codex ascii ellipsis and lowercase anchor",
			input:      "Executing... (esc to interrupt)",
			want:       ad + " (esc to interrupt)",
			wantAds:    1,
			wantActive: true,
		},
		{
			name:       "codex unicode ellipsis and mixed case anchor",
			input:      "Executing… (eSc To InTeRrUpT)",
			want:       ad + " (eSc To InTeRrUpT)",
			wantAds:    1,
			wantActive: true,
		},
		{
			name:       "factory ascii ellipsis",
			input:      "Streaming... (Press ESC to stop)",
			want:       ad + " (Press ESC to stop)",
			wantAds:    1,
			wantActive: true,
		},
		{
			name:       "factory unicode ellipsis and lowercase anchor",
			input:      "Streaming… (press esc to stop)",
			want:       ad + " (press esc to stop)",
			wantAds:    1,
			wantActive: true,
		},
		{
			name:       "unknown gerund falls back before anchor",
			input:      "Brewing (Esc to interrupt)",
			want:       "Brewing (" + ad + "  Esc to interrupt)",
			wantAds:    1,
			wantActive: true,
		},
		{
			name:    "working prose without anchor passes through",
			input:   "INFO Working on fixture coverage\n",
			want:    "INFO Working on fixture coverage\n",
			wantAds: 0,
		},
		{
			name:    "anchor prose outside spinner passes through",
			input:   "Help text: choose Esc to interrupt a recording.\n",
			want:    "Help text: choose Esc to interrupt a recording.\n",
			wantAds: 0,
		},
		{
			name:       "carriage return keeps partial status and log text",
			input:      "build log: Working on tests\n\r\x1b[2K\x1b[33mWorking\x1b[0m (Esc to interrupt)\rpartial status",
			want:       "build log: Working on tests\n\r\x1b[2K\x1b[33m" + ad + "\x1b[0m (Esc to interrupt)\rpartial status",
			wantAds:    1,
			wantActive: true,
		},
		{
			name:       "multiple anchors still get one label",
			input:      "Working (Esc to interrupt) (press esc to stop)",
			want:       ad + " (Esc to interrupt) (press esc to stop)",
			wantAds:    1,
			wantActive: true,
		},
	}

	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			rw := &spinnerRewriter{ad: []byte(ad)}
			got := rw.transform([]byte(fixture.input))
			if string(got) != fixture.want {
				t.Fatalf("transform() = %q, want %q", got, fixture.want)
			}
			if count := bytes.Count(got, []byte(ad)); count != fixture.wantAds {
				t.Fatalf("transform() contains %d ads, want %d: %q", count, fixture.wantAds, got)
			}
			if rw.active != fixture.wantActive {
				t.Fatalf("active = %v, want %v", rw.active, fixture.wantActive)
			}
		})
	}
}

func TestSpinnerRewriterIsIdempotent(t *testing.T) {
	const ad = "ad · fd"
	for _, input := range []string{
		"Working (Esc to interrupt)",
		"Brewing (Press ESC to stop)",
	} {
		rw := &spinnerRewriter{ad: []byte(ad)}
		first := rw.transform([]byte(input))
		second := rw.transform(first)
		if !bytes.Equal(second, first) {
			t.Errorf("second transform stacked output for %q:\nfirst:  %q\nsecond: %q", input, first, second)
		}
		if count := bytes.Count(second, []byte(ad)); count != 1 {
			t.Errorf("second transform contains %d ads, want 1: %q", count, second)
		}
	}
}

func TestSpinnerAdBytesCapsLongLabel(t *testing.T) {
	maxCols := spinnerVerbCols()
	longLabel := strings.Repeat("sponsor", maxCols+1)
	wantLabel := capSpinnerVerb(longLabel, maxCols)
	got := string(spinnerAdBytes(Ad{SpinnerText: longLabel}))

	if want := "ad · " + wantLabel; got != want {
		t.Fatalf("spinnerAdBytes() = %q, want %q", got, want)
	}
	label := strings.TrimPrefix(got, "ad · ")
	if width := runewidth.StringWidth(label); width > maxCols {
		t.Fatalf("label width = %d, want <= %d: %q", width, maxCols, label)
	}
	if !strings.HasSuffix(label, "…") {
		t.Fatalf("capped label = %q, want trailing ellipsis", label)
	}
}
