package main

import (
	"strings"
	"testing"
)

func TestIsScaffoldCommand(t *testing.T) {
	tests := []struct {
		args []string
		want bool
	}{
		// positive: npm/pnpm/yarn/bun create and init
		{[]string{"npm", "create", "vite@latest"}, true},
		{[]string{"npm", "init", "."}, true},
		{[]string{"pnpm", "create", "next-app"}, true},
		{[]string{"yarn", "create", "react-app", "myapp"}, true},
		{[]string{"bun", "create", "next"}, true},

		// positive: npx create-* scaffolders
		{[]string{"npx", "create-react-app", "myapp"}, true},
		{[]string{"npx", "create-next-app", "--typescript"}, true},
		{[]string{"npx", "create-vite"}, true},

		// positive: bare create-* binaries
		{[]string{"create-react-app", "myapp"}, true},
		{[]string{"create-next-app", "."}, true},
		{[]string{"create-vite", "myproject"}, true},

		// positive: cargo new / init
		{[]string{"cargo", "new", "myproject"}, true},
		{[]string{"cargo", "init"}, true},

		// positive: framework scaffolders
		{[]string{"django-admin", "startproject", "mysite"}, true},
		{[]string{"rails", "new", "myapp"}, true},
		{[]string{"dotnet", "new", "webapi"}, true},

		// negative: npm install, run, etc.
		{[]string{"npm", "install"}, false},
		{[]string{"npm", "run", "build"}, false},
		{[]string{"npm", "test"}, false},
		{[]string{"npm"}, false},

		// negative: cargo build, test, etc.
		{[]string{"cargo", "build"}, false},
		{[]string{"cargo", "test"}, false},
		{[]string{"cargo", "run"}, false},

		// negative: plain npx (no create- prefix)
		{[]string{"npx", "eslint", "."}, false},
		{[]string{"npx", "jest"}, false},

		// negative: empty args
		{[]string{}, false},
		{nil, false},

		// negative: bare command names that don't match
		{[]string{"rails"}, false},
		{[]string{"dotnet"}, false},
		{[]string{"django-admin"}, false},
	}

	for _, tc := range tests {
		got := isScaffoldCommand(tc.args)
		if got != tc.want {
			t.Errorf("isScaffoldCommand(%v) = %v, want %v", tc.args, got, tc.want)
		}
	}
}

func TestCompletionAdLine(t *testing.T) {
	cfg := &Config{
		APIBase:  "https://backfill.sh",
		DeviceID: "dev_abc123",
	}
	ad := Ad{
		ID:   "ad_xyz",
		Text: "Try Acme Cloud — fast deploys",
	}

	line := completionAdLine(cfg, ad)

	// Must contain the ad text
	if !strings.Contains(line, ad.Text) {
		t.Errorf("completionAdLine: missing ad text %q in %q", ad.Text, line)
	}

	// Must contain the /r/<id>?d=<device> link
	expectedLink := "https://backfill.sh/r/ad_xyz?d=dev_abc123"
	if !strings.Contains(line, expectedLink) {
		t.Errorf("completionAdLine: missing link %q in %q", expectedLink, line)
	}

	// Must contain the dim "ad" label sequence
	if !strings.Contains(line, "\x1b[2mad\x1b[0m") {
		t.Errorf("completionAdLine: missing dim 'ad' label in %q", line)
	}

	// Must start and end with newlines so it sits clearly below command output
	if !strings.HasPrefix(line, "\n") {
		t.Errorf("completionAdLine: expected leading newline, got %q", line)
	}
	if !strings.HasSuffix(line, "\n") {
		t.Errorf("completionAdLine: expected trailing newline, got %q", line)
	}

	// Must NOT contain scroll-region escapes (footer-only)
	if strings.Contains(line, "\x1b[") && strings.Contains(line, "r") {
		// More precise: must not contain the scroll-region pattern \x1b[<n>;<m>r
		// Check for the specific scroll-region CSI sequences used in footer
		if strings.Contains(line, "\x1b[1;") {
			t.Errorf("completionAdLine: must not contain scroll-region escape in %q", line)
		}
	}

	// Must NOT contain cursor-save escape \x1b7
	if strings.Contains(line, "\x1b7") {
		t.Errorf("completionAdLine: must not contain cursor-save escape \\x1b7 in %q", line)
	}

	// Must have two-space indent
	// Find the non-newline content start
	trimmed := strings.TrimPrefix(line, "\n")
	if !strings.HasPrefix(trimmed, "  ") {
		t.Errorf("completionAdLine: expected two-space indent, got %q", trimmed)
	}
}
