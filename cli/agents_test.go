package main

import (
	"strings"
	"testing"
)

func TestParseAgentArgs(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		allowForce bool
		want       []agentTarget
		wantForce  bool
		wantOK     bool
	}{
		{name: "default all", allowForce: true, want: []agentTarget{agentClaude, agentCodex, agentFactory}, wantOK: true},
		{name: "claude", args: []string{"claude"}, allowForce: true, want: []agentTarget{agentClaude}, wantOK: true},
		{name: "codex", args: []string{"codex"}, allowForce: true, want: []agentTarget{agentCodex}, wantOK: true},
		{name: "factory", args: []string{"factory"}, allowForce: true, want: []agentTarget{agentFactory}, wantOK: true},
		{name: "droid alias", args: []string{"droid"}, allowForce: true, want: []agentTarget{agentFactory}, wantOK: true},
		{name: "all explicit", args: []string{"all"}, allowForce: true, want: []agentTarget{agentClaude, agentCodex, agentFactory}, wantOK: true},
		{name: "force before target", args: []string{"--force", "claude"}, allowForce: true, want: []agentTarget{agentClaude}, wantForce: true, wantOK: true},
		{name: "force after target", args: []string{"claude", "--force"}, allowForce: true, want: []agentTarget{agentClaude}, wantForce: true, wantOK: true},
		{name: "force rejected", args: []string{"--force"}, allowForce: false, wantOK: false},
		{name: "unknown target", args: []string{"cursor"}, allowForce: true, wantOK: false},
		{name: "multiple targets", args: []string{"claude", "factory"}, allowForce: true, wantOK: false},
		{name: "all plus target", args: []string{"all", "claude"}, allowForce: true, wantOK: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			targets, force, ok := parseAgentArgs(tc.args, tc.allowForce)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if !tc.wantOK {
				return
			}
			if force != tc.wantForce {
				t.Fatalf("force = %v, want %v", force, tc.wantForce)
			}
			if len(targets) != len(tc.want) {
				t.Fatalf("targets = %#v, want %#v", targets, tc.want)
			}
			for i := range targets {
				if targets[i] != tc.want[i] {
					t.Fatalf("targets = %#v, want %#v", targets, tc.want)
				}
			}
		})
	}
}

func TestRemoveCodexStatusLineKeepsOtherSections(t *testing.T) {
	input := []byte("[tui]\nmodel = \"gpt-5\"\nstatus_line = [\"/tmp/bf\", \"statusline\"]\nstatus_line_timeout_ms = 450\n[other]\nstatus_line_timeout_ms = 999\n")

	got := string(removeCodexStatusLine(input, ""))

	if strings.Contains(got, "status_line = [\"/tmp/bf\", \"statusline\"]") {
		t.Fatalf("backfill status line was not removed:\n%s", got)
	}
	if strings.Contains(got, "[tui]\nmodel = \"gpt-5\"\nstatus_line_timeout_ms = 450") {
		t.Fatalf("tui timeout was not removed:\n%s", got)
	}
	if !strings.Contains(got, "[other]\nstatus_line_timeout_ms = 999") {
		t.Fatalf("unrelated section timeout was removed:\n%s", got)
	}
}
