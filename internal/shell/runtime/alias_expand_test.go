package runtime_test

import (
	"path/filepath"
	"strings"
	"testing"
)

// An alias is read as shell text, with the rest of its command after it, as busybox-w32 and
// bash read it; every answer here was measured in both. The value was words put in the command
// name's place once the command had been expanded, so `$v` stayed as written and a value with
// an operator in it was refused.
func TestRuntime_readsAnAliasAsShellText(t *testing.T) {
	tests := []struct {
		name, script, stdout string
		status               int
	}{
		{name: "commands in sequence", script: "alias x='echo one; echo two'\nx\n", stdout: "one\ntwo\n"},
		{name: "a parameter expanded where it is used", script: "alias h='echo $v'\nv=set\nh\n", stdout: "set\n"},
		{name: "an and-or list", script: "alias s='echo a && echo b'\ns\n", stdout: "a\nb\n"},
		{name: "a pipeline", script: "alias t1='echo hi | tr h H'\nt1\n", stdout: "Hi\n"},
		{name: "the rest joins its last command", script: "alias e_='for i in 1 2 3; do echo $i;'\ne_ done\n", stdout: "1\n2\n3\n"},
		{name: "quoting in the rest kept", script: "alias q='echo \"a  b\" c'\nq \"d  e\"\n", stdout: "a  b c d  e\n"},
		{name: "a trailing newline leaves a command to begin", script: "alias e_='echo 1\n'\nv='echo 2'\ne_ $v\n", stdout: "1\n2\n"},
		{
			// A value that ends where a command begins makes the next word a command name,
			// looked up once the value is behind it, so it is x again; inside x's own value it
			// is only a command name.
			name:   "the next word after an operator",
			script: "alias x='echo a;'\nx x\nalias y='echo b; y'\ny 2>/dev/null\necho \"st=$?\"\n",
			stdout: "a\na\nb\nst=127\n",
		},
		{name: "a trailing blank, each time", script: "alias tb='echo tb '\ntb tb tb\n", stdout: "tb echo tb echo tb\n"},
		{name: "assignments stay in front", script: "f() { echo \"[$FOO]\"; }\nalias p=f\nFOO=2 p\n", stdout: "[2]\n"},
		{name: "an empty value leaves $?", script: "alias e=''\nfalse\ne\necho \"st=$?\"\n", stdout: "st=1\n"},
		{name: "a heredoc in the rest", script: "alias c=cat\nc <<EOF\nbody $v\nEOF\n", stdout: "body \n"},
		{name: "a cycle ends at a command", script: "alias a=b b=a\na 2>/dev/null\necho \"st=$?\"\n", stdout: "st=127\n"},
		{name: "a syntax error ends the script", script: "alias e_=';; oops'\ne_ x\necho after\n", status: 2},
		{name: "set -e acts inside it", script: "set -e\nalias fl='false; echo continued'\nfl\necho after\n", status: 1},
		{name: "its own pipelines set PIPESTATUS", script: "alias tp='true | false'\ntp\necho \"${PIPESTATUS[@]}\"\n", stdout: "0 1\n"},
		{name: "ERR once", script: "trap 'echo ERR' ERR\nalias f1=false\nf1\necho after\n", stdout: "ERR\nafter\n"},
		{name: "LINENO is the command's", script: "alias ln_='echo $LINENO'\n\nln_\n", stdout: "3\n"},
		{name: "DEBUG once, for the command it runs", script: "alias hi='echo hi'\ntrap 'echo \"D:$BASH_COMMAND\"' DEBUG\nhi\n", stdout: "D:echo hi\nhi\n"},
		{name: "in a command substitution", script: "alias hi='echo hi'\nx=$(hi sub)\necho \"$x\"\n", stdout: "hi sub\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, stdout, stderr := runSetScript(t, test.script)

			// Then
			if stdout != test.stdout || status != test.status {
				t.Fatalf("got %q/%d, want %q/%d; stderr = %q", stdout, status, test.stdout, test.status, stderr)
			}
		})
	}
}

// A redirection keeps its place: written after the command it goes to the value's last command,
// and before the name to its first, as busybox-w32 and bash have it.
func TestRuntime_aliasKeepsWhereARedirectionWasWritten(t *testing.T) {
	dir := filepath.ToSlash(t.TempDir())
	script := "cd '" + dir + "'\nalias q='echo q1; echo q2'\nq > after\n> before q\necho ---\ncat after before\n"

	// When
	status, stdout, stderr := runSetScript(t, script)

	// Then
	if want := "q1\nq2\n---\nq2\nq1\n"; stdout != want || status != 0 {
		t.Fatalf("got %q/%d, want %q/0; stderr = %q", stdout, status, want, stderr)
	}
}

// alias -p is bash's listing, as commands; busybox has no option, and reads -p as a name.
// `--` ends the options in both alias and unalias, and unalias takes -a as busybox's does.
func TestRuntime_aliasAndUnaliasOptions(t *testing.T) {
	tests := []struct {
		name, script, stdout, stderr string
		status                       int
	}{
		{name: "alias -p", script: "alias b='two words' a=1\nalias -p\n", stdout: "alias a='1'\nalias b='two words'\n"},
		{
			// bash 5.3 lists every alias and reads no operand after -p: c is not defined.
			name:   "alias -p reads no operand",
			script: "alias a=1 b=2\nalias -p a c=3\nalias c\n",
			stdout: "alias a='1'\nalias b='2'\n", stderr: "nemosh: line 3: alias: c not found\n", status: 1,
		},
		{name: "a name that reads as an option", script: "alias -- -p=dash\nalias -p\n", stdout: "alias -- -p='dash'\n"},
		{name: "another option is a name", script: "alias -x\n", stderr: "nemosh: line 1: alias: -x not found\n", status: 1},
		{name: "unalias --", script: "alias a=1 b=2\nunalias -- a\nalias\n", stdout: "b='2'\n"},
		{name: "unalias -a ends the command", script: "alias a=1 b=2\nunalias -a nosuch\nalias\n"},
		{name: "unalias -x", script: "unalias -x\n", stderr: "nemosh: line 1: unalias: illegal option -x\n", status: 2},
		{name: "unalias NAME not found", script: "unalias nosuch\n", stderr: "nemosh: line 1: unalias: nosuch not found\n", status: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, stdout, stderr := runSetScript(t, test.script)

			// Then
			if stdout != test.stdout || stderr != test.stderr || status != test.status {
				t.Fatalf("got %q/%q/%d, want %q/%q/%d", stdout, stderr, status, test.stdout, test.stderr, test.status)
			}
		})
	}
}

// The syntax error an alias makes is reported where it is used, and the line's rest is not run.
func TestRuntime_aliasSyntaxErrorIsReported(t *testing.T) {
	// When
	_, _, stderr := runSetScript(t, "alias e_='echo \"'\ne_ x\n")

	// Then
	if !strings.Contains(stderr, "unterminated quote") {
		t.Fatalf("stderr = %q, want the unterminated quote reported", stderr)
	}
}
