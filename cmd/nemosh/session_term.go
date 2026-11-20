package main

import (
	"fmt"

	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// sayTERMSetAside says, once the startup files have run, that the TERM the environment brought
// was set aside (runtime.SetAsideInheritedTERM) and how to keep one -- unless they set one again,
// which is how a terminal that draws xterm's sequences is told to this shell.
func (c command) sayTERMSetAside(rt runtime.Runtime, term string, aside bool) {
	if !aside {
		return
	}
	if _, set := rt.LookupVariable("TERM"); set {
		return
	}
	fmt.Fprintf(c.stderr, "nemosh: set aside inherited TERM=%s; export TERM in ~/.nemoshrc to keep it\n", term)
}
