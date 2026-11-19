package applets

import (
	"bytes"
	"context"
	"errors"
	"net/netip"
	"os"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/netio"
)

// hostname is Debian's: the name, -s short, -f the FQDN, -d the domain, -i the addresses the name
// resolves to, -I every address of every adapter that is up but the loopback's and IPv6's
// link-local ones, the IPv4 ones first, -A the names those have, each followed by a space, as
// Debian prints them. Setting the name is refused with the reason. It ran Windows' hostname.exe,
// which answers `sethostname: Use the Network Control Panel Applet` to -I.
func TestHostname_asDebiansDoes(t *testing.T) {
	source := hostnameSource{
		names: func(kind uint32) (string, error) {
			return map[uint32]string{hostnameShort: "box", hostnameFull: "box.example.com", hostnameDomain: "example.com"}[kind], nil
		},
		interfaces: func() ([]netio.Interface, error) {
			prefixes := func(texts ...string) []netip.Prefix {
				var out []netip.Prefix
				for _, text := range texts {
					out = append(out, netip.MustParsePrefix(text))
				}
				return out
			}
			return []netio.Interface{
				{Index: 1, Name: "Loopback Pseudo-Interface 1", Up: true, Loopback: true, Addresses: prefixes("127.0.0.1/8", "::1/128")},
				{Index: 7, Name: "Ethernet", Up: true, Addresses: prefixes("fe80::1/64", "2001:db8::5/64", "192.168.1.5/24")},
				{Index: 9, Name: "vEthernet", Up: true, Addresses: prefixes("10.1.1.1/20")},
				{Index: 12, Name: "Wi-Fi", Addresses: prefixes("10.0.0.2/24")},
			}, nil
		},
		lookup: func(_ context.Context, host string) ([]netip.Addr, error) {
			if host != "box" {
				return nil, errors.New("no such host")
			}
			return []netip.Addr{netip.MustParseAddr("192.168.1.5"), netip.MustParseAddr("2001:db8::5")}, nil
		},
		reverse: func(_ context.Context, address string) ([]string, error) {
			if address == "192.168.1.5" {
				return []string{"box.example.com"}, nil
			}
			return nil, errors.New("no name")
		},
	}
	for _, test := range []struct {
		args []string
		want string
	}{
		{nil, "box\n"},
		{[]string{"-s"}, "box\n"},
		{[]string{"--short"}, "box\n"},
		{[]string{"-f"}, "box.example.com\n"},
		{[]string{"--fqdn"}, "box.example.com\n"},
		{[]string{"--long"}, "box.example.com\n"},
		{[]string{"-d"}, "example.com\n"},
		{[]string{"-i"}, "192.168.1.5 2001:db8::5 \n"},
		{[]string{"-I"}, "192.168.1.5 10.1.1.1 2001:db8::5 \n"},
		{[]string{"--all-ip-addresses"}, "192.168.1.5 10.1.1.1 2001:db8::5 \n"},
		{[]string{"-A"}, "box.example.com \n"},
	} {
		var stdout bytes.Buffer
		if err := runHostname(context.Background(), test.args, source, &stdout); err != nil || stdout.String() != test.want {
			t.Errorf("hostname %q = %q, %v; want %q", test.args, stdout.String(), err, test.want)
		}
	}
	for _, args := range [][]string{{"newname"}, {"-F", "file"}, {"-y"}} {
		var stdout bytes.Buffer
		err := runHostname(context.Background(), args, source, &stdout)
		if err == nil || !strings.Contains(err.Error(), "Rename-Computer") && !strings.Contains(err.Error(), "NIS") {
			t.Errorf("hostname %q = %v; want it refused with the reason", args, err)
		}
	}
}

// On this machine hostname says what Go's os.Hostname says, which asks Windows the same thing.
func TestHostname_namesThisMachine(t *testing.T) {
	want, err := os.Hostname()
	if err != nil {
		t.Skip(err)
	}
	stdout, _, status := runApplet(t, "hostname", nil, "")
	if status != 0 || stdout != want+"\n" {
		t.Errorf("hostname = %q, status %d; want %q", stdout, status, want)
	}
	if addresses, _, status := runApplet(t, "hostname", []string{"-I"}, ""); status != 0 || !strings.HasSuffix(addresses, "\n") {
		t.Errorf("hostname -I = %q, status %d", addresses, status)
	}
}
