package runtime_test

import "testing"

// An IFS character from an unquoted expansion delimits fields wherever it sits, at the edge
// of the expansion too, where it divides the expansion from the text beside it. IFS
// whitespace beside a non-whitespace separator is part of that one delimiter; a quoted empty
// string is a field even after a delimiter; and an unquoted expansion that comes to nothing
// is no field, whatever IFS is. Each expansion was split alone and its end pieces glued to
// their neighbours, so `$x$y` with x='a ' was one field. busybox-w32 and bash 5.3 give every
// answer here, and agree on all of them.
func TestRuntime_fieldSplittingAtExpansionEdges(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{"x='a '; y=b; printf '[%s]' $x$y", "[a][b]"},
		{"x=' a'; printf '[%s]' b$x", "[b][a]"},
		{"x=' '; printf '[%s]' a${x}b", "[a][b]"},
		{"x='a b '; printf '[%s]' \"q\"$x\"r\"", "[qa][b][r]"},
		{"x=' a b '; printf '[%s]' \"<\"$x\">\"", "[<][a][b][>]"},
		{"IFS=:; x='a:'; printf '[%s]' $x'b'", "[a][b]"},
		{"IFS=:; x='a:'; y=':b'; printf '[%s]' $x$y", "[a][][b]"},
		{"IFS=:; x='a'; y=':'; printf '[%s]' $x$y$y", "[a][]"},
		{"IFS=' :'; x='a : b'; printf '[%s]' $x", "[a][b]"},
		{"IFS=' :'; x='a : : b'; printf '[%s]' $x", "[a][][b]"},
		{"x='a '; printf '[%s]' $x\"\"", "[a][]"},
		{"x=' '; printf '[%s]' \"\"$x\"\"", "[][]"},
		{"IFS=; x=; set -- $x; echo $#", "0\n"},
		{"x=; set -- \"\"$x; echo $#", "1\n"},
		// Unchanged, and pinned beside the rest: the rules as they already worked.
		{"IFS=:; x=':a'; printf '[%s]' $x", "[][a]"},
		{"IFS=:; x='a::b'; printf '[%s]' $x", "[a][][b]"},
		{"IFS=' :'; x=' :a'; printf '[%s]' $x", "[][a]"},
		{"IFS=; x='a b'; set -- $x; echo $#", "1\n"},
		{"set -- \"a \" b; printf '[%s]' x$@y", "[xa][by]"},
		{"set -- a \"\" b; printf '[%s]' $@", "[a][b]"},
		{"set -- \"a b\" c; printf '[%s]' \"x$@y\"", "[xa b][cy]"},
		{"IFS=:; set -- $(printf 'a:b:'); echo $#", "2\n"},
		{"IFS=; set -- a \"\" b; printf '[%s]' x$@y", "[xa][by]"},
		{"x='a b'; y=\"c d\"; printf '[%s]' $x\"$y\"", "[a][bc d]"},
		{"x=' '; set -- $x$x; echo $#", "0\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as busybox and bash answer", stdout, status, test.want)
			}
		})
	}
}

// Unquoted, an array's `*` forms are a field per element, as unquoted `$*` is: with IFS empty
// nothing splits them, and they stay apart. Quoted, and in an assignment, they are joined with
// IFS's first character. bash's answers, measured; busybox has no arrays. They were joined
// first, so with IFS empty `${a[*]}` over (a 'b c') was the one field ab c.
func TestRuntime_unquotedArrayStarIsAFieldPerElement(t *testing.T) {
	script := "a=(a \"b c\"); IFS=\n" +
		"printf '<%s>' ${a[*]}; echo; printf '<%s>' ${a[*]/b/X}; echo; printf '<%s>' ${a[*]@Q}; echo\n" +
		"x=${a[*]}; echo \"[$x]\"; printf '<%s>' \"${a[*]}\"; echo; printf '<%s>' ${#a[*]}; echo\n" +
		"IFS=:; printf '<%s>' ${a[*]}; echo; e=(); printf '<%s>' ${e[*]}; echo '|'\n" +
		"set -- p \"q r\"; IFS=; printf '<%s>' ${*/p/P}; echo\n"
	want := "<a><b c>\n<a><X c>\n<'a'><'b c'>\n[ab c]\n<ab c>\n<2>\n<a><b c>\n<>|\n<P><q r>\n"
	if stdout, status := runScriptCapturing(script); stdout != want || status != 0 {
		t.Fatalf("got %q/%d, want %q/0, as bash answers", stdout, status, want)
	}
}
