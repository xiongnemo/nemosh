//go:build !windows

package main

// consoleInterrupts is false off Windows, where SIGINT may as well come from kill as from a
// terminal, and the program is ended as it always was.
const consoleInterrupts = false
