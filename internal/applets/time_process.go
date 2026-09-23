package applets

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/xiongnemo/nemosh/internal/proc"
)

// `usleep` and `ts` for time, `pidof` and `killall` for processes.
//
// The two process commands are `pgrep` and `pkill` asked a different way, and they reuse
// that code rather than repeating it: what differs is the interface, not the work. `pidof`
// matches a **name**, where `pgrep` matches a regular expression, and that is a real
// difference rather than a cosmetic one -- `pidof sh` must not find `bash`.

// usleep waits for a number of microseconds, which is the resolution `sleep` does not offer
// in the form a script wants to type.
//
// The wait is interruptible: a script killed while sleeping should stop, not finish its
// nap first.
func newUsleepApplet() Applet {
	return simpleApplet{name: "usleep", runContext: func(ctx context.Context, args []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
		_, operands, err := parseAppletOptions(args, "", "")
		if err != nil {
			return err
		}
		if len(operands) == 0 {
			return missingOperand()
		}
		if len(operands) > 1 {
			return fmt.Errorf("extra operand '%s'", operands[1])
		}
		microseconds, err := strconv.ParseInt(operands[0], 10, 64)
		if err != nil || microseconds < 0 {
			return fmt.Errorf("invalid number '%s'", operands[0])
		}
		timer := time.NewTimer(time.Duration(microseconds) * time.Microsecond)
		defer timer.Stop()
		select {
		case <-timer.C:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}}
}

// ts stamps each line of its input with the time it arrived, which is how a pipeline that
// is too slow gets diagnosed without changing the program that is running in it.
//
// The default format is the one both references use, `Mmm DD HH:MM:SS`. An operand replaces
// it with a strftime format. `-i` reports the gap since the *previous* line and `-s` the
// time since the first, and both of those are elapsed times rather than clock times -- so
// they are formatted as such, starting at `00:00:00`.
func newTsApplet() Applet {
	return simpleApplet{name: "ts", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, _ io.Writer) error {
		options, operands, err := parseAppletOptions(args, "is", "")
		if err != nil {
			return err
		}
		if len(operands) > 1 {
			return fmt.Errorf("extra operand '%s'", operands[1])
		}
		format := "%b %e %H:%M:%S"
		if len(operands) == 1 {
			format = operands[0]
		}
		return stampLines(ctx, stdin, stdout, format, options.has('i'), options.has('s'))
	}}
}

func stampLines(ctx context.Context, stdin io.Reader, stdout io.Writer, format string, incremental, sinceStart bool) error {
	reader := bufio.NewScanner(decodeTextInput(stdin))
	reader.Buffer(make([]byte, 0, 64*1024), maxTextLine)
	writer := bufio.NewWriter(stdout)
	defer writer.Flush()
	start := time.Now()
	previous := start
	for reader.Scan() {
		if err := ctx.Err(); err != nil {
			return err
		}
		now := time.Now()
		stamp := ""
		switch {
		case incremental:
			stamp = elapsedStamp(now.Sub(previous))
		case sinceStart:
			stamp = elapsedStamp(now.Sub(start))
		default:
			stamp = strftimeLike(now, format)
		}
		previous = now
		if _, err := fmt.Fprintf(writer, "%s %s\n", stamp, reader.Text()); err != nil {
			return err
		}
		// Flushed per line: the whole point of `ts` is watching something as it happens,
		// and a buffer would hold the evidence back until the pipeline ended.
		if err := writer.Flush(); err != nil {
			return err
		}
	}
	return reader.Err()
}

// elapsedStamp renders a duration the way `-i` and `-s` report one.
func elapsedStamp(elapsed time.Duration) string {
	if elapsed < 0 {
		elapsed = 0
	}
	total := int(elapsed.Seconds())
	return fmt.Sprintf("%02d:%02d:%02d", total/3600, (total/60)%60, total%60)
}

// strftimeLike renders the strftime conversions `ts` is given, which are a small set.
//
// Not every strftime conversion: the ones a timestamp uses. An unknown one is left as it
// was written rather than swallowed, so a format with a typo shows the typo.
func strftimeLike(when time.Time, format string) string {
	replacements := []struct{ verb, layout string }{
		{"%Y", "2006"}, {"%m", "01"}, {"%d", "02"}, {"%b", "Jan"}, {"%a", "Mon"},
		{"%H", "15"}, {"%M", "04"}, {"%S", "05"}, {"%Z", "MST"}, {"%F", "2006-01-02"},
		{"%T", "15:04:05"},
	}
	out := format
	for _, replacement := range replacements {
		out = strings.ReplaceAll(out, replacement.verb, when.Format(replacement.layout))
	}
	// %e is the day of the month, space-padded, which Go's layouts spell `_2`.
	out = strings.ReplaceAll(out, "%e", when.Format("_2"))
	out = strings.ReplaceAll(out, "%%", "%")
	return out
}

// pidof answers the process ids of everything running under a name.
//
// The name is matched **whole**, with or without an executable suffix, which is what
// separates it from `pgrep`: `pidof sh` must not find `bash`, and a pattern would.
func newPidofApplet() Applet {
	return simpleApplet{name: "pidof", run: func(args []string, _ io.Reader, stdout, _ io.Writer) error {
		options, names, err := parseAppletOptions(args, "s", "o")
		if err != nil {
			return err
		}
		if len(names) == 0 {
			return missingOperand()
		}
		omit := map[int]bool{}
		for _, text := range strings.Split(options.values['o'], ",") {
			if pid, err := strconv.Atoi(strings.TrimSpace(text)); err == nil {
				omit[pid] = true
			}
		}
		found, err := processesNamed(names, omit)
		if err != nil {
			return err
		}
		if len(found) == 0 {
			// Nothing matched is a status rather than a message: `pidof x || start x`
			// is the idiom, and a diagnostic would be noise in it.
			return ExitStatus(1)
		}
		if options.has('s') {
			found = found[:1]
		}
		text := make([]string, 0, len(found))
		for _, pid := range found {
			text = append(text, strconv.Itoa(pid))
		}
		_, err = fmt.Fprintln(stdout, strings.Join(text, " "))
		return err
	}}
}

// processesNamed lists the ids running under any of the given names, lowest first.
//
// Sorted because a process list arrives in whatever order the operating system walked it,
// and `pidof -s` promising "the single one" has to mean the same process twice running.
func processesNamed(names []string, omit map[int]bool) ([]int, error) {
	all, err := proc.List()
	if err != nil {
		return nil, err
	}
	wanted := map[string]bool{}
	for _, name := range names {
		wanted[strings.ToLower(trimExecutableSuffix(name))] = true
	}
	var found []int
	for _, process := range all {
		if omit[process.PID] {
			continue
		}
		if wanted[strings.ToLower(trimExecutableSuffix(process.Name))] {
			found = append(found, process.PID)
		}
	}
	sort.Ints(found)
	return found, nil
}

// killall signals every process running under a name.
//
// Like `pidof`, the name is matched whole rather than as a pattern -- `killall sh` must not
// take `bash` with it, and that is the difference between a tidy-up and an accident.
func newKillallApplet() Applet {
	return simpleApplet{name: "killall", run: func(args []string, _ io.Reader, stdout, stderr io.Writer) error {
		signal, rest, err := splitLeadingSignal(args)
		if err != nil {
			return err
		}
		options, names, err := parseAppletOptions(rest, "lq", "")
		if err != nil {
			return err
		}
		if options.has('l') {
			_, err := fmt.Fprintln(stdout, strings.Join(knownSignalNames(), " "))
			return err
		}
		if len(names) == 0 {
			return missingOperand()
		}
		return killallNamed(names, signal, options.has('q'), stderr)
	}}
}

func killallNamed(names []string, signal int, quiet bool, stderr io.Writer) error {
	failed := false
	for _, name := range names {
		found, err := processesNamed([]string{name}, nil)
		if err != nil {
			return err
		}
		if len(found) == 0 {
			if !quiet {
				fmt.Fprintf(stderr, "killall: %s: no process killed\n", name)
			}
			failed = true
			continue
		}
		for _, pid := range found {
			if err := proc.Terminate(pid, signal); err != nil {
				if !quiet {
					fmt.Fprintf(stderr, "killall: %v\n", err)
				}
				failed = true
			}
		}
	}
	if failed {
		return ExitStatus(1)
	}
	return nil
}
