package runtime_test

import "testing"

// A heredoc's delimiter is a word, and a `;` ends a word: `cat <<EOF;` is a heredoc
// delimited by EOF, and a command may follow on the line. Here the `;` was taken into the
// delimiter, no line matched `EOF;`, and the whole script was refused as incomplete. And a
// `<<` inside an arithmetic command is a shift, as inside $(( )): `(( x << 2 ))` went looking
// for a heredoc delimited by 2. busybox-w32 and bash agree on the heredocs; busybox has no
// (( )), so those are bash's.
func TestRuntime_heredocDelimiterEndsWhereAWordDoes(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{"cat <<EOF;\nhi\nEOF\necho done", "hi\ndone\n"},
		{"cat <<EOF;echo after\nhi\nEOF", "hi\nafter\n"},
		{"cat <<'EOF';\nhi $x\nEOF", "hi $x\n"},
		{"x=1; (( x << 2 )); echo $?", "0\n"},
		{"(( y = 1 << 3 )); echo $y", "8\n"},
		{"if (( 1 << 1 == 2 )); then echo yes; fi", "yes\n"},
		{"x=5; (( x <<= 1 )); echo $x", "10\n"},
		{"f() { (( $1 << 1 )); }; f 3 && echo nonzero", "nonzero\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0", stdout, status, test.want)
			}
		})
	}
}

// `((expr))` is `let "expr"`, as bash documents it: the expression is expanded, as in
// double quotes, before it is evaluated. It went to let single-quoted, and a `$` in it was
// a syntax error -- `(( $1 > 2 ))`, the way a function tests its argument, among them. A
// double quote inside is removed. The answers are bash 5.3's; busybox-w32 has no (( )).
func TestRuntime_arithmeticCommandExpandsItsExpression(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{`x=3; (( $x > 2 )) && echo big`, "big\n"},
		{`(( $(echo 2) + 1 == 3 )) && echo yes`, "yes\n"},
		{`a=(4 5); (( ${#a[@]} == 2 )) && echo two`, "two\n"},
		{`x=3; (( x == "3" )) && echo quoted`, "quoted\n"},
		{`(( 2 * 3 == 6 )) && echo star`, "star\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as bash answers", stdout, status, test.want)
			}
		})
	}
}
