package main

import (
	"path/filepath"
	"testing"
)

func TestShouldBypassWrapping(t *testing.T) {
	tests := []struct {
		name                         string
		enabled, stdinTTY, stdoutTTY bool
		want                         bool
	}{
		{name: "interactive enabled", enabled: true, stdinTTY: true, stdoutTTY: true, want: false},
		{name: "disabled", enabled: false, stdinTTY: true, stdoutTTY: true, want: true},
		{name: "non-TTY stdin", enabled: true, stdinTTY: false, stdoutTTY: true, want: true},
		{name: "non-TTY stdout", enabled: true, stdinTTY: true, stdoutTTY: false, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldBypassWrapping(tt.enabled, tt.stdinTTY, tt.stdoutTTY); got != tt.want {
				t.Fatalf("shouldBypassWrapping(%v, %v, %v) = %v, want %v", tt.enabled, tt.stdinTTY, tt.stdoutTTY, got, tt.want)
			}
		})
	}
}

func TestPlanWrappedRunPackageManagerContract(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want wrappedRunPlan
	}{
		// Installs keep native progress and get one completion ad on success.
		{name: "npm install", args: []string{"npm", "install"}, want: wrappedRunPlan{wrappedRunPlain, true}},
		{name: "npm i", args: []string{"npm", "i"}, want: wrappedRunPlan{wrappedRunPlain, true}},
		{name: "npm ci", args: []string{"npm", "ci"}, want: wrappedRunPlan{wrappedRunPlain, true}},
		{name: "npm update", args: []string{"npm", "update"}, want: wrappedRunPlan{wrappedRunPlain, true}},
		{name: "pnpm add", args: []string{"pnpm", "add", "x"}, want: wrappedRunPlan{wrappedRunPlain, true}},
		{name: "yarn bare", args: []string{"yarn"}, want: wrappedRunPlan{wrappedRunPlain, true}},
		{name: "bun install", args: []string{"bun", "install"}, want: wrappedRunPlan{wrappedRunPlain, true}},
		{name: "pip install", args: []string{"pip", "install", "x"}, want: wrappedRunPlan{wrappedRunPlain, true}},

		// Package-manager scripts remain collapsed.
		{name: "npm run build", args: []string{"npm", "run", "build"}, want: wrappedRunPlan{wrappedRunCollapsed, false}},
		{name: "npm test", args: []string{"npm", "test"}, want: wrappedRunPlan{wrappedRunCollapsed, false}},
		{name: "pnpm dev", args: []string{"pnpm", "dev"}, want: wrappedRunPlan{wrappedRunCollapsed, false}},
		{name: "bun run start", args: []string{"bun", "run", "start"}, want: wrappedRunPlan{wrappedRunCollapsed, false}},

		// Interactive package-manager scaffolders run plain but retain their ad.
		{name: "npm create", args: []string{"npm", "create", "vite"}, want: wrappedRunPlan{wrappedRunPlain, true}},
		{name: "npm init", args: []string{"npm", "init", "."}, want: wrappedRunPlan{wrappedRunPlain, true}},
		{name: "pnpm init", args: []string{"pnpm", "init"}, want: wrappedRunPlan{wrappedRunPlain, true}},
		{name: "npx create-app", args: []string{"npx", "create-app"}, want: wrappedRunPlan{wrappedRunPlain, true}},

		// Other interactive commands run plain with no Backfill output.
		{name: "npm login", args: []string{"npm", "login"}, want: wrappedRunPlan{wrappedRunPlain, false}},
		{name: "vim", args: []string{"vim", "README.md"}, want: wrappedRunPlan{wrappedRunPlain, false}},

		// Non-package scaffolders remain collapsed and keep their completion ad.
		{name: "create-npm binary", args: []string{"create-npm"}, want: wrappedRunPlan{wrappedRunCollapsed, true}},
		{name: "cargo new", args: []string{"cargo", "new", "demo"}, want: wrappedRunPlan{wrappedRunCollapsed, true}},

		// Ordinary and near-miss commands retain the normal collapsed route.
		{name: "ordinary command", args: []string{"make", "build"}, want: wrappedRunPlan{wrappedRunCollapsed, false}},
		{name: "npminstall near miss", args: []string{"npminstall", "x"}, want: wrappedRunPlan{wrappedRunCollapsed, false}},
		{name: "bunx near miss", args: []string{"bunx", "vite"}, want: wrappedRunPlan{wrappedRunCollapsed, false}},
		{name: "empty args", args: []string{}, want: wrappedRunPlan{wrappedRunCollapsed, false}},

		// Flags and resolved shim paths must not change the selected behavior.
		{name: "flag before install", args: []string{"npm", "--silent", "install"}, want: wrappedRunPlan{wrappedRunPlain, true}},
		{name: "flag before script", args: []string{"npm", "--silent", "run", "build"}, want: wrappedRunPlan{wrappedRunCollapsed, false}},
		{name: "filter before install", args: []string{"pnpm", "--filter", "web", "add", "x"}, want: wrappedRunPlan{wrappedRunPlain, true}},
		{name: "filter equals before install", args: []string{"pnpm", "--filter=web", "add", "x"}, want: wrappedRunPlan{wrappedRunPlain, true}},
		{name: "separator before install", args: []string{"npm", "--", "install"}, want: wrappedRunPlan{wrappedRunPlain, true}},
		{name: "workspace after install", args: []string{"npm", "install", "--workspace", "web"}, want: wrappedRunPlan{wrappedRunPlain, true}},
		{name: "resolved npm shim install", args: []string{filepath.Join("tmp", "shims", "npm"), "install"}, want: wrappedRunPlan{wrappedRunPlain, true}},
		{name: "resolved npm shim script", args: []string{filepath.Join("tmp", "shims", "npm"), "run", "build"}, want: wrappedRunPlan{wrappedRunCollapsed, false}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := planWrappedRun(tt.args); got != tt.want {
				t.Fatalf("planWrappedRun(%v) = %+v, want %+v", tt.args, got, tt.want)
			}
		})
	}
}
