package main

import (
	"strings"
	"testing"
	"time"
)

// TestRunLogStaysBounded feeds far more than the caps and asserts the buffer
// stays within both the line and byte limits (the trim must not be O(n^2) or
// unbounded — it ran in the hot drain path).
func TestRunLogStaysBounded(t *testing.T) {
	l := &runLog{}
	big := strings.Repeat("x", 300)
	done := make(chan struct{})
	go func() {
		for i := 0; i < 50000; i++ {
			l.line(big)
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("runLog.line did not keep up — trim is too slow")
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.lines) > runLogMaxLines {
		t.Errorf("lines over cap: %d > %d", len(l.lines), runLogMaxLines)
	}
	if b := logBytes(l.lines); b > runLogMaxBytes {
		t.Errorf("bytes over cap: %d > %d", b, runLogMaxBytes)
	}
	if l.linesTotal != 50000 {
		t.Errorf("linesTotal = %d, want 50000", l.linesTotal)
	}
}

// TestCheckpointsTotalTracksBeyondCap verifies linesFolded math uses the true
// checkpoint count, not the capped (20) retained slice.
func TestCheckpointsTotalTracksBeyondCap(t *testing.T) {
	l := &runLog{}
	for i := 0; i < 200; i++ {
		l.line("noise")
	}
	for i := 0; i < 50; i++ {
		l.checkpoint("milestone")
	}
	if len(l.checkpoints) != 20 {
		t.Errorf("retained checkpoints = %d, want 20", len(l.checkpoints))
	}
	if l.checkpointsTotal != 50 {
		t.Errorf("checkpointsTotal = %d, want 50", l.checkpointsTotal)
	}
}

func TestReceiptDurationFormats(t *testing.T) {
	cases := map[int]string{5: "5s", 65: "1m05s", 3661: "1:01:01"}
	for sec, want := range cases {
		if got := receiptDuration(sec); got != want {
			t.Errorf("receiptDuration(%d) = %q, want %q", sec, got, want)
		}
	}
}
