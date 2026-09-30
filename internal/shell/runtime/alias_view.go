package runtime

import "maps"

// The aliases a command is read with.
//
// The references substitute an alias as they read a line, so what is in force for a command is
// what was defined when its line was read: not an alias defined earlier on the same line, nor
// in the same `if` or loop, and inside a function's body what was defined when the function
// was, not when it is called. This shell reads a script whole before it runs any of it and
// substitutes as a command runs, so each of those came out the other way: `alias e=echo; e
// one` said one where both references say `e: not found`, and a function defined before an
// alias used it all the same.
//
// So each line takes a copy of the aliases as it begins -- a line of the script, of eval's
// text, of a sourced file, of a trap's or at a prompt -- and its commands are substituted from
// that copy; a function keeps the copy its definition's line had. A subshell, a command
// substitution, a pipeline's stage and a job take the line's copy with them (snapshot.go):
// `alias e=echo; (e same)` and `x=$(e same)` found the alias the way `e same` did not. alias, unalias, type and
// command -v go on reading the aliases as they are now, as in both references.

// aliasesInForce is the aliases a command is substituted from.
func (r Runtime) aliasesInForce() map[string]string {
	if r.aliasView != nil {
		return r.aliasView
	}
	return r.aliases
}

// aliasSnapshot is a copy of the aliases for a line that begins now.
func (r Runtime) aliasSnapshot() map[string]string {
	if copied := maps.Clone(r.aliases); copied != nil {
		return copied
	}
	return map[string]string{}
}
