package runtime

import "strings"

// bangSubshellAt reports whether the `!` at bang, with a `(` after it, negates a subshell
// rather than opening an extended pattern: where a command begins -- after nothing, a
// separator, `(` or `{`, or reserved words a command may follow -- and not a case pattern,
// which a `;;` goes before or a `)` follows. busybox-w32 has no extended patterns, and bash
// none in a script unless extglob is set, so both run `if !(a && b); then` as the negation it
// is. The word was taken for a pattern instead, matched against the directory, and the first
// file it matched run as the command.
func bangSubshellAt(line string, bang int) bool {
	back := previousNonBlankIndex(line, bang)
	start := back < 0 || strings.IndexByte(";&|\n({", line[back]) >= 0 || afterCommandIntroducer(line, bang)
	if !start || back > 0 && line[back-1] == ';' && (line[back] == ';' || line[back] == '&') {
		return false
	}
	rest := strings.TrimLeft(line[skipBalancedParens(line, bang+1):], " \t")
	return !strings.HasPrefix(rest, ")")
}
