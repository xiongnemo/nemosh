package applets

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/bits"
	"net/netip"
	"strconv"
	"strings"

	"github.com/xiongnemo/nemosh/internal/netio"
)

// ipcalc works out the network numbers people otherwise do in their heads and get wrong.
//
// The output is `KEY=value` lines in a fixed order -- NETMASK, BROADCAST, NETWORK, PREFIX,
// HOSTNAME -- whatever order the options were given in, because the point of the format is
// that a script can `eval` it.
//
// **A netmask with holes in it is not rejected.** `ipcalc -n 1.2.3.4 255.0.255.0` answers
// `1.0.3.0`, because the operation really is a bitwise and, and refusing the non-contiguous
// case would refuse a calculation that is perfectly well defined. That is what busybox does
// too, and its PREFIX is the bits the mask has set, 16 for that one, where it was 0. What *is*
// refused is a prefix outside 0..32 and an address that is not one.
func newIpcalcApplet() Applet {
	return simpleApplet{name: "ipcalc", runContext: func(ctx context.Context, args []string, _ io.Reader, stdout, _ io.Writer) error {
		options, operands, err := parseAppletLongOptions(ctx, args, ipcalcLongOptions, "bnmphs", "")
		if err != nil {
			return err
		}
		if len(operands) == 0 {
			return missingOperand()
		}
		if len(operands) > 2 {
			return fmt.Errorf("extra operand '%s'", operands[2])
		}
		if err := ipcalcAsked(options, operands); err != nil {
			return ExitStatusMessage(1, err)
		}
		lines, err := calculateIP(ctx, options, operands)
		if err != nil {
			if options.has('s') {
				// -s asks for the status without the diagnostic, which is what a script
				// testing an address wants.
				return ErrExitFalse
			}
			return ExitStatusMessage(1, err)
		}
		_, err = io.WriteString(stdout, strings.Join(lines, "\n")+"\n")
		return err
	}}
}

// parseIPv4 is a dotted IPv4 address as its 32 bits, or false for anything else.
func parseIPv4(text string) (uint32, bool) {
	address, err := netip.ParseAddr(text)
	if err != nil || !address.Is4() {
		return 0, false
	}
	octets := address.As4()
	return uint32(octets[0])<<24 | uint32(octets[1])<<16 | uint32(octets[2])<<8 | uint32(octets[3]), true
}

func ipv4Text(value uint32) string {
	return netip.AddrFrom4([4]byte{byte(value >> 24), byte(value >> 16), byte(value >> 8), byte(value)}).String()
}

func calculateIP(ctx context.Context, options appletOptions, operands []string) ([]string, error) {
	text, prefixText, hasPrefix := strings.Cut(operands[0], "/")
	address, ok := parseIPv4(text)
	if !ok {
		return nil, fmt.Errorf("bad IP address: %s", text)
	}
	mask, err := ipMask(prefixText, hasPrefix, operands, address)
	if err != nil {
		return nil, err
	}
	var lines []string
	if options.has('m') {
		lines = append(lines, "NETMASK="+ipv4Text(mask))
	}
	if options.has('b') {
		lines = append(lines, "BROADCAST="+ipv4Text(address|^mask))
	}
	if options.has('n') {
		lines = append(lines, "NETWORK="+ipv4Text(address&mask))
	}
	if options.has('p') {
		lines = append(lines, "PREFIX="+strconv.Itoa(bits.OnesCount32(mask)))
	}
	if options.has('h') {
		name, err := resolveIPName(ctx, ipv4Text(address))
		if err != nil {
			return nil, err
		}
		lines = append(lines, "HOSTNAME="+name)
	}
	return lines, nil
}

// ipcalcLongOptions are busybox's, by any prefix that names one alone. They were unrecognized.
const ipcalcLongOptions = "netmask=m broadcast=b network=n prefix=p hostname=h silent=s"

// ipcalcAsked is what busybox's ipcalc asks before it reads the address: with neither -b, -n
// nor -p, there must be an -m or an -h to answer, and no NETMASK, which only those three use.
// Either is its usage, which -s does not silence. They were asked after the address, so
// `ipcalc -s 300.1.1.1` said nothing where busybox's says what is wanted, and `ipcalc -m
// 10.0.0.5 255.255.255.0` printed back the netmask it was given.
func ipcalcAsked(options appletOptions, operands []string) error {
	switch {
	case options.has('b') || options.has('n') || options.has('p'):
		return nil
	case !options.has('m') && !options.has('h'):
		return errors.New("nothing was asked for; use -b, -n, -m, -p or -h")
	case len(operands) == 2:
		return errors.New("a NETMASK is only used by -b, -n or -p")
	}
	return nil
}

// ipMask answers the netmask: from a prefix after the address, from a netmask operand, holes
// and all, or from the old class rule.
//
// **The class rule is what makes a bare address answer anything at all.** Classful routing
// has not existed since 1993, but `ipcalc 10.1.2.3` still answers /8 in every version of
// this command, and a script reading NETMASK from it expects that number.
func ipMask(text string, present bool, operands []string, address uint32) (uint32, error) {
	prefix := 24
	switch {
	case present:
		// A netmask after the slash is no number, as busybox's xatoul_range reads it:
		// `invalid number '255.255.255.0'`. It was said to be out of range.
		value, err := xatouRange(text, 0, 32)
		if err != nil {
			return 0, err
		}
		prefix = value
	case len(operands) == 2:
		mask, ok := parseIPv4(operands[1])
		if !ok {
			return 0, fmt.Errorf("bad netmask: %s", operands[1])
		}
		return mask, nil
	case address>>24 < 128:
		prefix = 8
	case address>>24 < 192:
		prefix = 16
	}
	if prefix == 0 {
		return 0, nil
	}
	return ^uint32(0) << (32 - prefix), nil
}

// resolveIPName asks the resolver what the address is called, which is the one part of this
// command that talks to the network and so is done only when -h asks. One with no name is
// busybox's `can't find hostname for ADDRESS`, status 1, where it answered the address.
func resolveIPName(ctx context.Context, address string) (string, error) {
	names, err := lookupIPName(ctx, address)
	if err != nil || len(names) == 0 {
		reason := "no name"
		if err != nil {
			reason = err.Error()
		}
		return "", fmt.Errorf("can't find hostname for %s: %s", address, reason)
	}
	return strings.TrimSuffix(names[0], "."), nil
}

// lookupIPName is how -h asks for a name; a test makes it answer.
var lookupIPName = netio.LookupAddr
