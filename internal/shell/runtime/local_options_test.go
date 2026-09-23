package runtime_test

import "testing"

// `local -` makes the `set` options the call's own: what the body sets is undone when it
// returns, and what was set before `local -` stays. Every transcript is busybox-w32's; this
// used to be `local: -: bad variable name`, and the option leaked into the caller.
func TestLocalDash_givesAFunctionItsOwnOptions(t *testing.T) {
	for _, test := range []struct {
		name   string
		script string
		want   string
	}{
		{
			name:   "set -e and pipefail are undone",
			script: "f() { local -; set -e; set -o pipefail; }\nf\ncase $- in *e*) echo leaked;; *) echo restored;; esac\nset -o | grep -c 'pipefail.*on'\n",
			want:   "restored\n0\n",
		},
		{
			name:   "an option turned off comes back on",
			script: "set -u\nf() { local -; set +u; echo \"[$nope]\"; }\nf\ncase $- in *u*) echo u-back;; esac\n",
			want:   "[]\nu-back\n",
		},
		{
			name:   "what was set before local - stays",
			script: "f() { set -e; local -; set +e; }\nf\ncase $- in *e*) echo kept;; esac\n",
			want:   "kept\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if stdout, _ := runScriptCapturing(test.script); stdout != test.want {
				t.Fatalf("stdout = %q, want %q", stdout, test.want)
			}
		})
	}
}
