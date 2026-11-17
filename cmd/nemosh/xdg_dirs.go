package main

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// Where a user's own files are kept: in the XDG layout under the home directory, on Windows as
// anywhere, as every tool of the user's keeps them from 2026-11-18 -- not %APPDATA%. History is
// state, $XDG_STATE_HOME or ~/.local/state; the completion specs a user writes are configuration,
// $XDG_CONFIG_HOME or ~/.config. A variable is honoured only when it is absolute, as the XDG
// specification requires. The rc file is the exception, ~/.nemoshrc or $ENV, as a shell's is.
//
// Where an item is only at the place a release up to 1.5 kept it, it is read there and said
// once, on stderr, before the first prompt; `nemosh --migrate` moves it (migrate.go). A home that
// never had one is never given one.

// xdgBase is the directory variable names, when it names an absolute one, and home's fallback
// otherwise, in the shell's spelling: home as it is given, and a slash after it.
func xdgBase(lookup func(string) (string, bool), variable, home, fallback string) string {
	if value, ok := lookup(variable); ok && filepath.IsAbs(value) {
		return strings.TrimRight(value, `/\`)
	}
	return strings.TrimRight(home, `/\`) + "/" + fallback
}

// historyPaths is where history is kept, $XDG_STATE_HOME/nemosh/nemosh_history, and where every
// release up to 1.5 kept it, ~/.nemosh_history. Both empty without a home. The file is
// nemosh_history and not history, as fish's is fish_history, so HISTFILE ends in _history as
// bash's and busybox's do: Oils' interactive.test.sh asks exactly that, and a bare history
// failed it on CI.
func historyPaths(lookup func(string) (string, bool)) (current, legacy string) {
	home, ok := lookup("HOME")
	if !ok || home == "" {
		return "", ""
	}
	return xdgBase(lookup, "XDG_STATE_HOME", home, ".local/state") + "/nemosh/nemosh_history",
		strings.TrimRight(home, `/\`) + "/.nemosh_history"
}

// resolveHistoryPath is the history file a session uses when nothing names one: the current
// place if the file is there, the old one if it is there instead, and the current place
// otherwise. exists answers for a path in the shell's spelling.
func resolveHistoryPath(lookup func(string) (string, bool), exists func(string) bool) (path string, fromLegacy bool) {
	current, legacy := historyPaths(lookup)
	switch {
	case current == "" || exists(current):
		return current, false
	case exists(legacy):
		return legacy, true
	}
	return current, false
}

// specPaths is where a user's completion specs are kept, $XDG_CONFIG_HOME/nemosh/completions, and
// where releases up to 1.5 looked first, %APPDATA%\nemosh\completions, as native paths, each empty
// when what it is made from is not set. The home is HOME, or USERPROFILE without it.
func specPaths(lookup func(string) (string, bool)) (current, legacy string) {
	home, ok := lookup("HOME")
	if !ok || home == "" {
		home, _ = lookup("USERPROFILE")
	}
	if home != "" {
		current = filepath.FromSlash(xdgBase(lookup, "XDG_CONFIG_HOME", home, ".config") + "/nemosh/completions")
	}
	if appData, ok := lookup("APPDATA"); ok && appData != "" {
		legacy = filepath.Join(appData, "nemosh", "completions")
	}
	return current, legacy
}

// sessionSpecLookup is the environment a session reads to say where its completion specs are;
// the test binary's TestMain gives it one without APPDATA, so no test looks in a real one.
var sessionSpecLookup = environmentLookup

// sayOldSpecs says once that completion specs are read from where releases up to 1.5 kept them,
// when one is there; each is used where it is, the new place first (specSearchDirectories).
func sayOldSpecs(stderr io.Writer, lookup func(string) (string, bool)) {
	current, legacy := specPaths(lookup)
	if legacy == "" {
		return
	}
	if specs, _ := filepath.Glob(filepath.Join(legacy, "*.toml")); len(specs) > 0 {
		sayOldLocation(stderr, "completion specs", filepath.ToSlash(legacy), filepath.ToSlash(current))
	}
}

// sayOldLocation is the one line that says an item was read where an older release kept it.
func sayOldLocation(stderr io.Writer, what, old, current string) {
	fmt.Fprintf(stderr, "nemosh: using %s from the old location %s; run \"nemosh --migrate\" to move it to %s\n", what, old, current)
}
