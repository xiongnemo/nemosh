package applets

import (
	"fmt"
	"strconv"
	"strings"
)

// GNU's long options for head and tail, which busybox's have not got: --lines, --bytes, --quiet,
// --silent and --verbose for both, and tail's --follow, --retry, --sleep-interval and --pid.
// `tail -f --pid=$! log` follows a log until the process writing it ends.
//
// --follow is -f, and --follow=name is -F, which retries as well, as busybox's -F does: GNU's
// --follow=name gives up a FILE that goes away and only -F, which is --follow=name --retry, waits
// for it. --retry is -F's retry, so with -f it is -F.

// readHeadTailLong reads one --NAME[=VALUE], reporting how many arguments after it it took, or
// -1 when it is no option of this applet's.
func readHeadTailLong(arg string, args []string, index int, allowBytes bool, spec *countSpec, headers *headerMode, follow *tailFollow) (int, error) {
	name, value, attached := strings.Cut(arg[2:], "=")
	consumed := 0
	valued := func() (string, error) {
		if attached {
			return value, nil
		}
		if index+1 >= len(args) {
			return "", missingOptionArgument(name)
		}
		consumed = 1
		return args[index+1], nil
	}
	flag := func(set func()) error {
		if attached {
			return fmt.Errorf("--%s takes no argument", name)
		}
		set()
		return nil
	}
	var err error
	switch {
	case name == "quiet" || name == "silent":
		err = flag(func() { *headers = headersNever })
	case name == "verbose":
		err = flag(func() { *headers = headersAlways })
	case name == "lines" || name == "bytes" && allowBytes:
		var text string
		if text, err = valued(); err == nil {
			*spec, err = parseCountSpec(text, name == "bytes")
		}
	case follow == nil:
		return -1, nil
	case name == "follow":
		follow.follow = true
		switch value {
		case "name":
			follow.retry = true
		case "", "descriptor":
		default:
			err = fmt.Errorf("--follow takes name or descriptor, not '%s'", value)
		}
	case name == "retry":
		err = flag(func() { follow.retry = true })
	case name == "sleep-interval":
		var text string
		if text, err = valued(); err == nil {
			follow.period, err = positiveNumber(text)
		}
	case name == "pid":
		var text string
		if text, err = valued(); err == nil {
			follow.pid, err = strconv.Atoi(text)
			if err != nil || follow.pid <= 0 {
				err = fmt.Errorf("invalid PID: '%s'", text)
			}
		}
	default:
		return -1, nil
	}
	return consumed, err
}
