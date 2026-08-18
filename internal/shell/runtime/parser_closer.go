package runtime

import "strings"

// Recognising a compound closer that carries a redirection. Split from parser.go to stay
// under the 250-line ceiling.

// splitCompoundCloser reads a closer with a suffix: `done < file` gives `done` and
// `< file`.
//
// The suffix has to begin with a redirection or a pipe. `done extra` is not a closer
// with a suffix, it is a syntax error, and calling it one here would hide the mistake.
func splitCompoundCloser(line string) (string, string, bool) {
	for _, closer := range [...]string{"fi", "done", "esac"} {
		rest, ok := strings.CutPrefix(line, closer)
		if !ok || rest == "" {
			continue
		}
		if rest[0] != ' ' && rest[0] != '	' && rest[0] != '<' && rest[0] != '>' {
			continue
		}
		suffix := strings.TrimSpace(rest)
		// A redirection may name its descriptor: `done 2>/dev/null`. Skipping the
		// digits is what lets the check below see the `>`.
		digits := 0
		for digits < len(suffix) && suffix[digits] >= '0' && suffix[digits] <= '9' {
			digits++
		}
		if digits > 0 && digits < len(suffix) && strings.ContainsAny(suffix[digits:digits+1], "<>") {
			return closer, suffix, true
		}
		// A redirection only. A pipe after a closer is left to fail as it did, because
		// handling it needs the pipeline built from the remaining words and the brace
		// group spelling already works; see wrapCompoundWithSuffix.
		if suffix == "" || !strings.ContainsAny(suffix[:1], "<>") {
			continue
		}
		return closer, suffix, true
	}
	return "", "", false
}
