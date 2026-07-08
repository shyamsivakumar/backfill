package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDetectShadowingManagersAfterBackfillBlock(t *testing.T) {
	rc := writeTempRC(t, pathBlock()+`
export NVM_DIR="$HOME/.nvm"
[ -s "$NVM_DIR/nvm.sh" ] && . "$NVM_DIR/nvm.sh"
eval "$(fnm env --use-on-cd)"
eval "$(MISE activate zsh)"
eval "$(pyenv init -)"
`)

	got := detectShadowingManagers([]string{rc})
	want := []string{"nvm", "fnm", "mise", "pyenv"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("detectShadowingManagers() = %#v, want %#v", got, want)
	}
}

func TestDetectShadowingManagersIgnoresManagersBeforeBackfillBlock(t *testing.T) {
	rc := writeTempRC(t, `
export NVM_DIR="$HOME/.nvm"
[ -s "$NVM_DIR/nvm.sh" ] && . "$NVM_DIR/nvm.sh"
eval "$(pyenv init -)"
`+pathBlock()+`
echo ready
`)

	if got := detectShadowingManagers([]string{rc}); len(got) != 0 {
		t.Fatalf("detectShadowingManagers() = %#v, want none", got)
	}
}

func TestDetectShadowingManagersMultipleFilesAndFalsePositives(t *testing.T) {
	afterManagers := writeTempRC(t, pathBlock()+`
eval "$(fnm env)"
eval "$(mise activate zsh)"
`)
	proseOnly := writeTempRC(t, pathBlock()+`
# This note mentions nvm, fnm, mise, and pyenv without installing any of them.
echo "plain manager names should not count"
`)
	missingBlockEnd := writeTempRC(t, blockStart+`
eval "$(pyenv init -)"
`)

	got := detectShadowingManagers([]string{proseOnly, missingBlockEnd, afterManagers})
	want := []string{"fnm", "mise"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("detectShadowingManagers() = %#v, want %#v", got, want)
	}
}

func TestStripBlockRemovesCurrentLegacyAndDuplicateBlocks(t *testing.T) {
	legacy := legacyBlock()
	input := strings.Join([]string{
		"before",
		pathBlock(),
		"between",
		pathBlock(),
		"old",
		legacy,
		"after",
	}, "\n") + "\n"

	got := stripBlock(input)
	want := "before\nbetween\nold\nafter\n"

	if got != want {
		t.Fatalf("stripBlock() = %q, want %q", got, want)
	}
}

func TestStripBlockLeavesPartialMarkersAlone(t *testing.T) {
	cases := []string{
		"before\n" + blockStart + "\nuser content\n",
		"before\n" + blockEnd + "\nuser content\n",
		"before\n" + legacyStart + "\nuser content\n",
	}

	for _, input := range cases {
		if got := stripBlock(input); got != input {
			t.Fatalf("stripBlock(%q) = %q, want unchanged", input, got)
		}
	}
}

func TestStripBlockSkipsPartialMarkerBeforeCompleteBlock(t *testing.T) {
	input := "before\n" + blockStart + "\nuser content\n" + pathBlock() + "\nafter\n"

	got := stripBlock(input)
	want := "before\n" + blockStart + "\nuser content\nafter\n"

	if got != want {
		t.Fatalf("stripBlock() = %q, want %q", got, want)
	}
}

func TestWriteRCBlocksInReplacesBlocksIdempotently(t *testing.T) {
	home := t.TempDir()
	zshrc := filepath.Join(home, ".zshrc")
	initial := "export PATH=/usr/bin:$PATH\n" + legacyBlock() + "\n" + pathBlock() + "\n"
	if err := os.WriteFile(zshrc, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	if got, want := writeRCBlocksIn(home, false), []string{".zshrc"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("first writeRCBlocksIn() = %#v, want %#v", got, want)
	}
	afterFirst, err := os.ReadFile(zshrc)
	if err != nil {
		t.Fatal(err)
	}
	content := string(afterFirst)
	if strings.Contains(content, legacyStart) || strings.Count(content, blockStart) != 1 {
		t.Fatalf("writeRCBlocksIn() did not replace managed blocks cleanly:\n%s", content)
	}

	if got := writeRCBlocksIn(home, false); len(got) != 0 {
		t.Fatalf("second writeRCBlocksIn() = %#v, want no changes", got)
	}
}

func legacyBlock() string {
	return legacyStart + "\n" +
		"alias dbt='bf dbt'\n" +
		legacyEnd
}

func writeTempRC(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "rc")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}
