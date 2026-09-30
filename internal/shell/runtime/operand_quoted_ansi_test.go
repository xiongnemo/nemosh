package runtime_test

import "testing"

// Inside a double-quoted `${...}`, a pattern's and a replacement's `$'...'` is a quote as
// their single quotes are -- `"${x%$'b'*}"` is a, which git-prompt.sh relies on -- where a
// value takes it as its text. A value's backslash before the `}` that would end the
// expansion goes, and a replacement drops its backslashes as it would unquoted. Each kept
// what it should not; busybox-w32 and bash agree on every row.
// A reference in the double-quoted part of an operator's word is expanded as one in double
// quotes, whose own word takes a single quote as an ordinary character: here each pattern and
// replacement is two quote characters, and the ash_test case quote_in_varexp1 prints two of the
// four. Each was taken as an empty quoted string. busybox-w32 and bash agree on every one.
func TestRuntime_aReferenceInAQuotedOperandIsDoubleQuoted(t *testing.T) {
	for _, test := range []struct{ script, want string }{
		{`x=a; y="''b"; echo "${y#"${x+''}"}" ${y#"${x+''}"} "${x/a/"${x+''}"}"`, "b b ''\n"},
		{`x="''''"; echo "${x#"${x+''}"''}"`, "''\n"},
	} {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script + "\n"); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as busybox-w32 and bash answer", stdout, status, test.want)
			}
		})
	}
}

func TestRuntime_doubleQuotedOperandsQuoteAsBothReferencesDo(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{"x=abc; echo \"${x/$'b'/X}\" \"${x/b/$'Y'}\" \"${x#$'a'}\" \"${x%$'b'*}\"", "aXc aYc bc a\n"},
		{"x=abc; echo \"${x/b/\\}}\" \"${x#\\a}\" \"${x/b/\\a}\"", "a}c bc aac\n"},
		{"echo \"${var-}}\" \"${var-\\}}\" \"${var-\"}\"}\"", "} } }\n"},
		{"echo \"${var-\\a}\" \"${var-\\$}\" \"${u:-$'d\\'f'}\"", "\\a $ $'d\\'f'\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script + "\n"); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as busybox-w32 and bash answer", stdout, status, test.want)
			}
		})
	}
}
