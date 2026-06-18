package main

import (
	"fmt"
	"os"
	"os/exec"
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
		all := false
		var extra []string
		for _, a := range args[1:] {
			if a == "--all" {
				all = true
			} else {
				extra = append(extra, a)
			}
		}
		os.Exit(cmdInit(extra, all))
	case "uninit":
		os.Exit(cmdUninit())
	case "wrap":
		os.Exit(cmdWrap(args[1:]))
	case "unwrap":
		os.Exit(cmdUnwrap(args[1:]))
	case "on", "off":
		cfg := loadConfig()
		cfg.Enabled = args[0] == "on"
		saveConfig(cfg)
		fmt.Printf("backfill footer: %s\n", args[0])
	case "status":
		cmdStatus()
	case "claim":
		cmdClaim()
	case "refer":
		cmdRefer()
	case "statusline":
		cmdStatusline()
	case "statusline-refresh":
		cmdStatuslineRefresh()
	case "spinner-refresh":
		cmdSpinnerRefresh()
	case "agents":
		os.Exit(cmdAgents(args[1:]))
	case "spin":
		if len(args) < 2 {
			fmt.Println("usage: bf spin <agent-command> [args...]")
			os.Exit(2)
		}
		cfg := loadConfig()
		deshimPath()
		bin, err := exec.LookPath(args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "bf: %s: command not found\n", args[1])
			os.Exit(127)
		}
		os.Exit(runWithRewrite(cfg, bin, args[1:]))
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
  bf <command> [args...]   run a command and earn from the wait (e.g. bf dbt run)
  bf init [cmd...]         wrap common slow commands (dbt, cargo, docker, …) so they earn (one time)
  bf init --all            wrap every non-interactive command found on PATH
  bf uninit                remove the wrapping
  bf wrap <cmd>...         also wrap these commands
  bf unwrap <cmd>...       stop wrapping these commands
  bf on | off              enable/disable backfill
  bf status                show device id and dashboard link
  bf claim                 print your device claim code
  bf refer                 print your referral install command
  bf agents install        install the coding-agent spinner integration (claude, droid, codex)
  bf agents remove         remove the coding-agent spinner integration
  bf agents status         show coding-agent spinner integration status
`)
}

func cmdStatus() {
	cfg := loadConfig()
	fmt.Printf("device:    %s\nenabled:   %v\napi:       %s\ndashboard: %s/dashboard?d=%s\n",
		cfg.DeviceID, cfg.Enabled, cfg.APIBase, cfg.APIBase, cfg.DeviceID)
}

func cmdClaim() {
	cfg := loadConfig()
	fmt.Printf("%s/dashboard?d=%s\n", cfg.APIBase, cfg.DeviceID)
	fmt.Printf("claim code: %s\n", claimCode(cfg))
	fmt.Println("Paste your device id and this code at backfill.sh/dashboard after logging in. The code proves this machine is yours — anyone can see your device id in ad links, only you have this.")
}

func cmdRefer() {
	cfg := loadConfig()
	fmt.Printf("curl -fsSL https://backfill.sh/install.sh | BACKFILL_REF=%s sh\n", cfg.DeviceID)
	fmt.Println("You earn a 10% bonus on everything they earn (from our half, not theirs).")
}
