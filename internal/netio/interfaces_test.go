package netio

import (
	"context"
	"net/netip"
	"slices"
	"testing"

	"golang.org/x/sys/windows"
)

// Interfaces is every adapter GetAdaptersAddresses knows, with its addresses as prefixes. Every
// Windows machine has the loopback one, up, holding 127.0.0.1/8.
func TestInterfaces_findsTheLoopback(t *testing.T) {
	interfaces, err := Interfaces()
	if err != nil {
		t.Fatal(err)
	}
	for _, adapter := range interfaces {
		if adapter.Loopback && adapter.Up && slices.ContainsFunc(adapter.Addresses, func(a Address) bool { return a.Prefix == netip.MustParsePrefix("127.0.0.1/8") }) {
			if adapter.Index == 0 || adapter.Name == "" {
				t.Errorf("the loopback adapter has index %d and name %q", adapter.Index, adapter.Name)
			}
			return
		}
	}
	t.Fatalf("no loopback adapter holding 127.0.0.1/8 among %+v", interfaces)
}

// LookupHost is the addresses a name has, by the resolver Dial asks, each once: an address is
// itself, and localhost is the loopback.
func TestLookupHost_resolvesAsDialDoes(t *testing.T) {
	addresses, err := LookupHost(context.Background(), "127.0.0.1")
	if err != nil || len(addresses) != 1 || addresses[0] != netip.MustParseAddr("127.0.0.1") {
		t.Errorf("LookupHost(127.0.0.1) = %v, %v", addresses, err)
	}
	addresses, err = LookupHost(context.Background(), "localhost")
	if err != nil || !slices.ContainsFunc(addresses, func(a netip.Addr) bool { return a.IsLoopback() }) {
		t.Errorf("LookupHost(localhost) = %v, %v", addresses, err)
	}
	if _, err := LookupHost(context.Background(), "nosuchhost.invalid"); err == nil {
		t.Error("LookupHost of a name that is not there succeeded")
	}
}

// Routes is the route table, read at C's layout: on every Windows machine 127.0.0.0/8 is on the
// link of the loopback interface, so a row read from the wrong place -- windows/386 puts Go's rows
// four bytes early -- names no such route, or the wrong interface.
func TestRoutes_findsTheLoopbacksRoute(t *testing.T) {
	interfaces, err := Interfaces()
	if err != nil {
		t.Fatal(err)
	}
	loopback := 0
	for _, adapter := range interfaces {
		if adapter.Loopback {
			loopback = adapter.Index
		}
	}
	routes, err := Routes(windows.AF_INET)
	if err != nil {
		t.Fatal(err)
	}
	for _, route := range routes {
		if route.Destination == netip.MustParsePrefix("127.0.0.0/8") && route.Index == loopback && !route.NextHop.IsValid() {
			return
		}
	}
	t.Fatalf("no route to 127.0.0.0/8 on the loopback, %d, among %+v", loopback, routes)
}
