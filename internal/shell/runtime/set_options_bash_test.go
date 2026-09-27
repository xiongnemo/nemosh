package runtime_test

import (
	"strings"
	"testing"
)

// bash's `set -o` names that busybox has not got. `set +H` and `set -o posix` begin a good many
// scripts and were "illegal option", status 2. The answers are bash's.
func TestSet_bashOptionNames(t *testing.T) {
	tests := []struct {
		name, script, stdout string
		status               int
	}{
		{
			name:   "a preamble survives",
			script: "set -euo pipefail\nset +H\nset -o posix\nset +o posix\nset -o igncr\nset +h\nset -h\necho ok\n",
			stdout: "ok\n",
		},
		{
			name:   "+B turns brace expansion off",
			script: "echo {a,b}\nset +B\necho {a,b} x={c,d}\nset -o braceexpand\necho {e,f}\n",
			stdout: "a b\n{a,b} x={c,d}\ne f\n",
		},
		{
			// busybox decides what `$-` says, and it has none of bash's letters.
			name:   "bash's letters stay out of $-",
			script: "set -H\nset -h\nset -B\necho \"[$-]\"\n",
			stdout: "[]\n",
		},
		{
			name:   "a fixed name accepts the value it has",
			script: "set +k +t +P +p\nset -o igncr\necho \"st=$?\"\n",
			stdout: "st=0\n",
		},
		{
			name:   "and refuses the other",
			script: "set -k 2>&1\necho \"st=$?\"\nset +o igncr 2>&1\necho \"st=$?\"\n",
			stdout: "set: -o keyword: always off here: an assignment after the command name is an argument\nst=2\n" +
				"set: +o igncr: always on here: a carriage return before a newline is dropped, as busybox-w32 drops it\nst=2\n",
		},
		{
			// A script starts with bash's defaults: braceexpand and hashall on, the prompt's
			// three off.
			name:   "the listing",
			script: "set -o | grep -E '^(braceexpand|hashall|histexpand|history|emacs|posix|igncr|keyword) '\n",
			stdout: "braceexpand \ton\nhashall     \ton\nhistexpand  \toff\nemacs       \toff\nhistory     \toff\nigncr       \ton\nkeyword     \toff\nposix       \toff\n",
		},
		{
			name:   "+o reads back",
			script: "set -o posix\nset +B\nsaved=$(set +o)\nset +o posix\nset -B\neval \"$saved\"\nset -o | grep -E '^(posix|braceexpand) '\n",
			stdout: "braceexpand \toff\nposix       \ton\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, stdout, stderr := runSetScript(t, test.script)

			// Then
			if stdout != test.stdout || status != test.status {
				t.Fatalf("got %q/%d, want %q/%d; stderr = %q", stdout, status, test.stdout, test.status, stderr)
			}
		})
	}
}

// Every name `set -o` lists, `set -o NAME` or `set +o NAME` accepts with the value it has.
func TestSet_everyListedNameReadsBack(t *testing.T) {
	// When
	status, stdout, stderr := runSetScript(t, "eval \"$(set +o)\"\necho \"st=$?\"\n")

	// Then
	if status != 0 || !strings.HasSuffix(stdout, "st=0\n") || stderr != "" {
		t.Fatalf("got %q/%d; stderr = %q", stdout, status, stderr)
	}
}
