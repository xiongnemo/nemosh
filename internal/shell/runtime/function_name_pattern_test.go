package runtime_test

import "testing"

// A function's name may end in ?, *, +, @ or !, and hold a # after its first character, as
// busybox-w32 and bash (extglob off, its default) both read them: `f+() { ...; }` defines f+.
// Each was refused, since `+(` begins an extended pattern here, and the script never began.
// Empty parentheses make the definition; an extended pattern is still one.
func TestRuntime_functionNameMayEndInAPatternCharacter(t *testing.T) {
	for index, test := range []struct{ script, want string }{
		{"f+() { echo plus; }; f+", "plus\n"},
		{"f@ () { echo at; }; f@", "at\n"},
		{"f!() { echo bang; }; f!", "bang\n"},
		{"function g+ { echo kw; }; g+", "kw\n"},
		{"f#x() { echo hash; }; f#x", "hash\n"},
		{"f+() ( echo sub ); f+", "sub\n"},
		{"f+()\n{\n  echo next\n}\nf+", "next\n"},
		{"f+() { echo p; }; declare -F f+", "f+\n"},
		{"x=aaab; echo ${x##+(a)}", "b\n"},
		{"case aab in +(a)b) echo ext;; esac", "ext\n"},
		{"!(false) && echo negated", "negated\n"},
	} {
		if stdout, status := runScriptCapturing(test.script + "\n"); stdout != test.want || status != 0 {
			t.Errorf("%d: %q: got %q/%d, want %q/0", index, test.script, stdout, status, test.want)
		}
	}
}
