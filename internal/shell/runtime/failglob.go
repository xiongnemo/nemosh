package runtime

import "fmt"

// `shopt -s failglob` makes a pattern that matches nothing an error, bash's "no match", where
// it otherwise stays as written. As in bash the error abandons the whole command the shell was
// running -- the function it called, the loop it was in -- with status 1, and the shell goes
// on with the next one; `set -e` ends it there instead. It travels as flowDiscard, which a
// subshell's end and a script's top level stop. A redirection's word is not a pattern here,
// as in busybox, so there is nothing in one to fail. busybox has no shopt, and keeps the
// pattern, as this does with the option off.

// reportGlobFailure is the error, raised as a shell error that discards rather than aborts;
// see shellErrorResult.
func (r Runtime) reportGlobFailure(pattern string) {
	if r.expansion.shellError {
		return
	}
	fmt.Fprintf(r.streams.Stderr, "nemosh: no match: %s\n", pattern)
	r.expansion.shellError, r.expansion.discard = true, true
}
