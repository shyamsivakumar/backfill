package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	runLogMaxLines = 2000
	runLogMaxBytes = 512 * 1024
)

type Receipt struct {
	Cmd         string   `json:"cmd"`
	Args        []string `json:"args"`
	StartedAt   int64    `json:"startedAt"`
	Seconds     int      `json:"seconds"`
	Exit        int      `json:"exit"`
	OK          bool     `json:"ok"`
	Count       string   `json:"count"`
	Checkpoints []string `json:"checkpoints"`
	EstMicros   int64    `json:"estMicros"`
	LinesTotal  int      `json:"linesTotal"`
	LinesFolded int      `json:"linesFolded"`
}

// runLog captures a wrapped command's output (bounded ring) and the checkpoint
// lines that passed through, so a receipt + full log can be written at the end.
type runLog struct {
	mu               sync.Mutex
	lines            []string
	checkpoints      []string
	linesTotal       int
	checkpointsTotal int
}

func (l *runLog) line(s string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.linesTotal++
	l.lines = append(l.lines, s)
	if len(l.lines) > runLogMaxLines {
		copy(l.lines, l.lines[len(l.lines)-runLogMaxLines:])
		l.lines = l.lines[:runLogMaxLines]
	}
	// Single linear trim by bytes: keep the newest lines that fit the budget.
	// (Not a loop-of-scans — that was O(n^2) and stalled the drain goroutine.)
	if logBytes(l.lines) > runLogMaxBytes {
		total, start := 0, len(l.lines)
		for start > 1 {
			sz := len(l.lines[start-1]) + 1
			if total+sz > runLogMaxBytes {
				break
			}
			total += sz
			start--
		}
		copy(l.lines, l.lines[start:])
		l.lines = l.lines[:len(l.lines)-start]
	}
}

func (l *runLog) checkpoint(s string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.checkpointsTotal++
	l.checkpoints = append(l.checkpoints, s)
	if len(l.checkpoints) > 20 {
		copy(l.checkpoints, l.checkpoints[len(l.checkpoints)-20:])
		l.checkpoints = l.checkpoints[:20]
	}
}

func logBytes(lines []string) int {
	n := 0
	for _, line := range lines {
		n += len(line) + 1
	}
	return n
}

func finalizeReceipt(rl *runLog, cmd string, args []string, start time.Time, exit int, count string) {
	if rl == nil {
		rl = &runLog{}
	}

	seconds := int(time.Since(start).Seconds())
	if seconds < 0 {
		seconds = 0
	}

	rl.mu.Lock()
	lines := append([]string(nil), rl.lines...)
	checkpoints := append([]string(nil), rl.checkpoints...)
	linesTotal := rl.linesTotal
	checkpointsTotal := rl.checkpointsTotal
	rl.mu.Unlock()

	linesFolded := linesTotal - checkpointsTotal
	if linesFolded < 0 {
		linesFolded = 0
	}

	rec := Receipt{
		Cmd:         cmd,
		Args:        append([]string(nil), args...),
		StartedAt:   start.Unix(),
		Seconds:     seconds,
		Exit:        exit,
		OK:          exit == 0,
		Count:       count,
		Checkpoints: checkpoints,
		EstMicros:   estimatedEarnedMicros(time.Duration(seconds) * time.Second),
		LinesTotal:  linesTotal,
		LinesFolded: linesFolded,
	}

	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".backfill")
	_ = os.MkdirAll(dir, 0o700)

	if b, err := json.MarshalIndent(rec, "", "  "); err == nil {
		_ = os.WriteFile(filepath.Join(dir, "last-run.json"), append(b, '\n'), 0o600)
	}

	var log []byte
	for _, line := range lines {
		log = append(log, line...)
		log = append(log, '\n')
	}
	_ = os.WriteFile(filepath.Join(dir, "last-run.log"), log, 0o600)

	printReceiptLine(rec)
}

// printReceiptLine prints the one-line end-of-run receipt: a colored status, the
// rest dim. The status is kept outside the dim wrapper so its color survives.
func printReceiptLine(r Receipt) {
	status := "\x1b[32m✓\x1b[0m"
	action := "bf last"
	if !r.OK {
		status = "\x1b[31m✗\x1b[0m"
		action = "bf logs last"
	}

	rest := fmt.Sprintf(" %s · %s", displayCommand(r.Args), receiptDuration(r.Seconds))
	if r.Count != "" {
		rest += " · " + r.Count
	}
	if !r.OK {
		rest += fmt.Sprintf(" · exit %d", r.Exit)
	}
	rest += fmt.Sprintf(" · ~$%.4f earned · %s", float64(r.EstMicros)/1e6, action)

	fmt.Printf("%s\x1b[2m%s\x1b[0m\n", status, rest)
}

func cmdLast() {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".backfill", "last-run.json")

	b, err := os.ReadFile(path)
	if err != nil {
		fmt.Println("no runs yet")
		return
	}

	var r Receipt
	if err := json.Unmarshal(b, &r); err != nil {
		fmt.Println("no runs yet")
		return
	}

	status := "success"
	if !r.OK {
		status = fmt.Sprintf("failed (exit %d)", r.Exit)
	}

	fmt.Printf("status:      %s\n", status)
	fmt.Printf("command:     %s\n", displayCommand(r.Args))
	fmt.Printf("duration:    %s\n", receiptDuration(r.Seconds))
	fmt.Printf("est earned:  ~$%.4f\n", float64(r.EstMicros)/1e6)
	if r.Count != "" {
		fmt.Printf("count:       %s\n", r.Count)
	}
	if len(r.Checkpoints) > 0 {
		fmt.Println("checkpoints:")
		for _, cp := range r.Checkpoints {
			fmt.Printf("  - %s\n", cp)
		}
	}
	fmt.Println("logs:        bf logs last")
}

func cmdLogs(args []string) {
	if len(args) == 0 || args[0] != "last" {
		fmt.Fprintln(os.Stderr, "usage: bf logs last")
		return
	}

	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".backfill", "last-run.log")

	b, err := os.ReadFile(path)
	if err != nil {
		fmt.Println("no logs yet")
		return
	}
	fmt.Print(string(b))
}

func displayCommand(args []string) string {
	if len(args) == 0 {
		return ""
	}
	quote := func(a string) string {
		for _, r := range a {
			if r == ' ' || r == '\t' || r == '"' || r == '\'' {
				return fmt.Sprintf("%q", a)
			}
		}
		return a
	}
	out := quote(args[0])
	for _, a := range args[1:] {
		out += " " + quote(a)
	}
	return out
}

func receiptDuration(seconds int) string {
	if seconds < 0 {
		seconds = 0
	}
	h := seconds / 3600
	m := (seconds % 3600) / 60
	s := seconds % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	if m == 0 {
		return fmt.Sprintf("%ds", s)
	}
	return fmt.Sprintf("%dm%02ds", m, s)
}
