package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/term"
)

// collapseAlertRe matches lines that look like an error or failure even after
// ANSI stripping. These pass through the collapsed line so a failing run is
// never silent. Empty / whitespace-only lines are not alerts.
var collapseAlertRe = regexp.MustCompile(`(?i)\b(error|errors|failed|failure|fatal|panic|cannot|unable|exception|denied|refused|traceback)\b`)

// How long each item (ad, trending content, earnings) holds the collapsed line
// before the next one rotates in.
const collapseRotateSeconds = 6

// runCollapsed runs any wrapped command with stdout+stderr piped through a
// single in-place redrawing line that rotates through ads, trending content,
// and the running lifetime-earnings tally. Routine output is suppressed; alert
// lines pass through, and on a non-zero exit the captured tail is flushed so
// failures stay debuggable. Mirrors dbt's collapse behavior, generalized.
func runCollapsed(cfg *Config, bin string, args []string) (int, Ad, int) {
	pr, pw, err := os.Pipe()
	if err != nil {
		return runPlain(bin, args), Ad{}, 0
	}
	cmd := exec.Command(bin, args[1:]...)
	cmd.Stdout = pw
	cmd.Stderr = pw
	cmd.Stdin = os.Stdin // forward stdin so prompts still receive input
	if serr := cmd.Start(); serr != nil {
		pr.Close()
		pw.Close()
		return runPlain(bin, args), Ad{}, 0
	}
	pw.Close() // parent drops its write end; scanner sees EOF when the child exits

	r := &collapseRenderer{cfg: cfg, cmd: args[0], start: time.Now()}
	r.fill(cfg) // seed the rotation pool and keep filling it in the background

	// Clear the collapsed line and forward the signal to the child so Ctrl-C
	// leaves a clean terminal instead of a stranded ad line or an orphan.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT)
	go func() {
		s, ok := <-sigs
		if !ok {
			return
		}
		r.finish()
		if cmd.Process != nil {
			cmd.Process.Signal(s)
		}
		if sig, ok := s.(syscall.Signal); ok {
			os.Exit(128 + int(sig))
		}
		os.Exit(1)
	}()
	defer signal.Stop(sigs)

	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { // advance the spinner and rotate items even while output is quiet
		defer wg.Done()
		t := time.NewTicker(time.Second)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				r.draw()
			}
		}
	}()

	r.scan(pr)
	pr.Close()
	close(stop)
	wg.Wait()

	exit := 0
	if werr := cmd.Wait(); werr != nil {
		exit = commandExitCode(werr)
	}
	r.finish()

	// On failure, flush the captured tail so a collapsed run stays debuggable —
	// the user sees what broke instead of one cleared line.
	if exit != 0 {
		for _, line := range r.tail() {
			fmt.Fprintln(os.Stdout, line)
		}
	}

	primary, _ := r.billable()
	secs := int(time.Since(r.start).Seconds())
	if secs >= minBillableSeconds && primary.ID != "" {
		reportImpression(cfg, primary, args[0], secs)
	}
	return exit, primary, secs
}

type collapseRenderer struct {
	cfg   *Config
	cmd   string
	start time.Time

	// renderMu serializes the on-screen draw state (frame/drawn/lastDraw) and the
	// terminal writes themselves, since draw() runs from both the ticker and the
	// scan goroutine. Lock order is always renderMu before mu.
	renderMu sync.Mutex
	frame    int
	drawn    bool
	lastDraw time.Time

	mu    sync.Mutex
	items []Ad // rotation pool: ads + trending content (+ an earnings entry)
	buf   []string
}

// fill seeds the rotation with one ad synchronously (so the line has something
// immediately) and fetches a few more plus the earnings entry in the
// background, so starting the command is never blocked on the network.
func (r *collapseRenderer) fill(cfg *Config) {
	first := fetchAd(cfg, r.cmd)
	r.mu.Lock()
	r.items = []Ad{first}
	earned := first.EarnedMicros
	r.mu.Unlock()

	go func() {
		ads := fetchAdsConcurrent(cfg, r.cmd, 4)
		r.mu.Lock()
		defer r.mu.Unlock()
		for _, ad := range ads {
			if ad.EarnedMicros > earned {
				earned = ad.EarnedMicros
			}
			dup := false
			for _, existing := range r.items {
				if existing.ID == ad.ID {
					dup = true
					break
				}
			}
			if !dup {
				r.items = append(r.items, ad)
			}
		}
		if earned > 0 {
			r.items = append(r.items, Ad{
				ID:   "earnings",
				Text: fmt.Sprintf("$%.2f earned · backfill", float64(earned)/1e6),
				URL:  cfg.APIBase,
			})
		}
	}()
}

// current returns the item that should hold the line right now, rotating every
// collapseRotateSeconds.
func (r *collapseRenderer) current() Ad {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.items) == 0 {
		return Ad{}
	}
	slot := int(time.Since(r.start).Seconds()) / collapseRotateSeconds
	return r.items[slot%len(r.items)]
}

// billable returns the first real ad in the pool (not trending content, not the
// earnings entry) for honest single-impression accounting.
func (r *collapseRenderer) billable() (Ad, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, ad := range r.items {
		if isHouseContentID(ad.ID) {
			continue
		}
		return ad, true
	}
	return Ad{}, false
}

func (r *collapseRenderer) tail() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.buf))
	copy(out, r.buf)
	return out
}

func (r *collapseRenderer) scan(out io.Reader) {
	drainLines(out, r.handle)
}

func (r *collapseRenderer) handle(line string) {
	plain := stripANSI(line)

	r.mu.Lock()
	r.buf = append(r.buf, line)
	// Amortized O(1) trim: compact back to the last 400 lines only once the slack
	// reaches 400, so a chatty build doesn't reallocate on every line.
	if len(r.buf) > 800 {
		n := copy(r.buf, r.buf[len(r.buf)-400:])
		r.buf = r.buf[:n]
	}
	r.mu.Unlock()

	if strings.TrimSpace(plain) != "" && collapseAlertRe.MatchString(plain) {
		r.passthrough(line)
		return
	}
	r.draw()
}

// passthrough clears the live line, prints the real line, then redraws the
// collapsed line beneath it so the rotation stays visible.
func (r *collapseRenderer) passthrough(line string) {
	r.renderMu.Lock()
	defer r.renderMu.Unlock()
	if r.drawn {
		fmt.Fprint(os.Stdout, "\r\x1b[2K")
		r.drawn = false
	}
	fmt.Fprintln(os.Stdout, line)
	r.drawLocked()
}

func (r *collapseRenderer) draw() {
	r.renderMu.Lock()
	defer r.renderMu.Unlock()
	r.drawLocked()
}

// drawLocked renders the collapsed line. Caller must hold renderMu.
func (r *collapseRenderer) drawLocked() {
	now := time.Now()
	if r.drawn && now.Sub(r.lastDraw) < 80*time.Millisecond {
		return
	}
	r.lastDraw = now
	r.frame = (r.frame + 1) % len(dbtSpinFrames)

	item := r.current()
	if item.ID == "" {
		return
	}

	cols := 80
	if c, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && c > 0 {
		cols = c
	}

	left := fmt.Sprintf("%c %s · %s", dbtSpinFrames[r.frame], r.cmd, formatElapsed(time.Since(r.start)))
	text := item.Text
	if vis := visibleLen(left) + len("  ad · ") + len([]rune(text)); vis <= cols {
		line := fmt.Sprintf("\x1b[2m%s\x1b[0m  \x1b]8;;%s\x07\x1b[33mad · %s\x1b[0m\x1b]8;;\x07",
			left, r.link(item), text)
		fmt.Fprint(os.Stdout, "\r\x1b[2K"+line)
		r.drawn = true
		return
	}

	max := cols - visibleLen(left) - len("  ad · ")
	if max < 4 {
		fmt.Fprint(os.Stdout, "\r\x1b[2K\x1b[2m"+left+"\x1b[0m")
		r.drawn = true
		return
	}
	runes := []rune(text)
	if len(runes) > max {
		runes = append(runes[:max-1], '…')
	}
	line := fmt.Sprintf("\x1b[2m%s\x1b[0m  \x1b[33mad · %s\x1b[0m", left, string(runes))
	fmt.Fprint(os.Stdout, "\r\x1b[2K"+line)
	r.drawn = true
}

// link routes a real ad through the /r/ click tracker; the earnings entry links
// straight to the site so it is never billed as a click.
func (r *collapseRenderer) link(item Ad) string {
	if item.ID == "earnings" {
		return item.URL
	}
	return fmt.Sprintf("%s/r/%s?d=%s", r.cfg.APIBase, item.ID, r.cfg.DeviceID)
}

func (r *collapseRenderer) finish() {
	r.renderMu.Lock()
	defer r.renderMu.Unlock()
	if r.drawn {
		fmt.Fprint(os.Stdout, "\r\x1b[2K")
		r.drawn = false
	}
}

// formatElapsed renders a duration as mm:ss.
func formatElapsed(d time.Duration) string {
	total := int(d.Seconds())
	if total < 0 {
		total = 0
	}
	return fmt.Sprintf("%d:%02d", total/60, total%60)
}

// isInteractiveCommand flags commands that prompt or paint a full screen, where
// piping output through the collapsed line would hide the prompt and appear to
// hang. These run plain (no collapsed line, no footer — nothing extra), which
// still honors "no separate status line anywhere" while staying usable.
func isInteractiveCommand(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case "vim", "vi", "nvim", "nano", "emacs", "less", "more", "top", "htop", "btop",
		"ssh", "sudo", "gh", "aws", "gcloud", "heroku", "kubectl",
		"firebase", "supabase", "vercel", "netlify", "fly", "flyctl", "railway",
		"ipython", "jupyter", "mvn",
		"psql", "mysql", "mongosh", "redis-cli", "sqlite3", "python", "python3", "node", "irb":
		return true
	case "git":
		// interactive rebase, patch-add, conflict-resolving merges, editor commits.
		for _, a := range args[1:] {
			switch a {
			case "rebase", "add", "commit", "merge", "cherry-pick", "revert", "stash", "bisect":
				return true
			}
			if !strings.HasPrefix(a, "-") {
				return false
			}
		}
	case "terraform", "pulumi":
		for _, a := range args[1:] {
			switch a {
			case "apply", "destroy", "import", "console", "up", "refresh", "login":
				return true
			}
		}
	case "docker":
		for _, a := range args[1:] {
			if a == "-it" || a == "-ti" || a == "exec" || a == "attach" || a == "run" || a == "login" {
				return true
			}
		}
	case "npx", "pnpx", "bunx":
		// project scaffolders (create-next-app, create-vite, …) prompt interactively.
		for _, a := range args[1:] {
			if a == "create" || strings.HasPrefix(a, "create-") {
				return true
			}
			if !strings.HasPrefix(a, "-") {
				return false
			}
		}
	case "npm", "pnpm", "yarn", "bun":
		// install/ci/build/test collapse fine; init/create/login/publish prompt.
		for _, a := range args[1:] {
			switch a {
			case "init", "create", "login", "adduser", "publish", "logout", "link":
				return true
			}
		}
	}
	return false
}
