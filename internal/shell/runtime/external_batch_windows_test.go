//go:build windows

package runtime_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

func writeBatchFile(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return filepath.ToSlash(path)
}

// reportArgumentsBatch echoes its first two arguments through delayed expansion
// so that a metacharacter inside an argument is printed rather than re-parsed by
// the script itself. What this measures is the launch boundary, not cmd's
// well-known habit of substituting %1 textually.
const reportArgumentsBatch = "@echo off\r\n" +
	"setlocal enabledelayedexpansion\r\n" +
	"set \"one=%~1\"\r\n" +
	"set \"two=%~2\"\r\n" +
	"echo 1=[!one!]\r\n" +
	"echo 2=[!two!]\r\n"

func runBatchScript(t *testing.T, line string) (string, string, int) {
	t.Helper()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})
	status := rt.RunScript(context.Background(), line)
	return stdout.String(), stderr.String(), status
}

func TestRuntime_batchArgumentsCrossTheBoundaryWithoutSplittingTheCommandLine(t *testing.T) {
	script := writeBatchFile(t, "report.bat", reportArgumentsBatch)

	stdout, stderr, status := runBatchScript(t, "'"+script+"' 'a&b' 'x'\n")

	if status != 0 {
		t.Fatalf("status = %d, stderr = %q", status, stderr)
	}
	if want := "1=[a&b]\r\n2=[x]\r\n"; stdout != want {
		t.Fatalf("stdout = %q, want %q (stderr = %q)", stdout, want, stderr)
	}
}

func TestRuntime_batchArgumentsKeepSpacesAndLonePercentSigns(t *testing.T) {
	script := writeBatchFile(t, "report.cmd", reportArgumentsBatch)

	stdout, stderr, status := runBatchScript(t, "'"+script+"' 'two words' '50%'\n")

	if status != 0 {
		t.Fatalf("status = %d, stderr = %q", status, stderr)
	}
	if want := "1=[two words]\r\n2=[50%]\r\n"; stdout != want {
		t.Fatalf("stdout = %q, want %q (stderr = %q)", stdout, want, stderr)
	}
}

func TestRuntime_batchExitCodeBecomesTheCommandStatus(t *testing.T) {
	script := writeBatchFile(t, "fail.cmd", "@echo off\r\nexit /b 3\r\n")

	_, stderr, status := runBatchScript(t, "'"+script+"'\n")

	if status != 3 {
		t.Fatalf("status = %d, want 3, stderr = %q", status, stderr)
	}
}

func TestRuntime_batchVariableReferenceArgumentsFailBeforeLaunch(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "ran.txt")
	script := writeBatchFile(t, "mark.bat", "@echo off\r\ntype nul >\""+marker+"\"\r\n")

	_, stderr, status := runBatchScript(t, "'"+script+"' '%PATH%'\n")

	if status != 126 {
		t.Fatalf("status = %d, want 126, stderr = %q", status, stderr)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("batch file ran despite the rejected argument: %v", err)
	}
	if !bytes.Contains([]byte(stderr), []byte("%PATH%")) {
		t.Fatalf("stderr = %q, want it to name the rejected operand", stderr)
	}
}

// reportRawArgumentsBatch prints %1 without the ~ that strips quotes. The
// existing coverage all used %~1, which hid the defect this pins: every
// operand was quoted on the way in, so %1 was `"release"` and the commonest
// batch idiom there is -- if "%1"=="release" -- compared ""release"" against
// "release" and never matched.
const reportRawArgumentsBatch = "@echo off\r\n" +
	"echo raw=[%1]\r\n" +
	"if \"%1\"==\"release\" echo matched\r\n"

func TestRuntime_batchArgumentsArriveUnquoted_whenTheyNeedNoQuoting(t *testing.T) {
	script := writeBatchFile(t, "raw.bat", reportRawArgumentsBatch)

	stdout, stderr, status := runBatchScript(t, "'"+script+"' release\n")

	if status != 0 {
		t.Fatalf("status = %d, stderr = %q", status, stderr)
	}
	if !strings.Contains(stdout, "raw=[release]") {
		t.Fatalf("stdout = %q, want %%1 to arrive as release rather than \"release\"", stdout)
	}
	if !strings.Contains(stdout, "matched") {
		t.Fatalf("stdout = %q, want the if \"%%1\"==\"release\" comparison to match", stdout)
	}
}

// echoRawArgumentBatch has no `if` in it, because a quoted operand carrying a
// space would break the comparison line itself -- cmd's problem with its own
// syntax, not the launch boundary's.
const echoRawArgumentBatch = "@echo off\r\necho raw=[%1]\r\n"

func TestRuntime_batchArgumentsStayQuoted_whenTheyContainASpace(t *testing.T) {
	script := writeBatchFile(t, "spaced.bat", echoRawArgumentBatch)

	stdout, stderr, status := runBatchScript(t, "'"+script+"' 'two words'\n")

	if status != 0 {
		t.Fatalf("status = %d, stderr = %q", status, stderr)
	}
	if !strings.Contains(stdout, `raw=["two words"]`) {
		t.Fatalf("stdout = %q, want the spaced operand to arrive as one quoted field", stdout)
	}
}
