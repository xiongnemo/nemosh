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
func stripAttachConsoleOption(args []string) ([]string, int) {
	for index := 1; index+1 < len(args); index++ {
		if args[index] != "--attach-console" {
			break
		}
		pid, err := strconv.Atoi(args[index+1])
		if err != nil || pid <= 0 {
			break
		}
		return append(args[:1:1], args[index+2:]...), pid
	}
	return args, 0
}
