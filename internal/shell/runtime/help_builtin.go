package runtime

import (
	"fmt"
	"sort"
	"strings"
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
// Operands are refused rather than ignored. busybox's help takes none either,
// and swallowing one would hide a typo -- `help cd` looks like a request for
// something this cannot answer.
func (r Runtime) help(args []string) int {
	if len(args) > 0 {
		fmt.Fprintf(r.streams.Stderr, "help: takes no operands; `type %s` describes one command\n", args[0])
		return 2
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
	fmt.Fprintln(r.streams.Stdout, "`NAME --help` describes one applet; docs/support-matrix.md records them all.")
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
