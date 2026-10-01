package applets

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/xiongnemo/nemosh/internal/proc"
)

// pgrep and pkill find processes by name, which a clean Windows machine cannot
// do at all: it ships `tasklist` and `taskkill`, neither of which takes a
// pattern, and no `pgrep` under any name.
//
// The pattern is a regular expression matched against the executable's file
// name, which is what busybox matches too. Not the full command line: reading
// that on Windows means opening each process and walking its PEB, which needs
// privileges an ordinary session has not got for anything it does not own -- so
// `-f` is refused rather than silently matching the name instead.

func newPgrepApplet() Applet {
	return simpleApplet{name: "pgrep", runContext: func(ctx context.Context, args []string, _ io.Reader, stdout, _ io.Writer) error {
		matcher, err := parseProcessPattern(ctx, "pgrep", args, "lxve")
		if err != nil {
			return err
		}
		matches, err := matcher.find()
		if err != nil {
			return err
		}
		if len(matches) == 0 {
			// One, as pgrep everywhere: nothing matched is not an error, it is
			// an answer, and a script tests the status for it.
			return ExitStatus(1)
		}
		for _, process := range matches {
			if matcher.long {
				fmt.Fprintf(stdout, "%d %s\n", process.PID, process.Name)
				continue
			}
			fmt.Fprintln(stdout, process.PID)
		}
		return nil
	}}
}

func newPkillApplet() Applet {
	return simpleApplet{name: "pkill", runContext: func(ctx context.Context, args []string, _ io.Reader, stdout, stderr io.Writer) error {
		signal, rest, err := splitLeadingSignal(args)
		if err != nil {
			return err
		}
		// -l lists the signals instead, as kill -l does and busybox's pkill -l, whatever else
		// was given.
		if options, _, err := parseAppletOptions(ctx, rest, "xvel", "P"); err == nil && options.has('l') {
			for _, known := range proc.Signals() {
				fmt.Fprintf(stdout, "%2d) %s\n", known.Number, known.Name)
			}
			return nil
		}
		matcher, err := parseProcessPattern(ctx, "pkill", rest, "xve")
		if err != nil {
			return err
		}
		matches, err := matcher.find()
		if err != nil {
			return err
		}
		if len(matches) == 0 {
			return ExitStatus(1)
		}
		failed := false
		for _, process := range matches {
			if err := proc.Terminate(process.PID, signal); err != nil {
				fmt.Fprintf(stderr, "pkill: %v\n", err)
				failed = true
				continue
			}
			// -e says what it killed, in the words procps and busybox both print.
			if matcher.echo {
				fmt.Fprintf(stdout, "%s killed (pid %d)\n", process.Name, process.PID)
			}
		}
		if failed {
			return ExitStatus(1)
		}
		return nil
	}}
}

// processMatcher is what pgrep and pkill select by: a pattern, the parent -P names, and -v to
// take every process the two together do not match, as busybox's -v takes them.
type processMatcher struct {
	pattern            *regexp.Regexp
	parent             int
	long, invert, echo bool
}

// parseProcessPattern reads the options and the one pattern operand.
//
// An empty pattern is refused. `pkill ""` would match every process on the
// machine, and a command that can do that by omission is a command that will.
func parseProcessPattern(ctx context.Context, applet string, args []string, short string) (processMatcher, error) {
	options, operands, err := parseAppletOptions(ctx, args, short, "P")
	if err != nil {
		return processMatcher{}, err
	}
	matcher := processMatcher{parent: -1, long: options.has('l'), invert: options.has('v'), echo: options.has('e')}
	// -P PPID selects a parent's children, and with it the pattern may be left out, as
	// busybox's takes them: `pgrep -P $$` is this shell's children.
	if options.has('P') {
		if matcher.parent, err = strconv.Atoi(options.value('P')); err != nil || matcher.parent < 0 {
			return processMatcher{}, fmt.Errorf("invalid number '%s'", options.value('P'))
		}
		if len(operands) == 0 {
			return matcher, nil
		}
	}
	switch {
	case len(operands) == 0:
		return processMatcher{}, missingOperand()
	case len(operands) > 1:
		return processMatcher{}, fmt.Errorf("extra operand '%s'", operands[1])
	case operands[0] == "":
		return processMatcher{}, fmt.Errorf("%s: an empty pattern would match every process", applet)
	}
	expression := operands[0]
	if options.has('x') {
		expression = "^(?:" + expression + ")$"
	}
	// Case-insensitive, because the filesystem these names come from is: a
	// process is `Notepad.exe` on disk and `notepad` in the hand.
	compiled, err := regexp.Compile("(?i)" + expression)
	if err != nil {
		return processMatcher{}, fmt.Errorf("invalid pattern: %s", operands[0])
	}
	// -x is already in the pattern, anchored above; keeping a copy of the answer beside it
	// invited the two to disagree.
	matcher.pattern = compiled
	return matcher, nil
}

// find lists the matches. The executable suffix is matched with or without,
// since `pkill notepad` is what anyone types for `notepad.exe`.
func (m processMatcher) find() ([]proc.Process, error) {
	all, err := proc.List()
	if err != nil {
		return nil, err
	}
	var matches []proc.Process
	for _, process := range all {
		if m.matches(process) != m.invert {
			matches = append(matches, process)
		}
	}
	return matches, nil
}

// matches is whether a process is the one asked for: a child of -P's parent, if one was
// named, whose name the pattern matches, if there is one.
func (m processMatcher) matches(process proc.Process) bool {
	if m.parent >= 0 && process.PPID != m.parent {
		return false
	}
	return m.pattern == nil || m.pattern.MatchString(process.Name) || m.pattern.MatchString(trimExecutableSuffix(process.Name))
}

func trimExecutableSuffix(name string) string {
	for _, suffix := range []string{".exe", ".com", ".bat", ".cmd"} {
		if len(name) > len(suffix) && strings.EqualFold(name[len(name)-len(suffix):], suffix) {
			return name[:len(name)-len(suffix)]
		}
	}
	return name
}

// splitLeadingSignal reads pkill's optional `-SIG` or `-N`, which comes before
// the options and cannot be told from one by getopt -- which is why it is read
// first, exactly as kill does it.
//
// The table is proc's, the one the kill builtin reads as well. A spec it does not know is
// left for the option parser -- `-x` is an option, not a signal -- but one it refuses is
// refused here: `pkill -STOP` used to *terminate* every match, which is what any signal
// becomes on Windows, and a pause that kills is the worst reading of that request.
func splitLeadingSignal(args []string) (int, []string, error) {
	const terminate = 15
	if len(args) == 0 || !strings.HasPrefix(args[0], "-") || args[0] == "-" {
		return terminate, args, nil
	}
	number, err := proc.ParseSignal(args[0][1:])
	switch {
	case err == nil:
		return number, args[1:], nil
	case errors.Is(err, proc.ErrCannotSuspend):
		return 0, nil, err
	}
	return terminate, args, nil
}

// knownSignalNames lists the table in number order, which is how `kill -l` is read.
func knownSignalNames() []string {
	signals := proc.Signals()
	names := make([]string, len(signals))
	for index, signal := range signals {
		names[index] = signal.Name
	}
	return names
}
