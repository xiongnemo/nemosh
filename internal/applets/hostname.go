package applets

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"strings"

	"golang.org/x/sys/windows"

	"github.com/xiongnemo/nemosh/internal/netio"
)

// hostname, Debian's (the hostname package, hostname.c), which takes the name of Windows'
// hostname.exe: that one prints the name and nothing else, and answers -I with `sethostname: Use
// the Network Control Panel Applet`, so nothing of it is lost. busybox's is -s -i -d -f; Debian's
// adds -I, every address but the loopback's, which is what `hostname -I` is typed for, and -A.
//
//   - the name is Windows' DNS host name, as gethostname gives it, and -s cuts it at a dot;
//   - -f is the fully qualified name and -d its domain, as Windows keeps them, an empty line where
//     there is no domain;
//   - -i is what the name resolves to, -I every address of every adapter that is up but the
//     loopback's and IPv6's link-local ones, the IPv4 ones first, as getifaddrs lists them, and -A
//     the names those addresses have; each is followed by a space, as Debian prints them.
//
// Setting the name, NAME or -F FILE, is refused: on Windows that is Rename-Computer's, which takes
// an administrator and a restart. -y, the NIS domain, is refused too: Windows has none.

// The kinds of name GetComputerNameEx gives.
const (
	hostnameShort  = 1 // ComputerNameDnsHostname
	hostnameDomain = 2 // ComputerNameDnsDomain
	hostnameFull   = 3 // ComputerNameDnsFullyQualified
)

// hostnameSource is what hostname asks, which a test answers for itself.
type hostnameSource struct {
	names      func(kind uint32) (string, error)
	interfaces func() ([]netio.Interface, error)
	lookup     func(context.Context, string) ([]netip.Addr, error)
	reverse    func(context.Context, string) ([]string, error)
}

const hostnameLongOptions = "short=s fqdn=f long=f domain=d ip-address=i all-ip-addresses=I all-fqdns=A " +
	"alias=a verbose=v boot=b file=F yp=y nis=y"

func newHostnameApplet() Applet {
	return simpleApplet{name: "hostname", runContext: func(ctx context.Context, args []string, _ io.Reader, stdout, _ io.Writer) error {
		source := hostnameSource{names: computerName, interfaces: netio.Interfaces, lookup: netio.LookupHost, reverse: netio.LookupAddr}
		return runHostname(ctx, args, source, stdout)
	}}
}

func runHostname(ctx context.Context, args []string, source hostnameSource, stdout io.Writer) error {
	options, operands, err := parseAppletLongOptions(ctx, args, hostnameLongOptions, "sfdiIAavby", "F")
	if err != nil {
		return err
	}
	switch {
	case len(operands) > 0 || options.has('F'):
		return errors.New("the name is not set here: on Windows that is Rename-Computer's, which takes an administrator and a restart")
	case options.has('y'):
		return errors.New("Windows has no NIS domain to print")
	case options.has('i'):
		name, err := source.names(hostnameShort)
		if err != nil {
			return err
		}
		addresses, err := source.lookup(ctx, name)
		if err != nil {
			return fmt.Errorf("%s: %v", name, err)
		}
		return writeHostnameWords(stdout, addresses, func(address netip.Addr) string { return address.String() })
	case options.has('I'), options.has('A'):
		addresses, err := hostnameAddresses(source)
		if err != nil {
			return err
		}
		if options.has('I') {
			return writeHostnameWords(stdout, addresses, func(address netip.Addr) string { return address.String() })
		}
		return writeHostnameWords(stdout, addresses, func(address netip.Addr) string {
			names, err := source.reverse(ctx, address.String())
			if err != nil || len(names) == 0 {
				return ""
			}
			return strings.TrimSuffix(names[0], ".")
		})
	case options.has('a'):
		// Debian's aliases are /etc/hosts' for the name; Windows keeps none, and the line is empty.
		_, err := fmt.Fprintln(stdout)
		return err
	}
	kind := uint32(hostnameShort)
	switch {
	case options.has('f'):
		kind = hostnameFull
	case options.has('d'):
		kind = hostnameDomain
	}
	name, err := source.names(kind)
	if err != nil {
		return err
	}
	if options.has('s') {
		name, _, _ = strings.Cut(name, ".")
	}
	_, err = fmt.Fprintln(stdout, name)
	return err
}

// hostnameAddresses are -I's: every address of every adapter that is up, but the loopback's and
// IPv6's link-local ones, the IPv4 ones first.
func hostnameAddresses(source hostnameSource) ([]netip.Addr, error) {
	interfaces, err := source.interfaces()
	if err != nil {
		return nil, err
	}
	var four, six []netip.Addr
	for _, adapter := range interfaces {
		if !adapter.Up || adapter.Loopback {
			continue
		}
		for _, entry := range adapter.Addresses {
			switch address := entry.Prefix.Addr(); {
			case address.Is4():
				four = append(four, address)
			case !address.IsLinkLocalUnicast() && !address.IsLinkLocalMulticast():
				six = append(six, address)
			}
		}
	}
	return append(four, six...), nil
}

// writeHostnameWords writes each address as word gives it, followed by a space, and then a newline,
// as Debian's hostname writes them; an address word gives nothing for is left out.
func writeHostnameWords(stdout io.Writer, addresses []netip.Addr, word func(netip.Addr) string) error {
	var line strings.Builder
	for _, address := range addresses {
		if text := word(address); text != "" {
			line.WriteString(text + " ")
		}
	}
	line.WriteByte('\n')
	_, err := io.WriteString(stdout, line.String())
	return err
}

// computerName is one of Windows' names for this machine, by GetComputerNameEx.
func computerName(kind uint32) (string, error) {
	size := uint32(64)
	for {
		buffer := make([]uint16, size)
		err := windows.GetComputerNameEx(kind, &buffer[0], &size)
		if err == nil {
			return windows.UTF16ToString(buffer[:size]), nil
		}
		if !errors.Is(err, windows.ERROR_MORE_DATA) {
			return "", err
		}
	}
}
