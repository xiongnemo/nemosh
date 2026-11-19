package applets

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/xiongnemo/nemosh/internal/proc"
)

// What pgrep and pkill select by, and procps-ng's options for it.
//
// The pattern matches the image name, as busybox's does, and with -f the command line, as
// procps' does. A command line needs a handle on the process, which Windows refuses an unelevated
// session for another account's; such a process is matched by its image name, as procps matches
// one whose command line is empty. -u and -U select by the account the process runs as, which
// the same refusal hides, so a process whose owner cannot be read is no account's.

const (
	pgrepLetters = "lxvefanoci"
	pkillLetters = "xvefnoi"
)

// pgrepTable is what is chosen from: the processes, and how to ask about one beyond the table.
type pgrepTable struct {
	processes []proc.Process
	details   detailSource
}

// pgrepMatch is a process chosen, with its details once they were asked for.
type pgrepMatch struct {
	process proc.Process
	read    bool
	details proc.Details
}

func (m *pgrepMatch) detail(source detailSource) proc.Details {
	if !m.read {
		m.details, m.read = source.Lookup(m.process), true
	}
	return m.details
}

// commandLine is the command line, or the image name where it cannot be read.
func (m *pgrepMatch) commandLine(source detailSource) string {
	if line := m.detail(source).CommandLine; line != "" {
		return line
	}
	return m.process.Name
}

// processMatcher is what pgrep and pkill select by: a pattern, the parent -P names, the accounts
// -u names, and -v to take every process the rest does not match, as busybox's -v takes them.
type processMatcher struct {
	pattern                        *regexp.Regexp
	parent                         int
	users                          []string
	long, invert, echo, full, argv bool
	newest, oldest, count          bool
	delimiter                      string
}

// parseProcessPattern reads the options and the one pattern operand, which -P, -u or -U may leave
// out, as procps' do: `pgrep -u root` is everything root runs.
//
// An empty pattern is refused. `pkill ""` would match every process on the
// machine, and a command that can do that by omission is a command that will.
func parseProcessPattern(ctx context.Context, applet string, args []string, short string) (processMatcher, error) {
	valued := "PuU"
	if applet == "pgrep" {
		valued += "d"
	}
	options, operands, err := parseAppletOptions(ctx, args, short, valued)
	if err != nil {
		return processMatcher{}, err
	}
	matcher := processMatcher{parent: -1, long: options.has('l'), invert: options.has('v'), echo: options.has('e'),
		full: options.has('f'), argv: options.has('a'), newest: options.has('n'), oldest: options.has('o'),
		count: options.has('c'), delimiter: "\n"}
	if matcher.newest && matcher.oldest {
		return processMatcher{}, errors.New("-n and -o each choose one process; give one of them")
	}
	if options.has('d') {
		matcher.delimiter = options.value('d')
	}
	for _, letter := range []byte{'u', 'U'} {
		for _, list := range options.all(letter) {
			names := psList(list)
			if len(names) == 0 {
				return processMatcher{}, errors.New("user name list syntax error")
			}
			matcher.users = append(matcher.users, names...)
		}
	}
	// -P PPID selects a parent's children, as busybox's takes it: `pgrep -P $$` is this
	// shell's children.
	if options.has('P') {
		if matcher.parent, err = strconv.Atoi(options.value('P')); err != nil || matcher.parent < 0 {
			return processMatcher{}, fmt.Errorf("invalid number '%s'", options.value('P'))
		}
	}
	switch {
	case len(operands) == 0 && (matcher.parent >= 0 || len(matcher.users) > 0):
		return matcher, nil
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
	// Case-insensitive, because the filesystem these names come from is: a process is
	// `Notepad.exe` on disk and `notepad` in the hand. -i is therefore taken and changes nothing.
	compiled, err := regexp.Compile("(?i)" + expression)
	if err != nil {
		return processMatcher{}, fmt.Errorf("invalid pattern: %s", operands[0])
	}
	matcher.pattern = compiled
	return matcher, nil
}

// choose is the processes asked for, in the table's order, and under -n or -o the one most or
// least recently started of them.
func (m processMatcher) choose(table pgrepTable) []pgrepMatch {
	var chosen []pgrepMatch
	for _, process := range table.processes {
		match := pgrepMatch{process: process}
		if m.matches(&match, table.details) != m.invert {
			chosen = append(chosen, match)
		}
	}
	if len(chosen) > 1 && (m.newest || m.oldest) {
		pick := slices.MaxFunc(chosen, func(a, b pgrepMatch) int { return a.process.Created.Compare(b.process.Created) })
		if m.oldest {
			pick = slices.MinFunc(chosen, func(a, b pgrepMatch) int { return a.process.Created.Compare(b.process.Created) })
		}
		chosen = []pgrepMatch{pick}
	}
	return chosen
}

// matches is whether a process is the one asked for: a child of -P's parent, run by one of -u's
// accounts, whose name, or with -f command line, the pattern matches, each if it was given. The
// executable's suffix is matched with or without, since `pkill notepad` is what anyone types.
func (m processMatcher) matches(match *pgrepMatch, details detailSource) bool {
	if m.parent >= 0 && match.process.PPID != m.parent {
		return false
	}
	if len(m.users) > 0 {
		user := match.detail(details).User
		if user == "" || !slices.ContainsFunc(m.users, func(name string) bool { return strings.EqualFold(name, user) }) {
			return false
		}
	}
	if m.pattern == nil {
		return true
	}
	if m.full && match.detail(details).CommandLine != "" {
		return m.pattern.MatchString(match.detail(details).CommandLine)
	}
	name := match.process.Name
	return m.pattern.MatchString(name) || m.pattern.MatchString(trimExecutableSuffix(name))
}

func trimExecutableSuffix(name string) string {
	for _, suffix := range []string{".exe", ".com", ".bat", ".cmd"} {
		if len(name) > len(suffix) && strings.EqualFold(name[len(name)-len(suffix):], suffix) {
			return name[:len(name)-len(suffix)]
		}
	}
	return name
}
