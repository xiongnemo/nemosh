//go:build windows

package applets

import (
	"context"
	"fmt"
	"io"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The launch half of su. Everything decided beforehand is in su.go; this is the
// part that cannot be tested without starting a process, which is why `-t`
// exists.
var (
	shell32              = windows.NewLazySystemDLL("shell32.dll")
	procShellExecuteExW  = shell32.NewProc("ShellExecuteExW")
	errElevationRefused  = fmt.Errorf("elevation was refused")
	elevationPollTimeout = 100 * time.Millisecond
)

const (
	// SEE_MASK_NOCLOSEPROCESS keeps hProcess valid after the call, which is the
	// only way to wait for the child or learn its exit code.
	seeMaskNoCloseProcess = 0x00000040
	// SEE_MASK_NO_CONSOLE stops a new console being created. Test mode uses it
	// so a test run does not litter the desktop with windows.
	seeMaskNoConsole = 0x00008000
	swShowNormal     = 1
	// ERROR_CANCELLED, what the consent dialog returns when the answer is no.
	errorCancelled = syscall.Errno(1223)
)

// shellExecuteInfo is SHELLEXECUTEINFOW. The field order and the union are
// Windows'; Go lays this out identically on amd64, and cbSize is measured rather
// than written down so it cannot drift from the struct.
type shellExecuteInfo struct {
	size          uint32
	mask          uint32
	window        windows.HWND
	verb          *uint16
	file          *uint16
	parameters    *uint16
	directory     *uint16
	show          int32
	instance      windows.Handle
	idList        uintptr
	class         *uint16
	classKey      windows.Handle
	hotKey        uint32
	iconOrMonitor windows.Handle
	process       windows.Handle
}

// runElevated launches the planned shell and, if asked, waits for it.
func runElevated(ctx context.Context, plan elevationPlan, stderr io.Writer) error {
	verb := "runas"
	mask := uint32(0)
	if plan.test {
		// `open` launches without elevating, so the whole path -- quoting,
		// directory, waiting, exit status -- runs with no consent dialog.
		verb = "open"
		mask |= seeMaskNoConsole
	}
	if plan.wait {
		mask |= seeMaskNoCloseProcess
	}
	process, err := shellExecute(verb, plan.program, plan.arguments, plan.directory, mask)
	if err != nil {
		return elevationLaunchError(err)
	}
	if !plan.wait {
		// busybox returns here too. Without -W there is no handle to wait on and
		// nothing to report: the shell is somebody else's window now.
		if process != 0 {
			_ = windows.CloseHandle(process)
		}
		return nil
	}
	defer windows.CloseHandle(process)
	status, err := waitForElevated(ctx, process)
	if err != nil {
		return err
	}
	if status != 0 {
		return ExitStatus(status)
	}
	return nil
}

// elevationLaunchError names the one failure a person causes themselves.
func elevationLaunchError(err error) error {
	if err == errorCancelled {
		// Status 1, no elevation, nothing ran. Saying "cancelled" rather than
		// repeating the Win32 text is the difference between a refusal the
		// reader made and a fault they have to investigate.
		return ExitStatusMessage(1, errElevationRefused)
	}
	return fmt.Errorf("cannot launch an elevated shell: %w", err)
}

// waitForElevated waits for the child, watching the context as it goes.
//
// Cancelling does not kill it, and cannot: terminating a high-integrity process
// from a medium-integrity shell is refused by the same mechanism that made
// elevation necessary. So Ctrl-C stops waiting and says so; the window is still
// there.
func waitForElevated(ctx context.Context, process windows.Handle) (int, error) {
	for {
		if err := ctx.Err(); err != nil {
			return 0, fmt.Errorf("stopped waiting for the elevated shell, which is still running: %w", err)
		}
		event, err := windows.WaitForSingleObject(process, uint32(elevationPollTimeout.Milliseconds()))
		if err != nil {
			return 0, fmt.Errorf("cannot wait for the elevated shell: %w", err)
		}
		if event == uint32(windows.WAIT_TIMEOUT) {
			continue
		}
		var code uint32
		if err := windows.GetExitCodeProcess(process, &code); err != nil {
			return 0, fmt.Errorf("cannot read the elevated shell's exit code: %w", err)
		}
		return int(code), nil
	}
}

func shellExecute(verb, file, parameters, directory string, mask uint32) (windows.Handle, error) {
	info := shellExecuteInfo{
		mask: mask,
		verb: windows.StringToUTF16Ptr(verb),
		file: windows.StringToUTF16Ptr(file),
		show: swShowNormal,
	}
	info.size = uint32(unsafe.Sizeof(info))
	if parameters != "" {
		info.parameters = windows.StringToUTF16Ptr(parameters)
	}
	if directory != "" {
		info.directory = windows.StringToUTF16Ptr(directory)
	}
	// SyscallN with the conversion written inline, not LazyProc.Call: only the
	// first form is covered by the rule that keeps the pointed-at memory alive
	// across the call.
	result, _, err := syscall.SyscallN(procShellExecuteExW.Addr(), uintptr(unsafe.Pointer(&info)))
	if result == 0 {
		return 0, err
	}
	return info.process, nil
}
