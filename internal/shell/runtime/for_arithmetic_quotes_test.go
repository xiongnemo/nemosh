package runtime_test

import "testing"

// A counted loop's parts lose their double quotes, as bash's do and as `(( ))`'s already did:
// `i<"$n"` is i<$n, which is how `for ((i=0; i<"${#a[@]}"; i++))` is often written. A quote
// in a command substitution in a part is that command's own. Single quotes stay an error, as
// in bash. Measured against bash 5.3.
func TestArithmeticFor_removesTheExpressionsDoubleQuotes(t *testing.T) {
	for _, test := range []struct {
		name, script, want string
	}{
		{name: "a quoted count", script: "arr=(a b c); for (( i=0; i<\"${#arr[@]}\"; i++ )); do echo $i; done\n", want: "0\n1\n2\n"},
		{name: "a quoted variable", script: "n=2; for ((i=0; i<\"$n\"; i++)); do echo $i; done\n", want: "0\n1\n"},
		{name: "quoted numbers", script: "for ((i = \"3\"; i < \"5\"; ++i)); do echo $i; done\n", want: "3\n4\n"},
		{name: "a substitution keeps its own", script: "f=\"a b\"; for ((i=0; i<$(printf \"%s\" \"$f\" | wc -c); i+=2)); do echo $i; done\n", want: "0\n2\n"},
		{name: "single quotes are an error", script: "for ((i = '3'; i < 5; ++i)); do echo $i; done; echo s=$?\n", want: "s=1\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if stdout, _ := runScriptCapturing(test.script); stdout != test.want {
				t.Fatalf("stdout = %q, want %q", stdout, test.want)
			}
		})
	}
}

// `(( ))` removes the expression's own double quotes and leaves a command substitution's: every
// quote went, so `$(printf "%s" "$f")` ran printf on f's words, split. Measured against bash 5.3.
func TestArithmeticCommand_keepsASubstitutionsQuotes(t *testing.T) {
	script := "f=\"a b\"; (( $(printf \"%s\" \"$f\" | wc -c) > 2 )) && echo kept\nn=2; (( \"$n\" > 1 )) && echo removed\n"
	if stdout, _ := runScriptCapturing(script); stdout != "kept\nremoved\n" {
		t.Fatalf("stdout = %q, want both", stdout)
	}
}
