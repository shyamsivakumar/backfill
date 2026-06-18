package main

import (
	"strings"
	"testing"
)

// TestOsaQuoteEscapes ensures a command name or arg can never break out of the
// AppleScript string literal (no notification-driven AppleScript injection).
func TestOsaQuoteEscapes(t *testing.T) {
	// Every embedded double quote must be backslash-escaped so it can't terminate
	// the AppleScript string literal and inject a `do shell script` payload.
	if got, want := osaQuote(`x"y`), `"x\"y"`; got != want {
		t.Errorf("quote not escaped: got %s want %s", got, want)
	}
	if got, want := osaQuote(`back\slash`), `"back\\slash"`; got != want {
		t.Errorf("backslash not escaped: got %s want %s", got, want)
	}
	// Backslashes escaped before quotes, so an input `\"` becomes `\\\"`.
	if got, want := osaQuote(`a\"b`), `"a\\\"b"`; got != want {
		t.Errorf("ordering regressed: got %s want %s", got, want)
	}
}

func TestNotifyMsgSanitizesAndAvoidsQuoteCollision(t *testing.T) {
	rec := Receipt{Args: []string{"dbt", "run", "--select", "tag:fin\njab\"x"}, Seconds: 65, OK: true, Count: "5 models"}
	msg := notifyMsg(rec)
	if strings.ContainsAny(msg, "\n\r") {
		t.Errorf("notifyMsg leaked a newline: %q", msg)
	}
	// osaQuote on the message must stay a well-formed literal: a quote only ever
	// appears as \" (escaped), never bare, except the two wrapping quotes.
	q := osaQuote(msg)
	bare := 0
	for i := 0; i < len(q); i++ {
		if q[i] == '"' && (i == 0 || q[i-1] != '\\') {
			bare++
		}
	}
	if bare != 2 {
		t.Errorf("osaQuote(notifyMsg) has %d unescaped quotes (want 2 wrappers): %q", bare, q)
	}
}

func TestShouldNotifyGating(t *testing.T) {
	t.Setenv("BACKFILL_NO_NOTIFY", "")
	if shouldNotify(Receipt{Seconds: 5}) {
		t.Error("short run should not notify")
	}
	if !shouldNotify(Receipt{Seconds: notifyThresholdSeconds}) {
		t.Error("long run should notify")
	}
	t.Setenv("BACKFILL_NO_NOTIFY", "1")
	if shouldNotify(Receipt{Seconds: 300}) {
		t.Error("BACKFILL_NO_NOTIFY must disable notifications")
	}
}
