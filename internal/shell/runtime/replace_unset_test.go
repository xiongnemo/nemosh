package runtime_test

import "testing"

// A pattern substitution on an unset parameter is empty, as busybox and bash have it: there is
// nothing to replace in. `${v/#/:}` is the idiom for a colon before v only when v is set, and it
// gave the colon for an unset v, the empty string it read matching `*` and the anchored empty
// patterns. An empty v is set, and is replaced. busybox's ash_test var_bash_repl_empty_var.
func TestRuntime_replaceInAnUnsetParameterIsEmpty(t *testing.T) {
	for _, test := range []struct{ script, want string }{
		{"unset v; echo \"[${v/*/w}]\"; v=\"\"; echo \"[${v/*/w}]\"\n", "[]\n[w]\n"},
		{"unset u; echo \"[${u//x/y}]\" \"[${u/#/p}]\" \"[${u/%/s}]\"\n", "[] [] []\n"},
		{"set --; echo \"[${1/#/p}]\"\n", "[]\n"},
		{"x=abc; echo \"[${x/b/B}]\" \"[${x/#/p}]\"\n", "[aBc] [pabc]\n"},
	} {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as busybox and bash answer", stdout, status, test.want)
			}
		})
	}
}
