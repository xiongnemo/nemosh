//go:build !windows

package applets

import "os"

// interruptBlockedRead does nothing away from Windows, which leaves those platforms exactly
// as they were.
//
// The fix it enables is for the Windows console, where a blocked read cannot be ended any
// other way -- ENABLE_LINE_INPUT means the handle is signalled on every keystroke but
// ReadFile does not return until Enter, so waiting on the handle is not an answer either.
// Unix has one: a terminal added to the runtime poller takes SetReadDeadline. Nothing here
// needs it yet, and guessing at it without a terminal to measure against is how the wrong
// half of a platform difference gets written.
func interruptBlockedRead(_ *os.File) {}
