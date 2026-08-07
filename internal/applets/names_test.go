package applets_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/xiongnemo/nemosh/internal/appletmanifest"
	"github.com/xiongnemo/nemosh/internal/applets"
)

func TestRegistryNames_areSortedAndComplete(t *testing.T) {
	// When
	names := applets.DefaultRegistry.Names()

	// Then
	if len(names) == 0 {
		t.Fatal("Names() is empty")
	}
	if !slices.IsSorted(names) {
		t.Fatalf("Names() is not sorted: %v", names)
	}
	for _, want := range []string{"cat", "echo", "grep", "ls", "test", "[", "winpath"} {
		if !slices.Contains(names, want) {
			t.Errorf("Names() is missing %q", want)
		}
	}
	for _, name := range names {
		if _, ok := applets.DefaultRegistry.Lookup(name); !ok {
			t.Errorf("Names() reported %q, which Lookup does not resolve", name)
		}
	}
}

func TestRegistryNames_returnsACopy(t *testing.T) {
	// Given
	first := applets.DefaultRegistry.Names()

	// When
	first[0] = "clobbered"

	// Then
	if second := applets.DefaultRegistry.Names(); second[0] == "clobbered" {
		t.Fatal("Names() handed out its own backing array")
	}
}

// `nemosh --list` is what generates Scoop shims, and the applet-manifest gate
// reads the registry's *source text* instead. If those two ever disagree the
// package would ship a shim for an applet that does not exist, or miss one that
// does, so the agreement is pinned here rather than assumed.
func TestRegistryNames_agreeWithTheManifestSourceParse(t *testing.T) {
	// Given
	source, err := os.ReadFile(filepath.Join("registry.go"))
	if err != nil {
		t.Fatal(err)
	}

	// When
	parsed := appletmanifest.ParseNemoshRegistry(string(source))
	runtimeNames := applets.DefaultRegistry.Names()

	// Then
	slices.Sort(parsed)
	parsed = slices.Compact(parsed)
	if !slices.Equal(parsed, runtimeNames) {
		t.Fatalf("source parse and runtime registry disagree:\n  source:  %v\n  runtime: %v", parsed, runtimeNames)
	}
}
