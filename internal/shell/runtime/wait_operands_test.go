package runtime_test

import (
	"testing"
)

// `wait` with operands, and `wait -n`, each measured against both references.
//
// Every script prints what it is asserting, so a failure shows the whole answer rather than
// one number. runScriptCapturing is the corpus's runner: RunScript and then CloseBatch.
func TestWait_answersAsBothReferencesDo(t *testing.T) {
	for _, test := range []struct {
		name   string
		script string
		want   string
	}{
		{
			// POSIX: the status of the last operand. It used to refuse a second one.
			name:   "several operands answer the last",
			script: "(exit 3) & a=$!\n(exit 4) & b=$!\nwait $a $b\necho \"st=$?\"\n",
			want:   "st=4\n",
		},
		{
			// A job whose end `jobs` reported is not known any more, and a process id
			// the shell does not know is one that exited 127. busybox answers 127 here.
			name:   "a job already reported is 127",
			script: "(exit 7) & p=$!\nsleep 0.2\njobs >/dev/null\nwait $p\necho \"st=$?\"\n",
			want:   "st=127\n",
		},
		{
			name:   "a pid that is not a child is 127",
			script: "wait 99999 2>/dev/null\necho \"st=$?\"\n",
			want:   "st=127\n",
		},
		{
			// It used to be refused, and the plain `wait` a script puts after it hid that.
			name:   "wait -n returns when the first job does",
			script: "sleep 0.1 &\nsleep 5 &\nwait -n\necho \"st=$?\"\njobs\nkill %2\n",
			want:   "st=0\n[2] Running\n",
		},
		{
			// bash's answer; busybox-w32 says 0 for a job that exited 3.
			name:   "wait -n answers the job's own status",
			script: "(exit 3) &\nwait -n\necho \"st=$?\"\n",
			want:   "st=3\n",
		},
		{
			name:   "wait -n with nothing to wait for is 127",
			script: "wait -n\necho \"st=$?\"\n",
			want:   "st=127\n",
		},
		{
			// An ended job is the next one, rather than being passed over for one running.
			name:   "wait -n takes a job that has already ended",
			script: "sleep 5 &\n(exit 6) &\nsleep 0.2\nwait -n\necho \"st=$?\"\nkill %1\n",
			want:   "st=6\n",
		},
		{
			name:   "wait -n with operands waits only for those",
			script: "(exit 2) &\n(sleep 0.2; exit 5) &\nwait -n %2\necho \"st=$?\"\nwait %1\necho \"first=$?\"\n",
			want:   "st=5\nfirst=2\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if stdout, _ := runScriptCapturing(test.script); stdout != test.want {
				t.Fatalf("stdout = %q, want %q", stdout, test.want)
			}
		})
	}
}
