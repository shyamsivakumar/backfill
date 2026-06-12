package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func claudeSettingsPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "settings.json")
}

func claudeSettingsBackupPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".backfill", "claude-settings.backup.json")
}

func cmdAgents(args []string) int {
	if len(args) == 0 {
		agentsUsage()
		return 2
	}

	switch args[0] {
	case "install":
		force := false
		for _, arg := range args[1:] {
			if arg == "--force" {
				force = true
			} else {
				agentsUsage()
				return 2
			}
		}
		return cmdAgentsInstall(force)
	case "remove":
		if len(args) != 1 {
			agentsUsage()
			return 2
		}
		return cmdAgentsRemove()
	case "status":
		if len(args) != 1 {
			agentsUsage()
			return 2
		}
		return cmdAgentsStatus()
	default:
		agentsUsage()
		return 2
	}
}

func agentsUsage() {
	fmt.Print(`usage:
  bf agents install [--force]
  bf agents remove
  bf agents status
`)
}

func cmdAgentsInstall(force bool) int {
	settings, original, exists, err := readClaudeSettings()
	if err != nil {
		fmt.Println(err)
		return 1
	}

	if current, ok := settings["statusLine"]; ok && !isBackfillStatusLine(current) && !force {
		fmt.Printf("existing statusLine: %s\n", jsonValue(current))
		fmt.Println("refusing to overwrite; rerun with --force to replace it")
		return 1
	}

	if exists {
		backupClaudeSettings(original)
	}

	exe, err := os.Executable()
	if err != nil {
		fmt.Println(err)
		return 1
	}

	settings["statusLine"] = map[string]any{
		"type":    "command",
		"command": exe + " statusline",
		"padding": 0,
	}

	if err := writeClaudeSettings(settings); err != nil {
		fmt.Println(err)
		return 1
	}

	fmt.Printf("installed Claude Code statusLine: %s statusline\n", exe)
	fmt.Println("ads will appear in Claude Code's status line")
	return 0
}

func cmdAgentsRemove() int {
	settings, _, exists, err := readClaudeSettings()
	if err != nil {
		fmt.Println(err)
		return 1
	}
	if !exists {
		fmt.Println("Claude Code settings not found; backfill statusLine is not installed")
		return 0
	}

	current, ok := settings["statusLine"]
	if !ok || !isBackfillStatusLine(current) {
		fmt.Printf("current statusLine: %s\n", jsonValue(current))
		fmt.Println("backfill statusLine is not installed")
		return 0
	}

	backupSettings, ok := readClaudeSettingsBackup()
	if ok {
		if restored, hasStatusLine := backupSettings["statusLine"]; hasStatusLine {
			settings["statusLine"] = restored
			if err := writeClaudeSettings(settings); err != nil {
				fmt.Println(err)
				return 1
			}
			fmt.Printf("restored previous statusLine: %s\n", jsonValue(restored))
			return 0
		}
	}

	delete(settings, "statusLine")
	if err := writeClaudeSettings(settings); err != nil {
		fmt.Println(err)
		return 1
	}
	fmt.Println("removed backfill statusLine")
	return 0
}

func cmdAgentsStatus() int {
	settings, _, exists, err := readClaudeSettings()
	if err != nil {
		fmt.Println(err)
		return 1
	}
	if !exists {
		fmt.Println("current statusLine: null")
		fmt.Println("backfill: false")
		return 0
	}

	current, ok := settings["statusLine"]
	if !ok {
		fmt.Println("current statusLine: null")
		fmt.Println("backfill: false")
		return 0
	}

	fmt.Printf("current statusLine: %s\n", jsonValue(current))
	fmt.Printf("backfill: %v\n", isBackfillStatusLine(current))
	return 0
}

func readClaudeSettings() (map[string]any, []byte, bool, error) {
	p := claudeSettingsPath()
	b, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return map[string]any{}, nil, false, nil
	}
	if err != nil {
		return nil, nil, false, err
	}
	if len(b) == 0 {
		return map[string]any{}, b, true, nil
	}

	var settings map[string]any
	if err := json.Unmarshal(b, &settings); err != nil {
		return nil, nil, true, err
	}
	if settings == nil {
		settings = map[string]any{}
	}
	return settings, b, true, nil
}

func writeClaudeSettings(settings map[string]any) error {
	p := claudeSettingsPath()
	os.MkdirAll(filepath.Dir(p), 0o755)
	b, _ := json.MarshalIndent(settings, "", "  ")
	b = append(b, '\n')
	return os.WriteFile(p, b, 0o600)
}

func backupClaudeSettings(original []byte) {
	p := claudeSettingsBackupPath()
	if _, err := os.Stat(p); err == nil {
		return
	}
	os.MkdirAll(filepath.Dir(p), 0o755)
	os.WriteFile(p, original, 0o600)
}

func readClaudeSettingsBackup() (map[string]any, bool) {
	b, err := os.ReadFile(claudeSettingsBackupPath())
	if err != nil {
		return nil, false
	}
	var settings map[string]any
	if json.Unmarshal(b, &settings) != nil || settings == nil {
		return nil, false
	}
	return settings, true
}

func isBackfillStatusLine(v any) bool {
	command := statusLineCommand(v)
	return strings.Contains(command, "bf statusline")
}

func statusLineCommand(v any) string {
	m, ok := v.(map[string]any)
	if !ok {
		return ""
	}
	command, _ := m["command"].(string)
	return command
}

func jsonValue(v any) string {
	if v == nil {
		return "null"
	}
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(b)
}
