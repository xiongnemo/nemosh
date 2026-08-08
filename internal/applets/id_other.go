//go:build !windows

package applets

import "os"

// Off Windows the question has a real answer, so ask it.
func currentUserID() int { return os.Getuid() }
