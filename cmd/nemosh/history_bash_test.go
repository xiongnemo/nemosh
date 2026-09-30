package main

import (
	"strings"
	"testing"
)

// bash's history expansion, word designators and modifiers, each answer measured in bash
// 5.3 on the same history: one line with a path, two quoted words, a pipe and a command.
func TestHistoryExpander_designatorsAndModifiersAsBash(t *testing.T) {
	history := []string{`: /usr/local/lib/libfoo.tar.gz "a b" 'c d'|cat`}
	for _, test := range []struct{ line, want string }{
		{`echo !1:0`, `echo :`},
		{`echo !1:1:h`, `echo /usr/local/lib`},
		{`echo !1:1:t`, `echo libfoo.tar.gz`},
		{`echo !1:1:r`, `echo /usr/local/lib/libfoo.tar`},
		{`echo !1:1:e`, `echo .gz`},
		{`echo !1:1:t:r:r`, `echo libfoo`},
		{`echo !1:1:h:h:h:h:h:h`, `echo `},
		// The words are the shell's: quotes kept, the pipe a word of its own.
		{`echo !1:2`, `echo "a b"`},
		{`!1:4`, `|`},
		{`echo !1:$`, `echo cat`},
		{`!1:*`, `/usr/local/lib/libfoo.tar.gz "a b" 'c d' | cat`},
		{`!1:2-3`, `"a b" 'c d'`},
		{`!1:2*`, `"a b" 'c d' | cat`},
		{`!1:2-`, `"a b" 'c d' |`},
		{`!1:-2`, `: /usr/local/lib/libfoo.tar.gz "a b"`},
		{`echo !1^ !1$`, `echo /usr/local/lib/libfoo.tar.gz cat`},
		{`echo !1:1-x`, `echo /usr/local/lib/libfoo.tar.gz "a b" 'c d' |x`},
		{`echo x!1:0y`, `echo x:y`},
		{`echo "!1:1"`, `echo "/usr/local/lib/libfoo.tar.gz"`},
		{`!1:s/lib/LIB/`, `: /usr/local/LIB/libfoo.tar.gz "a b" 'c d'|cat`},
		{`!1:gs/lib/LIB/`, `: /usr/local/LIB/LIBfoo.tar.gz "a b" 'c d'|cat`},
		{`!1:s/lib/[&]/`, `: /usr/local/[lib]/libfoo.tar.gz "a b" 'c d'|cat`},
		{`!1:s|local|LOCAL|`, `: /usr/LOCAL/lib/libfoo.tar.gz "a b" 'c d'|cat`},
		{`!1:s/o/\//`, `: /usr/l/cal/lib/libfoo.tar.gz "a b" 'c d'|cat`},
		// The last delimiter may go only at the end: here the :p is the replacement's.
		{`!1:gs/l/L:p`, `: /usr/L:pocaL:p/L:pib/L:pibfoo.tar.gz "a b" 'c d'|cat`},
		{`echo !1:1:q`, `echo '/usr/local/lib/libfoo.tar.gz'`},
		{`echo !1:3:q`, `echo ''\''c d'\'''`},
		{`echo !1:3:x`, `echo ''\''c' 'd'\'''`},
		{`echo !#`, `echo echo `},
		{`echo !?libfoo?:%`, `echo /usr/local/lib/libfoo.tar.gz`},
	} {
		var expander historyExpander
		got, changed, _, err := expander.expand(test.line, history)
		if err != nil || !changed || got != test.want {
			t.Errorf("%s\n  got  %q (changed %v, err %v)\n  want %q", test.line, got, changed, err, test.want)
		}
	}
}

// What bash refuses, in bash's words: the text of the expansion that failed, then why.
func TestHistoryExpander_refusesAsBash(t *testing.T) {
	history := []string{`: /usr/local/lib/libfoo.tar.gz "a b" 'c d'|cat`}
	for _, test := range []struct{ line, says string }{
		{`!1:s/nomatch/x/`, `:s/nomatch/x/: substitution failed`},
		{`echo !1:9`, `:9: bad word specifier`},
		{`echo !1:3-1`, `:3-1: bad word specifier`},
		{`echo !1:1:H`, `H: unrecognized history modifier`},
		{`!1:G s/l/L/`, ` : unrecognized history modifier`},
		{`!1:&`, `:&: no previous substitution`},
		{`echo !x!y`, `!x!y: event not found`},
		{`echo "a!b"`, `!b: event not found`},
		{`echo !?:0`, `!?:0: event not found`},
	} {
		var expander historyExpander
		_, _, _, err := expander.expand(test.line, history)
		if err == nil || err.Error() != test.says {
			t.Errorf("%s: err %v, want %q", test.line, err, test.says)
		}
	}
}

// A `!` the shell itself uses, or one quoting hides, is no expansion, and a `#` that begins
// a word ends expansion for the rest of the line.
func TestHistoryExpander_leavesWhatBashLeaves(t *testing.T) {
	history := []string{"echo hi"}
	for _, line := range []string{
		`echo [!a]`, `echo ${!x}`, `echo ${!}`, `echo $!`, `echo 'q!!q'`,
		`echo "$(echo '!!')"`, "echo `echo '!!'`", `echo hi # !!`, `echo hi #!!`,
		`echo !(foo)`, `echo "\!!"`, `echo \!!`,
	} {
		var expander historyExpander
		got, changed, _, err := expander.expand(line, history)
		if err != nil || changed || got != line {
			t.Errorf("%s became %q (changed %v, err %v)", line, got, changed, err)
		}
	}
	var expander historyExpander
	if got, _, _, _ := expander.expand(`echo hi#!!`, history); got != `echo hi#echo hi` {
		t.Errorf("a # inside a word is no comment: got %q", got)
	}
}

// What an expander keeps from one line to the next, as bash does: the substitution, which
// :& repeats and an empty old takes, and the search, whose matched word % is.
func TestHistoryExpander_remembersAsBash(t *testing.T) {
	history := []string{"echo first line", "echo second"}
	var expander historyExpander
	steps := []struct{ line, want string }{
		{`!1:s/first/1st/`, `echo 1st line`},
		{`!1:&`, `echo 1st line`},
		{`!1:s//one/`, `echo one line`},
		{`^^eins`, ``},
		{`echo !?cond?`, `echo echo second`},
		{`echo !%`, `echo second`},
	}
	for _, step := range steps {
		got, _, _, err := expander.expand(step.line, history)
		if step.want == "" {
			// ^^eins takes first as old, and the previous line has none.
			if err == nil || !strings.Contains(err.Error(), "substitution failed") {
				t.Errorf("%s: got %q, err %v; want the substitution to fail", step.line, got, err)
			}
			continue
		}
		if err != nil || got != step.want {
			t.Errorf("%s: got %q, err %v; want %q", step.line, got, err, step.want)
		}
	}
}

// :p prints the line and records it, and it does not run; the expansion after it finds it.
func TestSession_printOnlyRecordsWithoutRunning(t *testing.T) {
	t.Setenv("PS1", "")
	t.Setenv("PS2", "")

	result := runInvocation(t, "echo a b\n!!:s/a/x/:p\n!!\n", "--norc", "-i")

	if result.stdout != "a b\nx b\n" || strings.Count(result.stderr, "echo x b\n") != 2 {
		t.Fatalf("stdout %q, stderr %q", result.stdout, result.stderr)
	}
}
