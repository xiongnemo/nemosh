package applets

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/xiongnemo/nemosh/internal/netio"
)

// ip, iproute2's, to read with: `ip a`, `ip -br a`, `ip link`, `ip r`, with -4, -6, -br and -o,
// and `show dev NAME`. busybox-w32 has no ip. What it prints is iproute2's, field by field (see
// ip_addr.go and ip_route.go), from what Windows' IP Helper says: the adapters by
// GetAdaptersAddresses, under the names Windows shows -- Ethernet, Wi-Fi -- and the routes by
// GetIpForwardTable2. A change, `ip addr add`, `ip link set`, `ip route add` and the like, is
// refused with the PowerShell command that makes it, as an administrator; an object other than
// addr, link and route is refused by name.

// ipSource is what ip asks, which a test answers for itself.
type ipSource struct {
	interfaces func() ([]netio.Interface, error)
	routes     func(family uint16) ([]netio.Route, error)
}

// ipSettings are the options before the object: the family, 4 or 6 or 0 for both, -br and -o.
type ipSettings struct {
	family         int
	brief, oneline bool
}

func newIpApplet() Applet {
	return simpleApplet{name: "ip", runContext: func(ctx context.Context, args []string, _ io.Reader, stdout, _ io.Writer) error {
		return runIp(ctx, args, ipSource{interfaces: netio.Interfaces, routes: netio.Routes}, stdout)
	}}
}

func runIp(_ context.Context, args []string, source ipSource, stdout io.Writer) error {
	var settings ipSettings
	for len(args) > 0 && strings.HasPrefix(args[0], "-") {
		switch option := strings.TrimPrefix(strings.TrimPrefix(args[0], "-"), "-"); {
		case option == "4":
			settings.family = 4
		case option == "6":
			settings.family = 6
		case len(option) >= 2 && ipMatches(option, "brief"):
			// At least -br: iproute2's -b alone is -batch, which this ip has not.
			settings.brief = true
		case ipMatches(option, "oneline"):
			settings.oneline = true
		default:
			return fmt.Errorf(`option "%s" is unknown, try "ip -help".`, args[0])
		}
		args = args[1:]
	}
	if len(args) == 0 {
		return errors.New("Usage: ip [ OPTIONS ] OBJECT { COMMAND | help }, OBJECT being address, link or route here")
	}
	object, rest := args[0], args[1:]
	command := "show"
	if len(rest) > 0 {
		command, rest = rest[0], rest[1:]
	}
	if !ipMatches(command, "show") && !ipMatches(command, "list") && command != "lst" {
		return ipRefusal(object, command)
	}
	device := ""
	for index := 0; index < len(rest); index++ {
		if rest[index] == "dev" && index+1 < len(rest) {
			index++
		}
		device = rest[index]
	}
	switch {
	case ipMatches(object, "address"):
		return writeIpAddresses(stdout, source, settings, device, true)
	case ipMatches(object, "link"):
		return writeIpAddresses(stdout, source, settings, device, false)
	case ipMatches(object, "route"):
		return writeIpRoutes(stdout, source, settings, device)
	}
	return fmt.Errorf(`Object "%s" is unknown here; this ip reads address, link and route`, object)
}

// ipMatches is iproute2's matches(): word is the name, or the start of it.
func ipMatches(word, name string) bool {
	return word != "" && strings.HasPrefix(name, word)
}

// ipRefusal is a command that would change something, refused with what makes the change on
// Windows, or one that is not a command at all.
func ipRefusal(object, command string) error {
	for _, change := range []string{"add", "delete", "change", "replace", "flush", "set", "append", "prepend"} {
		if !ipMatches(command, change) {
			continue
		}
		switch {
		case ipMatches(object, "address"):
			return errors.New("ip changes nothing here: an address is New-NetIPAddress's or Remove-NetIPAddress's, as an administrator")
		case ipMatches(object, "link"):
			return errors.New("ip changes nothing here: an adapter is Disable-NetAdapter's, Enable-NetAdapter's or Set-NetIPInterface's, as an administrator")
		case ipMatches(object, "route"):
			return errors.New("ip changes nothing here: a route is New-NetRoute's or Remove-NetRoute's, as an administrator")
		}
	}
	return fmt.Errorf(`Command "%s" is unknown, try "ip %s help".`, command, object)
}

// ipDevice is the adapter named device, or every one when device is empty.
func ipDevice(interfaces []netio.Interface, device string) ([]netio.Interface, error) {
	if device == "" {
		return interfaces, nil
	}
	for _, adapter := range interfaces {
		if adapter.Name == device {
			return []netio.Interface{adapter}, nil
		}
	}
	return nil, fmt.Errorf(`Device "%s" does not exist.`, device)
}
