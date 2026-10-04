package runtime

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// watch is busybox's applet (procps/watch.c), a builtin here because it runs a command line as
// the shell runs one. Every SEC seconds, two unless -n says, it clears the screen, heads it
// `Every 2.0s: CMD` with the date and time at its right edge and a blank line under, and runs
// CMD: its words joined with blanks and read as shell text, as `sh -c` would read them, or under
// -x run as the words they are. -t leaves the heading out, and -d is taken and does nothing, as
// busybox takes it. It runs until it is interrupted, and answers as an interrupted command
// does. It was not here, and Windows has no program of the name.
func (r Runtime) watchBuiltin(ctx context.Context, args []string) int {
	period, heading, execute, words, status := r.parseWatchArgs(args)
	if status != 0 {
		return status
	}
	command := strings.Join(words, " ")
	for {
		fmt.Fprint(r.streams.Stdout, "\033[H\033[J")
		if heading {
			width := applets.TerminalColumns(r.streams.Stderr)
			fmt.Fprintf(r.streams.Stdout, "%s\n\n", watchHeading(period, command, width, time.Now()))
		}
		if execute {
			r.runCommandResolved(ctx, words, true)
		} else {
			r.runShellText(ctx, command)
		}
		select {
		case <-ctx.Done():
			return contextStatus(ctx)
		case <-time.After(period):
		}
	}
}

// parseWatchArgs reads watch's options, up to the first word that is not one, as busybox's "+"
// stops there.
func (r Runtime) parseWatchArgs(args []string) (time.Duration, bool, bool, []string, int) {
	period, heading, execute := 2*time.Second, true, false
	for len(args) > 0 && len(args[0]) > 1 && args[0][0] == '-' {
		option := args[0]
		args = args[1:]
		if option == "--" {
			break
		}
		for index := 1; index < len(option); index++ {
			switch option[index] {
			case 'd':
			case 't':
				heading = false
			case 'x':
				execute = true
			case 'n':
				value := option[index+1:]
				if value == "" && len(args) > 0 {
					value, args = args[0], args[1:]
				}
				parsed, err := applets.ParseDuration(value)
				if err != nil || value == "" {
					fmt.Fprintf(r.streams.Stderr, "%swatch: invalid number '%s'\n", r.diagnosticPrefix(), value)
					return 0, false, false, nil, 1
				}
				period, index = parsed, len(option)
			default:
				fmt.Fprintf(r.streams.Stderr, "%swatch: unknown option -- %c\n", r.diagnosticPrefix(), option[index])
				return 0, false, false, nil, 1
			}
		}
	}
	if len(args) == 0 {
		fmt.Fprintf(r.streams.Stderr, "%swatch: expected a command to run\n", r.diagnosticPrefix())
		return 0, false, false, nil, 1
	}
	return period, heading, execute, args, 0
}

// watchHeading is busybox's: `Every 2.0s: CMD` padded to the terminal's width, and the date and
// time written over its last twenty columns but one, where the width leaves room for them.
func watchHeading(period time.Duration, command string, width int, now time.Time) string {
	heading := fmt.Sprintf("Every %.1fs: %-*s", period.Seconds(), width, command)
	const stamp = len("2006-01-02 15:04:05") + 1
	if stamp < width && width <= len(heading) {
		heading = heading[:width-stamp] + now.Format("2006-01-02 15:04:05")
	}
	return heading
}
