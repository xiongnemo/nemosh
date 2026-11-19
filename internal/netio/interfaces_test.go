package netio

import (
	"context"
	"net/netip"
	"slices"
	"testing"
)

// Interfaces is every adapter GetAdaptersAddresses knows, with its addresses as prefixes. Every
// Windows machine has the loopback one, up, holding 127.0.0.1/8.
func TestInterfaces_findsTheLoopback(t *testing.T) {
	interfaces, err := Interfaces()
	if err != nil {
		t.Fatal(err)
	}
	for _, adapter := range interfaces {
		if adapter.Loopback && adapter.Up && slices.Contains(adapter.Addresses, netip.MustParsePrefix("127.0.0.1/8")) {
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
