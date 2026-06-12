package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const aliasStart = "# >>> backfill aliases >>>"
const aliasEnd = "# <<< backfill aliases <<<"

var wrapCommands = []string{
	"dbt", "sqlmesh", "bq", "snowsql", "spark-submit",
	"cargo", "docker", "gradle", "xcodebuild",
}

func aliasBlock() string {
	var b strings.Builder
	b.WriteString(aliasStart + "\n")
	for _, c := range wrapCommands {
		fmt.Fprintf(&b, "alias %s='bf %s'\n", c, c)
	}
	b.WriteString(aliasEnd)
	return b.String()
}

func stripBlock(s string) string {
	start := strings.Index(s, aliasStart)
	end := strings.Index(s, aliasEnd)
	if start == -1 || end == -1 {
		return s
	}
	tail := s[end+len(aliasEnd):]
	tail = strings.TrimPrefix(tail, "\n")
	return strings.TrimRight(s[:start], "\n") + "\n" + tail
}

func cmdAliases(remove bool) {
	if !remove {
		fmt.Printf("This adds the following block to your shell rc files:\n\n%s\n\n", aliasBlock())
		if !confirm("Proceed? [y/N] ") {
			fmt.Println("aborted")
			return
		}
	}
	home, _ := os.UserHomeDir()
	var changed []string
	for _, rc := range []string{".zshrc", ".bashrc"} {
		p := filepath.Join(home, rc)
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		s := stripBlock(string(b))
		if !remove {
			s = strings.TrimRight(s, "\n") + "\n\n" + aliasBlock() + "\n"
		}
		if s != string(b) {
			if err := os.WriteFile(p, []byte(s), 0o644); err == nil {
				changed = append(changed, rc)
			}
		}
	}
	if len(changed) == 0 {
		fmt.Println("nothing to change")
		return
	}
	verb := "added to"
	if remove {
		verb = "removed from"
	}
	fmt.Printf("aliases %s %s — restart your shell or run: source ~/%s\n",
		verb, strings.Join(changed, ", "), changed[0])
}

func confirm(prompt string) bool {
	fmt.Print(prompt)
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	line = strings.ToLower(strings.TrimSpace(line))
	return line == "y" || line == "yes"
}
