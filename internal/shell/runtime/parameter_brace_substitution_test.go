package runtime_test

import "testing"

// A `}` inside a command substitution in a `${...}` does not end the expansion, as
// busybox-w32 and bash read it: the substitution is stepped over whole, quotes of its own and
// all. Each scan took the first `}`, a brace group's or a quoted one's, so these were
// `unexpected )` or came out as their own text.
func TestRuntime_braceInsideSubstitutionInParameter(t *testing.T) {
	script := "echo ${foo:-$({ echo hi; })}\n" +
		"echo \"${foo:-$({ echo hi; })}\"\n" +
		"x=${foo:-$(echo \"}\")}; echo \"$x\"\n" +
		"echo ${foo:-`echo }`}\n" +
		"echo \"${foo:-\"$(echo \"}\")\"}\"\n" +
		"{ echo ${foo:-$({ echo in; })}; }\n" +
		"f() { echo ${1:-$(case x in x) echo c;; esac)}; }; f\n" +
		"echo \"${foo:-$(echo ')')}\"\n"
	want := "hi\nhi\n}\n}\n}\nin\nc\n)\n"
	if stdout, status := runScriptCapturing(script); stdout != want || status != 0 {
		t.Errorf("got %q/%d, want %q/0, as busybox-w32 and bash answer", stdout, status, want)
	}
}
