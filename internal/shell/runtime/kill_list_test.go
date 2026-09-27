package runtime_test

import "testing"

// kill's listing and its translations, busybox's: `kill -l` lists ` 1) HUP` a line each, and
// with operands translates each, a number to its name and a name to its number. A number
// with no signal is refused wherever one is sent, as busybox refuses it. bash's -L and -n,
// which busybox lacks, are its synonyms for -l and -s.
func TestRuntime_killListsAndTranslates(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{`kill -l | head -2`, " 1) HUP\n 2) INT\n"},
		{`kill -l 0 INT 143 130 999 64 SIGKILL; echo "st=$?"`, "EXIT\n2\nTERM\nINT\n103\n64\n9\nst=0\n"},
		{`kill -l NOPE 2>/dev/null; echo "st=$?"`, "st=1\n"},
		{`kill -L | head -1; trap -l | head -1`, " 1) HUP\n 1) HUP\n"},
		{`sleep 5 & p=$!; kill -9999 $p 2>/dev/null; echo "st=$?"; kill -10 $p 2>/dev/null; echo "st=$?"; kill $p; wait $p; echo "w=$?"`, "st=1\nst=1\nw=143\n"},
		{`sleep 5 & p=$!; kill -n 9 $p; echo "st=$?"; wait $p; echo "w=$?"`, "st=0\nw=137\n"},
		{`kill -n 2>/dev/null; echo "st=$?"`, "st=1\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0", stdout, status, test.want)
			}
		})
	}
}
