package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// notifyThresholdSeconds: only notify after a genuinely long run, where the user
// has likely switched away. Short commands never fire a notification.
const notifyThresholdSeconds = 30

// notifyDone fires a desktop notification when a long wrapped command finishes,
// so backfill doubles as a build monitor: "your build is done, here's how it
// went, here's what it earned." Best-effort and time-bounded — it can never
// hang or fail the command. Opt out with BACKFILL_NO_NOTIFY=1.
func shouldNotify(rec Receipt) bool {
	return rec.Seconds >= notifyThresholdSeconds && os.Getenv("BACKFILL_NO_NOTIFY") == ""
}

func notifyDone(rec Receipt) {
	if !shouldNotify(rec) {
		return
	}

	msg := notifyMsg(rec)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	switch runtime.GOOS {
	case "darwin":
		if path, err := exec.LookPath("osascript"); err == nil {
			script := fmt.Sprintf("display notification %s with title %s", osaQuote(msg), osaQuote("backfill"))
			_ = exec.CommandContext(ctx, path, "-e", script).Run()
		}
	case "linux":
		// No GUI session (headless / CI / bare SSH): skip rather than burn the
		// timeout waiting on a notification daemon that isn't there.
		if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
			return
		}
		if path, err := exec.LookPath("notify-send"); err == nil {
			_ = exec.CommandContext(ctx, path, "backfill", msg).Run()
		}
	}
}

// notifyMsg builds the notification text. It uses a PLAIN space-join of the args
// (not displayCommand's %q quoting, whose backslashes collide with osaQuote and
// silently break the AppleScript), and strips control characters. osaQuote then
// safely escapes it for AppleScript; notify-send receives it as a single argv.
func notifyMsg(rec Receipt) string {
	icon := "✓"
	if !rec.OK {
		icon = "✗"
	}
	cmd := sanitizeNotify(strings.Join(rec.Args, " "))
	msg := fmt.Sprintf("%s %s · %s", icon, cmd, receiptDuration(rec.Seconds))
	if rec.Count != "" {
		msg += " · " + rec.Count
	}
	if !rec.OK {
		msg += fmt.Sprintf(" · exit %d", rec.Exit)
	}
	msg += fmt.Sprintf(" · ~$%.4f earned", float64(rec.EstMicros)/1e6)
	return msg
}

func sanitizeNotify(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\x00", "")
	return s
}

// osaQuote returns an AppleScript string literal, escaping backslashes and
// double quotes so command names or args can never inject AppleScript.
func osaQuote(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	return "\"" + s + "\""
}
