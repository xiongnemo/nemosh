// Package capability says what each command of this shell can be given: which
// options it accepts, and what kind of operand.
//
// It exists because two features want the same knowledge and would otherwise
// each invent it. Tab completion needs it to offer `-a` after `ls -` and to
// offer only directories after `cd `. The suggestion renderer needs it to colour
// a command by whether it exists and an option by whether it is accepted. A
// second copy of that knowledge would drift from the first within a release.
//
// What is claimed here is only what is checked, and every row is a command this
// shell ships. The applet table is bound to behaviour by a test that runs each
// applet and fails if a declared option is refused or an undeclared one is
// accepted, so it cannot quietly fall behind the code. Builtins carry an operand
// kind and no option claims, because nothing here measures their options yet --
// saying nothing is the honest form of not knowing.
//
// Commands this shell does *not* ship are described in completions/ instead, as
// data with its provenance attached. That separation is the point: for a while
// `ssh` had a row here marked External, meaning "transcribed, nobody can check
// it", and one unverified row in a table whose whole value is that it is
// verified was a hole that would only ever widen.
package capability

import (
	"sort"
	"strings"
)

// OperandKind is what a command's operands can be.
//
// Only two, deliberately. Narrowing what completion offers is safe only when the
// omitted candidates could never have been meant, so a command that merely
// prefers directories is not a separate kind; it is AnyPath like everything else.
type OperandKind int

const (
	AnyPath OperandKind = iota
	Directory
	// HostName is a remote machine: `ssh HOST`. Unlike the other two this
	// narrows to a source that is not the filesystem at all, so completing the
	// wrong kind here is not "too many candidates" but "candidates from the
	// wrong universe" -- `ssh notes.txt` is never what was meant.
	HostName
)

// Command is one command's surface.
type Command struct {
	Name string
	// Short is the accepted option letters, unordered, as one string. `ls -al`
	// clusters them, so a letter rather than a word is the unit.
	Short string
	// Long is the accepted long option names without their leading dashes.
	Long []string
	// Operand is what a non-option word can be.
	Operand OperandKind
	// Builtin marks a command the shell runs itself rather than an applet. The
	// distinction matters for the drift test, which can run an applet in
	// isolation and cannot run a builtin without a whole runtime.
	Builtin bool
	// ValueShort is the subset of Short whose option takes the next word as its
	// argument. Completion needs it to know that the word after `ssh -p` is a
	// port and not a host: offering a host there is the wrong universe, which is
	// worse than offering nothing.
	ValueShort string
	// FileShort is the subset of ValueShort whose argument is a path, so
	// `ssh -i ` completes a file. zsh knows this because its completion grammar
	// declares an argument type per option; bash-completion hand-writes a
	// `case $prev in` per command. This is the same knowledge as data.
	FileShort string
}

// TakesValue reports whether the option letter consumes the following word.
func (c Command) TakesValue(flag rune) bool {
	return strings.ContainsRune(c.ValueShort, flag)
}

// TakesFile reports whether that consumed word is a path.
func (c Command) TakesFile(flag rune) bool {
	return strings.ContainsRune(c.FileShort, flag)
}

// AcceptsShort reports whether the command takes a one-letter option.
func (c Command) AcceptsShort(flag rune) bool {
	return strings.ContainsRune(c.Short, flag)
}

// AcceptsLong reports whether the command takes a long option, given the name
// with its dashes and any `=value` already removed.
func (c Command) AcceptsLong(name string) bool {
	for _, known := range c.Long {
		if known == name {
			return true
		}
	}
	return false
}

// Lookup finds a command by name.
func Lookup(name string) (Command, bool) {
	command, ok := byName[name]
	return command, ok
}

// Known reports whether this shell has a command of that name at all, which is
// what decides the colour it is drawn in.
//
// A name absent here is not necessarily unrunnable: an external program on PATH
// is not in this table, and neither is a function or an alias the session has
// defined. Callers that can see those must consult them first; this answers only
// for what the shell itself carries.
func Known(name string) bool {
	_, ok := byName[name]
	return ok
}

// OperandKindOf answers for a name that may not be a known command, because the
// half-typed line completion runs against usually contains one.
func OperandKindOf(name string) OperandKind {
	if command, ok := byName[name]; ok {
		return command.Operand
	}
	return AnyPath
}

// Options lists what could follow a `-` for a command, long forms first written
// with their dashes, sorted so the offer is stable.
func Options(name string) []string {
	command, ok := byName[name]
	if !ok {
		return nil
	}
	offers := make([]string, 0, len(command.Short)+len(command.Long))
	for _, flag := range command.Short {
		offers = append(offers, "-"+string(flag))
	}
	for _, long := range command.Long {
		offers = append(offers, "--"+long)
	}
	return offers
}

var byName = func() map[string]Command {
	index := make(map[string]Command, len(commands))
	for _, command := range commands {
		index[command.Name] = command
	}
	return index
}()

// Names lists every command with a row, sorted. Tests walk it to hold the whole
// table to a rule rather than to one row at a time.
func Names() []string {
	names := make([]string, 0, len(commands))
	for _, command := range commands {
		names = append(names, command.Name)
	}
	sort.Strings(names)
	return names
}
