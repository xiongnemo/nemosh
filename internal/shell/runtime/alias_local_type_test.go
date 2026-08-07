package runtime_test

import (
	"strings"
	"testing"
)

func TestRuntime_substitutesAnAlias(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{name: "simple", script: "alias e='echo hi'\ne\n", want: "hi\n"},
		{name: "arguments follow", script: "alias e='echo one'\ne two\n", want: "one two\n"},
		{name: "chained", script: "alias a=b\nalias b='echo chained'\na\n", want: "chained\n"},
		{
			name:   "a name is not substituted into its own value",
			script: "alias echo='echo prefix'\necho tail\n",
			want:   "prefix tail\n",
		},
		{
			// The `alias sudo='sudo '` idiom: the trailing blank is what lets
			// the word after it be an alias too.
			name:   "a trailing blank makes the next word eligible",
			script: "alias e='echo '\nalias hi=there\ne hi\n",
			want:   "there\n",
		},
		{
			name:   "without the trailing blank the argument is left alone",
			script: "alias e=echo\nalias hi=there\ne hi\n",
			want:   "hi\n",
		},
		{name: "quoting in the value survives", script: "alias e='echo \"two words\"'\ne\n", want: "two words\n"},
		{name: "not substituted when quoted", script: "alias e='echo hi'\n'e'\n", want: ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			_, stdout, _ := runSetScript(t, test.script)

			// Then
			if test.want == "" {
				if stdout != "" {
					t.Fatalf("stdout = %q, want nothing", stdout)
				}
				return
			}
			if stdout != test.want {
				t.Fatalf("stdout = %q, want %q", stdout, test.want)
			}
		})
	}
}

func TestRuntime_listsAndRemovesAliases(t *testing.T) {
	// When
	status, stdout, _ := runSetScript(t, "alias b='echo two'\nalias a='echo one'\nalias\nunalias a\nalias\n")

	// Then
	if status != 0 {
		t.Fatalf("status = %d, want 0", status)
	}
	if want := "alias a='echo one'\nalias b='echo two'\nalias b='echo two'\n"; stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestRuntime_refusesAnAliasValueThatIsNotWords(t *testing.T) {
	// Substitution happens after parsing, so an alias contributes words to a
	// command that already exists. It cannot introduce a pipeline, and saying
	// so beats accepting it and running something else.
	// When
	status, _, stderr := runSetScript(t, "alias c='a | b'\n")

	// Then
	if status != 1 || !strings.Contains(stderr, "list of words") {
		t.Fatalf("status = %d, stderr = %q, want 1 and a words-only diagnostic", status, stderr)
	}
}

func TestRuntime_reportsAnUnknownAlias(t *testing.T) {
	// When
	status, _, stderr := runSetScript(t, "alias nosuch\n")

	// Then
	if status != 1 || !strings.Contains(stderr, "nosuch") {
		t.Fatalf("status = %d, stderr = %q, want 1 and the name", status, stderr)
	}
}

func TestRuntime_keepsALocalInsideItsFunction(t *testing.T) {
	// When
	status, stdout, _ := runSetScript(t, "x=outer\nf() { local x=inner; echo [$x]; }\nf\necho [$x]\n")

	// Then
	if status != 0 || stdout != "[inner]\n[outer]\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "[inner]\n[outer]\n")
	}
}

func TestRuntime_leavesANameUnsetAfterAValuelessLocal(t *testing.T) {
	// When
	status, stdout, _ := runSetScript(t, "f() { local y; echo [$y]; y=set; }\nf\necho [$y]\n")

	// Then
	if status != 0 || stdout != "[]\n[]\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "[]\n[]\n")
	}
}

func TestRuntime_restoresALocal_whenTheFunctionReturnsEarly(t *testing.T) {
	// When
	status, stdout, _ := runSetScript(t, "x=outer\nf() { local x=inner; return 0; }\nf\necho [$x]\n")

	// Then
	if status != 0 || stdout != "[outer]\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "[outer]\n")
	}
}

func TestRuntime_unwindsNestedLocalsInOrder(t *testing.T) {
	// When
	status, stdout, _ := runSetScript(t,
		"x=top\ninner() { local x=deep; echo [$x]; }\nouter() { local x=middle; inner; echo [$x]; }\nouter\necho [$x]\n")

	// Then
	if status != 0 || stdout != "[deep]\n[middle]\n[top]\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "[deep]\n[middle]\n[top]\n")
	}
}

func TestRuntime_leavesAnAssignmentGlobal_whenLocalIsNotUsed(t *testing.T) {
	// POSIX 2.9.5 makes every variable a function touches global; `local` is the
	// extension that opts out, so without it nothing changes.
	// When
	status, stdout, _ := runSetScript(t, "f() { z=made; }\nf\necho [$z]\n")

	// Then
	if status != 0 || stdout != "[made]\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "[made]\n")
	}
}

func TestRuntime_reportsLocalOutsideAFunction(t *testing.T) {
	// When
	status, _, stderr := runSetScript(t, "local x=1\n")

	// Then
	if status != 1 || !strings.Contains(stderr, "not in a function") {
		t.Fatalf("status = %d, stderr = %q, want 1 and a not-in-a-function diagnostic", status, stderr)
	}
}

func TestRuntime_describesHowANameResolves(t *testing.T) {
	tests := []struct {
		name     string
		script   string
		fragment string
	}{
		{name: "builtin", script: "type cd\n", fragment: "cd is a shell builtin"},
		{name: "applet", script: "type grep\n", fragment: "grep is a bundled applet"},
		{name: "function", script: "f() { :; }\ntype f\n", fragment: "f is a function"},
		{name: "alias", script: "alias e='echo hi'\ntype e\n", fragment: "e is an alias for echo hi"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, stdout, stderr := runSetScript(t, test.script)

			// Then
			if status != 0 {
				t.Fatalf("status = %d, stderr = %q, want 0", status, stderr)
			}
			if !strings.Contains(stdout, test.fragment) {
				t.Fatalf("stdout = %q, want it to contain %q", stdout, test.fragment)
			}
		})
	}
}

func TestRuntime_reportsAnUnresolvableNameToType(t *testing.T) {
	// When
	status, _, stderr := runSetScript(t, "type definitely-not-a-command\n")

	// Then
	if status != 1 || !strings.Contains(stderr, "not found") {
		t.Fatalf("status = %d, stderr = %q, want 1 and a not-found diagnostic", status, stderr)
	}
}
