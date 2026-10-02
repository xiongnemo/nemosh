package runtime

import (
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

// Brace expansion takes any word a script writes, so a word that panics it ends the shell.
// Its output is allowed to be large -- `{1..1000}` is a thousand words, in bash too -- so the
// target keeps to words whose ranges are short and asks only that expansion end, and that a
// word with no brace in it come back as itself.
func FuzzExpandBraceWord(f *testing.F) {
	for _, seed := range []string{
		"{a,b}", "x{a,b}y", "{1..5}", "{a..e..2}", "{5..1}", "{-3..3}", "{a,{b,c}}", "{,}",
		"{a}", "{}", "{", "}", "{a,b", "a,b}", "{1..}", "{..1}", "{z..A}", "{01..10}", "{1..2..0}",
		"{a,b}{c,d}", "\\{a,b}", "{a\\,b,c}", "{$x,y}", "{a..c}{1..3}", "{😀,b}", "{a..😀}",
	} {
		f.Add(seed)
	}
	longRange := regexp.MustCompile(`[0-9]{4}|\.\.[^.}]*\.\.`)
	f.Fuzz(func(t *testing.T, text string) {
		if !utf8.ValidString(text) || len(text) > 64 || strings.Count(text, "{") > 6 || longRange.MatchString(text) {
			t.Skip()
		}
		tokens, err := scanShellTokens(text)
		if err != nil {
			return
		}
		for _, token := range tokens {
			if token.parsed == nil {
				continue
			}
			words := expandBraceWord(*token.parsed)
			if !strings.ContainsAny(token.value, "{}") && len(words) != 1 {
				t.Fatalf("%q has no brace and expanded to %d words", token.value, len(words))
			}
		}
	})
}
