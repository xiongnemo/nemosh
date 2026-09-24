package runtime_test

import "testing"

// getopts over the forms a script meets: grouped letters, an attached argument, an unknown
// option and a missing argument in both modes, `--`, operands after the name, and OPTIND
// ready for `shift`. Every transcript is busybox-w32's, measured; bash agrees except where
// getopts.go says it does not.
func TestGetopts_readsOptionsAsPOSIXDescribes(t *testing.T) {
	for _, test := range []struct {
		name   string
		script string
		want   string
	}{
		{name: "grouped letters and a separate argument", script: "set -- -ab -c val x\nwhile getopts abc: o; do echo \"$o|$OPTARG|$OPTIND\"; done\necho \"end $OPTIND $o\"\n", want: "a||2\nb||2\nc|val|4\nend 4 ?\n"},
		{name: "an attached argument", script: "set -- -bval\nwhile getopts ab: o; do echo \"$o|$OPTARG\"; done\n", want: "b|val\n"},
		{name: "an unknown option carries on", script: "set -- -z -a\nwhile getopts a o 2>/dev/null; do echo \"$o\"; done\n", want: "?\na\n"},
		{name: "silent mode names the problem", script: "set -- -z -b\nwhile getopts :ab: o; do echo \"$o|$OPTARG\"; done\n", want: "?|z\n:|b\n"},
		{name: "-- ends the options", script: "set -- -a -- -b\nwhile getopts ab o; do echo \"$o\"; done\necho \"ind=$OPTIND\"\n", want: "a\nind=3\n"},
		{name: "operands after the name", script: "while getopts ab: o -a -b val rest; do echo \"$o=$OPTARG\"; done\necho \"ind=$OPTIND\"\n", want: "a=\nb=val\nind=4\n"},
		{name: "OPTIND is ready for shift", script: "set -- -a file\nwhile getopts a o; do :; done\nshift $((OPTIND-1))\necho \"rest=$1\"\n", want: "rest=file\n"},
		{name: "a local OPTIND starts fresh", script: "f() { local OPTIND; while getopts x o \"$@\"; do echo \"f:$o\"; done; }\nf -x\nf -x\n", want: "f:x\nf:x\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if stdout, _ := runScriptCapturing(test.script); stdout != test.want {
				t.Fatalf("stdout = %q, want %q", stdout, test.want)
			}
		})
	}
}
