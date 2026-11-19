package applets

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/xiongnemo/nemosh/internal/proc"
)

// pgrep and pkill find processes by name, which a clean Windows machine cannot
// do at all: it ships `tasklist` and `taskkill`, neither of which takes a
// pattern, and no `pgrep` under any name. Their options are procps-ng's, which
// busybox's are a subset of; see pgrep_match.go for what they select.

func newPgrepApplet() Applet {
	return simpleApplet{name: "pgrep", runContext: func(ctx context.Context, args []string, _ io.Reader, stdout, _ io.Writer) error {
		table, err := livePgrepTable()
		if err != nil {
			return err
		}
		return writePgrep(ctx, args, table, stdout)
	}}
}

// livePgrepTable is this machine's processes, but the one this runs in, which busybox's pgrep
// leaves out too, and the cache that opens a process to ask about it.
func livePgrepTable() (pgrepTable, error) {
	processes, err := proc.List()
	if err != nil {
		return pgrepTable{}, err
	}
	return pgrepTable{processes: processes, details: proc.NewDetailCache()}, nil
}

// writePgrep prints what pgrep chose: ids, `id name` under -l and `id command-line` under -a,
// between -d's delimiter, or under -c how many. Nothing chosen is status 1, as pgrep everywhere
// has it: not an error but an answer, which a script tests the status for.
func writePgrep(ctx context.Context, args []string, table pgrepTable, stdout io.Writer) error {
	matcher, err := parseProcessPattern(ctx, "pgrep", args, pgrepLetters)
	if err != nil {
		return err
	}
	matches := matcher.choose(table)
	if matcher.count {
		fmt.Fprintln(stdout, len(matches))
	}
	if len(matches) == 0 {
		return ExitStatus(1)
	}
	if matcher.count {
		return nil
	}
	items := make([]string, len(matches))
	for index := range matches {
		match := &matches[index]
		items[index] = strconv.Itoa(match.process.PID)
		switch {
		case matcher.argv:
			items[index] += " " + printableCommand(match.commandLine(table.details))
		case matcher.long:
			items[index] += " " + match.process.Name
		}
	}
	_, err = fmt.Fprintln(stdout, strings.Join(items, matcher.delimiter))
	return err
}

func newPkillApplet() Applet {
	return simpleApplet{name: "pkill", runContext: func(ctx context.Context, args []string, _ io.Reader, stdout, stderr io.Writer) error {
		signal, rest, err := splitLeadingSignal(args)
		if err != nil {
			return err
		}
		// -l lists the signals instead, as kill -l does and busybox's pkill -l, whatever else
		// was given.
		if options, _, err := parseAppletOptions(ctx, rest, pkillLetters+"l", "PuU"); err == nil && options.has('l') {
			for _, known := range proc.Signals() {
				fmt.Fprintf(stdout, "%2d) %s\n", known.Number, known.Name)
			}
			return nil
		}
		matcher, err := parseProcessPattern(ctx, "pkill", rest, pkillLetters)
		if err != nil {
			return err
		}
		table, err := livePgrepTable()
		if err != nil {
			return err
		}
		matches := matcher.choose(table)
		if len(matches) == 0 {
			return ExitStatus(1)
		}
		failed := false
		for _, match := range matches {
			process := match.process
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
