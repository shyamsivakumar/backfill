package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
	"golang.org/x/term"
)

// Stable "interrupt" hints terminal agents print on the live processing line while
// the model works. Anchoring on these is robust: Codex cycles a random gerund verb,
// but always shows "Esc to interrupt"; Factory shows "Press ESC to stop". We inject
// the ad just before the anchor so it rides the processing line every frame.
var spinnerAnchors = [][]byte{
	[]byte("Esc to interrupt"),
	[]byte("Press ESC to stop"),
}

// Spinner verbs agents print in the verb slot of the processing line. Replacing the
// verb (rather than tacking the ad onto the parenthetical) puts the sponsored text
// where the eye lands. Codex's steady verb is "Working" plus a rotating gerund;
// Factory's is "Executing…"/"Streaming…".
var spinnerVerbs = [][]byte{
	[]byte("Executing…"), []byte("Executing..."),
	[]byte("Streaming…"), []byte("Streaming..."),
	[]byte("Working"),
}

// spinnerLeadRE distinguishes a live status frame from prose that happens to
// mention an interrupt shortcut. ANSI is stripped before matching, so a colored
// spinner glyph and verb still count as a leading gerund.
var spinnerLeadRE = regexp.MustCompile(`(?i)^[^[:alpha:]]*[[:alpha:]][[:alpha:]-]*ing(?:\.\.\.|…)?(?:[^[:alpha:]]|$)`)

type spinnerRewriter struct {
	ad      []byte
	active  bool
	lastHit time.Time
}

// transform injects the ad into the processing line, but only on frames that carry
// an interrupt anchor ("Esc to interrupt" / "Press ESC to stop"). Gating on the
// anchor means a stray "Working" in prose is never touched — only the live spinner.
// On a spinner frame it replaces the known verb with the ad; if the verb is an
// unknown rotating gerund, it injects the ad just before the anchor as a fallback.
func (r *spinnerRewriter) transform(b []byte) []byte {
	out := make([]byte, 0, len(b)+len(r.ad))
	start := 0
	changed := false
	for i, c := range b {
		if c != '\r' && c != '\n' {
			continue
		}
		frame, rewritten := r.transformFrame(b[start:i])
		out = append(out, frame...)
		out = append(out, c)
		changed = changed || rewritten
		start = i + 1
	}
	frame, rewritten := r.transformFrame(b[start:])
	out = append(out, frame...)
	changed = changed || rewritten
	if !changed {
		return b
	}
	return out
}

func (r *spinnerRewriter) transformFrame(frame []byte) ([]byte, bool) {
	anchorStart, _, ok := findSpinnerAnchor(frame)
	if !ok || !spinnerLeadRE.MatchString(stripANSI(string(frame[:anchorStart]))) {
		return frame, false
	}

	r.active = true
	r.lastHit = time.Now()
	if len(r.ad) == 0 || bytes.Contains(frame, r.ad) {
		return frame, false
	}

	if start, end, ok := findSpinnerVerb(frame[:anchorStart]); ok {
		out := make([]byte, 0, len(frame)-end+start+len(r.ad))
		out = append(out, frame[:start]...)
		out = append(out, r.ad...)
		out = append(out, frame[end:]...)
		return out, true
	}

	out := make([]byte, 0, len(frame)+len(r.ad)+2)
	out = append(out, frame[:anchorStart]...)
	out = append(out, r.ad...)
	out = append(out, ' ', ' ')
	out = append(out, frame[anchorStart:]...)
	return out, true
}

func findSpinnerAnchor(frame []byte) (int, int, bool) {
	lower := bytes.ToLower(frame)
	bestStart := -1
	bestLen := 0
	for _, anchor := range spinnerAnchors {
		needle := bytes.ToLower(anchor)
		if start := bytes.Index(lower, needle); start >= 0 && (bestStart < 0 || start < bestStart) {
			bestStart = start
			bestLen = len(needle)
		}
	}
	if bestStart < 0 {
		return 0, 0, false
	}
	return bestStart, bestStart + bestLen, true
}

func findSpinnerVerb(framePrefix []byte) (int, int, bool) {
	bestStart := -1
	bestLen := 0
	for _, verb := range spinnerVerbs {
		if start := bytes.Index(framePrefix, verb); start >= 0 && (bestStart < 0 || start < bestStart) {
			bestStart = start
			bestLen = len(verb)
		}
	}
	if bestStart < 0 {
		return 0, 0, false
	}
	return bestStart, bestStart + bestLen, true
}

func runWithRewrite(cfg *Config, bin string, args []string) int {
	if !cfg.Enabled || !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
		return runPlain(bin, args)
	}

	cols, rows, err := termSize()
	if err != nil {
		return runPlain(bin, args)
	}

	cmd := exec.Command(bin, args[1:]...)
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
	if err != nil {
		return runPlain(bin, args)
	}
	defer ptmx.Close()

	ad := fetchAd(cfg, args[0])
	rw := &spinnerRewriter{ad: spinnerAdBytes(ad)}

	// BF_CAPTURE=<path> dumps every raw PTY read (pre-rewrite) as NDJSON, so we can
	// see exactly how the agent paints its spinner and pick the right technique.
	var capFile *os.File
	chunkN := 0
	if p := os.Getenv("BF_CAPTURE"); p != "" {
		capFile, _ = os.Create(p)
		if capFile != nil {
			defer capFile.Close()
		}
	}

	winch := make(chan os.Signal, 1)
	signal.Notify(winch, syscall.SIGWINCH)
	go func() {
		for range winch {
			if c, r, err := termSize(); err == nil {
				pty.Setsize(ptmx, &pty.Winsize{Rows: uint16(r), Cols: uint16(c)})
			}
		}
	}()

	var oldState *term.State
	if old, err := term.MakeRaw(int(os.Stdin.Fd())); err == nil {
		oldState = old
	}
	go io.Copy(ptmx, os.Stdin)

	var mu sync.Mutex
	var visible time.Duration
	done := make(chan struct{})
	go func() {
		t := time.NewTicker(time.Second)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-t.C:
				mu.Lock()
				if rw.active && time.Since(rw.lastHit) < 3*time.Second {
					visible += time.Second
				}
				mu.Unlock()
			}
		}
	}()

	out := os.Stdout
	buf := make([]byte, 32*1024)
	for {
		n, rerr := ptmx.Read(buf)
		if n > 0 {
			mu.Lock()
			if capFile != nil {
				chunkN++
				fmt.Fprintf(capFile, "{\"chunk\":%d,\"len\":%d,\"hex\":\"%x\"}\n", chunkN, n, buf[:n])
			}
			out.Write(rw.transform(buf[:n]))
			mu.Unlock()
		}
		if rerr != nil {
			break
		}
	}
	close(done)
	signal.Stop(winch)

	if oldState != nil {
		term.Restore(int(os.Stdin.Fd()), oldState)
	}

	exit := 0
	if werr := cmd.Wait(); werr != nil {
		exit = commandExitCode(werr)
	}

	if secs := int(visible.Seconds()); secs >= minBillableSeconds {
		reportImpression(cfg, ad, args[0], secs)
	}
	return exit
}

// spinnerAdBytes builds the styled replacement: the "ad · " disclosure plus a short
// label the agent renders in place of its spinner verb. Uses the same lead-label +
// column cap as the Claude spinner so a verbose server description ("fd: a simple,
// fast alternative …") never pushes the agent's own status off the right edge.
func spinnerAdBytes(ad Ad) []byte {
	text := ad.SpinnerText
	if text == "" {
		text = ad.Text
	}
	label := capSpinnerVerb(spinnerLabel(stripControlChars(text)), spinnerVerbCols())
	return []byte(fmt.Sprintf("ad · %s", label))
}
