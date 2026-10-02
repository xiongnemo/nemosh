package runtime

import (
	"context"
	"io"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// The expansion operators take values a script computed, and patterns too: `${x#$p}` matches
// whatever p holds. The script here is fixed and the fuzzer chooses only the values, so
// nothing it writes is ever run as a command -- it reaches the operators, the pattern
// matcher, field splitting, `read`'s splitting and substring arithmetic as data, and each has
// to end with an answer or an error, never a panic or a hang.
const fuzzExpansionScript = `set -f
x=$1 p=$2
: "${x#$p}" "${x##$p}" "${x%$p}" "${x%%$p}" "${x/$p/r}" "${x//$p/r}" "${x/#$p/r}" "${x/%$p/r}"
: "${x^^$p}" "${x,,$p}" "${x^$p}" "${x@Q}" "${#x}" "${x:-$p}" "${x:+$p}"
: "${x:$3}" "${x:$3:$4}"
IFS=$p
set -- $x
read -r a b c <<< "$x"
read -r -a parts <<< "$x"
case $x in $p) : ;; esac
[[ $x == $p ]]
`

func FuzzExpansionOperators(f *testing.F) {
	for _, seed := range [][4]string{
		{"hello world", "l*", "1", "2"}, {"aaa", "a", "-1", "-1"}, {"a:b:c", ":", "0", "9"}, {"", "", "", ""},
		{"x", "@(x|y)", "x", "y"}, {"abc", "[!a]", "1+1", "a=1"}, {"a b", " ", "-9", "0"}, {"é😀", "?", "1", "1"},
		{"a\\b", "\\", "(1)", "2*3"}, {"aaaa", "+(a)", "2**3", "1"}, {"--", "*(*)", "1", "1"}, {"x", "[[:alpha:]]", "", "-0"},
	} {
		f.Add(seed[0], seed[1], seed[2], seed[3])
	}
	// Nothing here should touch a file; if something does, it touches a directory of its own.
	f.Chdir(f.TempDir())
	f.Fuzz(func(t *testing.T, x, p, offset, length string) {
		for _, value := range []string{x, p, offset, length} {
			// A NUL ends a C string in both references and is dropped here; it says nothing new.
			if !utf8.ValidString(value) || len(value) > 64 || strings.ContainsRune(value, 0) {
				t.Skip()
			}
		}
		r := New(applets.DefaultRegistry, Streams{Stdin: strings.NewReader(""), Stdout: io.Discard, Stderr: io.Discard})
		r.SetArguments("fuzz", []string{x, p, offset, length})
		r.RunScript(context.Background(), fuzzExpansionScript)
	})
}
