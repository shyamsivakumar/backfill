package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

var packageManagerValueOptions = map[string]map[string]bool{
	"npm": {
		"-w": true, "--workspace": true, "--prefix": true,
		"--cache": true, "--registry": true, "--userconfig": true,
		"--loglevel": true,
	},
	"pnpm": {
		"-C": true, "--dir": true, "-F": true, "--filter": true,
		"--workspace-dir": true, "--config-dir": true,
		"--store-dir": true, "--virtual-store-dir": true,
		"--reporter": true, "--loglevel": true,
	},
	"yarn": {
		"--cwd": true, "--mutex": true, "--cache-folder": true,
		"--modules-folder": true, "--network-concurrency": true,
		"--network-timeout": true, "--registry": true,
	},
	"bun": {
		"--cwd": true, "--config": true,
	},
	"npx": {
		"-p": true, "--package": true, "-c": true, "--call": true,
		"--shell": true,
	},
	"pip": {
		"--python": true, "--proxy": true, "--timeout": true,
		"--retries": true, "--cache-dir": true, "--index-url": true,
		"--extra-index-url": true,
	},
	"pip3": {
		"--python": true, "--proxy": true, "--timeout": true,
		"--retries": true, "--cache-dir": true, "--index-url": true,
		"--extra-index-url": true,
	},
}

// packageManagerInvocation returns an exact package-manager name and its first
// positional subcommand. Global options are skipped, including the common
// options whose values are separate arguments. Using filepath.Base keeps the
// same classification when tests or callers pass a resolved shim path.
func packageManagerInvocation(args []string) (manager, subcommand string, ok bool) {
	if len(args) == 0 {
		return "", "", false
	}
	manager = filepath.Base(args[0])
	switch manager {
	case "npm", "pnpm", "yarn", "bun", "npx", "pip", "pip3":
	default:
		return "", "", false
	}

	for i := 1; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			if i+1 < len(args) {
				return manager, args[i+1], true
			}
			return manager, "", true
		}
		if arg == "" || (strings.HasPrefix(arg, "-") && arg != "-") {
			option := arg
			if eq := strings.IndexByte(option, '='); eq >= 0 {
				option = option[:eq]
			}
			if option == arg && packageManagerValueOptions[manager][option] && i+1 < len(args) {
				i++
			}
			continue
		}
		return manager, arg, true
	}
	return manager, "", true
}

func isPackageManagerScaffoldCommand(args []string) bool {
	manager, subcommand, ok := packageManagerInvocation(args)
	if !ok {
		return false
	}
	switch manager {
	case "npm", "pnpm", "yarn", "bun":
		return subcommand == "create" || subcommand == "init"
	case "npx":
		return strings.HasPrefix(subcommand, "create-")
	}
	return false
}

// isScaffoldCommand returns true for scaffold/create commands that finish too
// quickly for the collapsed line to earn but produce a success screen worth
// sponsoring. Package-manager scaffolders run plainly because they may prompt;
// other scaffolders stay collapsed. Both get one completion ad after success.
func isScaffoldCommand(args []string) bool {
	if len(args) == 0 {
		return false
	}
	if isPackageManagerScaffoldCommand(args) {
		return true
	}
	command := filepath.Base(args[0])
	switch command {
	case "cargo":
		return len(args) >= 2 && (args[1] == "new" || args[1] == "init")
	case "django-admin":
		return len(args) >= 2 && args[1] == "startproject"
	case "rails":
		return len(args) >= 2 && args[1] == "new"
	case "dotnet":
		return len(args) >= 2 && args[1] == "new"
	}
	return strings.HasPrefix(command, "create-")
}

// isInstallCommand returns true for package-install commands that draw their
// own progress UI and should run plainly, followed by one completion ad on
// success. Package-manager scripts are not installs and remain collapsed.
func isInstallCommand(args []string) bool {
	manager, subcommand, ok := packageManagerInvocation(args)
	if !ok {
		return false
	}
	switch manager {
	case "npm", "pnpm", "bun":
		switch subcommand {
		case "install", "i", "add", "ci", "update", "upgrade":
			return true
		}
	case "yarn":
		if subcommand == "" {
			return true
		}
		switch subcommand {
		case "install", "i", "add", "ci", "update", "upgrade":
			return true
		}
	case "pip", "pip3":
		return subcommand == "install"
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
