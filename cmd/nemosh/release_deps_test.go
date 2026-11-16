package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// releaseTags are the build tags every release build of nemosh is made with: product.yml's,
// release.yml's and scripts/build.sh's, and the measurement AGENTS.md gives.
//
// tcell_minimal leaves out the terminal descriptions tcell carries for terminals elsewhere,
// twenty-two packages of them, which nothing here can use: on Windows tcell draws as
// xterm-truecolor whatever $TERM says, and that description is in terminfo/base. 32,768 bytes
// and 43 package-init allocations, measured 2026-11-17.
const releaseTags = "tcell_minimal"

// forbiddenInRelease are packages a release build must not link. Each was taken out on purpose,
// and a change that brought one back would put back what taking it out saved.
var forbiddenInRelease = []string{
	"github.com/gdamore/tcell/v2/terminfo/extended",
	// go-colorful's HexColor, a database/sql Scanner and Valuer nothing here uses; see
	// third_party/lucasb-eyer-go-colorful.
	"database/sql/driver",
	// The background jobs' state has a codec of its own; see job_state_codec.go.
	"encoding/json",
}

// requiredInRelease are packages a release build must still link: without terminfo/base, tcell
// could not draw at all.
var requiredInRelease = []string{
	"github.com/gdamore/tcell/v2/terminfo/base",
}

// What a release build links, asked of the Go tool the way the release builds it.
func TestReleaseBuild_linksNothingTakenOut(t *testing.T) {
	list := exec.Command("go", "list", "-deps", "-tags", releaseTags, "./cmd/nemosh")
	list.Dir = filepath.Join("..", "..")
	list.Env = append(os.Environ(), "GOOS=windows", "GOARCH=amd64", "CGO_ENABLED=0")
	output, err := list.Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	packages := strings.Fields(string(output))
	for _, name := range forbiddenInRelease {
		if slices.Contains(packages, name) {
			t.Errorf("a release build links %s, which was taken out", name)
		}
	}
	for _, name := range requiredInRelease {
		if !slices.Contains(packages, name) {
			t.Errorf("a release build does not link %s", name)
		}
	}
}

// Every place nemosh is built for release builds it with releaseTags. A build without them is
// the larger binary, and a startup comparison against one built with them compares two
// different programs.
func TestReleaseBuild_everyBuildCarriesTheTags(t *testing.T) {
	root := filepath.Join("..", "..")
	files := []string{"scripts/build.sh", "AGENTS.md"}
	workflows, err := filepath.Glob(filepath.Join(root, ".github", "workflows", "*.yml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, workflow := range workflows {
		relative, _ := filepath.Rel(root, workflow)
		files = append(files, filepath.ToSlash(relative))
	}
	// A command, not a mention of one: a comment's or prose's `go build ./cmd/nemosh` is not run.
	build := regexp.MustCompile("(?m)^[^#`\n]*?(go build[^\n]*)")
	found := 0
	for _, name := range files {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		// A command continued with a backslash is one command.
		text := strings.ReplaceAll(strings.ReplaceAll(string(data), "\\\r\n", " "), "\\\n", " ")
		for _, match := range build.FindAllStringSubmatch(text, -1) {
			command := match[1]
			if !strings.Contains(command, "./cmd/nemosh") {
				continue
			}
			found++
			if !strings.Contains(command, "-tags "+releaseTags) {
				t.Errorf("%s builds nemosh without -tags %s: %s", name, releaseTags, strings.Join(strings.Fields(command), " "))
			}
		}
	}
	if found < 4 {
		t.Fatalf("found %d release builds of nemosh, so this proves nothing", found)
	}
}
