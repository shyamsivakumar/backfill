package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const blockStart = "# >>> backfill >>>"
const blockEnd = "# <<< backfill <<<"

// legacy markers from the alias-based releases, stripped on uninit/init.
const legacyStart = "# >>> backfill aliases >>>"
const legacyEnd = "# <<< backfill aliases <<<"

// defaultWrap is the curated set of slow, watch-worthy commands seeded on a
// plain `bf init`, so one command sets up earning across the tools developers
// actually wait on. Extend it with `bf wrap`, or wrap every non-interactive
// command on PATH with `bf init --all`.
var defaultWrap = []string{
	"dbt", "sqlmesh", "bq", "snowsql", "spark-submit",
	"cargo", "docker", "gradle", "xcodebuild",
	"make", "terraform", "pulumi", "bazel",
	"npm", "pnpm", "yarn", "pip", "poetry", "uv",
	"mvn", "pytest", "tox", "go",
	"droid",
}

// spinAgents get a `bf spin <cmd>` shim (ad injected into their live spinner)
// instead of the footer wrapper. Only agents that repaint the spinner as one
// contiguous line belong here — verified for Factory droid.
var spinAgents = map[string]bool{"droid": true}

var toolNameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._+-]*$`)

// noWrap are commands `bf init --all` skips: interactive REPLs, editors, pagers,
// shells, monitors, and trivially-instant utilities. The runtime alt-screen
// guard already keeps the footer off full-screen apps, but there is no reason to
// shim something you never wait on.
var noWrap = toSet([]string{
	"bf", "droid", "claude", "codex", "gemini",
	"sh", "bash", "zsh", "fish", "dash", "csh", "tcsh", "ksh", "nu", "xonsh",
	"vi", "vim", "nvim", "nano", "emacs", "pico", "ed", "micro", "helix", "hx", "code", "subl", "man",
	"less", "more", "most", "w3m", "lynx",
	"python", "python2", "python3", "ipython", "bpython", "node", "deno", "bun", "irb", "pry",
	"ghci", "iex", "psql", "mysql", "mariadb", "sqlite3", "redis-cli", "mongo", "mongosh",
	"clickhouse-client", "duckdb", "julia", "scala", "clj", "lua", "php", "R",
	"tmux", "screen", "zellij", "top", "htop", "btop", "btm", "glances", "watch",
	"ssh", "telnet", "ftp", "sftp", "mosh", "nc", "ncat", "gdb", "lldb", "sudo", "doas", "su",
	"ls", "cd", "pwd", "echo", "printf", "cat", "head", "tail", "cp", "mv", "rm",
	"mkdir", "rmdir", "touch", "ln", "chmod", "chown", "which", "type", "env",
	"true", "false", "test", "basename", "dirname", "sleep", "clear", "reset",
	"tput", "stty", "whoami", "id", "date", "uname", "hostname", "seq", "yes", "tee",
})

func toSet(xs []string) map[string]bool {
	m := make(map[string]bool, len(xs))
	for _, x := range xs {
		m[x] = true
	}
	return m
}

func shimDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".backfill", "shims")
}

// deshimPath drops the backfill shim dir from PATH for this process and its
// children, so resolving the wrapped command finds the real binary instead of
// recursing back into the shim.
func deshimPath() {
	dir := shimDir()
	parts := strings.Split(os.Getenv("PATH"), string(os.PathListSeparator))
	kept := parts[:0]
	for _, p := range parts {
		if p != dir {
			kept = append(kept, p)
		}
	}
	os.Setenv("PATH", strings.Join(kept, string(os.PathListSeparator)))
}

func effectiveWrap(cfg *Config) []string {
	if len(cfg.Wrap) > 0 {
		return cfg.Wrap
	}
	return defaultWrap
}

func pathBlock() string {
	return blockStart + "\n" +
		"export PATH=\"$HOME/.backfill/shims:$PATH\"\n" +
		blockEnd
}

func shimScript(bf, cmd string) string {
	if spinAgents[cmd] {
		return fmt.Sprintf("#!/bin/sh\nexec \"%s\" spin %s \"$@\"\n", bf, cmd)
	}
	return fmt.Sprintf("#!/bin/sh\nexec \"%s\" %s \"$@\"\n", bf, cmd)
}

func writeShims(list []string) error {
	dir := shimDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	bf, err := os.Executable()
	if err != nil || bf == "" {
		bf = "bf"
	}
	for _, c := range list {
		if !toolNameRe.MatchString(c) {
			continue
		}
		p := filepath.Join(dir, c)
		if err := os.WriteFile(p, []byte(shimScript(bf, c)), 0o755); err != nil {
			return err
		}
	}
	return nil
}

// removeShims clears every shim in the dir (backfill owns it) and removes it.
func removeShims() {
	dir := shimDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			os.Remove(filepath.Join(dir, e.Name()))
		}
	}
	os.Remove(dir)
}

// discoverTools scans every PATH dir for executables, minus the shim dir and the
// noWrap set, returning a sorted unique list.
func discoverTools() []string {
	deshimPath()
	seen := map[string]bool{}
	for _, dir := range strings.Split(os.Getenv("PATH"), string(os.PathListSeparator)) {
		if dir == "" || dir == shimDir() {
			continue
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			name := e.Name()
			if seen[name] || noWrap[name] || !toolNameRe.MatchString(name) {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			if info.IsDir() || info.Mode()&0o111 == 0 {
				continue
			}
			seen[name] = true
		}
	}
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func writeRCBlocks(remove bool) []string {
	home, _ := os.UserHomeDir()
	var changed []string
	for _, rc := range []string{".zshrc", ".bashrc"} {
		p := filepath.Join(home, rc)
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		s := stripBlock(string(b))
		if !remove {
			s = strings.TrimRight(s, "\n") + "\n\n" + pathBlock() + "\n"
		}
		if s != string(b) {
			if err := os.WriteFile(p, []byte(s), 0o644); err == nil {
				changed = append(changed, rc)
			}
		}
	}
	return changed
}

func stripMarked(s, start, end string) string {
	a := strings.Index(s, start)
	b := strings.Index(s, end)
	if a == -1 || b == -1 || b < a {
		return s
	}
	tail := strings.TrimPrefix(s[b+len(end):], "\n")
	return strings.TrimRight(s[:a], "\n") + "\n" + tail
}

func stripBlock(s string) string {
	s = stripMarked(s, blockStart, blockEnd)
	s = stripMarked(s, legacyStart, legacyEnd)
	return s
}

func cmdInit(extra []string, all bool) int {
	cfg := loadConfig()
	var list []string
	if all {
		list = discoverTools()
		fmt.Printf("Found %d wrappable commands on PATH (interactive and instant tools excluded).\n", len(list))
	} else {
		list = append([]string{}, effectiveWrap(cfg)...)
		list = mergeTools(list, extra)
	}

	fmt.Printf("This wraps these commands so plain `dbt run` and friends earn while you watch them run:\n\n  %s\n\n",
		previewList(list))
	fmt.Printf("It installs pass-through shims in %s and adds this to your shell rc:\n\n%s\n\n",
		shimDir(), pathBlock())
	if !confirm("Proceed? [y/N] ") {
		fmt.Println("aborted")
		return 0
	}

	if err := writeShims(list); err != nil {
		fmt.Fprintf(os.Stderr, "bf: could not install shims: %v\n", err)
		return 1
	}
	cfg.Wrap = list
	saveConfig(cfg)

	changed := writeRCBlocks(false)
	if len(changed) == 0 {
		fmt.Println("shims installed, but no shell rc was found to add to PATH. Add this yourself:\n  " + pathBlock())
		return 0
	}
	fmt.Printf("backfill set up %d commands in %s — restart your shell or run: source ~/%s\n",
		len(list), strings.Join(changed, ", "), changed[0])
	return 0
}

func cmdUninit() int {
	removeShims()
	cfg := loadConfig()
	cfg.Wrap = nil
	saveConfig(cfg)
	changed := writeRCBlocks(true)
	if len(changed) == 0 {
		fmt.Println("nothing to change")
		return 0
	}
	fmt.Printf("backfill removed from %s — restart your shell or run: source ~/%s\n",
		strings.Join(changed, ", "), changed[0])
	return 0
}

func cmdWrap(tools []string) int {
	if len(tools) == 0 {
		fmt.Println("usage: bf wrap <command>...")
		return 2
	}
	cfg := loadConfig()
	list := append([]string{}, effectiveWrap(cfg)...)
	added := []string{}
	for _, t := range tools {
		if !toolNameRe.MatchString(t) {
			fmt.Printf("skipping invalid command name: %q\n", t)
			continue
		}
		if !contains(list, t) {
			list = append(list, t)
			added = append(added, t)
		}
	}
	cfg.Wrap = list
	saveConfig(cfg)
	if err := writeShims(list); err != nil {
		fmt.Fprintf(os.Stderr, "bf: could not install shims: %v\n", err)
		return 1
	}
	if len(added) == 0 {
		fmt.Println("already wrapped; nothing to add")
		return 0
	}
	fmt.Printf("now wrapping: %s\n", strings.Join(added, ", "))
	fmt.Println("restart your shell or run: source ~/.zshrc")
	return 0
}

func cmdUnwrap(tools []string) int {
	if len(tools) == 0 {
		fmt.Println("usage: bf unwrap <command>...")
		return 2
	}
	cfg := loadConfig()
	list := effectiveWrap(cfg)
	drop := toSet(tools)
	kept := []string{}
	removed := []string{}
	for _, t := range list {
		if drop[t] {
			removed = append(removed, t)
			os.Remove(filepath.Join(shimDir(), t))
			continue
		}
		kept = append(kept, t)
	}
	cfg.Wrap = kept
	saveConfig(cfg)
	if len(removed) == 0 {
		fmt.Println("none of those were wrapped")
		return 0
	}
	fmt.Printf("stopped wrapping: %s\n", strings.Join(removed, ", "))
	return 0
}

func mergeTools(base, extra []string) []string {
	for _, t := range extra {
		if toolNameRe.MatchString(t) && !contains(base, t) {
			base = append(base, t)
		}
	}
	return base
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func previewList(list []string) string {
	if len(list) <= 24 {
		return strings.Join(list, ", ")
	}
	return strings.Join(list[:24], ", ") + fmt.Sprintf(", … (+%d more)", len(list)-24)
}

func confirm(prompt string) bool {
	fmt.Print(prompt)
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	line = strings.ToLower(strings.TrimSpace(line))
	return line == "y" || line == "yes"
}
