package main

import (
	"io"
	"strings"
	"testing"
	"time"
)

// TestDrainLinesNeverHangsOnHugeLine is the regression test for the deadlock:
// a child that emits megabytes with no newline must still be drained to EOF so
// it can never block on a full pipe. The old bufio.Scanner (1MB cap) hung here.
func TestDrainLinesNeverHangsOnHugeLine(t *testing.T) {
	pr, pw := io.Pipe()
	go func() {
		// 4 MiB with no newline, then a normal line, then EOF.
		pw.Write([]byte(strings.Repeat("A", 4*1024*1024)))
		pw.Write([]byte("\nlast line\n"))
		pw.Close()
	}()

	done := make(chan int, 1)
	go func() {
		n := 0
		drainLines(pr, func(string) { n++ })
		done <- n
	}()

	select {
	case n := <-done:
		if n < 2 {
			t.Fatalf("expected at least 2 lines (huge + last), got %d", n)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("drainLines did not reach EOF within 3s — pipe drain hung")
	}
}

// TestDrainLinesTruncatesButKeepsDraining verifies an overlong line is capped in
// what the renderer sees, while the trailing short line still arrives intact.
func TestDrainLinesTruncatesButKeepsDraining(t *testing.T) {
	pr, pw := io.Pipe()
	go func() {
		pw.Write([]byte(strings.Repeat("x", drainMaxLine*3)))
		pw.Write([]byte("\nok\n"))
		pw.Close()
	}()

	var lines []string
	drainLines(pr, func(s string) { lines = append(lines, s) })

	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %v", len(lines), lines)
	}
	if len([]rune(lines[0])) > drainMaxLine+1 { // +1 for the … marker
		t.Fatalf("first line not truncated: %d runes", len([]rune(lines[0])))
	}
	if lines[1] != "ok" {
		t.Fatalf("trailing line lost or corrupted: %q", lines[1])
	}
}
