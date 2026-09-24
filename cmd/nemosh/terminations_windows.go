//go:build windows

package main

import (
	"os"
	"syscall"
)

// terminationSignals is SIGTERM alone, which is how Go reports the console closing, the
// user logging off and the machine shutting down. Nothing else sends one here.
var terminationSignals = []os.Signal{syscall.SIGTERM}

// terminationsAreFinal, because each of those ends every process on the console within
// seconds; see runtime.ReceiveSignals.
const terminationsAreFinal = true
