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

func codexConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".codex", "config.toml")
}

func codexConfigBackupPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".backfill", "codex-config.backup.toml")
}

type agentTarget string

const (
	agentClaude agentTarget = "claude"
	agentCodex  agentTarget = "codex"
)

func cmdAgents(args []string) int {
	if len(args) == 0 {
		agentsUsage()
		return 2
	}

	switch args[0] {
	case "install":
		targets, force, ok := parseAgentArgs(args[1:], true)
		if !ok {
			agentsUsage()
			return 2
		}
		return cmdAgentsInstall(targets, force)
	case "remove":
		targets, _, ok := parseAgentArgs(args[1:], false)
		if !ok {
			agentsUsage()
			return 2
		}
		return cmdAgentsRemove(targets)
	case "status":
		targets, _, ok := parseAgentArgs(args[1:], false)
		if !ok {
			agentsUsage()
			return 2
		}
		return cmdAgentsStatus(targets)
	default:
		agentsUsage()
		return 2
	}
}

func agentsUsage() {
	fmt.Print(`usage:
  bf agents install [claude|codex|all] [--force]
  bf agents remove [claude|codex|all]
  bf agents status [claude|codex|all]
`)
}

func parseAgentArgs(args []string, allowForce bool) ([]agentTarget, bool, bool) {
	target := "all"
	force := false

	for _, arg := range args {
		switch arg {
		case "--force":
			if !allowForce {
				return nil, false, false
			}
			force = true
		case "claude", "codex", "all":
			if target != "all" {
				return nil, false, false
			}
			target = arg
		default:
			return nil, false, false
		}
	}

	switch target {
	case "claude":
		return []agentTarget{agentClaude}, force, true
	case "codex":
		return []agentTarget{agentCodex}, force, true
	default:
		return []agentTarget{agentClaude, agentCodex}, force, true
	}
}

func cmdAgentsInstall(targets []agentTarget, force bool) int {
	exit := 0
	for _, target := range targets {
		var code int
		switch target {
		case agentClaude:
			code = installClaudeAgent(force)
		case agentCodex:
			code = installCodexAgent(force)
		}
		if code != 0 {
			exit = code
		}
	}
	return exit
}

func cmdAgentsRemove(targets []agentTarget) int {
	exit := 0
	for _, target := range targets {
		var code int
		switch target {
		case agentClaude:
			code = removeClaudeAgent()
		case agentCodex:
			code = removeCodexAgent()
		}
		if code != 0 {
			exit = code
		}
	}
	return exit
}

func cmdAgentsStatus(targets []agentTarget) int {
	exit := 0
	for _, target := range targets {
		var code int
		switch target {
		case agentClaude:
			code = statusClaudeAgent()
		case agentCodex:
			code = statusCodexAgent()
		}
		if code != 0 {
			exit = code
		}
	}
	return exit
}

func installClaudeAgent(force bool) int {
	settings, original, exists, err := readClaudeSettings()
	if err != nil {
		fmt.Println(err)
		return 1
	}

	if current, ok := settings["statusLine"]; ok && !isBackfillStatusLine(current) && !force {
		fmt.Printf("Claude Code existing statusLine: %s\n", jsonValue(current))
		fmt.Println("Claude Code refusing to overwrite; rerun with --force to replace it")
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

	fmt.Printf("Claude Code installed statusLine: %s statusline\n", exe)
	fmt.Println("Claude Code ads will appear in the status line")
	return 0
}

func removeClaudeAgent() int {
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
		fmt.Printf("Claude Code current statusLine: %s\n", jsonValue(current))
		fmt.Println("Claude Code backfill statusLine is not installed")
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
			fmt.Printf("Claude Code restored previous statusLine: %s\n", jsonValue(restored))
			return 0
		}
	}

	delete(settings, "statusLine")
	if err := writeClaudeSettings(settings); err != nil {
		fmt.Println(err)
		return 1
	}
	fmt.Println("Claude Code removed backfill statusLine")
	return 0
}

func statusClaudeAgent() int {
	settings, _, exists, err := readClaudeSettings()
	if err != nil {
		fmt.Println(err)
		return 1
	}
	if !exists {
		fmt.Println("Claude Code current statusLine: null")
		fmt.Println("Claude Code backfill: false")
		return 0
	}

	current, ok := settings["statusLine"]
	if !ok {
		fmt.Println("Claude Code current statusLine: null")
		fmt.Println("Claude Code backfill: false")
		return 0
	}

	fmt.Printf("Claude Code current statusLine: %s\n", jsonValue(current))
	fmt.Printf("Claude Code backfill: %v\n", isBackfillStatusLine(current))
	return 0
}

func installCodexAgent(force bool) int {
	original, exists, err := readCodexConfig()
	if err != nil {
		fmt.Println(err)
		return 1
	}

	line, hasLine := currentCodexStatusLine(original)
	if hasLine && !isBackfillCodexStatusLine(line) && !force {
		fmt.Printf("Codex existing status_line: %s\n", strings.TrimRight(line, "\r\n"))
		fmt.Println("Codex refusing to overwrite; rerun with --force to replace it")
		return 1
	}

	if exists {
		backupCodexConfig(original)
	} else {
		backupCodexConfig(nil)
	}

	exe, err := os.Executable()
	if err != nil {
		fmt.Println(err)
		return 1
	}

	updated := installCodexStatusLine(original, exe)
	if err := writeCodexConfig(updated); err != nil {
		fmt.Println(err)
		return 1
	}

	fmt.Printf("Codex installed status_line: %s statusline\n", exe)
	fmt.Println("Codex ads will appear in the status line")
	return 0
}

func removeCodexAgent() int {
	original, exists, err := readCodexConfig()
	if err != nil {
		fmt.Println(err)
		return 1
	}
	if !exists {
		fmt.Println("Codex config not found; backfill status_line is not installed")
		return 0
	}

	line, hasLine := currentCodexStatusLine(original)
	if !hasLine || !isBackfillCodexStatusLine(line) {
		if hasLine {
			fmt.Printf("Codex current status_line: %s\n", strings.TrimRight(line, "\r\n"))
		} else {
			fmt.Println("Codex current status_line: none")
		}
		fmt.Println("Codex backfill status_line is not installed")
		return 0
	}

	restoreLine := ""
	if backup, ok := readCodexConfigBackup(); ok {
		if backupLine, hasBackupLine := currentCodexStatusLine(backup); hasBackupLine {
			restoreLine = strings.TrimRight(backupLine, "\r\n")
		}
	}

	updated := removeCodexStatusLine(original, restoreLine)
	if err := writeCodexConfig(updated); err != nil {
		fmt.Println(err)
		return 1
	}

	if restoreLine != "" {
		fmt.Printf("Codex restored previous status_line: %s\n", restoreLine)
	} else {
		fmt.Println("Codex removed backfill status_line")
	}
	return 0
}

func statusCodexAgent() int {
	b, exists, err := readCodexConfig()
	if err != nil {
		fmt.Println(err)
		return 1
	}
	if !exists {
		fmt.Println("Codex current status_line: none")
		fmt.Println("Codex backfill: false")
		return 0
	}

	line, ok := currentCodexStatusLine(b)
	if !ok {
		fmt.Println("Codex current status_line: none")
		fmt.Println("Codex backfill: false")
		return 0
	}

	fmt.Printf("Codex current status_line: %s\n", strings.TrimRight(line, "\r\n"))
	fmt.Printf("Codex backfill: %v\n", isBackfillCodexStatusLine(line))
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

func readCodexConfig() ([]byte, bool, error) {
	p := codexConfigPath()
	b, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return b, true, nil
}

func writeCodexConfig(b []byte) error {
	p := codexConfigPath()
	os.MkdirAll(filepath.Dir(p), 0o755)
	return os.WriteFile(p, b, 0o600)
}

func backupCodexConfig(original []byte) {
	p := codexConfigBackupPath()
	if _, err := os.Stat(p); err == nil {
		return
	}
	os.MkdirAll(filepath.Dir(p), 0o755)
	os.WriteFile(p, original, 0o600)
}

func readCodexConfigBackup() ([]byte, bool) {
	b, err := os.ReadFile(codexConfigBackupPath())
	if err != nil {
		return nil, false
	}
	return b, true
}

func installCodexStatusLine(b []byte, exe string) []byte {
	lines := splitLines(string(b))
	statusLine := fmt.Sprintf("status_line = [%q, \"statusline\"]", exe)
	timeoutLine := "status_line_timeout_ms = 450"

	tuiStart, tuiEnd := findSection(lines, "tui")
	if tuiStart >= 0 {
		out := make([]string, 0, len(lines)+2)
		out = append(out, lines[:tuiStart+1]...)
		out = append(out, statusLine+"\n", timeoutLine+"\n")
		for i := tuiStart + 1; i < tuiEnd; i++ {
			if isCodexStatusLineKey(lines[i]) || isCodexStatusLineTimeoutKey(lines[i]) {
				continue
			}
			out = append(out, lines[i])
		}
		out = append(out, lines[tuiEnd:]...)
		return []byte(strings.Join(out, ""))
	}

	s := string(b)
	if s != "" && !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	s += "\n[tui]\n" + statusLine + "\n" + timeoutLine + "\n"
	return []byte(s)
}

func removeCodexStatusLine(b []byte, restoreLine string) []byte {
	lines := splitLines(string(b))
	statusIdx := -1
	for i, line := range lines {
		if isCodexStatusLineKey(line) && isBackfillCodexStatusLine(line) {
			statusIdx = i
			break
		}
	}
	if statusIdx < 0 {
		return b
	}

	tuiStart, tuiEnd := findSectionContaining(lines, statusIdx)
	out := make([]string, 0, len(lines))
	for i, line := range lines {
		if i == statusIdx {
			if restoreLine != "" {
				out = append(out, restoreLine+"\n")
			}
			continue
		}
		if tuiStart >= 0 && i > tuiStart && i < tuiEnd && isCodexStatusLineTimeoutKey(line) {
			continue
		}
		out = append(out, line)
	}
	return []byte(strings.Join(out, ""))
}

func currentCodexStatusLine(b []byte) (string, bool) {
	for _, line := range splitLines(string(b)) {
		if isCodexStatusLineKey(line) {
			return line, true
		}
	}
	return "", false
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.SplitAfter(s, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func findSection(lines []string, name string) (int, int) {
	header := "[" + name + "]"
	for i, line := range lines {
		if strings.TrimSpace(line) == header {
			end := len(lines)
			for j := i + 1; j < len(lines); j++ {
				trimmed := strings.TrimSpace(lines[j])
				if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
					end = j
					break
				}
			}
			return i, end
		}
	}
	return -1, -1
}

func findSectionContaining(lines []string, idx int) (int, int) {
	start := -1
	for i := idx; i >= 0; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			start = i
			break
		}
	}
	if start < 0 {
		return -1, len(lines)
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			end = i
			break
		}
	}
	return start, end
}

func isCodexStatusLineKey(line string) bool {
	trimmed := strings.TrimSpace(line)
	return trimmed == "status_line" || strings.HasPrefix(trimmed, "status_line ") || strings.HasPrefix(trimmed, "status_line=")
}

func isCodexStatusLineTimeoutKey(line string) bool {
	trimmed := strings.TrimSpace(line)
	return trimmed == "status_line_timeout_ms" || strings.HasPrefix(trimmed, "status_line_timeout_ms ") || strings.HasPrefix(trimmed, "status_line_timeout_ms=")
}

func isBackfillCodexStatusLine(line string) bool {
	return strings.Contains(line, "\" statusline") || strings.Contains(line, "statusline\"")
}

func isBackfillStatusLine(v any) bool {
	command := statusLineCommand(v)
	return strings.Contains(command, " statusline") || strings.HasSuffix(command, "statusline")
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
