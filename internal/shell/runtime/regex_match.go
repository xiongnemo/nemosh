package runtime

// BASH_REMATCH: what `[[ string =~ regex ]]` matched.
//
// The match ran and the answer was right; the captures were thrown away. That is how a
// script pulls fields out of a string in bash --
//
//	[[ $line =~ ^([0-9]+):(.*)$ ]] && echo "${BASH_REMATCH[1]} ${BASH_REMATCH[2]}"
//
// -- and with nothing recorded the second half of every such line read as empty. A
// condition that answers correctly and then loses the reason is worse than one that
// fails, because the script carries on.

// bashRematch is the array name bash uses. Spelled out once so the two places that
// touch it cannot disagree.
const bashRematch = "BASH_REMATCH"

// recordRegexMatch stores the groups and reports whether there was a match.
//
// Element 0 is the whole match and the rest are the groups, which is bash's layout. A
// group that did not participate is the empty string rather than absent, because the
// array is indexed by group number and a gap would shift every later one.
//
// On no match the array is left as it was, which is also bash's behaviour: a script
// that tests one pattern and then reads BASH_REMATCH after a *different*, failed test
// sees the last successful match. Clearing it would be tidier and would not be bash.
func (r Runtime) recordRegexMatch(groups []string) bool {
	if groups == nil {
		return false
	}
	if r.arrays != nil {
		r.arrays.set(bashRematch, groups)
	}
	return true
}
