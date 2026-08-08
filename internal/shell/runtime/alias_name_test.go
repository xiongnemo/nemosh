package runtime

import "testing"

// The first aliases anyone writes are `..`, `...`, and `~`. isVariableName
// rejected all three, so a startup file written for busybox ash failed on its
// own first lines. busybox validates nothing here; Nemosh accepts anything that
// could be typed as a command word and refuses the rest with a reason, rather
// than storing a name no command could ever match.
func TestIsAliasName(t *testing.T) {
	for _, test := range []struct {
		name  string
		alias string
		want  bool
	}{
		{name: "parent directory", alias: "..", want: true},
		{name: "grandparent", alias: "...", want: true},
		{name: "home", alias: "~", want: true},
		{name: "ordinary", alias: "ll", want: true},
		{name: "hyphenated", alias: "a-b", want: true},
		{name: "dotted", alias: "git.st", want: true},
		{name: "plus", alias: "g+", want: true},
		{name: "unicode", alias: "别名", want: true},
		{name: "empty", alias: "", want: false},
		{name: "a space makes two words", alias: "a b", want: false},
		{name: "a pipe is an operator", alias: "a|b", want: false},
		{name: "a semicolon is an operator", alias: "a;b", want: false},
		{name: "a redirect is an operator", alias: "a>b", want: false},
		{name: "a dollar would expand", alias: "a$b", want: false},
		{name: "a quote would not survive parsing", alias: `a"b`, want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := isAliasName(test.alias); got != test.want {
				t.Fatalf("isAliasName(%q) = %v, want %v", test.alias, got, test.want)
			}
		})
	}
}
