package runtime

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// time is busybox-w32's applet (miscutils/time.c): `time [-pa] [-f FMT] [-o FILE] PROG ARGS`
// runs PROG and then says how long it took, on stderr or in -o's FILE:
//
//	real	0m 0.01s
//	user	0m 0.00s
//	sys	0m 0.01s
//
// The shell had no time at all. `time cmd` ran whatever time.exe PATH held, busybox's through
// scoop's shim on the machine this was measured on, and was `not found` where PATH held none.
// PROG runs here as any command does, so a function or a builtin can be timed as well, which
// busybox's cannot, having only a program to start. user and sys are the CPU the shell and
// its children used while it ran. $TIME is the format when -p and -f give none, and a PROG
// that fails is reported above the times, `Command exited with non-zero status N`, its status
// being time's.
func (r Runtime) timeBuiltin(ctx context.Context, args []string) int {
	format, _ := r.env.LookupEnv("TIME")
	if format == "" {
		format = "real\t%E\nuser\t%u\nsys\t%T"
	}
	output, appending := "", false
	for len(args) > 0 && len(args[0]) > 1 && args[0][0] == '-' {
		option := args[0]
		args = args[1:]
		if option == "--" {
			break
		}
		switch option {
		case "-p":
			format = "real %e\nuser %U\nsys %S"
		case "-a":
			appending = true
		case "-f", "-o":
			if len(args) == 0 {
				fmt.Fprintf(r.streams.Stderr, "time: option requires an argument -- '%c'\n", option[1])
				return 1
			}
			if option == "-f" {
				format = args[0]
			} else {
				output = args[0]
			}
			args = args[1:]
		default:
			fmt.Fprintf(r.streams.Stderr, "time: unrecognized option '%s'\n", option)
			return 1
		}
	}
	if len(args) == 0 {
		fmt.Fprintln(r.streams.Stderr, "time: expected a command to run")
		return 1
	}
	report := r.streams.Stderr
	if output != "" {
		flags := os.O_CREATE | os.O_WRONLY | os.O_TRUNC
		if appending {
			flags = os.O_CREATE | os.O_WRONLY | os.O_APPEND
		}
		file, err := os.OpenFile(r.resolvePath(output), flags, r.createMode(0o666))
		if err != nil {
			fmt.Fprintf(r.streams.Stderr, "time: can't open '%s': %s\n", output, applets.CauseText(err))
			return 1
		}
		defer file.Close()
		report = file
	}
	usage := r.measure(func() int { return r.runCommandResolved(ctx, args, true) })
	writeTimeReport(report, format, args, usage)
	return usage.status
}

// timeUsage is what one timed command used: its status, and the wall clock, user and system
// time of its run.
type timeUsage struct {
	status             int
	elapsed, user, sys time.Duration
}

// measure runs command and reports what it used: the shell's own CPU and its finished
// children's, across the run.
func (r Runtime) measure(command func() int) timeUsage {
	selfUser, selfSys, _ := processCPUTime()
	var childUser, childSys time.Duration
	if r.childCPU != nil {
		childUser, childSys = r.childCPU.total()
	}
	start := time.Now()
	status := command()
	elapsed := time.Since(start)
	usage := timeUsage{status: status, elapsed: elapsed}
	if user, sys, err := processCPUTime(); err == nil {
		usage.user, usage.sys = user-selfUser, sys-selfSys
	}
	if r.childCPU != nil {
		user, sys := r.childCPU.total()
		usage.user += user - childUser
		usage.sys += sys - childSys
	}
	return usage
}

// writeTimeReport renders format as busybox-w32's summarize does. %E, %u and %T are the
// times as minutes and seconds, %e, %U and %S as seconds, %C the command and %x its status;
// \n, \t and \\ are what they say. Anything else is written as ? and the letter, and a
// trailing % ends the report with no newline.
func writeTimeReport(out io.Writer, format string, command []string, usage timeUsage) {
	var text strings.Builder
	if usage.status != 0 {
		fmt.Fprintf(&text, "Command exited with non-zero status %d\n", usage.status)
	}
	for index := 0; index < len(format); index++ {
		char := format[index]
		if char != '%' && char != '\\' {
			text.WriteByte(char)
			continue
		}
		index++
		if index == len(format) {
			if char == '%' {
				text.WriteString("?")
			} else {
				text.WriteString("?\\" + command[0] + "\n")
			}
			io.WriteString(out, text.String())
			return
		}
		if char == '\\' {
			switch format[index] {
			case 'n':
				text.WriteByte('\n')
			case 't':
				text.WriteByte('\t')
			case '\\':
				text.WriteByte('\\')
			default:
				text.WriteString("?\\" + string(format[index]))
			}
			continue
		}
		text.WriteString(timeField(format[index], command, usage))
	}
	text.WriteByte('\n')
	io.WriteString(out, text.String())
}

func timeField(letter byte, command []string, usage timeUsage) string {
	switch letter {
	case '%':
		return "%"
	case 'C':
		return strings.Join(command, " ")
	case 'E':
		return minutesAndSeconds(usage.elapsed)
	case 'u':
		return minutesAndSeconds(usage.user)
	case 'T':
		return minutesAndSeconds(usage.sys)
	case 'e':
		return plainSeconds(usage.elapsed)
	case 'U':
		return plainSeconds(usage.user)
	case 'S':
		return plainSeconds(usage.sys)
	case 'x':
		return fmt.Sprint(usage.status)
	}
	return "?" + string(letter)
}

// minutesAndSeconds is busybox's `%um %u.%02us`, and `%uh %um %02us` from an hour up.
func minutesAndSeconds(d time.Duration) string {
	seconds := int64(d / time.Second)
	if seconds >= 3600 {
		return fmt.Sprintf("%dh %dm %02ds", seconds/3600, seconds%3600/60, seconds%60)
	}
	return fmt.Sprintf("%dm %d.%02ds", seconds/60, seconds%60, int64(d/(10*time.Millisecond))%100)
}

// plainSeconds is busybox's `%u.%02u`: seconds, and hundredths truncated.
func plainSeconds(d time.Duration) string {
	return fmt.Sprintf("%d.%02d", int64(d/time.Second), int64(d/(10*time.Millisecond))%100)
}
