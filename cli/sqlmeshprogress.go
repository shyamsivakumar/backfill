package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"golang.org/x/term"
)

// sqlmesh prints a "Models needing backfill:" list ("* `model`: [...]") and then one
// "[a/b] model_name   [action]" line as each model executes. We collapse those into a
// single live ad-carrying line, like the dbt renderer, and let through the summary and
// any errors. The child runs on an os.Pipe (not a pty) so sqlmesh emits plain line
// output instead of its Rich live-progress UI, which would otherwise hide the ad.
var sqlmeshStepRe = regexp.MustCompile(`^\s*\[\d+/\d+\]\s+([\w.]+)`)
var sqlmeshBackfillRe = regexp.MustCompile("^\\s*\\*\\s+`([\\w.]+)`")
var sqlmeshElapsedRe = regexp.MustCompile(`^\s*\d+(\.\d+)?s\s*$`)

var sqlmeshProgressSubcmds = map[string]bool{"plan": true, "run": true}

// isSqlmeshRunFamily reports whether this is a sqlmesh invocation that emits per-model
// progress worth collapsing (plan/run).
func isSqlmeshRunFamily(args []string) bool {
	if len(args) < 2 || baseName(args[0]) != "sqlmesh" {
		return false
	}
	for _, a := range args[1:] {
		if strings.HasPrefix(a, "-") {
			continue
		}
		return sqlmeshProgressSubcmds[a]
	}
	return false
}

// runSqlmeshProgress runs a sqlmesh invocation with its routine per-model output
// collapsed into one live ad-carrying line. Errors and the summary still print. Falls
// back to the footer wrapper when stdout is not a TTY (CI, pipes).
func runSqlmeshProgress(cfg *Config, bin string, args []string) int {
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
	pw.Close()

	ad := fetchAd(cfg, args[0])
	r := &sqlmeshRenderer{cfg: cfg, ad: ad, start: time.Now()}
	r.scan(pr)
	pr.Close()

	exit := 0
	if werr := cmd.Wait(); werr != nil {
		exit = commandExitCode(werr)
	}
	r.finish()

	if secs := int(time.Since(r.start).Seconds()); secs >= minBillableSeconds {
		reportImpression(cfg, ad, args[0], secs)
	}
	return exit
}

type sqlmeshRenderer struct {
	cfg      *Config
	ad       Ad
	start    time.Time
	total    int
	done     int
	current  string
	frame    int
	drawn    bool
	lastDraw time.Time
}

func (r *sqlmeshRenderer) scan(out io.Reader) {
	sc := bufio.NewScanner(out)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		r.handle(sc.Text())
	}
}

func (r *sqlmeshRenderer) handle(line string) {
	plain := stripANSI(line)

	// a model executed
	if m := sqlmeshStepRe.FindStringSubmatch(plain); m != nil {
		r.current = m[1]
		r.done++
		r.draw()
		return
	}

	// "* `model`: [...]" under "Models needing backfill:" — count toward the total
	// and keep the line visible so the user sees what's being built.
	if m := sqlmeshBackfillRe.FindStringSubmatch(plain); m != nil {
		r.total++
		r.passthrough(line)
		return
	}

	if isSqlmeshNoise(plain) {
		return
	}
	r.passthrough(line)
}

func (r *sqlmeshRenderer) passthrough(line string) {
	if r.drawn {
		fmt.Fprint(os.Stdout, "\r\x1b[2K")
		r.drawn = false
	}
	fmt.Fprintln(os.Stdout, line)
	if r.total > 0 || r.done > 0 {
		r.draw()
	}
}

func (r *sqlmeshRenderer) draw() {
	now := time.Now()
	if r.drawn && now.Sub(r.lastDraw) < 80*time.Millisecond {
		return
	}
	r.lastDraw = now
	r.frame = (r.frame + 1) % len(dbtSpinFrames)

	cols := 80
	if c, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && c > 0 {
		cols = c
	}

	left := fmt.Sprintf("%c sqlmesh", dbtSpinFrames[r.frame])
	if r.total > 0 {
		done := r.done
		if done > r.total {
			done = r.total
		}
		left += fmt.Sprintf(" %d/%d", done, r.total)
	}
	if r.current != "" {
		left += "  " + r.current
	}

	adText := r.ad.Text
	line := fmt.Sprintf("\x1b[2m%s\x1b[0m  \x1b]8;;%s\x07\x1b[33mad · %s\x1b[0m\x1b]8;;\x07",
		left, r.adLink(), adText)

	if vis := visibleLen(left) + len("ad · ") + len([]rune(adText)) + 2; vis > cols {
		line = fmt.Sprintf("\x1b[2m%s\x1b[0m  \x1b[33mad · %s\x1b[0m", left, adText)
	}

	fmt.Fprint(os.Stdout, "\r\x1b[2K"+line)
	r.drawn = true
}

func (r *sqlmeshRenderer) adLink() string {
	return fmt.Sprintf("%s/r/%s?d=%s", r.cfg.APIBase, r.ad.ID, r.cfg.DeviceID)
}

func (r *sqlmeshRenderer) finish() {
	if r.drawn {
		fmt.Fprint(os.Stdout, "\r\x1b[2K")
		r.drawn = false
	}
}

func isSqlmeshNoise(plain string) bool {
	t := strings.TrimSpace(plain)
	if t == "" {
		return true
	}
	// keep failures and the meaningful summary lines
	for _, keep := range []string{"Error", "error", "Traceback", "Failed", "Failure",
		"Model batches executed", "Virtual layer updated", "Successfully Ran",
		"No changes", "No models are ready", "environment will be", "Models needing backfill"} {
		if strings.Contains(t, keep) {
			return false
		}
	}
	// drop python deprecation/future/user warnings and their source lines
	if strings.Contains(t, "Warning:") || strings.Contains(t, "warnings.warn") || strings.HasPrefix(t, "df[") {
		return true
	}
	// drop the Rich progress bar ("Executing model batches …", "Updating virtual layer …")
	if strings.Contains(t, "Executing model batches") || strings.Contains(t, "Updating virtual layer") {
		return true
	}
	// drop bare elapsed lines ("0.02s") and leftover progress-glyph/percent lines
	if sqlmeshElapsedRe.MatchString(t) || strings.Contains(t, "•") || strings.Contains(t, "━") || strings.Contains(t, "%") {
		return true
	}
	return false
}
