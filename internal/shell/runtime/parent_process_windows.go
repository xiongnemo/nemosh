//go:build windows

package runtime

import (
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

// parentProcessID is the process that started this one, for $PPID.
//
// Not os.Getppid, which on Windows takes a snapshot of every process on the machine and
// walks it for this one's entry. That was 13.7 ms of every start on the machine it was
// measured on -- `nemosh -c true` at 25 ms against 8 for `--version` -- and every script,
// every `nemosh -c` and every applet shim paid it, since all of them build a shell's
// state. The snapshot reads the parent from the process's own basic information; asking
// for that directly is one call, and the same number.
func parentProcessID() int {
	var info windows.PROCESS_BASIC_INFORMATION
	err := windows.NtQueryInformationProcess(windows.CurrentProcess(), windows.ProcessBasicInformation,
		unsafe.Pointer(&info), uint32(unsafe.Sizeof(info)), nil)
	if err != nil {
		return os.Getppid()
	}
	return int(info.InheritedFromUniqueProcessId)
}
