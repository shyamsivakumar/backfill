package main

import (
	"fmt"
	"strings"
)

// isScaffoldCommand returns true for scaffold/create commands that finish too
// quickly for the footer to earn but produce a success screen worth sponsoring.
func isScaffoldCommand(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case "npm", "pnpm", "yarn", "bun":
		if len(args) < 2 {
			return false
		}
		return args[1] == "create" || args[1] == "init"
	case "npx":
		return len(args) >= 2 && strings.HasPrefix(args[1], "create-")
	case "cargo":
		return len(args) >= 2 && (args[1] == "new" || args[1] == "init")
	case "django-admin":
		return len(args) >= 2 && args[1] == "startproject"
	case "rails":
		return len(args) >= 2 && args[1] == "new"
	case "dotnet":
		return len(args) >= 2 && args[1] == "new"
	}
	return strings.HasPrefix(args[0], "create-")
}

// isInstallCommand returns true for package-install commands that draw their
// own progress UI and should run plainly (no footer), earning via a completion
// line instead, since a scroll-region footer fights their download bars.
func isInstallCommand(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case "npm", "pnpm", "bun":
		if len(args) < 2 {
			return false
		}
		switch args[1] {
		case "install", "i", "add", "ci", "update", "upgrade":
			return true
		}
	case "yarn":
		if len(args) == 1 {
			return true
		}
		switch args[1] {
		case "install", "i", "add", "ci", "update", "upgrade":
			return true
		}
	case "pip", "pip3":
		return len(args) >= 2 && args[1] == "install"
	}
	return false
}

// isNpmFamily returns true for any npm/pnpm/yarn/bun invocation. These tools
// draw their own progress UI (spinners, download bars, server output) that
// fights a scroll-region footer, so they always run plainly with no footer.
func isNpmFamily(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case "npm", "pnpm", "yarn", "bun":
		return true
	}
	return false
}

// completionAdLine formats one persistent sponsored line to print to stdout
// after the command output. It is a plain printed line — no scroll-region or
// cursor-save escapes used in the footer.
func completionAdLine(cfg *Config, ad Ad) string {
	link := fmt.Sprintf("%s/r/%s?d=%s", cfg.APIBase, ad.ID, cfg.DeviceID)
	return fmt.Sprintf("\n  \x1b[2mad\x1b[0m \x1b]8;;%s\x07\x1b[33m%s\x1b[0m\x1b]8;;\x07\n", link, ad.Text)
}
