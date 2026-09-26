package runtime_test

import "testing"

// A function's name can be nearly any word, as in both references: `lib::fn`, `my-fn`,
// `a.b` and `1fn` are the names library code gives functions to fake namespaces, and each
// was a syntax error here that stopped the whole script before its first line. Only a
// word that is something else is refused: one with blanks, quotes, expansions, operators
// or `=`. A name with a slash is defined but never called by that name, since a slash
// makes a command a path -- busybox-w32's answer; bash calls the function.
func TestRuntime_functionNamesAreAlmostAnyWord(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{`lib::fn() { echo ok; }; lib::fn`, "ok\n"},
		{`my-fn() { echo ok; }; my-fn`, "ok\n"},
		{`a.b() { echo ok; }; a.b`, "ok\n"},
		{`1fn() { echo ok; }; 1fn`, "ok\n"},
		{`x+y() { echo ok; }; x+y`, "ok\n"},
		{`f%() { echo ok; }; f%`, "ok\n"},
		{`a,b() { echo ok; }; a,b`, "ok\n"},
		{`function lib::g { echo ok; }; lib::g`, "ok\n"},
		{`function my-g() { echo ok; }; my-g`, "ok\n"},
		{`my-fn() { echo "$FUNCNAME"; }; my-fn`, "my-fn\n"},
		{`lib::fn() { :; }; type lib::fn | head -1`, "lib::fn is a function\n"},
		{`lib::fn() { :; }; unset -f lib::fn; type lib::fn >/dev/null 2>&1 || echo gone`, "gone\n"},
		{`a/b() { echo fn; }; a/b 2>/dev/null || echo not-called`, "not-called\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0", stdout, status, test.want)
			}
		})
	}
}
