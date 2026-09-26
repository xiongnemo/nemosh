// Command spechelper stands in for the helpers the vendored Oils spec cases run by name:
// argv.py, printenv.py, stdout_stderr.py and read_from_fd.py, which are Python 2, and
// spec/bin/foo=bar, a script. So the suite runs where neither Python 2 nor a Unix shell
// for the script is at hand. The harness installs one copy of it under each name, and a
// copy does what its name says.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func main() {
	os.Exit(run(installedName(), os.Args[1:], os.Environ(), os.Stdout, os.Stderr))
}

// installedName is the name this copy was installed under, read from the file itself
// rather than from argv[0], which each shell spells as it likes.
func installedName() string {
	path, err := os.Executable()
	if err != nil {
		path = os.Args[0]
	}
	name := filepath.Base(path)
	if extension := filepath.Ext(name); strings.EqualFold(extension, ".exe") {
		name = strings.TrimSuffix(name, extension)
	}
	return name
}

func run(name string, args, env []string, stdout, stderr io.Writer) int {
	switch name {
	case "argv.py":
		fmt.Fprintln(stdout, pythonList(args))
	case "printenv.py":
		for _, variable := range args {
			fmt.Fprintln(stdout, lookup(env, variable))
		}
	case "stdout_stderr.py":
		return stdoutStderr(args, stdout, stderr)
	case "read_from_fd.py":
		return readFromFDs(args, stdout, stderr)
	case "foo=bar":
		fmt.Fprintln(stdout, "HI")
	default:
		fmt.Fprintf(stderr, "spechelper: no helper is named %s\n", name)
		return 2
	}
	return 0
}

// lookup answers a variable's value as Python's os.environ.get prints it: None when it is
// not set. The name is matched exactly, as on the Unix the cases were written on.
func lookup(env []string, variable string) string {
	for _, entry := range env {
		if name, value, found := strings.Cut(entry, "="); found && name == variable {
			return value
		}
	}
	return "None"
}

// stdoutStderr prints its first argument, STDOUT by default, and its second, STDERR by
// default, and returns its third. Stderr goes first: Python 2 writes stderr at once and
// holds stdout until exit when it is not a terminal, and a case that joins the two, as
// `stdout_stderr.py |& cat` does, expects them in that order.
func stdoutStderr(args []string, stdout, stderr io.Writer) int {
	out, errText, status := "STDOUT", "STDERR", 0
	if len(args) > 0 {
		out = args[0]
	}
	if len(args) > 1 {
		errText = args[1]
	}
	if len(args) > 2 {
		n, err := strconv.Atoi(strings.Trim(args[2], " \t\n\r\v\f"))
		if err != nil {
			fmt.Fprintf(stderr, "ValueError: invalid literal for int() with base 10: %s\n", pythonString(args[2]))
			return 1
		}
		status = n
	}
	fmt.Fprintln(stderr, errText)
	fmt.Fprintln(stdout, out)
	return status
}

// readFromFDs prints what one read of each descriptor named yields, after its number.
func readFromFDs(args []string, stdout, stderr io.Writer) int {
	for _, arg := range args {
		fd, err := strconv.Atoi(arg)
		if err != nil {
			fmt.Fprintf(stderr, "ValueError: invalid literal for int() with base 10: %s\n", pythonString(arg))
			return 1
		}
		data, err := readFD(fd)
		if err != nil {
			fmt.Fprintf(stderr, "FATAL: Error reading from fd %d: %v\n", fd, err)
			return 1
		}
		fmt.Fprintf(stdout, "%d: %s", fd, data)
	}
	return 0
}

// pythonList is how Python 2 prints a list of byte strings.
func pythonList(items []string) string {
	quoted := make([]string, len(items))
	for i, item := range items {
		quoted[i] = pythonString(item)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

// pythonString is Python 2's repr of a byte string: single quotes unless it holds one and
// no double quote, the quote and backslash escaped, tab, newline and carriage return by
// name, and every other byte outside printable ASCII in hex.
func pythonString(s string) string {
	quote := byte('\'')
	if strings.Contains(s, "'") && !strings.Contains(s, `"`) {
		quote = '"'
	}
	var b strings.Builder
	b.WriteByte(quote)
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == quote || c == '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
		case c == '\t':
			b.WriteString(`\t`)
		case c == '\n':
			b.WriteString(`\n`)
		case c == '\r':
			b.WriteString(`\r`)
		case c < ' ' || c >= 0x7f:
			fmt.Fprintf(&b, `\x%02x`, c)
		default:
			b.WriteByte(c)
		}
	}
	b.WriteByte(quote)
	return b.String()
}
