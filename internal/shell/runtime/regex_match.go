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
// A failed match empties the array. It was left holding the last successful match, on the
// belief that bash does the same, and bash does not: every attempt clears it first
// (sh_regmatch, lib/sh/shmatch.c), so after a failed test `${#BASH_REMATCH[@]}` is 0. A
// script that tried one pattern and then a second that failed read the first one's groups
// as the second's.
func (r Runtime) recordRegexMatch(groups []string) bool {
	if r.arrays != nil {
		r.arrays.set(bashRematch, groups)
	}
	return groups != nil
}
