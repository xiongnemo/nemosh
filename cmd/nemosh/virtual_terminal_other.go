//go:build !windows

package main

import "os"

// Every terminal here interprets escapes; there is no mode to ask for. See the
// Windows half for why that platform needs to be asked.
func enableVirtualTerminal(...*os.File) func() { return func() {} }
