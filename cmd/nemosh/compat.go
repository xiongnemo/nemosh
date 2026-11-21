package main

import (
	"fmt"

	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// compatVariable is NEMOSH_COMPAT, which gives the nemosh name the bash name's mode: with it
// set to bash in the environment, `nemosh` is `nemosh bash` but for $0, so a session can follow
// Git for Windows' bash without a bash on PATH that would stand in front of Git's own. sh is
// busybox's name and is left as it is. It is read once, as the shell starts.
const compatVariable = "NEMOSH_COMPAT"

// bashMode is whether the shell starts in bash mode: under the bash name, or under the nemosh
// name with NEMOSH_COMPAT=bash. A value of the variable this shell does not know is said, on
// stderr, since a misspelt one would otherwise leave the shell as it was without a word.
func (c command) bashMode(rt runtime.Runtime) bool {
	if c.name == "bash" {
		return true
	}
	if c.name != "" {
		return false
	}
	value, set := rt.LookupEnv(compatVariable)
	switch {
	case !set || value == "":
		return false
	case value == "bash":
		return true
	}
	fmt.Fprintf(c.stderr, "nemosh: %s=%s is not a setting this shell knows; bash is, and nothing else\n", compatVariable, value)
	return false
}
