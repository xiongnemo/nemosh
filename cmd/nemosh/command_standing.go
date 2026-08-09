package main

import (
	"sort"
	"strings"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/capability"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// commandNames is every name this shell carries itself. Built once: it cannot
// change during a session, and this is on the keystroke path.
var commandNames = func() func() []string {
	var names []string
	return func() []string {
		if names != nil {
			return names
		}
		names = append(names, runtime.BuiltinNames()...)
		names = append(names, applets.DefaultRegistry.Names()...)
		return names
	}
}()

// commandStanding is what the shell can say about a name, and there are three
// answers rather than two.
//
// The third one is the point. PATH is read in the background, so for the first
// moments of a session the shell does not yet know whether `wsl` can run. Drawing
// it red then and green a moment later would be worse than drawing it plainly:
// a colour that changes under you is a colour you learn to ignore.
type commandStanding int

const (
	standingUnknown commandStanding = iota
	standingRunnable
	standingUndetermined
)

// commandOracle answers for one name. Passed in rather than reached for, so the
// renderer can be tested without a PATH and so the editor decides what counts.
type commandOracle func(name string) commandStanding

// shellCommands is what a session can run beyond its builtins and applets: the
// aliases and functions it has defined. Refreshed each prompt from the runtime,
// because both change while the session runs.
type shellCommands struct {
	names map[string]bool
	path  *pathIndex
}

func newShellCommands(path *pathIndex) *shellCommands {
	return &shellCommands{names: map[string]bool{}, path: path}
}

func (s *shellCommands) set(names []string) {
	fresh := make(map[string]bool, len(names))
	for _, name := range names {
		fresh[name] = true
	}
	s.names = fresh
}

// standing asks the cheap sources first and PATH last, which is also the order
// this shell resolves a command in.
func (s *shellCommands) standing(name string) commandStanding {
	if name == "" {
		return standingUndetermined
	}
	// A name with a path in it is not a command name, it is a file, and whether
	// it can run is a question about that file rather than about PATH.
	if strings.ContainsAny(name, "/\\") {
		return standingUndetermined
	}
	if s.names[name] || capability.Known(name) {
		return standingRunnable
	}
	found, ready := s.path.has(name)
	if !ready {
		return standingUndetermined
	}
	if found {
		return standingRunnable
	}
	return standingUnknown
}

// candidates is every name a suggestion may propose, cheapest source first.
// PATH is included because a command the shell can run is a command worth
// finishing, and it is why this index exists at all.
func (s *shellCommands) candidates() []string {
	names := make([]string, 0, len(s.names)+len(commandNames()))
	for name := range s.names {
		names = append(names, name)
	}
	sort.Strings(names)
	names = append(names, commandNames()...)
	return append(names, s.path.candidates()...)
}
