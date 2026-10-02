package runtime

import (
	"context"
	"errors"
	"fmt"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/proc"
)

// timeout is busybox-w32's applet (coreutils/timeout.c): `timeout [-s SIG] [-k KILL_SECS] SECS
// PROG ARGS` runs PROG and ends it once SECS have passed, answering 124 then, or 137 when SIG
// is KILL, and PROG's own status when it finishes first. SECS is sleep's `N[.N][smhd]`.
//
// The shell had none, so `timeout 5 cmd` found whatever PATH held. On a Windows that is
// System32's timeout.exe, a different program, which refused the line as invalid syntax and
// never ran cmd. PROG runs as any command does here, a function or a builtin too, and ends as
// Ctrl-C ends one: at once, so -k's second wait has nothing left to wait for. A bad option, a
// bad signal or a missing PROG is 125, as busybox answers on Windows.
func (r Runtime) timeoutBuiltin(ctx context.Context, args []string) int {
	signalName := "TERM"
	for len(args) > 0 && len(args[0]) > 1 && args[0][0] == '-' {
		option := args[0]
		args = args[1:]
		if option == "--" {
			break
		}
		letter, value := option[1], option[2:]
		if letter != 's' && letter != 'k' {
			// getopt's words, as both references say them on Windows, whose getopt is NetBSD's.
			name := string(letter)
			if letter == '-' {
				name = value
			}
			fmt.Fprintf(r.streams.Stderr, "timeout: unknown option -- %s\n", name)
			return 125
		}
		if value == "" {
			if len(args) == 0 {
				fmt.Fprintf(r.streams.Stderr, "timeout: option requires an argument -- %c\n", letter)
				return 125
			}
			value, args = args[0], args[1:]
		}
		if letter == 's' {
			signalName = value
		} else if _, err := applets.ParseDuration(value); err != nil {
			fmt.Fprintf(r.streams.Stderr, "timeout: invalid number '%s'\n", value)
			return 125
		}
	}
	signal, err := proc.ParseSignal(signalName)
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "timeout: unknown signal '%s'\n", signalName)
		return 125
	}
	if len(args) < 2 {
		fmt.Fprintln(r.streams.Stderr, "timeout: expected SECS and a command to run")
		return 125
	}
	limit, err := applets.ParseDuration(args[0])
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "timeout: invalid number '%s'\n", args[0])
		return 125
	}
	timed, cancel := context.WithTimeout(ctx, limit)
	defer cancel()
	status := r.runCommandResolved(timed, args[1:], true)
	if errors.Is(timed.Err(), context.DeadlineExceeded) && ctx.Err() == nil {
		if signal == 9 {
			return 137
		}
		return 124
	}
	return status
}
