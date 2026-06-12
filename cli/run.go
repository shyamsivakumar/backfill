package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
	"golang.org/x/term"
)

const minBillableSeconds = 5

func runWrapped(args []string) int {
	bin, err := exec.LookPath(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "bf: %s: command not found\n", args[0])
		return 127
	}
	cfg := loadConfig()
	if !cfg.Enabled || !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
		return runPlain(bin, args)
	}
	return runWithFooter(cfg, bin, args)
}

func runPlain(bin string, args []string) int {
	cmd := exec.Command(bin, args[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return commandExitCode(err)
	}
	return 0
}

// runWithFooter gives the child a PTY one row shorter than the terminal,
// pins the scroll region to those rows, and owns the last row for the ad.
func runWithFooter(cfg *Config, bin string, args []string) int {
	cols, rows, err := termSize()
	if err != nil || rows < 5 {
		return runPlain(bin, args)
	}

	cmd := exec.Command(bin, args[1:]...)
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: uint16(rows - 1), Cols: uint16(cols)})
	if err != nil {
		return runPlain(bin, args)
	}
	defer ptmx.Close()

	out := os.Stdout
	var mu sync.Mutex
	var wg sync.WaitGroup
	var oldState *term.State
	raw := false

	f := &footer{cfg: cfg, cmd: args[0], cols: cols, rows: rows}
	f.ad = fetchAd(cfg, args[0])
	f.lastTick = time.Now()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT)
	go func() {
		s := <-sigs
		sig, _ := s.(syscall.Signal)
		mu.Lock()
		if raw {
			term.Restore(int(os.Stdin.Fd()), oldState)
			raw = false
		}
		fmt.Fprintf(out, "\x1b[r\x1b[%d;1H\x1b[2K\r", f.rows)
		ptmx.Close()
		if cmd.Process != nil {
			cmd.Process.Signal(sig)
		}
		os.Exit(128 + int(sig))
	}()

	// Scroll one line for safety, restrict scrolling to rows 1..rows-1,
	// park the cursor at the bottom of that region.
	mu.Lock()
	fmt.Fprintf(out, "\n\x1b[1;%dr\x1b[%d;1H", rows-1, rows-1)
	f.draw(out)
	mu.Unlock()

	winch := make(chan os.Signal, 1)
	winchDone := make(chan struct{})
	signal.Notify(winch, syscall.SIGWINCH)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-winchDone:
				return
			case <-winch:
				c, r, err := termSize()
				if err != nil || r < 5 {
					continue
				}
				mu.Lock()
				f.cols, f.rows = c, r
				pty.Setsize(ptmx, &pty.Winsize{Rows: uint16(r - 1), Cols: uint16(c)})
				fmt.Fprintf(out, "\x1b7\x1b[1;%dr\x1b8", r-1)
				f.draw(out)
				mu.Unlock()
			}
		}
	}()

	if term.IsTerminal(int(os.Stdin.Fd())) {
		mu.Lock()
		if old, err := term.MakeRaw(int(os.Stdin.Fd())); err == nil {
			oldState = old
			raw = true
		}
		mu.Unlock()
	}
	go io.Copy(ptmx, os.Stdin)

	done := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		t := time.NewTicker(time.Second)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-t.C:
				mu.Lock()
				f.accrue()
				f.draw(out)
				mu.Unlock()
			}
		}
	}()

	buf := make([]byte, 32*1024)
	for {
		n, rerr := ptmx.Read(buf)
		if n > 0 {
			mu.Lock()
			out.Write(buf[:n])
			f.draw(out)
			mu.Unlock()
		}
		if rerr != nil {
			break
		}
	}
	close(done)
	signal.Stop(winch)
	close(winchDone)
	wg.Wait()

	exit := 0
	if werr := cmd.Wait(); werr != nil {
		exit = commandExitCode(werr)
	}

	mu.Lock()
	if raw {
		term.Restore(int(os.Stdin.Fd()), oldState)
		raw = false
	}
	f.accrue()
	secs := int(f.visible.Seconds())
	fmt.Fprintf(out, "\x1b[r\x1b[%d;1H\x1b[2K\r", f.rows)
	mu.Unlock()
	signal.Stop(sigs)

	if secs >= minBillableSeconds {
		reportImpression(cfg, f.ad, args[0], secs)
	}
	return exit
}

func commandExitCode(err error) int {
	if ee, ok := err.(*exec.ExitError); ok {
		if ws, ok := ee.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			return 128 + int(ws.Signal())
		}
		return ee.ExitCode()
	}
	return 1
}

func termSize() (cols, rows int, err error) {
	return term.GetSize(int(os.Stdout.Fd()))
}

type footer struct {
	cfg      *Config
	cmd      string
	cols     int
	rows     int
	ad       Ad
	visible  time.Duration
	lastTick time.Time
}

// Accrue visible time in clamped deltas so a laptop suspend or stopped
// process contributes at most one tick's worth, not the whole gap.
func (f *footer) accrue() {
	now := time.Now()
	delta := now.Sub(f.lastTick)
	if delta > 2*time.Second {
		delta = 2 * time.Second
	}
	if delta > 0 {
		f.visible += delta
	}
	f.lastTick = now
}

func (f *footer) draw(out io.Writer) {
	max := f.cols - 4
	if max < 12 {
		return
	}
	text := []rune(f.ad.Text)
	if len(text) > max {
		text = append(text[:max-1], '…')
	}
	link := fmt.Sprintf("%s/r/%s?d=%s", f.cfg.APIBase, f.ad.ID, f.cfg.DeviceID)
	fmt.Fprintf(out,
		"\x1b7\x1b[%d;1H\x1b[2K\x1b[2mad\x1b[0m \x1b]8;;%s\x07\x1b[33m%s\x1b[0m\x1b]8;;\x07\x1b8",
		f.rows, link, string(text))
}
