package runtime

import (
	"fmt"
	"sort"
	"strings"

	"github.com/xiongnemo/nemosh/internal/capability"
)

// help lists what this shell can run without going to PATH.
//
// It exists because busybox ash has one, and that is the whole reason a user
// there never meets Windows' own `help.exe`. Without this builtin, `help`
// resolves to `C:\Windows\system32\help.exe`, which writes in the console code
// page -- CP936 on a Chinese Windows -- and arrives as mojibake in a UTF-8
// terminal. Measured on 2026-08-08: busybox produces byte-identical CP936 output
// when forced to run `help.exe`, so the difference was never encoding handling.
//
// With operands it describes them, as bash's help does: busybox's takes none, and this one
// refused them, so `help cd`, the first thing anyone types, was an error. A name there is no
// help for is still an error, so a typo is not hidden.
func (r Runtime) help(args []string) int {
	if len(args) > 0 {
		return r.helpTopics(args)
	}

	fmt.Fprintln(r.streams.Stdout, "Built-in commands:")
	fmt.Fprintln(r.streams.Stdout, "------------------")
	writeNameColumns(r.streams.Stdout, runtimeBuiltinNames())

	fmt.Fprintln(r.streams.Stdout)
	fmt.Fprintln(r.streams.Stdout, "Applets:")
	fmt.Fprintln(r.streams.Stdout, "--------")
	writeNameColumns(r.streams.Stdout, r.registry.Names())

	fmt.Fprintln(r.streams.Stdout)
	fmt.Fprintln(r.streams.Stdout, "`nemosh --help` describes the command line; `type NAME` describes one name.")
	fmt.Fprintln(r.streams.Stdout, "`help NAME` describes a builtin or an applet; docs/support-matrix.md records them all.")
	return 0
}

// writeNameColumns wraps a sorted name list the way busybox's help does: tab
// indented, filled to a fixed width, so a long list stays readable.
func writeNameColumns(out interface{ Write([]byte) (int, error) }, names []string) {
	const width = 68
	var line strings.Builder
	for _, name := range names {
		if line.Len() > 0 && line.Len()+1+len(name) > width {
			fmt.Fprintf(out, "\t%s\n", line.String())
			line.Reset()
		}
		if line.Len() > 0 {
			line.WriteByte(' ')
		}
		line.WriteString(name)
	}
	if line.Len() > 0 {
		fmt.Fprintf(out, "\t%s\n", line.String())
	}
}

// runtimeBuiltinNames is the same set isRuntimeBuiltin answers for, so `help`
// and `type` cannot disagree about what a builtin is.
func runtimeBuiltinNames() []string {
	names := make([]string, 0, len(builtinNames))
	names = append(names, builtinNames...)
	sort.Strings(names)
	return names
}

// helpTopics prints the usage of each name, a builtin's or an applet's, from the text the
// capability table keeps for both; and says of a builtin with none where it is described.
func (r Runtime) helpTopics(names []string) int {
	status := 0
	for _, name := range names {
		if text, ok := capability.UsageFor(name); ok {
			fmt.Fprintln(r.streams.Stdout, strings.TrimRight(text, "\n"))
			continue
		}
		if isRuntimeBuiltin(name) {
			fmt.Fprintf(r.streams.Stdout, "%s is a shell builtin; docs/support-matrix.md records what it takes\n", name)
			continue
		}
		fmt.Fprintf(r.streams.Stderr, "%shelp: no help topics match `%s'\n", r.diagnosticPrefix(), name)
		status = 1
	}
	return status
}

// builtinHelp answers `NAME --help` for a builtin, as bash's builtins answer it: as the one
// argument, so `NAME -- --help` and an operand that happens to be --help are left alone. `:`
// takes its arguments as data.
func (r Runtime) builtinHelp(args []string) (int, bool) {
	if len(args) != 2 || args[1] != "--help" || args[0] == ":" || !isRuntimeBuiltin(args[0]) {
		return 0, false
	}
	return r.helpTopics(args[:1]), true
}
