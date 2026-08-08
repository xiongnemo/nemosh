package runtime_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// `history` is not POSIX; it is a shell extension, and it has to be a builtin
// because the list it prints belongs to the shell. busybox has it under
// `#if MAX_HISTORY` in the ash builtin table.
//
// The format is busybox's and bash's: a right-aligned number, two spaces, the
// line. A startup file that pipes `history | grep` depends on the line being
// the tail of the row.
func TestHistory_printsNumberedEntries(t *testing.T) {
	// When
	status, stdout, stderr := runSetScript(t, "history\n")

	// Then
	if status != 0 {
		t.Fatalf("status = %d, stderr = %q, want 0", status, stderr)
	}
	// A non-interactive shell has recorded nothing, so this is empty rather
	// than an error: `history` in a script is a question with a valid answer.
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty in a non-interactive shell", stdout)
	}
}

func TestHistory_recordsAndReportsWhatWasAdded(t *testing.T) {
	// Given
	rt, stdout := newHistoryRuntime(t)
	rt.RecordHistory("echo one")
	rt.RecordHistory("echo two")

	// When
	status := rt.RunHistoryBuiltin(nil)

	// Then
	if status != 0 {
		t.Fatalf("status = %d, want 0", status)
	}
	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("history printed %q, want two rows", stdout.String())
	}
	for index, want := range []string{"echo one", "echo two"} {
		if !strings.HasSuffix(lines[index], want) {
			t.Errorf("row %d = %q, want it to end with %q", index+1, lines[index], want)
		}
		if !strings.Contains(lines[index], strings.TrimSpace(itoaTest(index+1))) {
			t.Errorf("row %d = %q, want it numbered %d", index+1, lines[index], index+1)
		}
	}
}

// `history -c` clears, which is the one option busybox and bash agree on.
func TestHistory_clearsWithDashC(t *testing.T) {
	// Given
	rt, stdout := newHistoryRuntime(t)
	rt.RecordHistory("echo one")

	// When
	if status := rt.RunHistoryBuiltin([]string{"-c"}); status != 0 {
		t.Fatalf("history -c status = %d, want 0", status)
	}
	stdout.Reset()
	rt.RunHistoryBuiltin(nil)

	// Then
	if stdout.String() != "" {
		t.Fatalf("after -c history printed %q, want nothing", stdout.String())
	}
}

// An option this build does not implement is refused by name rather than
// ignored, which is the rule the rest of the shell follows.
func TestHistory_refusesAnUnknownOption(t *testing.T) {
	// Given
	rt, stdout := newHistoryRuntime(t)

	// When
	status := rt.RunHistoryBuiltin([]string{"-w"})

	// Then
	if status == 0 {
		t.Fatal("history -w succeeded, want a refusal")
	}
	if stdout.String() != "" {
		t.Fatalf("history -w wrote %q before refusing", stdout.String())
	}
}

// A blank line and an immediate repeat are not recorded, matching the editor
// and every other shell.
func TestHistory_skipsBlanksAndRepeats(t *testing.T) {
	// Given
	rt, stdout := newHistoryRuntime(t)

	// When
	rt.RecordHistory("same")
	rt.RecordHistory("same")
	rt.RecordHistory("   ")
	rt.RecordHistory("")
	rt.RecordHistory("other")
	rt.RunHistoryBuiltin(nil)

	// Then
	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("history printed %q, want two rows", stdout.String())
	}
}

func itoaTest(value int) string {
	if value == 0 {
		return "0"
	}
	var digits []byte
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	return string(digits)
}

func newHistoryRuntime(t *testing.T) (runtime.Runtime, *bytes.Buffer) {
	t.Helper()
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &bytes.Buffer{}})
	return rt, &stdout
}
