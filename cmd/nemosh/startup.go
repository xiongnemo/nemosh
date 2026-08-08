package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// sourceStartupFile runs $ENV for an interactive shell, which is what POSIX
// specifies and what busybox ash does (shell/ash.c:16801). Nothing else is
// read: /etc/profile and ~/.profile belong to a login shell, and Nemosh has no
// login mode to justify them.
//
// Using $ENV rather than inventing a `.nemoshrc` is the point. A machine that
// already configures busybox has ENV pointing at its rc file, so the same
// configuration reaches Nemosh with nothing to change.
//
// Silence when there is nothing to source: not every user has an ENV, and a
// shell that complained on every launch would be unusable. A file that exists
// and fails is reported, because a quietly unconfigured shell is worse.
func sourceStartupFile(ctx context.Context, rt runtime.Runtime, stderr io.Writer) {
	path, present := rt.LookupVariable("ENV")
	if !present || path == "" {
		if value, ok := rt.LookupEnv("ENV"); ok {
			path = value
		}
	}
	if path == "" {
		return
	}
	resolved := rt.ResolvePath(rt.ExpandPromptString(ctx, path, 0))
	contents, err := os.ReadFile(resolved)
	if err != nil {
		// A missing startup file is the ordinary case for a machine that has
		// never configured one, so only a real failure is worth a line.
		if !os.IsNotExist(err) {
			fmt.Fprintf(stderr, "nemosh: %s: %v\n", path, err)
		}
		return
	}
	if status := rt.RunScript(ctx, string(contents)); status != 0 {
		fmt.Fprintf(stderr, "nemosh: %s: exited %d\n", path, status)
	}
}
