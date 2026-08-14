package applets

import (
	"fmt"
	"io"
	"regexp"
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
	return simpleApplet{name: "pgrep", run: func(args []string, _ io.Reader, stdout, _ io.Writer) error {
		matcher, err := parseProcessPattern("pgrep", args, "lx")
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
	return simpleApplet{name: "pkill", run: func(args []string, _ io.Reader, _ io.Writer, stderr io.Writer) error {
		signal, rest := splitLeadingSignal(args)
		matcher, err := parseProcessPattern("pkill", rest, "x")
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
			}
		}
		if failed {
			return ExitStatus(1)
		}
		return nil
	}}
}

type processMatcher struct {
	pattern *regexp.Regexp
	exact   bool
	long    bool
}

// parseProcessPattern reads the options and the one pattern operand.
//
// An empty pattern is refused. `pkill ""` would match every process on the
// machine, and a command that can do that by omission is a command that will.
func parseProcessPattern(applet string, args []string, short string) (processMatcher, error) {
	options, operands, err := parseAppletOptions(args, short, "")
	if err != nil {
		return processMatcher{}, err
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
	return processMatcher{pattern: compiled, exact: options.has('x'), long: options.has('l')}, nil
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
		if m.pattern.MatchString(process.Name) || m.pattern.MatchString(trimExecutableSuffix(process.Name)) {
			matches = append(matches, process)
		}
	}
	return matches, nil
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
func splitLeadingSignal(args []string) (int, []string) {
	const terminate = 15
	if len(args) == 0 || !strings.HasPrefix(args[0], "-") || args[0] == "-" {
		return terminate, args
	}
	if number, ok := processSignalNumber(args[0][1:]); ok {
		return number, args[1:]
	}
	return terminate, args
}

func processSignalNumber(spec string) (int, bool) {
	numbers := map[string]int{"HUP": 1, "INT": 2, "QUIT": 3, "KILL": 9, "TERM": 15, "STOP": 19, "CONT": 18}
	if number, ok := numbers[strings.TrimPrefix(strings.ToUpper(spec), "SIG")]; ok {
		return number, true
	}
	var number int
	if _, err := fmt.Sscanf(spec, "%d", &number); err == nil && number > 0 && spec == fmt.Sprint(number) {
		return number, true
	}
	return 0, false
}
