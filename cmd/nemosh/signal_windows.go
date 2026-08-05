//go:build windows

package main

const windowsInterruptBoundary = "Go on Windows cannot direct os.Interrupt to one child or process group"
