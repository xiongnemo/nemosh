package runtime_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

func TestRuntime_executesAppletCommand_whenScriptContainsEcho(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "echo hi\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "hi\n" {
		t.Fatalf("expected stdout %q, got %q", "hi\n", got)
	}
}

func TestRuntime_changesDirectory_whenScriptContainsCdAndPwd(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "cd /\npwd\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if stdout.String() == "" {
		t.Fatal("expected pwd output")
	}
}

func TestRuntime_stopsWithStatus_whenScriptExits(t *testing.T) {
	// Given
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{})

	// When
	status := rt.RunScript(context.Background(), "exit 7\necho unreachable\n")

	// Then
	if status != 7 {
		t.Fatalf("expected status 7, got %d", status)
	}
}

func TestRuntime_expandsVariable_whenAssignmentPrecedesEcho(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "name=nemo\necho $name\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "nemo\n" {
		t.Fatalf("expected stdout %q, got %q", "nemo\n", got)
	}
}

func TestRuntime_runsAndOrLists_withShortCircuit(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "false && echo bad\ntrue || echo bad\necho ok\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "ok\n" {
		t.Fatalf("expected stdout %q, got %q", "ok\n", got)
	}
}

func TestRuntime_redirectsStdout_whenCommandUsesOutputRedirection(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	tmpDir := t.TempDir()
	outputPath := filepath.ToSlash(filepath.Join(tmpDir, "out.txt"))
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "echo hi > "+outputPath+"\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if stdout.String() != "" {
		t.Fatalf("expected stdout to be empty, got %q", stdout.String())
	}
	contents, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("expected redirected output file, got %v", err)
	}
	if string(contents) != "hi\n" {
		t.Fatalf("expected redirected contents %q, got %q", "hi\n", string(contents))
	}
}

func TestRuntime_redirectsStdin_whenCommandUsesInputRedirection(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	tmpDir := t.TempDir()
	inputPath := filepath.ToSlash(filepath.Join(tmpDir, "in.txt"))
	if err := os.WriteFile(inputPath, []byte("from-file\n"), 0o600); err != nil {
		t.Fatalf("expected fixture write to succeed, got %v", err)
	}
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "cat < "+inputPath+"\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "from-file\n" {
		t.Fatalf("expected stdout %q, got %q", "from-file\n", got)
	}
}

func TestRuntime_pipesStdoutIntoNextCommand_whenLineUsesPipe(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "echo hi | cat\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "hi\n" {
		t.Fatalf("expected stdout %q, got %q", "hi\n", got)
	}
}

func TestRuntime_returnsLastPipelineStatus_whenPipelineContainsFailure(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "echo hi | false\n")

	// Then
	if status != 1 {
		t.Fatalf("expected status 1, got %d", status)
	}
	if stdout.String() != "" {
		t.Fatalf("expected stdout to be empty, got %q", stdout.String())
	}
}

func TestRuntime_exportsVariable_whenExportUsesAssignment(t *testing.T) {
	// Given
	name := "NEMOSH_TEST_EXPORT_RUNTIME"
	t.Setenv(name, "")
	if err := os.Unsetenv(name); err != nil {
		t.Fatalf("expected env cleanup to succeed, got %v", err)
	}
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "export "+name+"=ok\nprintenv "+name+"\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "ok\n" {
		t.Fatalf("expected exported value %q, got %q", "ok\n", got)
	}
}

func TestRuntime_unsetsVariable_whenUnsetRuns(t *testing.T) {
	// Given
	name := "NEMOSH_TEST_UNSET_RUNTIME"
	t.Setenv(name, "old")
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "name=local\nunset name "+name+"\necho $name\nprintenv "+name+"\n")

	// Then
	if status != 1 {
		t.Fatalf("expected final printenv status 1, got %d", status)
	}
	if got := stdout.String(); got != "\n" {
		t.Fatalf("expected only empty echo output, got %q", got)
	}
}

func TestRuntime_readsLineIntoVariable_whenReadRuns(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{
		Stdin:  bytes.NewBufferString("from stdin\n"),
		Stdout: &stdout,
	})

	// When
	status := rt.RunScript(context.Background(), "read value\necho $value\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "from stdin\n" {
		t.Fatalf("expected read value output %q, got %q", "from stdin\n", got)
	}
}

func TestRuntime_returnsFailure_whenReadHitsEOF(t *testing.T) {
	// Given
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdin: bytes.NewReader(nil)})

	// When
	status := rt.RunScript(context.Background(), "read value\n")

	// Then
	if status != 1 {
		t.Fatalf("expected EOF read status 1, got %d", status)
	}
}

func TestRuntime_readsConsecutiveLines_whenReadRunsTwice(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{
		Stdin:  bytes.NewBufferString("first\nsecond\n"),
		Stdout: &stdout,
	})

	// When
	status := rt.RunScript(context.Background(), "read one\nread two\necho $one-$two\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "first-second\n" {
		t.Fatalf("expected two read values %q, got %q", "first-second\n", got)
	}
}

func TestRuntime_expandsCommandSubstitution_whenWordContainsDollarParen(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "echo $(echo hi)\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "hi\n" {
		t.Fatalf("expected command substitution output %q, got %q", "hi\n", got)
	}
}

func TestRuntime_trimsTrailingNewlines_whenCommandSubstitutionExpands(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "echo $(printf 'hi\\n\\n')x\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "hix\n" {
		t.Fatalf("expected trimmed command substitution output %q, got %q", "hix\n", got)
	}
}

func TestRuntime_executesExternalCommand_whenCommandIsNotBuiltinOrApplet(t *testing.T) {
	// Given
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("expected test executable path, got %v", err)
	}
	t.Setenv("NEMOSH_RUNTIME_HELPER_PROCESS", "1")
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), filepath.ToSlash(exe)+" -test.run=TestRuntimeHelperProcess -- external-ok\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "external-ok\n" {
		t.Fatalf("expected external stdout %q, got %q", "external-ok\n", got)
	}
}

func TestRuntime_returnsExternalCommandStatus_whenCommandExitsNonZero(t *testing.T) {
	// Given
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("expected test executable path, got %v", err)
	}
	t.Setenv("NEMOSH_RUNTIME_HELPER_PROCESS", "1")
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{})

	// When
	status := rt.RunScript(context.Background(), filepath.ToSlash(exe)+" -test.run=TestRuntimeHelperProcess -- exit-7\n")

	// Then
	if status != 7 {
		t.Fatalf("expected status 7, got %d", status)
	}
}

func TestRuntimeHelperProcess(t *testing.T) {
	if os.Getenv("NEMOSH_RUNTIME_HELPER_PROCESS") != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg != "--" {
			continue
		}
		if os.Args[i+1] == "exit-7" {
			os.Exit(7)
		}
		fmt.Fprintln(os.Stdout, os.Args[i+1])
		os.Exit(0)
	}
	os.Exit(2)
}
