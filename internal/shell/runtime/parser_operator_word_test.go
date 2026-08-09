package runtime

import (
	"strings"
	"testing"
)

// An operator where a word belongs must be refused, not dereferenced.
//
// Only a word token carries a parsed form; `|`, `>` and `2>` leave it nil. Three
// places took it without looking and panicked the entire shell -- on
// `for i in a|b; do :; done`, which is not exotic input, just wrong input. A
// shell that crashes on a typo is worse than one that rejects it, because the
// session and everything in it goes with the crash.
//
// Found by the parser fuzzer on a 60-second exploration run, which is what that
// step is in CI for. The seeds are in FuzzParseScript so the corpus carries them
// from here on.
func TestParse_refusesAnOperatorWhereAWordBelongs(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
		want   string
	}{
		{name: "a pipe in a for list", source: "for i in a|b; do :; done", want: "syntax error: unexpected |"},
		{name: "a redirect in a for list", source: "for i in 2>a; do :; done", want: "syntax error: unexpected 2>"},
		{name: "an ampersand in a for list", source: "for i in a&b; do :; done", want: "syntax error: unexpected &"},
		{name: "a pipe as the case selector", source: "case | in x) :;; esac", want: "syntax error: unexpected |"},
		{name: "a redirect in a case pattern", source: "case x in a|>) :;; esac", want: "syntax error: unexpected >"},
	} {
		t.Run(test.name, func(t *testing.T) {
			// When
			_, err := ParseScript(test.source)

			// Then
			if err == nil {
				t.Fatalf("ParseScript(%q) succeeded, want a syntax error", test.source)
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("ParseScript(%q) = %v, want it to contain %q", test.source, err, test.want)
			}
		})
	}
}

// The guard must not start refusing input that was always valid. A word that
// merely contains an operator character, quoted or escaped, is still a word.
func TestParse_stillAcceptsOperatorCharactersInsideWords(t *testing.T) {
	for _, source := range []string{
		"for i in 'a|b'; do :; done",
		`for i in a\|b; do :; done`,
		`for i in "a>b"; do :; done`,
		"case 'a|b' in 'a|b') :;; esac",
		"for i in a b c; do :; done",
	} {
		t.Run(source, func(t *testing.T) {
			if _, err := ParseScript(source); err != nil {
				t.Fatalf("ParseScript(%q) = %v, want it accepted", source, err)
			}
		})
	}
}
