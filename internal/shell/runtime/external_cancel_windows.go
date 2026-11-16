package runtime

import (
	"slices"
	"unsafe"
)

var procGetConsoleProcessList = kernel32.NewProc("GetConsoleProcessList")

// childSharesConsole is whether pid is attached to this process's console, and so was sent the
// Ctrl-C the shell was. A GUI program is not, nor is anything when the shell has no console.
var childSharesConsole = func(pid int) bool {
	list := make([]uint32, 64)
	for {
		count, _, _ := procGetConsoleProcessList.Call(uintptr(unsafe.Pointer(&list[0])), uintptr(len(list)))
		if count == 0 {
			return false
		}
		if int(count) <= len(list) {
			return slices.Contains(list[:count], uint32(pid))
		}
		list = make([]uint32, count)
	}
}
