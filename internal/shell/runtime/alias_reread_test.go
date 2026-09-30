package runtime_test

import "testing"

// An alias that follows a value ending in a blank is read again, as busybox and bash read it:
// its first word is looked up, and the word after any alias in it that ends in a blank, while
// each names an alias not already being read. With `alias foo='echo ' bar=baz baz=quux`, `foo
// bar` is quux; it was baz. busybox's ash_test alias. The last case is busybox's: a quoted word
// ends the looking up, where bash looks up the word after it.
func TestRuntime_anAliasAfterABlankIsReadAgain(t *testing.T) {
	for _, test := range []struct{ aliases, command, want string }{
		{"alias foo='echo ' bar=baz baz=quux", "foo bar", "quux\n"},
		{"alias e='echo ' x='y ' y=z", "e x x", "z z\n"},
		{"alias a1='echo ' a2='a3 ' a3='echo '", "a1 a2 a2 end", "echo echo end\n"},
		{"alias e='echo ' x=y y='echo ' w=W", "e x w", "echo W\n"},
		{"alias e='echo ' x='a b' a='echo ' b=B", "e x", "echo B\n"},
		{"alias e='echo ' x='y w' y='echo ' w=W", "e x w", "echo W w\n"},
		{"alias e='echo ' x='x y'", "e x", "x y\n"},
		{"alias e='echo ' p=q q=p", "e p", "p\n"},
		{"alias e='echo ' nothing='' w=W", "e nothing w", "W\n"},
		{"alias e='echo ' x=' y' y=z", "e x", "z\n"},
		{"alias e='echo ' x='y' y='z\t' w=W", "e x w", "z W\n"},
		{"alias e='echo ' x='y;echo w' y=Y", "e x w", "Y\nw w\n"},
		{"alias e='echo ' x='\"y\" w' w=W", "e x", "y w\n"},
	} {
		t.Run(test.command, func(t *testing.T) {
			script := test.aliases + "\n" + test.command + "\n"
			if stdout, status := runScriptCapturing(script); stdout != test.want || status != 0 {
				t.Errorf("%s: got %q/%d, want %q/0, as busybox answers", test.aliases, stdout, status, test.want)
			}
		})
	}
}
