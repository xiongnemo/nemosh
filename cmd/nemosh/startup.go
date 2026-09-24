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
// and cannot be read or parsed is reported, because a quietly unconfigured shell
// is worse. The answers are sourceProfile's.
func sourceStartupFile(ctx context.Context, rt runtime.Runtime, stderr io.Writer) (int, bool) {
	path, present := rt.LookupVariable("ENV")
	if !present || path == "" {
		if value, ok := rt.LookupEnv("ENV"); ok {
			path = value
		}
	}
	if path == "" {
		return 0, false
	}
	return sourceProfile(ctx, rt, stderr, path, rt.ResolvePath(rt.ExpandPromptString(ctx, path, 0)))
}

// sourceLoginProfiles is what busybox's login shell reads, in its order: /etc/profile and
// then $HOME/.profile (shell/ash.c, read_profile), each through the shell's own path
// model, so /etc/profile is the file `cat /etc/profile` shows. `-l` was an invalid option,
// and it is the one a terminal profile adds to start a login shell.
func sourceLoginProfiles(ctx context.Context, rt runtime.Runtime, stderr io.Writer) (int, bool) {
	if status, exited := sourceProfile(ctx, rt, stderr, "/etc/profile", rt.ResolvePath("/etc/profile")); exited {
		return status, true
	}
	home, present := rt.LookupVariable("HOME")
	if !present {
		home, present = rt.LookupEnv("HOME")
	}
	if !present || home == "" {
		return 0, false
	}
	path := strings.TrimRight(home, `/\`) + "/.profile"
	return sourceProfile(ctx, rt, stderr, path, rt.ResolvePath(path))
}

// sourceProfile runs one startup file, named in diagnostics as it was spelled, and answers
// whether an `exit` in it ended the shell, with the status to end it with (startupExit).
//
// A file that ran and finished non-zero is not reported. A profile's last status is not a
// failure: one ending `[ -f ~/.local ] && . ~/.local` returns 1 wherever that file is
// missing, and was reported as "exited 1" at every start. Neither reference says anything,
// and a command that fails inside a profile says so itself.
func sourceProfile(ctx context.Context, rt runtime.Runtime, stderr io.Writer, path, resolved string) (int, bool) {
	contents, err := os.ReadFile(resolved)
	if err != nil {
		// A missing startup file is the ordinary case for a machine that has
		// never configured one, so only a real failure is worth a line.
		if !os.IsNotExist(err) {
			fmt.Fprintf(stderr, "nemosh: %s: %v\n", path, err)
		}
		return 0, false
	}
	status, exited, err := rt.SourceStartup(ctx, string(contents))
	if err != nil {
		fmt.Fprintf(stderr, "nemosh: %s: %v\n", path, err)
	}
	return status, exited
}

// startupExit ends the shell from inside a startup file. `exit` there is the shell's exit,
// EXIT trap and all, as it is in busybox; status 0 is an exitStatus too, so the caller
// stops rather than carrying on to the script or the prompt.
func startupExit(rt runtime.Runtime, status int) error {
	rt.CloseBatch(status)
	return exitStatus(status)
}
