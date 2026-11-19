package applets

import (
	"fmt"
	"io"
	"net/netip"
	"strings"

	"github.com/rivo/uniseg"

	"github.com/xiongnemo/nemosh/internal/netio"
)

// `ip addr` and `ip link`, as iproute2's ipaddress.c prints them: each field followed by its space,
// so a line can end in one; an IPv4 address's label, which is the adapter's name, at its end; the
// lifetimes on a line of their own; and under -br the name in 16 columns and the state in 14.
//
// The first line has no qdisc, group or qlen: Windows has nothing behind them, and iproute2's
// would be made up. UP and LOWER_UP say the link is up as Windows reports it, an adapter that is
// down showing neither, for GetAdaptersAddresses keeps no administrative state apart. The state is
// RFC 2863's, which Windows and Linux both report, so the loopback is UP where Linux's says
// UNKNOWN. With -4 or -6 an adapter with no address of that family is left out, as iproute2 leaves
// it out.

var ipStates = [...]string{"UNKNOWN", "UP", "DOWN", "TESTING", "UNKNOWN", "DORMANT", "NOTPRESENT", "LOWERLAYERDOWN"}

func writeIpAddresses(stdout io.Writer, source ipSource, settings ipSettings, device string, addresses bool) error {
	interfaces, err := source.interfaces()
	if err != nil {
		return err
	}
	if interfaces, err = ipDevice(interfaces, device); err != nil {
		return err
	}
	var out strings.Builder
	for _, adapter := range interfaces {
		var four, six []netio.Address
		for _, address := range adapter.Addresses {
			if address.Prefix.Addr().Is4() {
				four = append(four, address)
			} else {
				six = append(six, address)
			}
		}
		shown := append(ipFamily(settings.family, 4, four), ipFamily(settings.family, 6, six)...)
		if settings.family != 0 && len(shown) == 0 {
			continue
		}
		switch {
		case settings.brief && addresses:
			out.WriteString(ipColumn(adapter.Name, 16) + ipColumn(ipState(adapter), 14))
			for _, address := range shown {
				out.WriteString(address.Prefix.String() + " ")
			}
			out.WriteByte('\n')
		case settings.brief:
			out.WriteString(ipColumn(adapter.Name, 16) + ipColumn(ipState(adapter), 14))
			if hardware := ipHardware(adapter); hardware != "" {
				out.WriteString(hardware + " ")
			}
			out.WriteString("<" + ipFlags(adapter) + "> \n")
		case settings.oneline && addresses:
			for _, address := range shown {
				line, lifetimes := ipAddressLines(adapter, address)
				fmt.Fprintf(&out, "%d: %s%s\\%s\n", adapter.Index, adapter.Name, line, lifetimes)
			}
		default:
			first, link := ipLinkLines(adapter)
			if settings.oneline {
				fmt.Fprintf(&out, "%s\\%s\n", first, link)
				continue
			}
			out.WriteString(first + "\n" + link + "\n")
			if !addresses {
				continue
			}
			for _, address := range shown {
				line, lifetimes := ipAddressLines(adapter, address)
				out.WriteString(line + "\n" + lifetimes + "\n")
			}
		}
	}
	_, err = io.WriteString(stdout, out.String())
	return err
}

// ipFamily is addresses when the family asked for is theirs, or none was.
func ipFamily(asked, family int, addresses []netio.Address) []netio.Address {
	if asked != 0 && asked != family {
		return nil
	}
	return addresses
}

// ipLinkLines are an adapter's first two lines, without their newlines.
func ipLinkLines(adapter netio.Interface) (string, string) {
	first := fmt.Sprintf("%d: %s: <%s> mtu %d state %s ", adapter.Index, adapter.Name, ipFlags(adapter), adapter.MTU, ipState(adapter))
	switch hardware := ipHardware(adapter); {
	case adapter.Loopback:
		return first, "    link/loopback " + hardware + " brd " + hardware
	case len(adapter.Hardware) == 6:
		return first, "    link/ether " + hardware + " brd ff:ff:ff:ff:ff:ff"
	}
	return first, "    link/none"
}

// ipAddressLines are an address's line and its lifetimes' line, without their newlines.
func ipAddressLines(adapter netio.Interface, address netio.Address) (string, string) {
	family, prefix := "inet6", address.Prefix
	if prefix.Addr().Is4() {
		family = "inet"
	}
	line := fmt.Sprintf("    %s %s ", family, prefix)
	if family == "inet" && !adapter.Loopback && prefix.Bits() < 31 {
		line += "brd " + ipBroadcast(prefix).String() + " "
	}
	line += "scope " + ipScope(prefix.Addr()) + " "
	if address.Valid != netio.AddressForever {
		line += "dynamic "
	}
	if family == "inet" {
		line += adapter.Name
	}
	return line, "       valid_lft " + ipLifetime(address.Valid) + " preferred_lft " + ipLifetime(address.Preferred)
}

// ipFlags are the flags iproute2 prints between < and >, in its order.
func ipFlags(adapter netio.Interface) string {
	var flags []string
	switch adapter.Type {
	case 24:
		flags = append(flags, "LOOPBACK")
	case 6, 71:
		flags = append(flags, "BROADCAST")
	case 23, 131:
		flags = append(flags, "POINTOPOINT")
	}
	if adapter.Multicast && !adapter.Loopback {
		flags = append(flags, "MULTICAST")
	}
	if adapter.Up {
		flags = append(flags, "UP", "LOWER_UP")
	}
	return strings.Join(flags, ",")
}

func ipState(adapter netio.Interface) string {
	if int(adapter.State) < len(ipStates) {
		return ipStates[adapter.State]
	}
	return "UNKNOWN"
}

// ipHardware is the adapter's address as iproute2 writes a link's, the loopback's all zeros.
func ipHardware(adapter netio.Interface) string {
	if adapter.Loopback && len(adapter.Hardware) == 0 {
		return "00:00:00:00:00:00"
	}
	parts := make([]string, len(adapter.Hardware))
	for index, octet := range adapter.Hardware {
		parts[index] = fmt.Sprintf("%02x", octet)
	}
	return strings.Join(parts, ":")
}

// ipScope is the address's scope, as Linux gives one: host for the loopback's, link for a
// link-local one, global for the rest.
func ipScope(address netip.Addr) string {
	switch {
	case address.IsLoopback():
		return "host"
	case address.IsLinkLocalUnicast():
		return "link"
	}
	return "global"
}

// ipBroadcast is the last address of prefix, which Windows does not keep and Linux prints as brd.
func ipBroadcast(prefix netip.Prefix) netip.Addr {
	bytes := prefix.Masked().Addr().As4()
	for bit := prefix.Bits(); bit < 32; bit++ {
		bytes[bit/8] |= 0x80 >> (bit % 8)
	}
	return netip.AddrFrom4(bytes)
}

// ipColumn is text padded to width columns and a space, as iproute2's %-16s pads a name -- by the
// columns a character takes rather than its bytes, for Windows names an adapter in the language it
// speaks, 以太网 for Ethernet, where a Linux name is ASCII.
func ipColumn(text string, width int) string {
	return text + strings.Repeat(" ", max(0, width-uniseg.StringWidth(text))) + " "
}

func ipLifetime(seconds uint32) string {
	if seconds == netio.AddressForever {
		return "forever"
	}
	return fmt.Sprintf("%dsec", seconds)
}
