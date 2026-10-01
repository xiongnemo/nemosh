package runtime

import (
	"strings"
	"testing"
)

// A compound list with no command in it is no part of POSIX's grammar, and busybox and bash
// both refuse each of these as a syntax error. nemosh ran them, successfully, and `while; do
// :; done` ran forever. A case arm may be empty, as both take it, and so may $( ).
func TestParse_refusesACompoundListWithNoCommand(t *testing.T) {
	for script, want := range map[string]string{
		"if; then :; fi\n":                     "if with no condition",
		"if true; then fi\n":                   "then with no command",
		"if true; then :; else fi\n":           "else with no command",
		"if true; then :; elif; then :; fi\n":  "if with no condition",
		"while; do :; done\n":                  "while with no condition",
		"until; do :; done\n":                  "until with no condition",
		"while false; do done\n":               "do with no command",
		"for x in a; do done\n":                "do with no command",
		"( )\n":                                "( with no command",
		"while # a comment\ndo :; done\n":      "while with no condition",
		"if true\nthen\n# nothing\nfi\necho\n": "then with no command",
	} {
		stdout, stderr, status := runKill(t, script)
		if stdout != "" || status != 2 || !strings.Contains(stderr, "syntax error: "+want) {
			t.Errorf("%q: stdout %q, stderr %q, status %d; want a syntax error, %q", script, stdout, stderr, status, want)
		}
	}
	for script, want := range map[string]string{
		"case x in esac; echo st=$?\n":       "st=0\n",
		"case x in a) ;; esac; echo st=$?\n": "st=0\n",
		"x=$( ); echo \"[$x]\"\n":            "[]\n",
		"( : ); echo st=$?\n":                "st=0\n",
	} {
		if stdout, stderr, _ := runKill(t, script); stdout != want || stderr != "" {
			t.Errorf("%q: stdout %q, stderr %q; want %q", script, stdout, stderr, want)
		}
	}
}
