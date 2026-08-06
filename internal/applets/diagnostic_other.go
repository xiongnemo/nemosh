//go:build !windows

package applets

import (
	"errors"
	"syscall"
)

// Only Windows needs an errno override; elsewhere the portable fs sentinels and
// the underlying syscall error text already agree with strerror.
func platformCauseText(error) (string, bool) { return "", false }

func isCrossDeviceRename(err error) bool { return errors.Is(err, syscall.EXDEV) }
