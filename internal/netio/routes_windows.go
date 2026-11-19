package netio

import (
	"net/netip"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Route is one entry of Windows' route table, GetIpForwardTable2's: where it leads, the hop it
// goes through -- invalid for a destination on the link -- the interface it leaves by, its own
// metric, and how it came to be there.
type Route struct {
	Destination netip.Prefix
	NextHop     netip.Addr
	Index       int
	Metric      uint32
	// Protocol is MIB_IPPROTO's, 2 for the system's own and 3 for one set by hand or by DHCP,
	// and Origin NL_ROUTE_ORIGIN's: 0 by hand, 1 well known, 2 DHCP, 3 a router's advertisement.
	Protocol, Origin uint32
}

// mibForwardTableRows is where a MIB_IPFORWARD_TABLE2's rows begin. Not unsafe.Offsetof's: a row
// begins with a NET_LUID, which C aligns to 8 on every architecture, where Go on windows/386 aligns
// a uint64 to 4 and would put them at 4. The rows themselves have the same layout in both.
const mibForwardTableRows = 8

// Routes is the route table for family, windows.AF_INET or windows.AF_INET6.
func Routes(family uint16) ([]Route, error) {
	var table *windows.MibIpForwardTable2
	if err := windows.GetIpForwardTable2(family, &table); err != nil {
		return nil, err
	}
	defer windows.FreeMibTable(unsafe.Pointer(table))
	rows := unsafe.Slice((*windows.MibIpForwardRow2)(unsafe.Add(unsafe.Pointer(table), mibForwardTableRows)), table.NumEntries)
	routes := make([]Route, 0, len(rows))
	for _, row := range rows {
		destination, ok := inetAddress(&row.DestinationPrefix.Prefix)
		if !ok {
			continue
		}
		route := Route{
			Destination: netip.PrefixFrom(destination, int(row.DestinationPrefix.PrefixLength)),
			Index:       int(row.InterfaceIndex), Metric: row.Metric, Protocol: row.Protocol, Origin: row.Origin,
		}
		if hop, ok := inetAddress(&row.NextHop); ok && !hop.IsUnspecified() {
			route.NextHop = hop
		}
		routes = append(routes, route)
	}
	return routes, nil
}

// inetAddress is the address a SOCKADDR_INET holds.
func inetAddress(raw *windows.RawSockaddrInet) (netip.Addr, bool) {
	switch raw.Family {
	case windows.AF_INET:
		return netip.AddrFrom4((*windows.RawSockaddrInet4)(unsafe.Pointer(raw)).Addr), true
	case windows.AF_INET6:
		return netip.AddrFrom16((*windows.RawSockaddrInet6)(unsafe.Pointer(raw)).Addr), true
	}
	return netip.Addr{}, false
}
