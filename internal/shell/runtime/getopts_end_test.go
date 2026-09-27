package runtime_test

import "testing"

// getopts at the end of its words, and past it, as busybox-w32 has it (getoptscmd): the end
// unsets OPTARG, where the last option's argument stayed; an OPTIND past the end of the words
// -- after `set --`, or set by hand -- starts the parse again from the first one, where it was
// left where it was; and a name that cannot be assigned is an error, status 2, once the parse
// has set OPTARG and OPTIND, and the next call starts again. It was assigned anyway.
func TestRuntime_getoptsAtAndPastTheEnd(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{"getopts 'c:' opt -c10; getopts 'c:' opt -c10; echo \"O=$OPTIND opt=$opt A=[${OPTARG-unset}]\"", "O=2 opt=? A=[unset]\n"},
		{"set -- -h -c foo x y z; while getopts 'hc:' opt; do :; done; echo O=$OPTIND; set --; getopts 'hc:' opt; echo \"st=$? O=$OPTIND\"", "O=4\nst=1 O=1\n"},
		{"set -- a b c; OPTIND=9; getopts h o; echo \"st=$? O=$OPTIND\"", "st=1 O=1\n"},
		{"set -- -h; OPTIND=9; getopts h o; echo \"st=$? O=$OPTIND o=$o\"", "st=0 O=2 o=h\n"},
		{"set -- -c foo -h; getopts 'hc:' opt- 2>/dev/null; echo \"st=$? opt=$opt A=$OPTARG O=$OPTIND\"", "st=2 opt= A=foo O=3\n"},
		{"getopts a o -a; getopts a opt- -a 2>/dev/null; echo \"st=$? O=$OPTIND\"; getopts a o -a; echo \"st=$? o=$o O=$OPTIND\"", "st=2 O=2\nst=0 o=a O=2\n"},
		{"readonly o; getopts a o -a 2>/dev/null; echo \"st=$? o=$o O=$OPTIND\"", "st=2 o= O=2\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script + "\n"); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as busybox-w32 answers", stdout, status, test.want)
			}
		})
	}
}

// Where getopts is belongs to the positional parameters, as busybox keeps it, and OPTIND only
// reports it: `shift` and a call start it again, a call leaves the caller's where it was, and
// a subshell or a job carries on from it. Assigning OPTIND, unsetting it, or a local one put
// back starts it at the word the value names, which drops a group's remaining letters.
// Each transcript is busybox-w32's.
func TestRuntime_getoptsKeepsItsOwnPlace(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{"set -- -a -b; getopts ab o; echo \"$o $OPTIND\"; shift; getopts ab o; echo \"$o $OPTIND\"", "a 2\nb 2\n"},
		{"f() { getopts a o; echo \"f: $o $OPTIND\"; }; set -- -x; OPTIND=5; f -a; echo \"after: $OPTIND\"", "f: a 2\nafter: 2\n"},
		{"f() { while getopts x o; do :; done; }; set -- -a -b; getopts ab o; f -x -x; echo \"OPTIND=$OPTIND\"; getopts ab o; echo \"$o $OPTIND\"", "OPTIND=3\nb 3\n"},
		{"set -- -ab; getopts ab o; echo \"$o $OPTIND\"; OPTIND=2; getopts ab o; echo \"st=$? $o $OPTIND\"", "a 2\nst=1 ? 2\n"},
		{"f() { local OPTIND; getopts x o -x; echo \"f $o $OPTIND\"; }; set -- -ab; getopts ab o; f; echo \"after $OPTIND\"; getopts ab o; echo \"st=$? $o $OPTIND\"", "f x 2\nafter 2\nst=1 ? 2\n"},
		{"set -- -a -b; getopts ab o; unset OPTIND; getopts ab o; echo \"$o $OPTIND\"", "a 2\n"},
		{"set -- -a -b -c; getopts abc o; ( getopts abc o; echo \"sub $o $OPTIND\" ); { getopts abc o; echo \"job $o $OPTIND\"; } & wait; getopts abc o; echo \"$o $OPTIND\"", "sub b 3\njob b 3\nb 3\n"},
		{"getopts :ab o -:; echo \"st=$? [$o] [${OPTARG-unset}]\"", "st=0 [:] []\n"},
		{"OPTERR=0; getopts a o -b; echo \"[$o] [${OPTARG-unset}]\"; OPTIND=1; getopts a: o -a; echo \"[$o] [${OPTARG-unset}]\"", "[?] [b]\n[:] [a]\n"},
		{"set -- -a -b; OPTIND=abc; getopts ab o; echo \"$o $OPTIND\"; OPTIND=0; getopts ab o; echo \"$o $OPTIND\"", "a 2\na 2\n"},
		{"set -- -a -b -c; getopts abc o; set -- -x -b; getopts abc o 2>/dev/null; echo \"st=$? $o $OPTIND\"", "st=0 ? 2\n"},
		{"f() { local OPTIND; while getopts x o \"$@\"; do echo \"f:$o\"; done; }; f -x; f -x", "f:x\nf:x\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script + "\n"); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as busybox-w32 answers", stdout, status, test.want)
			}
		})
	}
}
