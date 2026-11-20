package runtime

import (
	"errors"
	"strings"
)

// sh and bash are this shell, as both names are busybox-w32's own shell, an applet there and so
// run before PATH is asked. A command named either runs this binary as `nemosh bash ARGS`, which
// main reads as bash would read ARGS: run as bash it sets BASH_VERSION, so the guard a bash script
// opens with, `[ -z "$BASH_VERSION" ] && exec bash "$0" "$@"`, runs it once more and not for ever.
// `#!/bin/bash` and `#!/usr/bin/env bash` are the same; see interpreterExecutable.
//
// Until 2026-11-20 PATH came first, so the same script ran under Git's bash, under WSL's
// System32\bash.exe -- in Linux, where C:\ is /mnt/c -- or under this shell, according to what the
// machine had installed. NEMOSH_OVERRIDE_APPLETS=bash, or sh, puts PATH's first again, as
// BB_OVERRIDE_APPLETS does for busybox's applets, and with nothing on PATH by that name it is
// still this shell. `command -v bash` names this binary, since it is what runs.

// shellNames are the names this shell answers to.
func shellNames(key string) bool {
	switch key {
	case "sh", "bash":
		return true
	}
	return false
}

// shellAnswers is whether a command named name is this shell: sh or bash, unless the override
// names it, the conditional forms of which ask PATH alone whether it has one.
func (r Runtime) shellAnswers(name string) bool {
	return shellNames(name) && preferApplet(name, r.vars[overrideAppletsVariable], func(name string) bool {
		_, err := r.programPath(name, false)
		return err == nil
	})
}

// commandPath is where name is run from: this binary for a name the shell answers to, and PATH's
// program otherwise, or this binary again for sh or bash when PATH has none.
func (r Runtime) commandPath(name string, remember bool) (string, error) {
	if r.shellAnswers(name) {
		if self, err := jobExecutable(); err == nil {
			return self, nil
		}
	}
	found, err := r.programPath(name, remember)
	if errors.Is(err, errExternalNotFound) && shellNames(name) {
		if self, selfErr := jobExecutable(); selfErr == nil {
			return self, nil
		}
	}
	return found, err
}

// asShell is the launch of a command named sh or bash that is this binary: the name goes first,
// so the process knows which it is.
func asShell(name, executable string, args []string) ([]string, bool) {
	if !shellNames(name) {
		return nil, false
	}
	self, err := jobExecutable()
	if err != nil || !strings.EqualFold(self, executable) {
		return nil, false
	}
	return append([]string{name}, args...), true
}
