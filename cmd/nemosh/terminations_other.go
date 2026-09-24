//go:build !windows

package main

import (
	"os"
	"syscall"
)

// terminationSignals are the ones a script can trap besides INT, which the interrupt
// controller has.
var terminationSignals = []os.Signal{syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT}

// terminationsAreFinal is false: a `kill` here is addressed to the shell alone, and a
// trap for it runs once the command in progress finishes, as bash's does.
const terminationsAreFinal = false
