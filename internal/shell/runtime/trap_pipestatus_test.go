package runtime

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// A trap's own commands leave $PIPESTATUS as the command before it set it: bash saves the
// statuses before every trap and puts them back after (trap.c), so with a DEBUG trap
// `true | false; echo ${PIPESTATUS[*]}` says 0 1, and an ERR trap leaves the failed pipeline's
// 1 1, measured in bash 5.3 on 2026-11-21. The trap's commands set it, and 0 was all a DEBUG
// trap let through -- starship's and bash-preexec's, which read it from one, among them.
func TestTrap_leavesPIPESTATUSAsTheCommandSetIt(t *testing.T) {
	script := `trap 'true' DEBUG
true | false
echo "debug: ${PIPESTATUS[*]}"
trap - DEBUG
trap 'true' ERR
false | false
echo "err: ${PIPESTATUS[*]}"
trap - ERR
trap 'x=$(true)' DEBUG
true | false
echo "debug-subst: ${PIPESTATUS[*]} $?"
`
	var stdout bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: new(bytes.Buffer)})
	rt.RunScript(context.Background(), script)
	if want := "debug: 0 1\nerr: 1 1\ndebug-subst: 0 1 1\n"; stdout.String() != want {
		t.Fatalf("stdout %q, want %q", stdout.String(), want)
	}
}
