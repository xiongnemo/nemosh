//go:build windows

package runtime

import (
	"errors"
	"fmt"
	goruntime "runtime"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	cfUnicodeText = 13
	gmemMoveable  = 0x0002

	clipboardOpenAttempts = 8
	clipboardOpenBackoff  = 10 * time.Millisecond
)

// kernel32 is declared once for the package in pipe_interrupt_windows.go; a
// second lazy handle to the same DLL would buy nothing.
var (
	user32                         = windows.NewLazySystemDLL("user32.dll")
	procOpenClipboard              = user32.NewProc("OpenClipboard")
	procCloseClipboard             = user32.NewProc("CloseClipboard")
	procEmptyClipboard             = user32.NewProc("EmptyClipboard")
	procGetClipboardData           = user32.NewProc("GetClipboardData")
	procSetClipboardData           = user32.NewProc("SetClipboardData")
	procIsClipboardFormatAvailable = user32.NewProc("IsClipboardFormatAvailable")
	procCountClipboardFormats      = user32.NewProc("CountClipboardFormats")

	procGlobalAlloc  = kernel32.NewProc("GlobalAlloc")
	procGlobalFree   = kernel32.NewProc("GlobalFree")
	procGlobalLock   = kernel32.NewProc("GlobalLock")
	procGlobalUnlock = kernel32.NewProc("GlobalUnlock")
)

// withClipboard runs body with the clipboard open. Windows associates the open
// clipboard with the calling *thread*, and a goroutine is free to change
// threads at any call, so the whole sequence is pinned to one OS thread.
func withClipboard(body func() error) error {
	goruntime.LockOSThread()
	defer goruntime.UnlockOSThread()
	if err := openClipboard(); err != nil {
		return err
	}
	defer procCloseClipboard.Call()
	return body()
}

// Only one process may hold the clipboard at a time and Windows offers no way
// to wait for it, so the documented remedy is to try again.
func openClipboard() error {
	var callErr error
	for attempt := range clipboardOpenAttempts {
		if attempt > 0 {
			time.Sleep(clipboardOpenBackoff)
		}
		opened, _, err := procOpenClipboard.Call(0)
		if opened != 0 {
			return nil
		}
		callErr = err
	}
	return clipboardError("open clipboard", callErr)
}

func readClipboardTextRaw() (string, error) {
	var callErr error
	for attempt := range clipboardOpenAttempts {
		if attempt > 0 {
			time.Sleep(clipboardOpenBackoff)
		}
		text, err := readClipboardTextOnce()
		if err == nil {
			return text, nil
		}
		callErr = err
	}
	return "", callErr
}

func readClipboardTextOnce() (string, error) {
	var text string
	err := withClipboard(func() error {
		available, _, _ := procIsClipboardFormatAvailable.Call(cfUnicodeText)
		if available == 0 {
			// No text on the clipboard reads as no text, not as a failure --
			// the same shape an empty file has.
			return nil
		}
		handle, _, callErr := procGetClipboardData.Call(cfUnicodeText)
		if handle == 0 {
			// The format is on the clipboard -- checked just above -- and fetching it
			// still failed, which on this platform means something else is mid-change.
			// Measured: writing and then reading back with no pause at all fails every
			// time, and with a one-millisecond pause succeeds every time. Windows 10 and
			// later run a clipboard history service that opens the clipboard on every
			// change, so "just written" is exactly when a read is most likely to lose.
			//
			// So the same remedy the open path already uses, for the same reason: try
			// again. Without it `printf x > /dev/clipboard && cat /dev/clipboard` in one
			// script could fail, and the failure would look like the clipboard being
			// empty rather than busy.
			return clipboardError("read clipboard", callErr)
		}
		pointer, _, callErr := procGlobalLock.Call(handle)
		if pointer == 0 {
			return clipboardError("lock clipboard", callErr)
		}
		defer procGlobalUnlock.Call(handle)
		text = windows.UTF16PtrToString(asUint16Pointer(pointer))
		return nil
	})
	return text, err
}

func writeClipboardTextRaw(text string) error {
	// UTF16FromString rejects an interior NUL, which is exactly the promise
	// "text-only" makes (docs/design/windows-path-model.md:241).
	encoded, err := windows.UTF16FromString(text)
	if err != nil {
		return fmt.Errorf("clipboard text: %w", err)
	}
	return withClipboard(func() error {
		if emptied, _, callErr := procEmptyClipboard.Call(); emptied == 0 {
			return clipboardError("empty clipboard", callErr)
		}
		handle, err := globalCopy(encoded)
		if err != nil {
			return err
		}
		if stored, _, callErr := procSetClipboardData.Call(cfUnicodeText, handle); stored == 0 {
			procGlobalFree.Call(handle)
			return clipboardError("set clipboard", callErr)
		}
		// SetClipboardData took ownership of the block on success; freeing it
		// here would hand the system a dangling handle.
		return nil
	})
}

// globalCopy puts encoded into a moveable global block, which is the only kind
// SetClipboardData accepts.
func globalCopy(encoded []uint16) (uintptr, error) {
	size := uintptr(len(encoded)) * unsafe.Sizeof(encoded[0])
	handle, _, callErr := procGlobalAlloc.Call(gmemMoveable, size)
	if handle == 0 {
		return 0, clipboardError("allocate clipboard memory", callErr)
	}
	pointer, _, callErr := procGlobalLock.Call(handle)
	if pointer == 0 {
		procGlobalFree.Call(handle)
		return 0, clipboardError("lock clipboard memory", callErr)
	}
	copy(unsafe.Slice(asUint16Pointer(pointer), len(encoded)), encoded)
	procGlobalUnlock.Call(handle)
	return handle, nil
}

// asUint16Pointer reinterprets the address GlobalLock returned. Reading through
// the uintptr's own address is what keeps `go vet`'s unsafeptr check satisfied:
// the block is locked for the whole span, so the address cannot move under us.
func asUint16Pointer(address uintptr) *uint16 {
	return *(**uint16)(unsafe.Pointer(&address))
}

// LazyProc.Call always returns a non-nil error carrying GetLastError, and a
// call that failed without setting it leaves 0 there -- which formats as "The
// operation completed successfully" and would read as nonsense in a diagnostic.
func clipboardError(action string, err error) error {
	var errno syscall.Errno
	if err == nil || (errors.As(err, &errno) && errno == 0) {
		return errors.New(action + ": failed")
	}
	return fmt.Errorf("%s: %w", action, err)
}
