package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// sourceStartupFile runs $ENV for an interactive shell, which is what POSIX
// specifies and what busybox ash does (shell/ash.c:16801). A login shell, `nemosh -l`,
// reads its profiles first (sourceLoginProfiles); nothing else is read.
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
	sourceProfile(ctx, rt, stderr, path, rt.ResolvePath(rt.ExpandPromptString(ctx, path, 0)))
}

// sourceLoginProfiles is what busybox's login shell reads, in its order: /etc/profile and
// then $HOME/.profile (shell/ash.c, read_profile), each through the shell's own path
// model, so /etc/profile is the file `cat /etc/profile` shows. `-l` was an invalid option,
// and it is the one a terminal profile adds to start a login shell.
func sourceLoginProfiles(ctx context.Context, rt runtime.Runtime, stderr io.Writer) {
	sourceProfile(ctx, rt, stderr, "/etc/profile", rt.ResolvePath("/etc/profile"))
	home, present := rt.LookupVariable("HOME")
	if !present {
		home, present = rt.LookupEnv("HOME")
	}
	if present && home != "" {
		path := strings.TrimRight(home, `/\`) + "/.profile"
		sourceProfile(ctx, rt, stderr, path, rt.ResolvePath(path))
	}
}

// sourceProfile runs one startup file, named in diagnostics as it was spelled.
func sourceProfile(ctx context.Context, rt runtime.Runtime, stderr io.Writer, path, resolved string) {
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
