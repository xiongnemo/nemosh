package main

import (
	"strings"

	"github.com/xiongnemo/nemosh/internal/capability"
)

// A command's operands are not all the same kind, and offering the wrong kind
// is the difference between Tab helping and Tab being in the way: `cd ` must not
// offer a regular file, because `cd notes.txt` cannot work.
//
// busybox carries exactly one rule of this shape and spells it out in the middle
// of its line editor -- if the first two characters are `c` and `d` and the third
// is a blank, completion switches to directories only (libbb/lineedit.c:1245).
// That is the whole of its per-command knowledge. bash keeps the same knowledge
// in shell functions outside the shell, and zsh in a declared option grammar per
// command. This is busybox's rule with the command name looked up instead of
// hardcoded, which costs nothing and holds the next command that wants it.
// The table itself lives in internal/capability, where a test binds it to what
// the applets actually do. Two features want this knowledge -- completion and
// the suggestion renderer -- and a second copy would drift from the first.
func completesDirectoriesOnly(command string) bool {
	return capability.OperandKindOf(command) == capability.Directory
}

// commandInProgress is the command word that the operand being completed belongs
// to, or empty when there is not one yet.
//
// It reads the text rather than parsing it, for the same reason completesCommand
// does: the line is half-typed and will usually not parse at all. Everything up
// to the last separator that begins a new command is dropped, so `ls | cd `
// answers `cd` rather than `ls`.
//
// A leading assignment is not skipped, so `HOME=/tmp cd ` answers `HOME=/tmp`
// and falls back to completing any path. That is the safe direction to be wrong
// in -- offering too much rather than hiding what was wanted -- and busybox does
// not handle it either.
func commandInProgress(prefix string) string {
	start := strings.LastIndexAny(prefix, "|&;({")
	fields := strings.Fields(prefix[start+1:])
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// disambiguateOperand makes a completed name usable as an operand, in the one
// case escaping cannot reach.
//
// A name beginning with `-` is read as options by the command that receives it,
// and no quoting changes that. Measured: `ls -l \-1.18-windows.xml` and
// `ls -l '-1.18-windows.xml'` both still fail, because quoting is resolved by
// the shell and the operand/option split happens afterwards, inside the applet.
// `./` is the only spelling that fixes it, and it names the same file.
//
// Only when there is no directory part yet -- `sub/-x` does not begin with a
// dash, so it is left alone, and a second Tab continuing from `./-1` is not
// prefixed twice.
//
// This is a divergence: bash and busybox both hand back the bare name and leave
// the user with a command that cannot run. It is kept small on purpose -- it
// applies to operands, never to a command word, where `./name` would mean
// something different.
func disambiguateOperand(name string) string {
	if strings.HasPrefix(name, "-") {
		return "./" + name
	}
	return name
}

// Escaping is what makes a completed name usable rather than merely correct. A
// Windows path is very likely to hold a blank -- `Program Files` is on every
// machine there is -- and inserting it raw produces a command line that names
// two operands, neither of which exists.
//
// The set is busybox's is_special_char (libbb/lineedit.c:1330), which is the
// list its own parser needs escaped, and this shell's parser reads the same
// characters the same way.
const shellSpecialCharacters = " `'\"\\#$~?*[{()&;|<>"

func escapeForInsertion(text string) string {
	var escaped strings.Builder
	for _, r := range text {
		if r < 0x80 && strings.ContainsRune(shellSpecialCharacters, r) {
			escaped.WriteByte('\\')
		}
		escaped.WriteRune(r)
	}
	return escaped.String()
}

// unescapeTypedWord undoes the above, so that what the user typed is matched
// against real filenames. Without it, completing `My\ Do` would look for a file
// whose name begins with a backslash.
func unescapeTypedWord(text string) string {
	var plain strings.Builder
	escaped := false
	for _, r := range text {
		if !escaped && r == '\\' {
			escaped = true
			continue
		}
		escaped = false
		plain.WriteRune(r)
	}
	return plain.String()
}
