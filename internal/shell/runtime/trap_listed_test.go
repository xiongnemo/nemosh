package runtime

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// A subshell lists the shell's DEBUG and ERR traps as it lists the rest, though it does not run
// them, until it sets or resets one of its own: bash's `$(trap -p DEBUG)` is the shell's trap,
// measured in bash 5.3 on 2026-11-21, which is how starship and bash-preexec find a trap already
// there to run beside theirs. The signal traps were listed and those two were not, so
// `saved=$(trap)` lost them too.
func TestTrap_aSubshellListsTheTrapsItDoesNotRun(t *testing.T) {
	script := `trap ': dbg' DEBUG
trap ': err' ERR
echo "[$(trap -p DEBUG)]"
echo "[$(trap -p ERR)]"
echo "[$(trap -P DEBUG)]"
( trap -p DEBUG )
x=$(trap - DEBUG; trap -p DEBUG); echo "[$x]"
y=$(trap ': mine' ERR; trap -p ERR); echo "[$y]"
f() { trap -p DEBUG; }
f
`
	var stdout bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: new(bytes.Buffer)})
	rt.RunScript(context.Background(), script)
	want := "[trap -- ': dbg' DEBUG]\n[trap -- ': err' ERR]\n[: dbg]\ntrap -- ': dbg' DEBUG\n[]\n[trap -- ': mine' ERR]\n"
	if stdout.String() != want {
		t.Fatalf("stdout\n%s\nwant\n%s", stdout.String(), want)
	}
}
