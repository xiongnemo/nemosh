package runtime_test

import "testing"

// A `$'...'` is one quoted piece whatever it holds, `\'` included, to every scan that steps
// over quotes: where a statement ends, a group, a command substitution, a case, a heredoc's
// operator, a default's word, an array literal. Each read it as a plain single-quoted string,
// which closes at the escaped quote and left the rest of the line inside a quote that never
// ended -- `echo $'a\'b'; echo two` printed the `;` and what followed, and a function or a
// loop holding one was "missing }" or "missing done". The transcripts are busybox-w32's, and
// bash agrees but for the quoted default, which it refuses; the array literal is bash's,
// busybox having none.
func TestRuntime_ansiQuoteWithAnEscapedQuoteIsOnePiece(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{`echo $'q\'r'; echo two`, "q'r\ntwo\n"},
		{`x=$'q\'r'; echo "[$x]"`, "[q'r]\n"},
		{"echo $(echo $'s\\'t') `echo $'b\\'q'`", "s't b'q\n"},
		{`f() { echo $'u\'v'; }; f; { echo $'g\'h'; }; ( echo $'p\'q' )`, "u'v\ng'h\np'q\n"},
		{`case "w'x" in $'w\'x') echo $'c\'d';; esac`, "c'd\n"},
		{`if true; then echo $'i\'f'; fi; for i in 1; do echo $'l\'p'; done`, "i'f\nl'p\n"},
		{"echo $'h\\'d' <<EOT\nbody\nEOT\necho after", "h'd\nafter\n"},
		{`echo ${u:-$'d\'f'} "${u:-$'d\'f'}"; x=abc; echo ${x/b/$'\''}`, "d'f $'d\\'f'\na'c\n"},
		{`a=($'e\'1' x\) two); echo "${a[0]} ${a[1]} ${#a[@]}"`, "e'1 x) 3\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script + "\n"); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0", stdout, status, test.want)
			}
		})
	}
}
