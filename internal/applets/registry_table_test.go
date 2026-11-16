package applets

import (
	"slices"
	"strings"
	"testing"
)

// The default applets are a table the linker lays down, and an applet is made the first time
// its name is looked up. DefaultRegistry made every one of them before main, on every start,
// for the one or two a run uses: 321 package-init allocations in this package, the most of any.
func TestTableRegistry_makesAnAppletOnlyWhenItIsLookedUp(t *testing.T) {
	made := map[string]int{}
	maker := func(name string) func() Applet {
		return func() Applet {
			made[name]++
			return &registryProbeApplet{name: name}
		}
	}
	entries := []appletEntry{{"cat", maker("cat")}, {"ls", maker("ls")}, {"wc", maker("wc")}}

	// When
	registry := newTableRegistry(entries)

	// Then
	if len(made) != 0 {
		t.Fatalf("making the registry made %v", made)
	}
	first, ok := registry.Lookup("ls")
	second, _ := registry.Lookup("ls")
	if !ok || first != second || made["ls"] != 1 || len(made) != 1 {
		t.Fatalf("two lookups of ls made %v, and answered %p and %p", made, first, second)
	}
	if _, ok := registry.Lookup("nope"); ok {
		t.Fatal("a name not in the table was found")
	}
	if got := registry.Names(); !slices.Equal(got, []string{"cat", "ls", "wc"}) {
		t.Fatalf("Names = %v", got)
	}
}

// The table is sorted, which is how a name is found in it, and each row's applet answers to
// the row's name.
func TestDefaultApplets_areSortedAndAnswerToTheirNames(t *testing.T) {
	if !slices.IsSortedFunc(defaultApplets[:], func(a, b appletEntry) int { return strings.Compare(a.name, b.name) }) {
		t.Fatal("defaultApplets is not sorted by name")
	}
	for index, entry := range defaultApplets {
		if index > 0 && defaultApplets[index-1].name == entry.name {
			t.Errorf("%s is in the table twice", entry.name)
		}
		if got := entry.make().Name(); got != entry.name {
			t.Errorf("the row for %s makes an applet that answers to %s", entry.name, got)
		}
	}
	if !DefaultRegistry.IsDefault() || NewRegistry().IsDefault() {
		t.Fatal("IsDefault does not tell DefaultRegistry from another")
	}
}
