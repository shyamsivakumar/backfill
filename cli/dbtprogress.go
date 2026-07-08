package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mattn/go-runewidth"
	"golang.org/x/term"
)

// dbt prints one "N of M START …" line when a node begins and one
// "N of M OK/ERROR/SKIP …" line when it finishes. We collapse those into a single
// live progress line that carries the ad, and let through only the header, the
// final summary, and anything that looks like an error or warning.
var dbtProgressRe = regexp.MustCompile(`(\d+) of (\d+) (START|OK|ERROR|SKIP|PASS|WARN|FAIL)`)

// dbt pads the node name with a run of dots before the [RUN]/[SUCCESS] marker:
// "… model analytics.fct_orders .......... [RUN]". Capture the name before the dots.
var dbtModelRe = regexp.MustCompile(`([\w.]+)\s*\.{2,}`)

var dbtSpinFrames = []rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'}

var dbtProgressSubcmds = map[string]bool{
	"run": true, "build": true, "test": true, "seed": true, "snapshot": true,
}

var dbtLeadingFlagsWithValue = map[string]bool{
	"--profiles-dir": true,
	"--project-dir":  true,
	"--profile":      true,
	"--target":       true,
	"--vars":         true,
	"--log-level":    true,
	"--log-format":   true,
	"--log-path":     true,
	"--target-path":  true,
	"--state":        true,
	"--defer-state":  true,
}

// isDbtRunFamily reports whether this is a dbt invocation that emits per-node
// progress worth collapsing (run/build/test/seed/snapshot).
func isDbtRunFamily(args []string) bool {
	if len(args) < 2 || baseName(args[0]) != "dbt" {
		return false
	}
	return firstProgressSubcommand(args[1:], dbtProgressSubcmds, dbtLeadingFlagsWithValue)
}

func baseName(p string) string {
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		return p[i+1:]
	}
	return p
}

func firstProgressSubcommand(args []string, subcmds map[string]bool, flagsWithValue map[string]bool) bool {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			if i+1 < len(args) {
				return subcmds[args[i+1]]
			}
			return false
		}
		if strings.HasPrefix(a, "--") {
			name := a
			if eq := strings.IndexByte(name, '='); eq >= 0 {
				name = name[:eq]
			}
			if flagsWithValue[name] && !strings.Contains(a, "=") && i+1 < len(args) {
				i++
			}
			continue
		}
		if strings.HasPrefix(a, "-") && a != "-" {
			if flagsWithValue[a] && i+1 < len(args) {
				i++
			}
			continue
		}
		return subcmds[a]
	}
	return false
}

// runDbtProgress runs a dbt invocation with its routine per-node output collapsed
// into one live ad-carrying progress line. Errors, warnings, the version header,
// and the final summary still print. Falls back to the footer wrapper when stdout
// is not a TTY (CI, pipes) so scripted runs are untouched.
func runDbtProgress(cfg *Config, bin string, args []string) int {
	if !cfg.Enabled || !term.IsTerminal(int(os.Stdout.Fd())) {
		exit, _, _ := runWithFooter(cfg, bin, args)
		return exit
	}

	pr, pw, err := os.Pipe()
	if err != nil {
		return runPlain(bin, args)
	}
	cmd := exec.Command(bin, args[1:]...)
	cmd.Stdout = pw
	cmd.Stderr = pw
	if err := cmd.Start(); err != nil {
		pr.Close()
		pw.Close()
		return runPlain(bin, args)
	}
	pw.Close() // parent drops its write end; scanner sees EOF when the child exits

	rl := &runLog{}
	r := &dbtRenderer{cfg: cfg, rot: newAdRotator(cfg, args[0]), start: time.Now(), log: rl}

	// Advance the spinner and rotate the ad/content/earnings line even while a
	// model is mid-run and dbt is quiet.
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		t := time.NewTicker(time.Second)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				if r.started() {
					r.draw()
				}
			}
		}
	}()

	r.scan(pr)
	close(stop)
	wg.Wait()
	pr.Close()

	exit := 0
	if werr := cmd.Wait(); werr != nil {
		exit = commandExitCode(werr)
	}
	r.finish()

	secs := int(time.Since(r.start).Seconds())
	if secs >= minBillableSeconds {
		if ad := r.rot.billable(); ad.ID != "" {
			reportImpression(cfg, ad, args[0], secs)
		}
	}
	count := ""
	if r.total > 0 {
		count = fmt.Sprintf("%d models", r.total)
	} else if r.done > 0 {
		count = fmt.Sprintf("%d models", r.done)
	}
	finalizeReceipt(rl, args[0], args, r.start, exit, count)
	return exit
}

type dbtRenderer struct {
	cfg      *Config
	rot      *adRotator
	start    time.Time
	log      *runLog
	renderMu sync.Mutex
	total    int
	done     int
	current  string
	frame    int
	drawn    bool
	lastDraw time.Time
}

// started reports whether dbt has emitted a progress line yet, so the ticker
// doesn't draw a bare ad line before the run header.
func (r *dbtRenderer) started() bool {
	r.renderMu.Lock()
	defer r.renderMu.Unlock()
	return r.total > 0
}

func (r *dbtRenderer) scan(out io.Reader) {
	drainLines(out, r.handle)
}

func (r *dbtRenderer) handle(line string) {
	plain := stripANSI(line)
	if r.log != nil {
		r.log.line(line)
	}

	if m := dbtProgressRe.FindStringSubmatch(plain); m != nil {
		r.renderMu.Lock()
		defer r.renderMu.Unlock()
		if total, err := strconv.Atoi(m[2]); err == nil {
			r.total = total
		}
		switch m[3] {
		case "START":
			if mm := dbtModelRe.FindStringSubmatch(plain); mm != nil {
				r.current = mm[1]
			}
		case "OK", "PASS":
			r.done++
		case "SKIP":
			r.done++
		case "WARN":
			r.done++
			if r.log != nil {
				r.log.checkpoint(plain)
			}
			r.passthroughLocked(line)
			return
		case "ERROR", "FAIL":
			r.done++
			if r.log != nil {
				r.log.checkpoint(plain)
			}
			r.passthroughLocked(line) // failures stay visible
			return
		}
		r.drawLocked()
		return
	}

	if isDbtNoise(plain) {
		return
	}
	if r.log != nil {
		r.log.checkpoint(plain)
	}
	r.renderMu.Lock()
	defer r.renderMu.Unlock()
	r.passthroughLocked(line)
}

// passthroughLocked clears the live line, prints a real line (header, summary,
// error), then redraws the progress line beneath it. Caller holds renderMu.
func (r *dbtRenderer) passthroughLocked(line string) {
	if r.drawn {
		fmt.Fprint(os.Stdout, "\r\x1b[2K")
		r.drawn = false
	}
	fmt.Fprintln(os.Stdout, line)
	if r.total > 0 {
		r.drawLocked()
	}
}

func (r *dbtRenderer) draw() {
	r.renderMu.Lock()
	defer r.renderMu.Unlock()
	r.drawLocked()
}

// drawLocked renders the progress line. Caller holds renderMu.
func (r *dbtRenderer) drawLocked() {
	now := time.Now()
	if r.drawn && now.Sub(r.lastDraw) < 80*time.Millisecond {
		return
	}
	r.lastDraw = now
	r.frame = (r.frame + 1) % len(dbtSpinFrames)

	left := fmt.Sprintf("%c dbt %d/%d", dbtSpinFrames[r.frame], r.done, r.total)
	if r.current != "" {
		left += "  " + r.current
	}
	item := r.rot.current()
	fmt.Fprint(os.Stdout, composeLine(left, time.Since(r.start), item, r.rot.link(item)))
	r.drawn = true
}

func (r *dbtRenderer) finish() {
	r.renderMu.Lock()
	defer r.renderMu.Unlock()
	if r.drawn {
		fmt.Fprint(os.Stdout, "\r\x1b[2K")
		r.drawn = false
	}
}

func isDbtNoise(plain string) bool {
	t := strings.TrimSpace(plain)
	if t == "" {
		return true
	}
	// drop the per-node "Concurrency"/"Building catalog"-style chatter but keep
	// anything that reads like an error, warning, or the run summary.
	for _, keep := range []string{"Error", "error", "Failure", "fail", "Warning", "WARN",
		"Completed", "Done.", "Finished running", "Running with dbt", "Found ", "PASS=", "Database Error", "Compilation Error"} {
		if strings.Contains(t, keep) {
			return false
		}
	}
	// suppress routine progress decoration and timestamps-only lines
	return true
}

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]|\x1b\][^\x07]*\x07`)

func stripANSI(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

// visibleLen is the terminal column width of s after stripping ANSI, so layout
// arithmetic stays correct for CJK / emoji ads that occupy two columns per rune.
func visibleLen(s string) int {
	return runewidth.StringWidth(stripANSI(s))
}
