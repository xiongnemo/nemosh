package runtime

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// A bind -x command reads the line and the cursor from READLINE_LINE and READLINE_POINT and
// leaves the line after in them, the point counting characters, as fzf's widgets write it; all
// three are unset afterwards, as bash 5 unsets them. fzf puts 0x7fffffff for the line's end.
func TestRunKeyCommand_isTheLineInAndOut(t *testing.T) {
	var stdout bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: new(bytes.Buffer)})
	ctx := context.Background()
	insert := `READLINE_LINE="${READLINE_LINE:0:READLINE_POINT}é${READLINE_LINE:READLINE_POINT}"; READLINE_POINT=$((READLINE_POINT + 1))`
	if line, point := rt.RunKeyCommand(ctx, insert, "aéb", 2); line != "aééb" || point != 3 {
		t.Errorf("inserting at 2: %q, %d; want aééb, 3", line, point)
	}
	if line, point := rt.RunKeyCommand(ctx, "READLINE_LINE=done; READLINE_POINT=0x7fffffff", "x", 0); line != "done" || point != 4 {
		t.Errorf("0x7fffffff: %q, %d; want done, 4", line, point)
	}
	rt.RunScript(ctx, `echo "[${READLINE_LINE-unset} ${READLINE_POINT-unset} ${READLINE_MARK-unset}]"`+"\n")
	if stdout.String() != "[unset unset unset]\n" {
		t.Errorf("after the command: %q", stdout.String())
	}
}

// fc in a bind -x command lists through the newest entry, the line typed last, since no fc line
// of its own is in the list then, as there is at a prompt; in a command substitution in it too,
// as fzf's Ctrl-R reads `$(builtin fc -lnr ... | ...)`. It left out the newest, so Ctrl-R never
// offered the command just run. Measured in Git for Windows' bash 5.3 in a console, 2026-11-21.
func TestRunKeyCommand_fcListsThroughTheNewestEntry(t *testing.T) {
	var stdout bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: new(bytes.Buffer)})
	ctx := context.Background()
	rt.RunScript(ctx, "set -o history\n")
	rt.RecordInteractiveLine("echo old", false)
	rt.RecordInteractiveLine("echo session-line", false)
	rt.RunKeyCommand(ctx, `fc -lnr -9; x=$(builtin fc -lnr -9 | cat); echo "[$x]"`, "", 0)
	if want := "\t echo session-line\n\t echo old\n[\t echo session-line\n\t echo old]\n"; stdout.String() != want {
		t.Errorf("in a bind -x command: %q, want %q", stdout.String(), want)
	}
	stdout.Reset()
	rt.RecordInteractiveLine("fc -lnr -9", false)
	rt.RunScript(ctx, "fc -lnr -9\n")
	if want := "\t echo session-line\n\t echo old\n"; stdout.String() != want {
		t.Errorf("at a prompt, its own line left out: %q, want %q", stdout.String(), want)
	}
}
