package runtime_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

func runWithArguments(t *testing.T, name string, args []string, script string) string {
	t.Helper()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})
	rt.SetArguments(name, args)
	if status := rt.RunScript(context.Background(), script); status != 0 {
		t.Fatalf("status = %d, stderr = %q", status, stderr.String())
	}
	return stdout.String()
}

func TestRuntime_scriptArgumentsSeedTheNameAndPositionals(t *testing.T) {
	got := runWithArguments(t, "hello.sh", []string{"a", "b"}, "echo $0 $1 $2 $#\n")

	if want := "hello.sh a b 2\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

// set -- replaces the positional parameters; $0 is not one of them, so a script
// still knows its own name afterwards.
func TestRuntime_setDashDashLeavesTheScriptNameAlone(t *testing.T) {
	got := runWithArguments(t, "hello.sh", []string{"a", "b"}, "set -- x\necho $0 $1 $#\n")

	if want := "hello.sh x 1\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

// A function call rebinds $1... but not $0, matching dash and busybox ash.
func TestRuntime_functionCallsLeaveTheScriptNameAlone(t *testing.T) {
	script := "report() {\n  echo $0 $1\n}\nreport inner\necho $0 $1\n"

	got := runWithArguments(t, "hello.sh", []string{"outer"}, script)

	if want := "hello.sh inner\nhello.sh outer\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestRuntime_commandSubstitutionInheritsTheScriptName(t *testing.T) {
	got := runWithArguments(t, "hello.sh", nil, "echo $(echo $0)\n")

	if want := "hello.sh\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestRuntime_scriptNameIsAvailableAsADefaultWord(t *testing.T) {
	got := runWithArguments(t, "hello.sh", nil, "echo ${MISSING:-$0}\n")

	if want := "hello.sh\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}
