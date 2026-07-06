package main

import "testing"

func TestParseAgentArgsAcceptsDroidAlias(t *testing.T) {
	targets, force, ok := parseAgentArgs([]string{"droid"}, true)
	if !ok {
		t.Fatal("parseAgentArgs rejected droid alias")
	}
	if force {
		t.Fatal("parseAgentArgs unexpectedly set force")
	}
	if len(targets) != 1 || targets[0] != agentFactory {
		t.Fatalf("parseAgentArgs(droid) = %#v, want factory target", targets)
	}
}
