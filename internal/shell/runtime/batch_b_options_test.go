package runtime_test

import "testing"

// kill -s, umask -S and its symbolic masks, and read -t 0: busybox has each, and each was
// refused or wrong. The answers are busybox's.
func TestBuiltinOptions_batchB(t *testing.T) {
	tests := []struct {
		name, script, stdout string
		status               int
	}{
		{name: "umask -S", script: "umask 022\numask -S\n", stdout: "u=rwx,g=rx,o=rx\n"},
		{
			name:   "a symbolic mask",
			script: "umask u=rwx,g=rx,o=\numask\numask g-x\numask\numask a+r\numask\numask -S 077\numask\n",
			stdout: "0027\n0037\n0033\n0077\n",
		},
		{name: "an octal mask with -S sets it quietly", script: "umask -S 002\necho \"st=$?\"\numask\n", stdout: "st=0\n0002\n"},
		{name: "a mask that is not one", script: "umask u=q 2>/dev/null\necho \"st=$?\"\n", stdout: "st=2\n"},
		{
			// With no class letters, the mask filters what is given; bash would make it 0222.
			name:   "a clause with no class",
			script: "umask 0124\numask =rx\numask\numask 0124\numask +x\numask\n",
			stdout: "0326\n0124\n",
		},
		{
			name:   "copying a class, and an empty clause",
			script: "umask 0124\numask u=rwx,g=u\numask\numask 0124\numask u-r,,u-r\numask\numask a=u\numask\n",
			stdout: "0004\n0524\n0777\n",
		},
		{
			name:   "a special bit is refused where it would be set",
			script: "umask 0124\numask u+s 2>/dev/null\necho \"st=$?\"\numask o+s\necho \"st=$?\"\numask\n",
			stdout: "st=2\nst=0\n0124\n",
		},
		{
			name:   "a mode that begins with a dash reads as options",
			script: "umask 0124\numask -rwx 2>/dev/null\necho \"st=$?\"\numask -\necho \"st=$?\"\numask\n",
			stdout: "st=2\nst=0\n0124\n",
		},
		{name: "only the first operand", script: "umask 1 2\necho \"st=$?\"\numask\n", stdout: "st=0\n0001\n"},
		{name: "umask -p, bash's", script: "umask 0022\numask -p\numask -p -S\n", stdout: "umask 0022\numask -S u=rwx,g=rx,o=rx\n"},
		{
			name:   "kill -s",
			script: "sleep 5 &\np=$!\nkill -s TERM $p\necho \"st=$?\"\nwait $p\necho \"wait=$?\"\n",
			stdout: "st=0\nwait=143\n",
		},
		{
			// Input ready, or at its end, is 0; nothing is read either way.
			name:   "read -t 0 at the end of input",
			script: "read -t 0 </dev/null\necho \"st=$?\"\n",
			stdout: "st=0\n",
		},
		{
			name:   "read -t 0 with nothing yet",
			script: "sleep 0.5 | { read -t 0; echo \"st=$?\"; }\n",
			stdout: "st=1\n",
		},
		{
			// The pause is the writer's head start; without it this would race the echo.
			name:   "read -t 0 leaves the line to be read",
			script: "echo x | { sleep 0.2; read -t 0; echo \"st=$?\"; read v; echo \"[$v]\"; }\n",
			stdout: "st=0\n[x]\n",
		},
		{
			name:   "read -t 0 at a pipe whose writer has gone",
			script: "true | { sleep 0.2; read -t 0; echo \"st=$?\"; }\n",
			stdout: "st=0\n",
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
