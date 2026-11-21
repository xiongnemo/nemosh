package runtime

import "strings"

// bashArgumentTilde marks a command's argument that looks like an assignment -- a name, or an
// element, then `=` or `+=` -- for an assignment's tildes, after the `=` and after each `:`, as
// bash expands them outside its POSIX mode: `make PREFIX=~/x` gives make the home directory, and
// `echo x=~:~` both. busybox, POSIX and bash's POSIX mode leave the tilde as written, which
// nemosh and sh do; Oils calls bash's a mistake, and records it. A word that is not one,
// `--prefix=~` or `1x=~`, and one whose `=` is quoted, `'q'=~`, are left as they are.
func (r Runtime) bashArgumentTilde(item word) word {
	if !r.BashMode() || r.options.posix || item.assignmentTilde || len(item.parts) == 0 {
		return item
	}
	first := item.parts[0]
	if first.kind != wordPartLiteral || first.quote != quoteUnquoted || !strings.Contains(first.text, "=") || !mayHoldTilde(item) {
		return item
	}
	target, _, found := cutAssignment(first.text)
	name := strings.TrimSuffix(target, "+")
	if open := strings.IndexByte(name, '['); open > 0 && strings.HasSuffix(name, "]") {
		name = name[:open]
	}
	if !found || !isValidVariableName(name) {
		return item
	}
	return assignmentTildeWord(item)
}
