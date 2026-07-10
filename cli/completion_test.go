package main

import (
	"path/filepath"
	"regexp"
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
		{[]string{"npm", "--silent", "create", "vite@latest"}, true},
		{[]string{"pnpm", "--filter", "web", "init"}, true},
		{[]string{filepath.Join("tmp", "shims", "npm"), "create", "vite"}, true},

		// positive: npx create-* scaffolders
		{[]string{"npx", "create-react-app", "myapp"}, true},
		{[]string{"npx", "create-next-app", "--typescript"}, true},
		{[]string{"npx", "create-vite"}, true},
		{[]string{"npx", "--yes", "create-vite"}, true},

		// positive: bare create-* binaries
		{[]string{"create-react-app", "myapp"}, true},
		{[]string{"create-next-app", "."}, true},
		{[]string{"create-vite", "myproject"}, true},
		{[]string{"create-npm"}, true},

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

	// Must NOT contain a DECSTBM scroll-region escape (footer-only).
	if regexp.MustCompile("\x1b\\[[0-9;]*r").MatchString(line) {
		t.Errorf("completionAdLine: must not contain scroll-region escape in %q", line)
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

func TestIsInstallCommand(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want bool
	}{
		{name: "npm install", args: []string{"npm", "install"}, want: true},
		{name: "npm i", args: []string{"npm", "i"}, want: true},
		{name: "npm ci", args: []string{"npm", "ci"}, want: true},
		{name: "npm update", args: []string{"npm", "update"}, want: true},
		{name: "pnpm add", args: []string{"pnpm", "add", "x"}, want: true},
		{name: "yarn bare", args: []string{"yarn"}, want: true},
		{name: "yarn add", args: []string{"yarn", "add", "x"}, want: true},
		{name: "bun install", args: []string{"bun", "install"}, want: true},
		{name: "pip install", args: []string{"pip", "install", "foo"}, want: true},
		{name: "pip3 install", args: []string{"pip3", "install"}, want: true},

		{name: "flag before npm install", args: []string{"npm", "--silent", "install"}, want: true},
		{name: "workspace before npm install", args: []string{"npm", "--workspace", "web", "install"}, want: true},
		{name: "workspace equals before npm ci", args: []string{"npm", "--workspace=web", "ci"}, want: true},
		{name: "filter before pnpm add", args: []string{"pnpm", "--filter", "web", "add", "x"}, want: true},
		{name: "filter equals before pnpm add", args: []string{"pnpm", "--filter=web", "add", "x"}, want: true},
		{name: "cwd before bun install", args: []string{"bun", "--cwd", "web", "install"}, want: true},
		{name: "separator before npm install", args: []string{"npm", "--", "install"}, want: true},
		{name: "workspace after npm install", args: []string{"npm", "install", "--workspace", "web"}, want: true},
		{name: "global flag after npm install", args: []string{"npm", "install", "-g", "x"}, want: true},
		{name: "filter after pnpm add", args: []string{"pnpm", "add", "--filter", "web", "x"}, want: true},
		{name: "pip option after install", args: []string{"pip3", "install", "--no-deps", "x"}, want: true},
		{name: "resolved npm shim", args: []string{filepath.Join("tmp", "shims", "npm"), "install"}, want: true},

		{name: "npm run build", args: []string{"npm", "run", "build"}, want: false},
		{name: "npm test", args: []string{"npm", "test"}, want: false},
		{name: "flag before npm run", args: []string{"npm", "--silent", "run", "build"}, want: false},
		{name: "pnpm dev", args: []string{"pnpm", "dev"}, want: false},
		{name: "filtered pnpm dev", args: []string{"pnpm", "--filter", "web", "dev"}, want: false},
		{name: "bun run", args: []string{"bun", "run", "start"}, want: false},
		{name: "npm scaffold", args: []string{"npm", "create", "vite"}, want: false},
		{name: "npx scaffold", args: []string{"npx", "create-app"}, want: false},
		{name: "npminstall near miss", args: []string{"npminstall", "x"}, want: false},
		{name: "bunx near miss", args: []string{"bunx", "install"}, want: false},
		{name: "create-npm near miss", args: []string{"create-npm"}, want: false},
		{name: "cargo", args: []string{"cargo", "build"}, want: false},
		{name: "empty", args: []string{}, want: false},
		{name: "nil", args: nil, want: false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isInstallCommand(c.args); got != c.want {
				t.Errorf("isInstallCommand(%v) = %v, want %v", c.args, got, c.want)
			}
		})
	}
}

func TestPackageManagerInvocation(t *testing.T) {
	tests := []struct {
		name                        string
		args                        []string
		wantManager, wantSubcommand string
		wantOK                      bool
	}{
		{name: "empty", args: nil},
		{name: "bare npm", args: []string{"npm"}, wantManager: "npm", wantOK: true},
		{name: "boolean option", args: []string{"npm", "--silent", "install"}, wantManager: "npm", wantSubcommand: "install", wantOK: true},
		{name: "equals value option", args: []string{"npm", "--workspace=web", "ci"}, wantManager: "npm", wantSubcommand: "ci", wantOK: true},
		{name: "separate value option", args: []string{"pnpm", "--filter", "web", "add"}, wantManager: "pnpm", wantSubcommand: "add", wantOK: true},
		{name: "separator", args: []string{"npm", "--", "install"}, wantManager: "npm", wantSubcommand: "install", wantOK: true},
		{name: "trailing separator", args: []string{"npm", "--"}, wantManager: "npm", wantOK: true},
		{name: "resolved shim", args: []string{filepath.Join("tmp", "shims", "npm"), "install"}, wantManager: "npm", wantSubcommand: "install", wantOK: true},
		{name: "npminstall near miss", args: []string{"npminstall", "x"}},
		{name: "bunx near miss", args: []string{"bunx", "vite"}},
		{name: "create-npm near miss", args: []string{"create-npm"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager, subcommand, ok := packageManagerInvocation(tt.args)
			if manager != tt.wantManager || subcommand != tt.wantSubcommand || ok != tt.wantOK {
				t.Fatalf("packageManagerInvocation(%v) = (%q, %q, %v), want (%q, %q, %v)", tt.args, manager, subcommand, ok, tt.wantManager, tt.wantSubcommand, tt.wantOK)
			}
		})
	}
}
