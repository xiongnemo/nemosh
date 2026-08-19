package runtime_test

import "testing"

// BASH_REMATCH: what `[[ string =~ regex ]]` matched.
//
// The match ran and answered correctly; the captures were thrown away, so the standard way
// to pull fields out of a string in bash --
//
//	[[ $line =~ ^([0-9]+):(.*)$ ]] && echo "${BASH_REMATCH[1]}"
//
// -- had an empty second half. A condition that answers correctly and loses the reason is
// worse than one that fails, because the script carries on.
func TestBashRematch_recordsWhatMatched(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{
			// Element 0 is the whole match, which is bash's layout.
			name: "the whole match", script: "[[ abc =~ b ]] && echo \"${BASH_REMATCH[0]}\"\n", want: "b\n",
		},
		{
			name:   "one group",
			script: "[[ abc =~ \"(b)\" ]] && echo \"${BASH_REMATCH[0]}-${BASH_REMATCH[1]}\"\n", want: "b-b\n",
		},
		{
			name:   "two groups",
			script: "[[ abc =~ \"(b)(c)\" ]] && echo \"${BASH_REMATCH[1]}${BASH_REMATCH[2]}\"\n", want: "bc\n",
		},
		{
			// The shape a script actually uses.
			name:   "pulling fields out of a line",
			script: "[[ 12:xy =~ \"^([0-9]+):(.*)$\" ]] && echo \"${BASH_REMATCH[1]}|${BASH_REMATCH[2]}\"\n",
			want:   "12|xy\n",
		},
		{name: "the count", script: "[[ abc =~ \"(b)\" ]]\necho \"${#BASH_REMATCH[@]}\"\n", want: "2\n"},
		{
			// A group that did not participate is empty rather than absent, because the
			// array is indexed by group number and a gap would shift every later one.
			name:   "a group that did not participate",
			script: "[[ ab =~ \"(a)(z)?\" ]] && echo \"[${BASH_REMATCH[2]}]\"\n", want: "[]\n",
		},
		{name: "a longer match wins", script: "[[ aaa =~ \"a*\" ]] && echo \"${BASH_REMATCH[0]}\"\n", want: "aaa\n"},
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

// No match leaves the array as it was, which is bash's behaviour: a script that tests one
// pattern and then reads BASH_REMATCH after a different, failed test sees the last
// successful match. Clearing it would be tidier and would not be bash.
func TestBashRematch_isLeftAloneWhenNothingMatches(t *testing.T) {
	// When
	status, stdout, stderr := runSetScript(t,
		"[[ abc =~ b ]]\n[[ xyz =~ q ]]\necho \"[${BASH_REMATCH[0]}]\"\n")

	// Then
	if status != 0 {
		t.Fatalf("status = %d, stderr = %q", status, stderr)
	}
	if stdout != "[b]\n" {
		t.Fatalf("stdout = %q, want the previous successful match", stdout)
	}
}

// The condition still answers correctly, which is what it did before the captures were
// kept and must go on doing.
func TestBashRematch_leavesTheConditionAlone(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{name: "a match", script: "[[ abc =~ ^a.c$ ]] && echo yes\n", want: "yes\n"},
		{name: "no match", script: "[[ abc =~ ^z ]] || echo no\n", want: "no\n"},
		{name: "a nested condition", script: "[[ ( a == a ) ]] && echo nested\n", want: "nested\n"},
		{name: "an and", script: "[[ 1 == 1 && 2 == 2 ]] && echo both\n", want: "both\n"},
		{name: "a pattern match", script: "[[ abc == a* ]] && echo glob\n", want: "glob\n"},
		{name: "a quoted pattern is literal", script: "[[ abc == \"a*\" ]] || echo literal\n", want: "literal\n"},
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
