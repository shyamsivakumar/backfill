package main

import (
	"bufio"
	"io"
)

// drainMaxLine caps how many bytes of a single line we keep in memory and hand
// to the renderer. The rest of an overlong line is still consumed from the pipe
// (so the child never blocks on a full pipe) but dropped from display.
const drainMaxLine = 16 * 1024

// drainLines reads rd to EOF, calling onLine for each newline-terminated line.
//
// It is the one hard guarantee that a wrapped command can never deadlock on
// output: the pipe is ALWAYS drained to EOF regardless of line length or
// content. A child that emits a multi-megabyte blob with no newline is read in
// bounded chunks (via ReadSlice, which never allocates unbounded), its display
// truncated to drainMaxLine, and the remaining bytes discarded while reading
// continues. No read error and no renderer behavior can stop the drain early.
//
// This replaces bufio.Scanner, whose 64KB/1MB token cap returns ErrTooLong and
// silently stops the loop, leaving the child blocked on write and cmd.Wait()
// hung forever.
func drainLines(rd io.Reader, onLine func(string)) {
	br := bufio.NewReaderSize(rd, 64*1024)
	var line []byte
	overlong := false

	keep := func(chunk []byte) {
		if overlong {
			return
		}
		if len(line)+len(chunk) <= drainMaxLine {
			line = append(line, chunk...)
			return
		}
		if room := drainMaxLine - len(line); room > 0 {
			line = append(line, chunk[:room]...)
		}
		overlong = true
	}

	emit := func() {
		s := string(line)
		if overlong {
			s += "…"
		}
		onLine(s)
		line = line[:0]
		overlong = false
	}

	for {
		chunk, err := br.ReadSlice('\n')
		if err == bufio.ErrBufferFull {
			// Long line, no newline yet: keep what fits, consume the rest.
			keep(chunk)
			continue
		}
		// Got a newline (err == nil) or hit EOF / a read error.
		if n := len(chunk); n > 0 {
			if err == nil && chunk[n-1] == '\n' {
				chunk = chunk[:n-1]
			}
			keep(chunk)
		}
		if err == nil {
			emit()
			continue
		}
		// EOF or any read error: flush a trailing partial line and stop. The
		// child has closed its write end (or died); nothing left to drain.
		if len(line) > 0 {
			emit()
		}
		return
	}
}
