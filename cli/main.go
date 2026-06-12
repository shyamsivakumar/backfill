package main

import (
	"fmt"
	"os"
)

var version = "0.1.0"

const defaultAPIBase = "https://backfill.sh"

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		usage()
		os.Exit(2)
	}
	switch args[0] {
	case "init":
		cmdAliases(false)
	case "uninit":
		cmdAliases(true)
	case "on", "off":
		cfg := loadConfig()
		cfg.Enabled = args[0] == "on"
		saveConfig(cfg)
		fmt.Printf("backfill footer: %s\n", args[0])
	case "status":
		cmdStatus()
	case "version", "--version", "-v":
		fmt.Println("bf", version)
	case "help", "--help", "-h":
		usage()
	default:
		os.Exit(runWrapped(args))
	}
}

func usage() {
	fmt.Print(`bf — get paid while your builds run

usage:
  bf <command> [args...]   run a command with a sponsored footer (e.g. bf dbt run)
  bf init                  add shell aliases for supported commands
  bf uninit                remove the aliases
  bf on | off              enable/disable the footer
  bf status                show device id and dashboard link
`)
}

func cmdStatus() {
	cfg := loadConfig()
	fmt.Printf("device:    %s\nenabled:   %v\napi:       %s\ndashboard: %s/dashboard?d=%s\n",
		cfg.DeviceID, cfg.Enabled, cfg.APIBase, cfg.APIBase, cfg.DeviceID)
}
