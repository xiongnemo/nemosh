package applets

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
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
// too. What *is* refused is a prefix outside 0..32 and an address that is not one.
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
		lines, err := calculateIP(options, operands)
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

func calculateIP(options appletOptions, operands []string) ([]string, error) {
	text, prefixText, hasPrefix := strings.Cut(operands[0], "/")
	address := net.ParseIP(text).To4()
	if address == nil {
		return nil, fmt.Errorf("bad IP address: %s", text)
	}
	prefix, err := ipPrefix(prefixText, hasPrefix, operands, address)
	if err != nil {
		return nil, err
	}
	mask := net.CIDRMask(prefix, 32)
	if len(operands) == 2 && !hasPrefix {
		// An explicit netmask operand is used as given, holes and all.
		given := net.ParseIP(operands[1]).To4()
		if given == nil {
			return nil, fmt.Errorf("bad netmask: %s", operands[1])
		}
		mask = net.IPMask(given)
	}
	var lines []string
	if options.has('m') {
		lines = append(lines, "NETMASK="+net.IP(mask).String())
	}
	if options.has('b') {
		lines = append(lines, "BROADCAST="+applyIPMask(address, mask, true).String())
	}
	if options.has('n') {
		lines = append(lines, "NETWORK="+applyIPMask(address, mask, false).String())
	}
	if options.has('p') {
		lines = append(lines, "PREFIX="+strconv.Itoa(prefix))
	}
	if options.has('h') {
		lines = append(lines, "HOSTNAME="+resolveIPName(address))
	}
	return lines, nil
}

// ipcalcLongOptions are busybox's, by any prefix that names one alone. They were unrecognized.
var ipcalcLongOptions = map[string]string{
	"netmask": "m", "broadcast": "b", "network": "n", "prefix": "p", "hostname": "h", "silent": "s",
}

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

// ipPrefix answers the prefix length, from the address, from a netmask operand, or from the
// old class rule.
//
// **The class rule is what makes a bare address answer anything at all.** Classful routing
// has not existed since 1993, but `ipcalc 10.1.2.3` still answers /8 in every version of
// this command, and a script reading NETMASK from it expects that number.
func ipPrefix(text string, present bool, operands []string, address net.IP) (int, error) {
	if present {
		// A netmask after the slash is no number, as busybox's xatoul_range reads it:
		// `invalid number '255.255.255.0'`. It was said to be out of range.
		return xatouRange(text, 0, 32)
	}
	if len(operands) == 2 {
		mask := net.ParseIP(operands[1]).To4()
		if mask == nil {
			return 0, fmt.Errorf("bad netmask: %s", operands[1])
		}
		ones, _ := net.IPMask(mask).Size()
		return ones, nil
	}
	switch {
	case address[0] < 128:
		return 8, nil
	case address[0] < 192:
		return 16, nil
	}
	return 24, nil
}

// applyIPMask answers the network address, or the broadcast address when the host bits are
// set rather than cleared.
func applyIPMask(address net.IP, mask net.IPMask, broadcast bool) net.IP {
	out := make(net.IP, 4)
	for index := 0; index < 4; index++ {
		if broadcast {
			out[index] = address[index] | ^mask[index]
			continue
		}
		out[index] = address[index] & mask[index]
	}
	return out
}

// resolveIPName asks the resolver what the address is called.
//
// The one part of this command that talks to the network, which is why it is only done when
// -h asks. An address with no name answers the address, rather than an error: the caller
// asked what it is called, and "itself" is a truthful answer.
func resolveIPName(address net.IP) string {
	names, err := net.LookupAddr(address.String())
	if err != nil || len(names) == 0 {
		return address.String()
	}
	return strings.TrimSuffix(names[0], ".")
}
