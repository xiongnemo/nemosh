package applets

import (
	"os"
	"sync"
)

// What stty last set the console's modes to, for an interactive session, which puts the console
// back after each command line as the line found it (cmd/nemosh's console_modes_windows.go): a
// program that changed the modes and died is undone that way, and `stty -echo` typed at the
// prompt would be undone with it. What stty set was asked for, so the session keeps it.
var sttySet struct {
	sync.Mutex
	changes uint64
	in, out uint32
	hasOut  bool
}

// noteSttySet records the console's modes as a setting has just left them.
func noteSttySet(input *os.File) {
	in, err := consoleMode(input)
	if err != nil {
		return
	}
	console, hasOut := sttyOutputConsole()
	var out uint32
	if hasOut {
		if out, err = consoleMode(console); err != nil {
			hasOut = false
		}
	}
	sttySet.Lock()
	defer sttySet.Unlock()
	sttySet.changes++
	sttySet.in, sttySet.out, sttySet.hasOut = in, out, hasOut
}

// SttySet is the console's input and output modes as stty or reset last set them, and how many
// times they have, which moves each time; hasOut is false where no output was a console.
func SttySet() (changes uint64, in, out uint32, hasOut bool) {
	sttySet.Lock()
	defer sttySet.Unlock()
	return sttySet.changes, sttySet.in, sttySet.out, sttySet.hasOut
}
