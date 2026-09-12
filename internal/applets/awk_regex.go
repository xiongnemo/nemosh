package applets

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// Compiling awk's regular expressions.
//
// **`regexp.CompilePOSIX`, not `regexp.Compile`**, and this was measured rather than
// assumed. POSIX ERE is leftmost-**longest**; Go's default is leftmost-first. On `/a|ab/`
// against `ab`:
//
//	regexp.Compile       matches "a"
//	regexp.CompilePOSIX  matches "ab"
//	gawk and busybox     RSTART 1, RLENGTH 2 -- that is, "ab"
//
// Getting this wrong would be a silently different answer in `match`, `sub`, `gsub`,
// `split` and `FS` -- the worst shape a bug can take, because every one of them would
// still return something plausible.
//
// Unlike `sed`, no translation is needed: awk uses ERE, which is what Go's syntax already
// is, give or take the escapes handled below. `sed` translates BRE (sed_regex.go) and that
// is a different and much larger job.

// awkRegexCache keeps compiled patterns, because a dynamic regex -- `$0 ~ x` -- is
// recompiled on every record otherwise, and that is the inner loop of most programs.
var awkRegexCache sync.Map

func compileAwkRegex(pattern string) (*regexp.Regexp, error) {
	if cached, ok := awkRegexCache.Load(pattern); ok {
		switch value := cached.(type) {
		case *regexp.Regexp:
			return value, nil
		case error:
			return nil, value
		}
	}
	compiled, err := regexp.CompilePOSIX(awkTranslateRegex(pattern))
	if err != nil {
		// The failure is cached too: a bad dynamic pattern in a loop would otherwise
		// pay for the parse on every record just to fail again.
		wrapped := fmt.Errorf("bad regular expression /%s/: %v", pattern, err)
		awkRegexCache.Store(pattern, wrapped)
		return nil, wrapped
	}
	awkRegexCache.Store(pattern, compiled)
	return compiled, nil
}

// awkTranslateRegex fixes the handful of places awk's ERE and Go's syntax disagree.
//
// Almost nothing needs doing, which is the point of awk using ERE. What does:
//
//   - **An escaped character Go does not know is the character itself.** awk allows `\/`
//     and `\.`; Go rejects `\/` outright as an invalid escape. So an unknown escape has
//     its backslash dropped when the character is not a regex metacharacter, and is kept
//     when it is.
//   - **A brace that is not a repetition is a literal.** `/{/` is a valid awk pattern and
//     an error in Go, which is strict about `{`.
func awkTranslateRegex(pattern string) string {
	var out strings.Builder
	for index := 0; index < len(pattern); index++ {
		character := pattern[index]
		if character != '\\' || index+1 >= len(pattern) {
			out.WriteByte(character)
			continue
		}
		next := pattern[index+1]
		index++
		if strings.IndexByte(`\.+*?()|[]{}^$`, next) >= 0 {
			// A metacharacter keeps its backslash: `\.` is a literal dot in both.
			out.WriteByte('\\')
			out.WriteByte(next)
			continue
		}
		if strings.IndexByte("nrtfvab", next) >= 0 {
			// Go knows these too, so they pass through.
			out.WriteByte('\\')
			out.WriteByte(next)
			continue
		}
		// Anything else -- `\/` most often -- is the character alone. Escaped so that a
		// character which happens to be a metacharacter in Go cannot change meaning.
		out.WriteString(regexp.QuoteMeta(string(next)))
	}
	return out.String()
}

// awkMatches reports whether a pattern matches, which is what `~` and a regex pattern ask.
func awkMatches(text, pattern string) (bool, error) {
	compiled, err := compileAwkRegex(pattern)
	if err != nil {
		return false, err
	}
	return compiled.MatchString(text), nil
}
