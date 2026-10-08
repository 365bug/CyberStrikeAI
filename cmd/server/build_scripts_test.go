package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Every `go build` in this repository's scripts has to target the cmd/server package.
// Building the single file cmd/server/main.go was tried once and broke the password reset
// entry: the file-based form silently leaves out sibling files of package main (it was
// update_cli.go that disappeared), and only `run.sh --reset-admin-password` used that
// build path, so nothing noticed until an operator needed it in an emergency. Compiling
// the package can never have that failure mode.
func TestScriptsBuildThePackageNotASingleFile(t *testing.T) {
	root := filepath.Join("..", "..")
	scripts := []string{filepath.Join(root, "run.sh"), filepath.Join(root, "upgrade.sh")}
	more, err := filepath.Glob(filepath.Join(root, "scripts", "*.sh"))
	if err != nil {
		t.Fatal(err)
	}
	scripts = append(scripts, more...)

	found := 0
	for _, script := range scripts {
		data, err := os.ReadFile(script)
		if err != nil {
			t.Fatalf("read %s: %v", script, err)
		}
		for _, line := range strings.Split(string(data), "\n") {
			if !strings.Contains(line, "go build") {
				continue
			}
			found++
			if strings.Contains(line, ".go") {
				t.Errorf("%s builds a single .go file; build ./cmd/server instead:\n  %s",
					script, strings.TrimSpace(line))
			}
		}
	}
	// A scan that finds no build command at all is looking in the wrong place, and a test
	// that cannot see the thing it guards passes for the wrong reason.
	if found == 0 {
		t.Fatal("no `go build` command found in the scripts")
	}
}
