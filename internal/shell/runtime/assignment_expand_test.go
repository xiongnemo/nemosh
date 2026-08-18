package runtime_test

import (
	"strings"
	"testing"
)

// The worst defect found in this pass: `d=$(date)` ran `Aug`.
//
// The expansion of `$(date)` was field split, turning one word into six, and
// assignments were recognised *after* expansion -- so the code that looks for
// `name=value` saw `d=Tue` and took the remaining five words for a command and its
// arguments. `x=$(cmd)` is one of the most common lines in any shell script.
//
// POSIX 2.6.5 exempts an assignment's value from field splitting, and all three
// reference shells agree: bash, dash and busybox ash all print `[a b]` for the
// first case below, where this shell printed `b: not found`.
func TestAssignment_doesNotFieldSplitItsValue(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{
			name: "a command substitution with a space", script: "q=$(echo a b)\necho \"[$q]\"\n",
			want: "[a b]\n",
		},
		{
			// Proof that this is genuinely not splitting rather than splitting and
			// rejoining with a space: a rejoin would collapse the run.
			name: "inner runs of blanks survive", script: "q=$(printf 'a   b')\necho \"[$q]\"\n",
			want: "[a   b]\n",
		},
		{
			name: "a tab in the output", script: "q=$(printf 'a\\tb')\necho \"[$q]\"\n",
			want: "[a\tb]\n",
		},
		{
			name: "a variable holding blanks", script: "v='a b'\nq=$v\necho \"[$q]\"\n",
			want: "[a b]\n",
		},
		{
			name: "several assignments in a row", script: "a=$(echo 1 2) b=$(echo 3 4)\necho \"[$a][$b]\"\n",
			want: "[1 2][3 4]\n",
		},
		{
			// An assignment prefix on a real command: the command still gets its
			// own words split normally.
			name:   "an assignment prefix before a command",
			script: "v='x y' \nq=$v echo one two\n", want: "one two\n",
		},
		{
			name: "an empty substitution", script: "q=$(true)\necho \"[$q]\"\n", want: "[]\n",
		},
		{
			name:   "a substitution with a trailing newline stripped",
			script: "q=$(printf 'a\\nb\\n')\necho \"[$q]\"\n", want: "[a\nb]\n",
		},
		{
			// The array element form is an assignment too and must not split.
			name: "an element assignment", script: "a=(x)\na[0]=$(echo p q)\necho \"[${a[0]}]\"\n",
			want: "[p q]\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, stdout, stderr := runSetScript(t, test.script)

			// Then
			if status != 0 {
				t.Fatalf("status = %d, stderr = %q", status, stderr)
			}
			if strings.Contains(stderr, "not found") {
				t.Fatalf("something was run that should not have been: %q", stderr)
			}
			if stdout != test.want {
				t.Fatalf("%q printed %q, want %q", test.script, stdout, test.want)
			}
		})
	}
}

// The exemption is for assignments only. Everywhere else an unquoted expansion still
// splits, and taking that away would be a worse bug than the one being fixed.
func TestAssignment_leavesFieldSplittingAloneEverywhereElse(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{name: "set --", script: "set -- $(echo a b c)\necho $#\n", want: "3\n"},
		{
			name: "a for list", script: "for w in $(echo a b c); do printf '<%s>' \"$w\"; done\necho\n",
			want: "<a><b><c>\n",
		},
		{
			name: "command arguments", script: "printf '[%s]' $(echo a b)\necho\n", want: "[a][b]\n",
		},
		{
			name: "a variable used unquoted", script: "v='a b'\nprintf '[%s]' $v\necho\n", want: "[a][b]\n",
		},
		{
			// Not an assignment: the name is not a name, so this is a command.
			name: "a word that only looks like one", script: "printf '%s\\n' --flag=$(echo a b)\n",
			want: "--flag=a\nb\n",
		},
		{
			// A quoted name is a command name, not an assignment, in every shell.
			name: "an assignment after a command is an argument", script: "echo q=$(echo a b)\n",
			want: "q=a b\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, stdout, stderr := runSetScript(t, test.script)

			// Then
			if status != 0 {
				t.Fatalf("status = %d, stderr = %q", status, stderr)
			}
			if stdout != test.want {
				t.Fatalf("%q printed %q, want %q", test.script, stdout, test.want)
			}
		})
	}
}

// A tilde after the `=` expands, which POSIX requires and which the lexer could not
// see: it marks a word for tilde expansion only when the word *starts* with one, and
// in an assignment it starts after the name.
func TestAssignment_expandsATildeAfterTheEquals(t *testing.T) {
	tests := []struct {
		name   string
		script string
	}{
		{name: "a tilde alone", script: "q=~\necho \"$q\"\n"},
		{name: "a tilde with a path", script: "q=~/bin\necho \"$q\"\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When -- compared against what the shell says HOME is, rather than
			// against a spelling, because the two must agree whatever HOME holds.
			_, home, _ := runSetScript(t, "echo \"$HOME\"\n")
			status, stdout, stderr := runSetScript(t, test.script)

			// Then
			if status != 0 {
				t.Fatalf("status = %d, stderr = %q", status, stderr)
			}
			if strings.TrimSpace(home) == "" {
				t.Skip("HOME is not set in this environment, so there is nothing to expand to")
			}
			if !strings.HasPrefix(stdout, strings.TrimSpace(home)) {
				t.Fatalf("%q printed %q, want it to begin with HOME (%q)",
					test.script, stdout, strings.TrimSpace(home))
			}
			if strings.Contains(stdout, "~") {
				t.Fatalf("%q printed %q, which still holds a tilde", test.script, stdout)
			}
		})
	}
}

// A quoted tilde is a tilde. Expanding it would make it impossible to assign the
// character.
func TestAssignment_leavesAQuotedTildeAlone(t *testing.T) {
	// When
	status, stdout, stderr := runSetScript(t, "q='~'\necho \"[$q]\"\n")

	// Then
	if status != 0 {
		t.Fatalf("status = %d, stderr = %q", status, stderr)
	}
	if stdout != "[~]\n" {
		t.Fatalf("stdout = %q, want the tilde left alone", stdout)
	}
}
