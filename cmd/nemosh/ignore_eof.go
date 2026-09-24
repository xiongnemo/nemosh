package main

import (
	"fmt"
	"io"

	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// maxIgnoredEOFs is busybox's: under ignoreeof the fiftieth end of input in a row is
// refused and the next one leaves anyway (cmdloop in shell/ash.c), so input that has
// really ended -- a pipe -- cannot hold the shell for ever.
const maxIgnoredEOFs = 50

// refuseEOF answers an end of input at the prompt under `set -o ignoreeof`: busybox's
// words and another prompt, rather than `exit`. ignored counts the refusals in a row.
func refuseEOF(rt runtime.Runtime, stderr io.Writer, ignored *int) bool {
	if !rt.IgnoresEOF() || *ignored >= maxIgnoredEOFs {
		return false
	}
	*ignored++
	fmt.Fprint(stderr, "\nUse \"exit\" to leave shell.\n")
	return true
}
