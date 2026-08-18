package runtime_test

import "testing"

// The two case-arm terminators bash adds to `;;`.
//
//	;;&  run the body, then go on *testing* the patterns below
//	;&   run the body, then run the next arm's body without testing it
//
// Both were syntax errors. `;;&` reported `case: invalid pattern "& a"` and `;&`
// reported `unexpected )`, because the `&` was read as a background operator and the
// arm never closed.
func TestCaseFallthrough_keepsTestingAfterDoubleSemicolonAmpersand(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{
			name:   "two arms with the same pattern",
			script: "case a in a) echo one ;;& a) echo two ;; esac\n", want: "one\ntwo\n",
		},
		{
			// The arm between them does not match, so it does not run: `;;&` resumes
			// *testing*, it does not run everything below.
			name:   "an arm between that does not match",
			script: "case a in a) echo one ;;& b) echo no ;; a) echo three ;; esac\n", want: "one\nthree\n",
		},
		{name: "then a star", script: "case a in a) echo one ;;& *) echo star ;; esac\n", want: "one\nstar\n"},
		{name: "last arm in the case", script: "case a in a) echo one ;;& esac\n", want: "one\n"},
		{
			name:   "three in a row",
			script: "case a in a) echo one ;;& a) echo two ;;& a) echo three ;; esac\n", want: "one\ntwo\nthree\n",
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

func TestCaseFallthrough_runsTheNextArmAfterSemicolonAmpersand(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{
			// The next arm runs *without* being tested, which is the whole difference
			// from `;;&`.
			name:   "the next arm runs untested",
			script: "case a in a) echo one ;& b) echo two ;; esac\n", want: "one\ntwo\n",
		},
		{name: "then a star", script: "case a in a) echo one ;& *) echo next ;; esac\n", want: "one\nnext\n"},
		{name: "last arm in the case", script: "case a in a) echo one ;& esac\n", want: "one\n"},
		{
			name:   "chained",
			script: "case a in a) echo one ;& x) echo two ;& y) echo three ;; esac\n", want: "one\ntwo\nthree\n",
		},
		{
			// A `;&` after an arm that did not match changes nothing: the arm has to
			// run for its terminator to matter.
			name:   "not reached when the arm does not match",
			script: "case z in a) echo one ;& b) echo two ;; *) echo star ;; esac\n", want: "star\n",
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

// Written across lines, which is how a real case is written.
func TestCaseFallthrough_worksAcrossLines(t *testing.T) {
	// When
	status, stdout, stderr := runSetScript(t, "case a in\na) echo one ;;&\na) echo two ;;\nesac\n")

	// Then
	if status != 0 {
		t.Fatalf("status = %d, stderr = %q", status, stderr)
	}
	if want := "one\ntwo\n"; stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

// The `&` in a terminator is not a background operator, and the background operator
// still is one. The two live one character apart.
func TestCaseFallthrough_leavesTheBackgroundOperatorAlone(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{name: "plain semicolon-semicolon", script: "case a in a) echo one ;; esac\n", want: "one\n"},
		{name: "background then wait", script: "echo bg &\nwait\n", want: "bg\n"},
		{name: "background in a case body", script: "case a in a) echo body & wait ;; esac\n", want: "body\n"},
		{name: "and-if", script: "true && echo yes\n", want: "yes\n"},
		{
			name:   "a case after a background command",
			script: "true &\nwait\ncase a in a) echo after ;; esac\n", want: "after\n",
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
