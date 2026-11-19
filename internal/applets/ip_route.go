package applets

import (
	"cmp"
	"io"
	"net/netip"
	"slices"
	"strconv"
	"strings"

	"github.com/xiongnemo/nemosh/internal/netio"
)

// `ip route`, as iproute2's iproute.c prints it, from Windows' route table. Windows keeps in one
// table what Linux keeps in two, and `ip route` shows Linux's main one: an address's own host
// route, a subnet's broadcast, 255.255.255.255, multicast and everything on the loopback adapter
// are Linux's local table's, and are left out. The metric is the route's and the adapter's
// together, which is the one Windows chooses by and route print shows. The protocol is dhcp or ra
// for a route those gave, kernel for one the system made for an address, and static for the rest.
// An IPv6 route says `pref medium`, as Linux's does when nothing set another.

func writeIpRoutes(stdout io.Writer, source ipSource, settings ipSettings, device string) error {
	family := uint16(2) // AF_INET
	if settings.family == 6 {
		family = 23 // AF_INET6
	}
	interfaces, err := source.interfaces()
	if err != nil {
		return err
	}
	if _, err := ipDevice(interfaces, device); err != nil {
		return err
	}
	routes, err := source.routes(family)
	if err != nil {
		return err
	}
	byIndex := make(map[int]netio.Interface, len(interfaces))
	for _, adapter := range interfaces {
		byIndex[adapter.Index] = adapter
	}
	routes = slices.DeleteFunc(routes, func(route netio.Route) bool {
		adapter, known := byIndex[route.Index]
		return !known || adapter.Loopback || device != "" && adapter.Name != device || ipLocalRoute(route, adapter)
	})
	slices.SortStableFunc(routes, func(a, b netio.Route) int {
		if order := cmp.Compare(min(a.Destination.Bits(), 1), min(b.Destination.Bits(), 1)); order != 0 {
			return order
		}
		if order := a.Destination.Addr().Compare(b.Destination.Addr()); order != 0 {
			return order
		}
		return cmp.Compare(a.Destination.Bits(), b.Destination.Bits())
	})
	var out strings.Builder
	for _, route := range routes {
		adapter := byIndex[route.Index]
		fields := []string{route.Destination.String()}
		if route.Destination.Bits() == 0 {
			fields[0] = "default"
		}
		if route.NextHop.IsValid() {
			fields = append(fields, "via", route.NextHop.String())
		}
		fields = append(fields, "dev", adapter.Name, "proto", ipProtocol(route))
		metric := adapter.Metric4
		if family == 2 && !route.NextHop.IsValid() {
			fields = append(fields, "scope", "link")
			if from, ok := ipLinkAddress(adapter, route.Destination); ok {
				fields = append(fields, "src", from.String())
			}
		} else if family != 2 {
			metric = adapter.Metric6
		}
		fields = append(fields, "metric", strconv.FormatUint(uint64(route.Metric+metric), 10))
		if family != 2 {
			fields = append(fields, "pref", "medium")
		}
		out.WriteString(strings.Join(fields, " ") + " \n")
	}
	_, err = io.WriteString(stdout, out.String())
	return err
}

// ipLocalRoute is whether a route is one Linux keeps in its local table: multicast, the limited
// broadcast, or a host route to one of the adapter's addresses or to a subnet's broadcast.
func ipLocalRoute(route netio.Route, adapter netio.Interface) bool {
	destination := route.Destination
	if destination.Addr().IsMulticast() || destination.Addr() == netip.AddrFrom4([4]byte{255, 255, 255, 255}) {
		return true
	}
	if !destination.IsSingleIP() {
		return false
	}
	for _, address := range adapter.Addresses {
		prefix := address.Prefix
		if prefix.Addr() == destination.Addr() || prefix.Addr().Is4() && prefix.Bits() < 31 && ipBroadcast(prefix) == destination.Addr() {
			return true
		}
	}
	return false
}

// ipLinkAddress is the adapter's IPv4 address in destination, which a route on the link has as src.
func ipLinkAddress(adapter netio.Interface, destination netip.Prefix) (netip.Addr, bool) {
	for _, address := range adapter.Addresses {
		if address.Prefix.Addr().Is4() && destination.Contains(address.Prefix.Addr()) {
			return address.Prefix.Addr(), true
		}
	}
	return netip.Addr{}, false
}

func ipProtocol(route netio.Route) string {
	switch {
	case route.Origin == 2:
		return "dhcp"
	case route.Origin == 3:
		return "ra"
	case route.Protocol == 2:
		return "kernel"
	}
	return "static"
}
