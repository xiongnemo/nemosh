package applets

import (
	"testing"
	"unicode/utf8"
)

// An awk program is text a script hands over, often built from variables, so one that panics
// the parser ends the applet with a Go trace instead of awk's syntax error. Parsing only:
// running a program could write files and start commands.
func FuzzParseAwkProgram(f *testing.F) {
	for _, seed := range []string{
		"{ print $1 }", "BEGIN { x = 1; print x+1 }", "/re/ { n++ } END { print n }",
		"{ a[$1] += $2 } END { for (k in a) print k, a[k] }", "function f(x) { return x*2 } { print f($1) }",
		"$1 ~ /^a/ && $2 !~ /b$/", "{ printf \"%s %d\\n\", $1, $2 }", "BEGIN { while (i < 3) i++; print i }",
		"{ getline line < \"f\"; print line }", "{ print > \"out\" }", "{ x = (1, 2) in a }", "BEGIN { print -x^2 }",
		"{", "}", "BEGIN {", "/unterminated", "\"unterminated", "{ print $ }", "{ a[ }", "function () {}",
		"BEGIN { print substr(\"abc\", 2) }", "{ $0 = toupper($0); print }", "BEGIN { split(\"a b\", x); print x[1] }",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, source string) {
		if !utf8.ValidString(source) || len(source) > 1024 {
			t.Skip()
		}
		_, _ = parseAwkProgram(source)
	})
}
