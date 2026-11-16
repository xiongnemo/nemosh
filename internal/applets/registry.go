package applets

import (
	"context"
	"io"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync/atomic"
)

type Applet interface {
	Name() string
	Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error
}

// Registry is the applets by name: a table of how to make each (DefaultRegistry's), or the
// applets themselves, made by whoever built the registry (NewRegistry's).
//
// applets holds each one wrapped, and index finds one by name. Lookup hands out a pointer
// into applets, which an interface holds without an allocation, so neither building a
// registry nor looking an applet up allocates per applet. It was a map of wrapped applets,
// and putting each wrapper into the map's interface type was an allocation apiece.
type Registry struct {
	applets []contextApplet
	index   map[string]int
	table   *appletTable
}

// appletEntry is a row of a table: a name, and how to make the applet that answers to it.
type appletEntry struct {
	name string
	make func() Applet
}

// appletTable is a table of applets, sorted by name, and each applet once it has been made.
// An applet is made the first time its name is looked up, and every lookup after hands out
// that one. DefaultRegistry was every applet made before main, on every start, for the one or
// two a run uses: 321 package-init allocations in this package, the most of any.
type appletTable struct {
	entries []appletEntry
	made    []atomic.Pointer[contextApplet]
}

var (
	defaultMade  [len(defaultApplets)]atomic.Pointer[contextApplet]
	defaultTable = appletTable{entries: defaultApplets[:], made: defaultMade[:]}
)

// newTableRegistry is a registry of entries, which are sorted by name.
func newTableRegistry(entries []appletEntry) Registry {
	return Registry{table: &appletTable{entries: entries, made: make([]atomic.Pointer[contextApplet], len(entries))}}
}

func (t *appletTable) lookup(name string) (Applet, bool) {
	at, found := slices.BinarySearchFunc(t.entries, name, compareEntryName)
	if !found {
		return nil, false
	}
	if made := t.made[at].Load(); made != nil {
		return made, true
	}
	t.made[at].CompareAndSwap(nil, &contextApplet{Applet: t.entries[at].make()})
	return t.made[at].Load(), true
}

func compareEntryName(entry appletEntry, name string) int {
	return strings.Compare(entry.name, name)
}

func NewRegistry(items ...Applet) Registry {
	r := Registry{applets: make([]contextApplet, 0, len(items)), index: make(map[string]int, len(items))}
	for _, item := range items {
		// A later applet of a name replaces an earlier one, as it did in the map.
		if at, ok := r.index[item.Name()]; ok {
			r.applets[at] = contextApplet{Applet: item}
			continue
		}
		r.index[item.Name()] = len(r.applets)
		r.applets = append(r.applets, contextApplet{Applet: item})
	}
	return r
}

type contextApplet struct {
	Applet
}

func (a contextApplet) Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	// Before the applet's own parser, which is the whole point: each of them
	// rejected `--help` in its own words, and this is the one place all of them
	// pass through. See help_option.go for which applets are excluded and why.
	if helpRequested(a.Applet.Name(), args) && writeUsage(a.Applet.Name(), stdout) {
		return nil
	}
	return a.Applet.Run(ctx, args, readerFor(ctx, stdin), stdout, stderr)
}

func (r Registry) Lookup(name string) (Applet, bool) {
	if r.table != nil {
		return r.table.lookup(name)
	}
	at, ok := r.index[name]
	if !ok {
		return nil, false
	}
	return &r.applets[at], true
}

// Names lists every registered applet, sorted, as a fresh slice. It backs
// `nemosh --list`, whose output generates Scoop shims, so the order has to be
// stable across runs rather than a map's.
func (r Registry) Names() []string {
	if r.table != nil {
		names := make([]string, len(r.table.entries))
		for index, entry := range r.table.entries {
			names[index] = entry.name
		}
		return names
	}
	names := make([]string, 0, len(r.index))
	for name := range r.index {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func InvocationName(args []string) string {
	if len(args) == 0 || args[0] == "" {
		return ""
	}
	base := filepath.Base(args[0])
	if strings.EqualFold(filepath.Ext(base), ".exe") {
		base = base[:len(base)-len(".exe")]
	}
	return strings.ToLower(base)
}

// DefaultRegistry is every applet this build carries; see defaultApplets.
var DefaultRegistry = Registry{table: &defaultTable}

// IsDefault reports whether this is DefaultRegistry, or a copy of it: the applets another
// process of this binary would have. A background job that is a process can only be one
// where the shell's applets are those (docs/design/background-processes.md).
func (r Registry) IsDefault() bool {
	return r.table == &defaultTable
}
