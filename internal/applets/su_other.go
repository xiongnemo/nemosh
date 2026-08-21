//go:build !windows

package applets

import (
	"context"
	"errors"
)

// su is not registered off Windows, so this exists only to keep su.go compiling
// and testable everywhere -- the argument assembly is ordinary string work and
// deserves to be checked on every platform the tests run on.
//
// It is not registered because the name is not ours here. Unix has a real `su`
// in util-linux, and an applet of the same name would shadow it: our own does no
// setuid, reads no user database, and answers only to `root`. Leaving the name
// alone lets PATH find the real one, which is the correct behaviour and the same
// choice busybox-w32 makes by building suw32 only under PLATFORM_MINGW32.
func runElevated(context.Context, elevationPlan) error {
	return errors.New("su elevates a Windows process and has no meaning here")
}
