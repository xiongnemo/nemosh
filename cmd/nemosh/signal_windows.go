//go:build windows

package main

// Go on Windows cannot direct os.Interrupt to one child, nor need it: the console sends Ctrl-C to
// every process on it, the program in the foreground with the shell.
const windowsInterruptBoundary = "Go on Windows cannot direct os.Interrupt to one child or process group; the console already sends Ctrl-C to every program on it"

// consoleInterrupts says the shell's os.Interrupt is the console's Ctrl-C or Ctrl-Break, which
// the foreground program was sent too; see interruptController.console.
const consoleInterrupts = true
