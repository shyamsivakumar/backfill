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

// completionAdLine formats one persistent sponsored line to print to stdout
// after the command output. It is a plain printed line — no scroll-region or
// cursor-save escapes used in the footer.
func completionAdLine(cfg *Config, ad Ad) string {
	link := fmt.Sprintf("%s/r/%s?d=%s", cfg.APIBase, ad.ID, cfg.DeviceID)
	return fmt.Sprintf("\n  \x1b[2mad\x1b[0m \x1b]8;;%s\x07\x1b[33m%s\x1b[0m\x1b]8;;\x07\n", link, ad.Text)
}
