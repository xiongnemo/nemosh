//go:build windows

package main

import (
	"os"

	"golang.org/x/sys/windows"
)

// This shell writes ANSI escapes -- colour, the grey suggestion, cursor
// movement -- and never asked the console to interpret them.
//
// It worked anyway wherever something else had already asked. Windows Terminal
// runs programs under a pseudoconsole, and ConPTY turns
// ENABLE_VIRTUAL_TERMINAL_PROCESSING on for it, so a session started there looks
// fine and hides the omission. A classic console window does not: the flag is
// off by default and every escape is drawn as literal text.
//
// Measured on this machine: a console handle opened with CONOUT$ reports mode
// 0x0003 -- ENABLE_PROCESSED_OUTPUT and ENABLE_WRAP_AT_EOL_OUTPUT -- with 0x0004
// clear. So the dependency was real and merely invisible.
//
// It surfaced through `su`, which launches its shell into a console of its own,
// and that console is a plain one.
func enableVirtualTerminal(files ...*os.File) func() {
	var restores []func()
	for _, file := range files {
		if restore := enableVirtualTerminalOn(file); restore != nil {
			restores = append(restores, restore)
		}
	}
	return func() {
		for _, restore := range restores {
			restore()
		}
	}
}

func enableVirtualTerminalOn(file *os.File) func() {
	if file == nil {
		return nil
	}
	handle := windows.Handle(file.Fd())
	var mode uint32
	// A pipe or a file has no console mode, which is how a redirected stream is
	// recognised here: there is nothing to interpret escapes, and nothing to set.
	if err := windows.GetConsoleMode(handle, &mode); err != nil {
		return nil
	}
	if mode&windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING != 0 {
		return nil
	}
	if err := windows.SetConsoleMode(handle, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING); err != nil {
		// An older console that cannot do it at all. Nothing to be done, and the
		// shell still works -- it just draws its escapes as text, which is what
		// it did before this existed.
		return nil
	}
	// Console mode is process-global and borrowed, not owned, exactly as raw mode
	// is: put it back on the way out so the next program gets the console it
	// would have had.
	return func() { _ = windows.SetConsoleMode(handle, mode) }
}
