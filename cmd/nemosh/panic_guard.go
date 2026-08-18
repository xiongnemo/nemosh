package main

import (
	"os"
	"runtime/debug"
	"strings"

	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// guardMain turns a panic outside any command into a diagnostic and an exit
// status. See internal/shell/runtime/panic_guard.go for why there are four of
// these and what each one covers.
//
// It exits rather than returning, because a panic has already unwound past
// whatever invariant it broke: continuing into the rest of main would be running
// on state nobody can vouch for. What it does not skip is the deferred terminal
// restoration below it in main, which is the one thing that must happen either way
// -- a shell that dies leaving the terminal in raw mode hands the next program a
// session with no echo.
func guardMain() {
	value := recover()
	if value == nil {
		return
	}
	runtime.ReportInternalError(os.Stderr, "running nemosh", value, debug.Stack(), traceRequested())
	os.Exit(runtime.InternalErrorStatus())
}

// traceRequested reads NEMOSH_DEBUG from the process environment.
//
// The process environment rather than the shell's, because this runs where there
// may be no shell: the variable has to have been exported to reach here, which is
// what someone reproducing a crash would do.
func traceRequested() bool {
	setting, ok := os.LookupEnv("NEMOSH_DEBUG")
	if !ok {
		return false
	}
	for _, name := range strings.Split(setting, ",") {
		switch strings.TrimSpace(name) {
		case "panic", "all":
			return true
		}
	}
	return false
}
