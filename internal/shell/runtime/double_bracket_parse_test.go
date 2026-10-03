package runtime_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// `[[ ]]` is read with the script, as bash reads it: an expression that is no expression is a
// syntax error before anything runs, status 2, and an operator is one only as it is written --
// `$op` holding `==` is a word, so its line is no comparison. Each of these ran up to the
// command and was found out there, or ran on with the wrong answer. bash 5.3 refuses each.
func TestDoubleBracket_aMalformedExpressionStopsTheScriptBeforeItRuns(t *testing.T) {
	for _, script := range []string{
		"echo before\n[[ -z ]]\n",
		"echo before\n[[ ]]\n",
		"echo before\n[[ && ]]\n",
		"echo before\n[[ a b ]]\n",
		"echo before\n[[ '(' foo ]]\n",
		"echo before\n[[ ( a == a ]]\n",
		"echo before\n[[ -f < ]]\n",
		"echo before\n[[ a == b c ]]\n",
		"echo before\n[[ a 3< b ]]\n",
		"echo before\nop='=='\n[[ a $op a ]]\n",
		"echo before\n[[ a ]] b\n",
		"echo before\n[[ a == b\n",
	} {
		if status, stdout, stderr := runSetScript(t, script); status != 2 || stdout != "" || !strings.Contains(stderr, "syntax error") && !strings.Contains(stderr, "incomplete script") {
			t.Errorf("%q: status %d, stdout %q, stderr %q; want 2, nothing run, and a syntax error", script, status, stdout, stderr)
		}
	}
}

// `&&` and `||` take their right side only when the left has not decided, and an operand is
// expanded only when its test is made: the substitution in each of these never runs, as in bash.
// Every operand was expanded first, so it ran whatever the left side said.
func TestDoubleBracket_expandsAnOperandOnlyWhenItsTestIsMade(t *testing.T) {
	script := "f() { echo ran >&2; echo v; }\n" +
		"[[ -n x || $(f) == v ]] && echo or\n" +
		"[[ -z x && $(f) == v ]] || echo and\n" +
		"[[ -z x && ${unset_zz?no} ]] || echo unset\n"
	if status, stdout, stderr := runSetScript(t, script); status != 0 || stdout != "or\nand\nunset\n" || stderr != "" {
		t.Errorf("got %d/%q/%q, want 0, or and unset, and nothing run", status, stdout, stderr)
	}
}

// The condition's operators end the word before them, and a `]]` before the shell's own ends
// the condition: `||` there is the list's, and `>` a redirection. A newline may come between
// its words, and after `[[`; a reserved word may follow `]]` with no separator. Each was a
// syntax error, an unclosed `[[`, or a word of its own here. Each answer is bash 5.3's.
func TestDoubleBracket_readsItsOperatorsAndLinesAsBashDoes(t *testing.T) {
	dir := filepath.ToSlash(t.TempDir())
	for _, test := range []struct{ script, want string }{
		{"[[ ''||! (1 == 2)&&(2 == 2)]] && echo y", "y\n"},
		{"[[ b>a ]] && echo gt; [[ b<a ]] || echo lt", "gt\nlt\n"},
		{"[[ a ]]|| echo no; [[ a ]]&&echo yes", "yes\n"},
		{"[[ foo == foo\n&& bar == bar\n]] && echo lines", "lines\n"},
		{"[[\n  -n x ]] && echo opened", "opened\n"},
		{"if [[ -n x ]] then echo then; fi", "then\n"},
		{"while [[ -z y ]] do :; done; echo do", "do\n"},
		{"x='a  b'; [[ $x == 'a  b' ]] && echo blanks", "blanks\n"},
		{"re='a  b'; [[ 'a  b' =~ $re ]] && echo regex", "regex\n"},
		{"[[ x == \"]]\" ]] || echo quoted", "quoted\n"},
		{"echo [[ x", "[[ x\n"},
		{"f() [[ -n $1 ]]; f a && echo function", "function\n"},
		{"[[ -a /dev/null ]] && echo exists", "exists\n"},
	} {
		if status, stdout, stderr := runSetScript(t, test.script+"\n"); status != 0 || stdout != test.want || stderr != "" {
			t.Errorf("%q: got %d/%q/%q, want 0 and %q", test.script, status, stdout, stderr, test.want)
		}
	}
	script := "cd '" + dir + "'\nx=1\n[[ $x ]]>out.txt && echo made\n"
	if status, stdout, _ := runSetScript(t, script); status != 0 || stdout != "made\n" {
		t.Errorf("a redirection after ]]: got %d/%q", status, stdout)
	}
	if _, err := os.Stat(filepath.Join(dir, "out.txt")); err != nil {
		t.Errorf("a redirection after ]] made no file: %v", err)
	}
}
