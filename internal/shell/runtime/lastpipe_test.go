package runtime_test

import "testing"

// `shopt -s lastpipe` runs a pipeline's last stage in the shell itself, so what it assigns
// stays assigned: `echo y | read w` sets w, where it set it in a subshell that then ended.
// The answers are bash 5.3's, which does this when job control is off, as it always is here.
// busybox has no shopt.
func TestLastpipe_runsTheLastStageInTheShell(t *testing.T) {
	tests := []struct {
		name, script, stdout string
		status               int
	}{
		{name: "read", script: "shopt -s lastpipe\necho y | read w\necho \"[$w]\"\n", stdout: "[y]\n"},
		{
			name:   "a loop",
			script: "shopt -s lastpipe\nprintf 'a\\nb\\n' | while read l; do n=$n$l; done\necho \"[$n]\"\n",
			stdout: "[ab]\n",
		},
		{name: "a group", script: "shopt -s lastpipe\necho b | { read x; y=set; }\necho \"[$x][$y]\"\n", stdout: "[b][set]\n"},
		{
			name:   "return is the function's",
			script: "shopt -s lastpipe\nf() { echo | return 4; echo after; }\nf\necho \"st=$?\"\n",
			stdout: "st=4\n",
		},
		{
			name:   "break is the loop's",
			script: "shopt -s lastpipe\nfor i in 1 2 3; do echo | break; done\necho \"i=$i\"\n",
			stdout: "i=1\n",
		},
		{name: "PIPESTATUS", script: "shopt -s lastpipe\nfalse | true | read z\necho \"${PIPESTATUS[*]}\"\n", stdout: "1 0 1\n"},
		{name: "a subshell stays one", script: "shopt -s lastpipe\necho q | (read s)\necho \"[$s]\"\n", stdout: "[]\n"},
		{name: "exit is the shell's", script: "shopt -s lastpipe\necho | exit 3\necho after\n", stdout: "", status: 3},
		{
			// POSIX 2.8.1's shell error ends a script that meets it in the shell itself.
			name:   "a shell error is the shell's",
			script: "set -u\nshopt -s lastpipe\necho | echo \"$nope\"\necho after\n",
			stdout: "", status: 2,
		},
		{name: "off, the stage is a subshell", script: "echo y | read w\necho \"[$w]\"\n", stdout: "[]\n"},
		{name: "off again", script: "shopt -s lastpipe\nshopt -u lastpipe\necho y | read w\necho \"[$w]\"\n", stdout: "[]\n"},
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

// The pipe is the last stage's standard input and nothing more: the shell's own is where it
// was afterwards.
func TestLastpipe_leavesTheShellsInputAlone(t *testing.T) {
	// When
	status, stdout, stderr := runReadScript(t, "outer\n", "shopt -s lastpipe\necho y | read w\nread v\necho \"[$w][$v]\"\n")

	// Then
	if stdout != "[y][outer]\n" || status != 0 {
		t.Fatalf("got %q/%d, want %q/0; stderr = %q", stdout, status, "[y][outer]\n", stderr)
	}
}
