package runtime_test

import (
	"testing"
	"time"
)

// A write into a pipe whose reader has gone ends the writer, as SIGPIPE ends it in both
// references: the shell's stage when the shell wrote, a builtin's output, and only the program
// when a program did. The shell's own writes only failed, quietly, so a loop that wrote into
// `head` wrote on for ever. Each output is bash's; see pipe_stage.go.
func TestPipeStage_aWriteWithNoReaderEndsTheWriter(t *testing.T) {
	for _, test := range []struct{ name, script, stdout, stderr string }{
		{
			name:   "a loop the shell writes from",
			script: "while :; do echo x; done | head -2; echo done\n",
			stdout: "x\nx\ndone\n",
		},
		{
			name:   "nothing after the write runs",
			script: "{ echo 1; while :; do echo c; done; echo after >&2; } | head -1; echo \"${PIPESTATUS[*]}\"\n",
			stdout: "1\n141 0\n",
		},
		{
			name:   "a program ends alone",
			script: "{ echo 1; seq 1 100000; echo \"after $?\" >&2; } | head -1; echo \"${PIPESTATUS[*]}\"\n",
			stdout: "1\n0 0\n", stderr: "after 141\n",
		},
		{
			name:   "a function's loop",
			script: "f() { while :; do echo f; done; }\nf | head -1; echo \"${PIPESTATUS[*]}\"\n",
			stdout: "f\n141 0\n",
		},
		{
			name:   "pipefail reports it",
			script: "set -o pipefail\nyes | head -1; echo $?\n",
			stdout: "y\n141\n",
		},
		{
			name:   "an inner pipeline's last stage writes into the outer pipe",
			script: "{ yes | cat; } | head -1; echo \"${PIPESTATUS[*]}\"\n",
			stdout: "y\n141 0\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			type outcome struct{ stdout, stderr string }
			done := make(chan outcome, 1)
			go func() {
				_, stdout, stderr := runSetScript(t, test.script)
				done <- outcome{stdout, stderr}
			}()
			select {
			case got := <-done:
				if got.stdout != test.stdout || got.stderr != test.stderr {
					t.Errorf("stdout %q stderr %q, want %q and %q", got.stdout, got.stderr, test.stdout, test.stderr)
				}
			case <-time.After(20 * time.Second):
				t.Fatal("the writer went on writing into a pipe no one reads")
			}
		})
	}
}

// A stage that reads nothing can end before the stage feeding it has written, which a reader
// that is a process seldom does, and the first write then found no reader: `echo a | true`
// said 141 for echo about one run in four, where both references say 0. Its end stays open a
// little while for the writer; see lingers.
func TestPipeStage_aReaderThatReadsNothingLetsTheFirstWriteIn(t *testing.T) {
	const script = "{ echo hi; exit 55; } | false; echo \"${PIPESTATUS[*]}\"\necho a | true; echo \"${PIPESTATUS[*]}\"\n"
	for range 20 {
		if _, stdout, _ := runSetScript(t, script); stdout != "55 1\n0 0\n" {
			t.Fatalf("stdout %q, want 55 1 and 0 0, as both references answer", stdout)
		}
	}
}
