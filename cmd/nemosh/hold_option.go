package main

import (
	"fmt"
	"os"

	"golang.org/x/term"
)

// -N holds the console open at exit.
//
// It exists for `su`. An elevated shell runs in a console of its own, and that
// console dies with the process, so `su -c 'ls'` would show its output for
// exactly as long as it took to print. busybox has the same option on the same
// path and reaches it the same way -- `su -N` passes `-N` to the shell it
// launches, where it sets `delayexit` and is read at the end of main
// (`shell/ash.c:13442`, `:16371`).
//
// It is stripped before the dispatch chain rather than added to it, because that
// chain asks about args[1] by position: `-c` has to still be args[1] after this
// runs, or `nemosh -N -c CMD` would be read as an invalid option.
func stripHoldOption(args []string) ([]string, bool) {
	if len(args) == 0 {
		return args, false
	}
	held := false
	index := 1
	for ; index < len(args); index++ {
		if args[index] != "-N" {
			break
		}
		held = true
	}
	if !held {
		return args, false
	}
	// Only the leading run is taken. Anything after the first other word is
	// somebody else's argument -- `nemosh -c 'echo -N'` prints -N.
	return append(args[:1:1], args[index:]...), true
}

// holdConsole waits for a keypress so a console that is about to close does not
// take the output with it.
//
// Only when stdin is a terminal. Otherwise there is nobody to press anything and
// this would hang forever, which is a worse failure than the one it prevents --
// and a redirected stdin means the output was captured anyway, so there is
// nothing to lose.
//
// The prompt goes to stderr: it is not output, and a caller redirecting stdout
// should not find it in the file.
func (c command) holdConsole() {
	if !c.stdinIsTerminal {
		return
	}
	file, ok := c.stdin.(*os.File)
	if !ok {
		return
	}
	fmt.Fprint(c.stderr, "Press any key to exit...")
	// Raw mode so it really is any key rather than a line ending in Enter,
	// which is what busybox's _getch does. A terminal that cannot be put into
	// raw mode still gets the wait, just a line-buffered one.
	if state, err := term.MakeRaw(int(file.Fd())); err == nil {
		defer term.Restore(int(file.Fd()), state)
	}
	var key [1]byte
	_, _ = file.Read(key[:])
	fmt.Fprintln(c.stderr)
}
