//go:build !windows

package runtime

import "errors"

// /dev/clipboard is a Windows promise (docs/design/windows-path-model.md:241)
// backed by the Win32 clipboard API. Off Windows the path still resolves as a
// device -- the path model is platform-independent -- and then says plainly
// that there is nothing behind it, rather than shelling out to xclip or pbcopy
// and pretending the promise is portable.
var errClipboardUnavailable = errors.New("/dev/clipboard is only available on Windows")

func readClipboardTextRaw() (string, error) { return "", errClipboardUnavailable }

func writeClipboardTextRaw(string) error { return errClipboardUnavailable }
