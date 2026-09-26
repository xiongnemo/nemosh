package runtime_test

import "testing"

// The right side of `==` and `!=` inside [[ ]] is a pattern where it is unquoted and a
// literal where it is quoted, part by part: `[[ abc == "$p"* ]]` asks whether abc begins
// with what p holds. Here a word with any quoted part was compared literally as a whole,
// so that was false, and the idiom for "starts with" never matched. The answers are bash
// 5.3's: busybox-w32's [[ is test with pattern matching, and it neither keeps quoted parts
// literal nor keeps unquoted expansions from matching file names.
func TestRuntime_doubleBracketMatchesAPartlyQuotedPattern(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{`p=ab; [[ abc == "$p"* ]] && echo yes || echo no`, "yes\n"},
		{`p=ab; [[ abc != "$p"* ]] && echo yes || echo no`, "no\n"},
		{`[[ 'a*c' == "a*"* ]] && echo yes || echo no`, "yes\n"},
		{`[[ abc == "a*"* ]] && echo yes || echo no`, "no\n"},
		{`[[ abc == a"b"? ]] && echo yes || echo no`, "yes\n"},
		{`[[ abc == a'?'c ]] && echo yes || echo no`, "no\n"},
		{`[[ abc == a\?c ]] && echo yes || echo no`, "no\n"},
		{`p='*'; [[ x == $p ]] && echo yes || echo no`, "yes\n"},
		{`p='*'; [[ x == "$p" ]] && echo yes || echo no`, "no\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as bash answers", stdout, status, test.want)
			}
		})
	}
}

// A quoted extended pattern is literal wherever a pattern is: `'@(cc)'` is those five
// characters, not a pattern matching cc. Quoting made `*`, `?` and `[` literal and left
// `@(` alone, and extglob is on here from the start, so a case arm, a ${x#...} and a [[ ]]
// all matched the operator. busybox-w32 has no extglob; the answers are bash 5.3's.
func TestRuntime_quotedExtendedPatternIsLiteral(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{`[[ cc == '@(cc)' ]] && echo yes || echo no`, "no\n"},
		{`[[ cc == @(cc) ]] && echo yes || echo no`, "yes\n"},
		{`case cc in '@(cc)') echo yes;; *) echo no;; esac`, "no\n"},
		{`case '@(cc)' in '@(cc)') echo yes;; *) echo no;; esac`, "yes\n"},
		{`x=cc; echo "${x#'@(c)'}"`, "cc\n"},
		{`x=cc; echo "${x#@(c)}"`, "c\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing("shopt -s extglob\n" + test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as bash answers", stdout, status, test.want)
			}
		})
	}
}
