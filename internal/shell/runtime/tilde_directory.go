package runtime

import (
	"strconv"
	"strings"
)

// `~+`, `~-`, `~N`, `~+N` and `~-N`: naming a directory by its place on the stack.
//
// A stack you cannot name positions in is a stack you can only walk, so these arrived with
// `pushd`. `~-` is the one that earns its keep on its own -- `cp file ~-` is "back where I
// just was" without typing it, and it works whether or not anything was ever pushed.
//
// The two directions are the ones `pushd +N` and `pushd -N` already use: `+N` counts from
// the current directory and `-N` from the far end. One mental model covers both, which is
// the argument for spelling them the same way here rather than inventing a third.

// expandDirectoryTilde answers a stack reference, or reports that this is not one.
func (r Runtime) expandDirectoryTilde(value string) (string, bool) {
	if !strings.HasPrefix(value, "~") || value == "~" {
		return "", false
	}
	// Only up to the first slash is the reference; the rest is a path under it, so
	// `~-/sub` works the way `~/sub` does.
	reference, rest, _ := strings.Cut(value[1:], "/")
	resolved, ok := r.directoryTildeTarget(reference)
	if !ok {
		return "", false
	}
	if rest == "" {
		return resolved, true
	}
	return strings.TrimRight(resolved, `/\`) + "/" + rest, true
}

// directoryTildeTarget resolves the part between the tilde and the slash.
func (r Runtime) directoryTildeTarget(reference string) (string, bool) {
	// `~/path` has nothing between them, and is the home-directory form rather than a
	// stack reference. Without this the indexing below panicked on it -- which the
	// existing tilde tests caught immediately, being the one case they all use.
	if reference == "" {
		return "", false
	}
	switch reference {
	case "+":
		return r.WorkingDirectory(), true
	case "-":
		// The previous directory, which is where `cd -` goes. Unset before the first
		// `cd` -- and then `~-` is left as written rather than becoming the empty
		// string, because an empty path silently means "the current directory" and
		// would move a file somewhere nobody asked for.
		previous := r.vars["OLDPWD"]
		return previous, previous != ""
	}
	sign, digits := byte('+'), reference
	if reference[0] == '+' || reference[0] == '-' {
		sign, digits = reference[0], reference[1:]
	}
	number, err := strconv.Atoi(digits)
	if err != nil || number < 0 {
		return "", false
	}
	entries := r.directoryEntries()
	index := number
	if sign == '-' {
		index = -number - 1
	}
	index, inRange := resolveDirStackOffset(index, len(entries))
	if !inRange {
		return "", false
	}
	return entries[index], true
}

// wordExpandsTilde reports whether a word begins with a tilde form worth expanding.
//
// One gate rather than the two that existed -- the lexer's and the brace rebuilder's --
// because they had the same list written out twice and adding `~+` to one of them would
// have made `~+` work everywhere except after a brace expansion.
//
// `~user` is deliberately not here: it is left as written, which support-matrix.md records
// as a measured non-goal.
func wordExpandsTilde(raw string) bool {
	if raw == "~" || strings.HasPrefix(raw, "~/") {
		return true
	}
	if !strings.HasPrefix(raw, "~") {
		return false
	}
	reference, _, _ := strings.Cut(raw[1:], "/")
	if reference == "+" || reference == "-" {
		return true
	}
	// `~N`, `~+N`, `~-N` -- a stack position. Anything else after the tilde is a user
	// name or a literal, and both are left alone.
	digits := strings.TrimPrefix(strings.TrimPrefix(reference, "+"), "-")
	if digits == "" || digits != reference && len(reference) == 1 {
		return false
	}
	for index := 0; index < len(digits); index++ {
		if digits[index] < '0' || digits[index] > '9' {
			return false
		}
	}
	return true
}
