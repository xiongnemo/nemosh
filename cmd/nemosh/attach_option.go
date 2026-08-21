package main

import "strconv"

// --attach-console PID is how `su` tells the elevated shell which console to
// join. See attach_console_windows.go for why the child does the joining.
//
// Stripped before the dispatch chain for the same reason -N is: that chain reads
// args[1] by position, so an option left in front of `-c` would come back as an
// invalid option rather than being understood.
//
// Not documented in --help, and deliberately so: it is not a thing to type. A
// PID typed by hand names a console this process has no business joining, and
// the option exists to be written by su and read by the shell it launched.
// Only position one is examined, and the loop that used to wrap this said otherwise: every path
// through its body broke or returned, so it ran exactly once and `index` was always 1. That is the
// right behaviour -- su writes the option immediately after the program name and nothing else emits
// it -- but code shaped like a scan that does not scan is code that will be read wrongly later.
func stripAttachConsoleOption(args []string) ([]string, int) {
	if len(args) < 3 || args[1] != "--attach-console" {
		return args, 0
	}
	pid, err := strconv.Atoi(args[2])
	if err != nil || pid <= 0 {
		return args, 0
	}
	// The three-index slice matters: it caps the capacity at one, so append allocates rather
	// than writing over the caller's own arguments.
	return append(args[:1:1], args[3:]...), pid
}
