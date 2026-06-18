package main

import (
	"strings"
	"testing"
	"time"
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

func TestComposeLineLabelsContentNotAsAd(t *testing.T) {
	line := composeLine("⠙ dbt 1/3", 10*time.Second,
		Ad{ID: "gh_foo_bar", Text: "foo/bar · a genuinely useful repo"}, "https://x/r/gh_foo_bar")
	if !strings.Contains(line, "trending · ") {
		t.Errorf("trending content not labeled 'trending ·': %q", line)
	}
	if strings.Contains(line, "ad · ") {
		t.Errorf("trending content mislabeled as an ad: %q", line)
	}
	if !strings.Contains(line, "~$") {
		t.Errorf("live earnings meter missing from line: %q", line)
	}

	adLine := composeLine("⠙ dbt 1/3", time.Second, Ad{ID: "house_uv", Text: "uv · fast Python"}, "https://x")
	if !strings.Contains(adLine, "ad · ") {
		t.Errorf("real ad not labeled 'ad ·': %q", adLine)
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
