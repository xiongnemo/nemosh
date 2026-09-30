package runtime_test

import "testing"

// A for loop's list is fixed before its first turn, as POSIX 2.9.4.2 has it and busybox and
// bash run it. Each word was expanded as the loop reached it, after the turns before had
// run, and `for name` walked the parameters as the body rewrote them. busybox's ash_test
// var_subst_in_for.
func TestFor_theListIsFixedBeforeTheFirstTurn(t *testing.T) {
	for _, test := range []struct{ name, script, want string }{
		{
			name:   "a word that names the loop's variable",
			script: "a=a\nempty=\nfor a in u $empty $empty$a v; do printf '.%s.' \"$a\"; done\necho\n",
			want:   ".u..a..v.\n",
		},
		{
			name:   "every expansion is made before the first turn",
			script: "n=0\nfor i in a $((n+=1)); do printf '%s:%s ' \"$i\" \"$n\"; done\necho\n",
			want:   "a:1 1:1 \n",
		},
		{
			name:   "set -- in the body leaves the arguments it began with",
			script: "set -- 1 2 3\nfor a; do set -- q r s; printf %s \"$a\"; done\necho\n",
			want:   "123\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			status, stdout, stderr := runSetScript(t, test.script)
			if status != 0 || stdout != test.want {
				t.Fatalf("status %d stdout %q stderr %q, want %q", status, stdout, stderr, test.want)
			}
		})
	}
}

// A word that fails to expand stops the loop before any turn has run: in both references
// `for i in a ${x?}` prints no turn at all. One ran for each word before it.
func TestFor_aWordThatFailsRunsNoTurn(t *testing.T) {
	status, stdout, stderr := runSetScript(t, "for i in a ${x?unset}; do echo \"turn $i\"; done\n")
	if status == 0 || stdout != "" {
		t.Fatalf("status %d stdout %q stderr %q, want a failure and no turn", status, stdout, stderr)
	}
}
