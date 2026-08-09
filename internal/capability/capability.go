// Package capability says what each command of this shell can be given: which
// options it accepts, and what kind of operand.
//
// It exists because two features want the same knowledge and would otherwise
// each invent it. Tab completion needs it to offer `-a` after `ls -` and to
// offer only directories after `cd `. The suggestion renderer needs it to colour
// a command by whether it exists and an option by whether it is accepted. A
// second copy of that knowledge would drift from the first within a release.
//
// What is claimed here is only what is checked. The applet table is bound to
// behaviour by a test that runs each applet and fails if a declared option is
// refused or an undeclared one is accepted, so the table cannot quietly fall
// behind the code. Builtins carry an operand kind and no option claims, because
// nothing here measures their options yet -- saying nothing is the honest form
// of not knowing.
package capability

import "strings"

// OperandKind is what a command's operands can be.
//
// Only two, deliberately. Narrowing what completion offers is safe only when the
// omitted candidates could never have been meant, so a command that merely
// prefers directories is not a separate kind; it is AnyPath like everything else.
type OperandKind int

const (
	AnyPath OperandKind = iota
	Directory
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
