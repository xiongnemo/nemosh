package completions_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/completions"
	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/capability"
	"github.com/xiongnemo/nemosh/internal/completionspec"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// Every bundled spec has to parse, and has to say where it came from.
//
// This is the only gate these files can have. CI cannot run adb or curl to check
// a word of them, so what is enforced is what can be: the shape is understood in
// full, the claims are internally consistent, and the provenance is present so a
// reader can weigh how stale a file may be.
func TestBundledSpecs_parseAndCarryTheirProvenance(t *testing.T) {
	entries, err := completions.Files.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("no bundled specs at all: the embed pattern matched nothing")
	}
	for _, entry := range entries {
		t.Run(entry.Name(), func(t *testing.T) {
			// Given
			source, err := completions.Files.ReadFile(entry.Name())
			if err != nil {
				t.Fatal(err)
			}
			name := strings.TrimSuffix(entry.Name(), ".toml")

			// When
			spec, err := completionspec.Parse(name, source)

			// Then
			if err != nil {
				t.Fatalf("%v", err)
			}
			// A version string that says nothing is worse than none, because it
			// looks like the question was answered.
			if len(spec.Meta.ToolVersion) < 4 {
				t.Fatalf("tool-version = %q, want something a reader could compare against their own", spec.Meta.ToolVersion)
			}
		})
	}
}

// A spec must not describe a command this shell ships.
//
// internal/capability is bound to behaviour by a test that runs each applet; a
// file that could override `ls` would take that guarantee away and replace it
// with data nobody checked. So the two sources cannot overlap, and the rule is
// enforced rather than written down.
func TestBundledSpecs_doNotShadowWhatThisShellShips(t *testing.T) {
	entries, err := completions.Files.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	shipped := map[string]bool{}
	for _, name := range applets.DefaultRegistry.Names() {
		shipped[name] = true
	}
	for _, name := range runtime.BuiltinNames() {
		shipped[name] = true
	}
	for _, entry := range entries {
		name := strings.TrimSuffix(entry.Name(), ".toml")
		if shipped[name] {
			t.Errorf("%s describes an applet or builtin; those are measured in internal/capability", entry.Name())
		}
		if capability.Known(name) {
			t.Errorf("%s is also a row in internal/capability, so two sources describe it", entry.Name())
		}
	}
}

// The README documents the format, and a format nobody can find is a format
// nobody contributes to.
func TestCompletionsDirectory_documentsItself(t *testing.T) {
	readme, err := os.ReadFile(filepath.Join("README.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{"value-short", "file-short", "[[subcommand]]", "generate.py"} {
		if !strings.Contains(string(readme), fragment) {
			t.Errorf("README.md does not mention %q", fragment)
		}
	}
}
