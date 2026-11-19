package applets

import (
	"bytes"
	"context"
	"net/netip"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/netio"
)

// ip is iproute2's, read: addr, link and route, as iproute2's ipaddress.c and iproute.c print
// them, field by field with the space after each, the IPv4 label at an address line's end, the
// lifetimes on a line of their own, and -br's columns of 16 and 14. What Windows has nothing
// behind -- a qdisc, a group, a queue length -- is left out rather than made up. The routes are
// the ones Linux keeps in its main table: an address's own host route, a broadcast, multicast and
// the loopback's are its local table's. A change is refused with the reason. There was no ip.
func TestIp_readsAsIproute2Does(t *testing.T) {
	source := ipSource{interfaces: ipFixtureInterfaces, routes: ipFixtureRoutes}
	lifetimes := "       valid_lft forever preferred_lft forever\n"
	loopback := "1: Loopback Pseudo-Interface 1: <LOOPBACK,UP,LOWER_UP> mtu 4294967295 state UP \n" +
		"    link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00\n"
	ethernet := "7: Ethernet: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 state UP \n" +
		"    link/ether 52:54:00:12:34:56 brd ff:ff:ff:ff:ff:ff\n"
	wifi := "12: Wi-Fi: <BROADCAST,MULTICAST> mtu 1500 state DOWN \n" +
		"    link/ether aa:bb:cc:dd:ee:ff brd ff:ff:ff:ff:ff:ff\n"
	inet := "    inet 192.168.1.5/24 brd 192.168.1.255 scope global dynamic Ethernet\n" +
		"       valid_lft 86000sec preferred_lft 86000sec\n"
	inet6 := "    inet6 fe80::5054:ff:fe12:3456/64 scope link \n" + lifetimes +
		"    inet6 2001:db8::5/64 scope global dynamic \n" +
		"       valid_lft 3600sec preferred_lft 1800sec\n"
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"a"}, loopback + "    inet 127.0.0.1/8 scope host Loopback Pseudo-Interface 1\n" + lifetimes +
			"    inet6 ::1/128 scope host \n" + lifetimes + ethernet + inet + inet6 + wifi},
		{[]string{"addr", "show", "dev", "Ethernet"}, ethernet + inet + inet6},
		{[]string{"-4", "address", "show", "Ethernet"}, ethernet + inet},
		{[]string{"-6", "a", "s", "Ethernet"}, ethernet + inet6},
		{[]string{"link"}, loopback + ethernet + wifi},
		{[]string{"-br", "a"}, "Loopback Pseudo-Interface 1 UP             127.0.0.1/8 ::1/128 \n" +
			"Ethernet         UP             192.168.1.5/24 fe80::5054:ff:fe12:3456/64 2001:db8::5/64 \n" +
			"Wi-Fi            DOWN           \n"},
		{[]string{"-br", "-4", "a"}, "Loopback Pseudo-Interface 1 UP             127.0.0.1/8 \n" +
			"Ethernet         UP             192.168.1.5/24 \n"},
		{[]string{"-br", "link"}, "Loopback Pseudo-Interface 1 UP             00:00:00:00:00:00 <LOOPBACK,UP,LOWER_UP> \n" +
			"Ethernet         UP             52:54:00:12:34:56 <BROADCAST,MULTICAST,UP,LOWER_UP> \n" +
			"Wi-Fi            DOWN           aa:bb:cc:dd:ee:ff <BROADCAST,MULTICAST> \n"},
		{[]string{"-o", "-4", "a", "show", "Ethernet"}, "7: Ethernet    inet 192.168.1.5/24 brd 192.168.1.255 scope global dynamic Ethernet\\       valid_lft 86000sec preferred_lft 86000sec\n"},
		{[]string{"r"}, "default via 192.168.1.1 dev Ethernet proto dhcp metric 25 \n" +
			"10.9.0.0/16 via 192.168.1.254 dev Ethernet proto static metric 35 \n" +
			"192.168.1.0/24 dev Ethernet proto kernel scope link src 192.168.1.5 metric 281 \n"},
		{[]string{"-6", "route", "show"}, "default via fe80::1 dev Ethernet proto ra metric 25 pref medium \n" +
			"2001:db8::/64 dev Ethernet proto kernel metric 281 pref medium \n" +
			"fe80::/64 dev Ethernet proto kernel metric 281 pref medium \n"},
	} {
		var stdout bytes.Buffer
		if err := runIp(context.Background(), test.args, source, &stdout); err != nil || stdout.String() != test.want {
			t.Errorf("ip %q = %v\n%s\nwant\n%s", test.args, err, stdout.String(), test.want)
		}
	}
	for _, test := range []struct {
		args []string
		says string
	}{
		{[]string{"a", "show", "dev", "eth9"}, `Device "eth9" does not exist.`},
		{[]string{"addr", "add", "10.0.0.1/24", "dev", "Ethernet"}, "New-NetIPAddress"},
		{[]string{"route", "del", "default"}, "route"},
		{[]string{"link", "set", "Ethernet", "down"}, "Disable-NetAdapter"},
		{[]string{"neigh"}, `Object "neigh" is unknown`},
	} {
		var stdout bytes.Buffer
		if err := runIp(context.Background(), test.args, source, &stdout); err == nil || !strings.Contains(err.Error(), test.says) {
			t.Errorf("ip %q = %v; want %q", test.args, err, test.says)
		}
	}
}

// On this machine ip reads what Windows has: the loopback's address, and both route tables.
func TestIp_readsThisMachine(t *testing.T) {
	addresses, stderr, status := runApplet(t, "ip", []string{"a"}, "")
	if status != 0 || !strings.Contains(addresses, "    inet 127.0.0.1/8 scope host ") {
		t.Errorf("ip a = %q, %s, status %d", addresses, stderr, status)
	}
	for _, args := range [][]string{{"-br", "a"}, {"r"}, {"-6", "r"}} {
		out, stderr, status := runApplet(t, "ip", args, "")
		if status != 0 {
			t.Errorf("ip %q = %s, status %d", args, stderr, status)
		}
		t.Logf("ip %s:\n%s", strings.Join(args, " "), out)
	}
}

func ipFixtureInterfaces() ([]netio.Interface, error) {
	address := func(text string, valid, preferred uint32) netio.Address {
		return netio.Address{Prefix: netip.MustParsePrefix(text), Valid: valid, Preferred: preferred}
	}
	forever := netio.AddressForever
	return []netio.Interface{
		{Index: 1, Name: "Loopback Pseudo-Interface 1", MTU: 4294967295, Up: true, Loopback: true, State: 1, Type: 24, Multicast: true,
			Addresses: []netio.Address{address("::1/128", forever, forever), address("127.0.0.1/8", forever, forever)}, Metric4: 75, Metric6: 75},
		{Index: 7, Name: "Ethernet", MTU: 1500, Up: true, State: 1, Type: 6, Multicast: true, Hardware: []byte{0x52, 0x54, 0, 0x12, 0x34, 0x56},
			Addresses: []netio.Address{address("fe80::5054:ff:fe12:3456/64", forever, forever), address("2001:db8::5/64", 3600, 1800), address("192.168.1.5/24", 86000, 86000)},
			Gateways:  []netip.Addr{netip.MustParseAddr("192.168.1.1")}, Metric4: 25, Metric6: 25},
		{Index: 12, Name: "Wi-Fi", MTU: 1500, State: 2, Type: 71, Multicast: true, Hardware: []byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}, Metric4: 35, Metric6: 35},
	}, nil
}

func ipFixtureRoutes(family uint16) ([]netio.Route, error) {
	route := func(destination, hop string, index int, metric, protocol, origin uint32) netio.Route {
		found := netio.Route{Destination: netip.MustParsePrefix(destination), Index: index, Metric: metric, Protocol: protocol, Origin: origin}
		if hop != "" {
			found.NextHop = netip.MustParseAddr(hop)
		}
		return found
	}
	if family == 2 {
		return []netio.Route{
			route("192.168.1.0/24", "", 7, 256, 2, 1),
			route("0.0.0.0/0", "192.168.1.1", 7, 0, 3, 2),
			route("127.0.0.0/8", "", 1, 256, 2, 1),
			route("127.0.0.1/32", "", 1, 256, 2, 1),
			route("192.168.1.5/32", "", 7, 256, 2, 1),
			route("192.168.1.255/32", "", 7, 256, 2, 1),
			route("224.0.0.0/4", "", 7, 256, 2, 1),
			route("255.255.255.255/32", "", 7, 256, 2, 1),
			route("10.9.0.0/16", "192.168.1.254", 7, 10, 3, 0),
		}, nil
	}
	return []netio.Route{
		route("::/0", "fe80::1", 7, 0, 3, 3),
		route("::1/128", "", 1, 256, 2, 1),
		route("fe80::/64", "", 7, 256, 2, 1),
		route("2001:db8::/64", "", 7, 256, 2, 1),
		route("2001:db8::5/128", "", 7, 256, 2, 1),
		route("fe80::5054:ff:fe12:3456/128", "", 7, 256, 2, 1),
		route("ff00::/8", "", 7, 256, 2, 1),
	}, nil
}
