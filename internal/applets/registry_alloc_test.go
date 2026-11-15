package applets

import (
	"context"
	"fmt"
	"io"
	"testing"
)

type registryProbeApplet struct{ name string }

func (a *registryProbeApplet) Name() string { return a.name }

func (*registryProbeApplet) Run(context.Context, []string, io.Reader, io.Writer, io.Writer) error {
	return nil
}

// DefaultRegistry is built before main on every start, so what building one costs per
// applet is paid by every run for every applet, where a run uses one or two. Wrapping
// each applet into the map's interface was an allocation apiece, 153 at start.
func TestNewRegistry_allocatesNothingPerApplet(t *testing.T) {
	// Given
	items := make([]Applet, 200)
	for index := range items {
		items[index] = &registryProbeApplet{name: fmt.Sprintf("probe-%d", index)}
	}

	// When
	allocations := testing.AllocsPerRun(20, func() { NewRegistry(items...) })

	// Then: the index and the list, and nothing that grows with the count
	if allocations > 20 {
		t.Fatalf("NewRegistry of %d applets allocated %v times; it should not allocate per applet", len(items), allocations)
	}
}

// Every command an applet runs is a lookup, so a lookup that allocated would be paid
// once per command.
func TestRegistry_lookupDoesNotAllocate(t *testing.T) {
	// When
	allocations := testing.AllocsPerRun(100, func() {
		if _, ok := DefaultRegistry.Lookup("true"); !ok {
			t.Fatal("DefaultRegistry has no true")
		}
	})

	// Then
	if allocations != 0 {
		t.Fatalf("Lookup allocated %v times", allocations)
	}
}

func TestRegistry_laterApplets_replaceEarlierOnesOfTheSameName(t *testing.T) {
	// Given
	first, second := &registryProbeApplet{name: "dup"}, &registryProbeApplet{name: "dup"}

	// When
	registry := NewRegistry(first, second)

	// Then: one name, and the later applet's
	applet, ok := registry.Lookup("dup")
	if !ok || len(registry.Names()) != 1 {
		t.Fatalf("Lookup = %v, Names = %v", ok, registry.Names())
	}
	if applet.(*contextApplet).Applet != second {
		t.Fatal("the earlier applet answered for the name")
	}
}
