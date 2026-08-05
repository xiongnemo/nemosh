package runtime_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

func TestRuntime_appliesOutputAndStderrDuplicationLeftToRight(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("executable: %v", err)
	}
	t.Setenv("NEMOSH_REDIRECT_HELPER_PROCESS", "1")
	tmp := t.TempDir()
	first := filepath.ToSlash(filepath.Join(tmp, "first.txt"))
	second := filepath.ToSlash(filepath.Join(tmp, "second.txt"))
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})
	probe := filepath.ToSlash(exe) + " -test.run=TestRedirectHelperProcess -- "

	status := rt.RunScript(context.Background(), probe+"first >"+first+" 2>&1\n"+probe+"second 2>&1 >"+second+"\n")

	if status != 0 {
		t.Fatalf("status: %d", status)
	}
	assertFileText(t, first, "first\n")
	assertFileText(t, second, "")
	if got, want := stdout.String(), "second\n"; got != want {
		t.Fatalf("stdout: got %q want %q", got, want)
	}
}

func TestRuntime_numberedDupPreservesOriginalStdout(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("executable: %v", err)
	}
	t.Setenv("NEMOSH_REDIRECT_HELPER_PROCESS", "1")
	out := filepath.ToSlash(filepath.Join(t.TempDir(), "out.txt"))
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})
	command := filepath.ToSlash(exe) + " -test.run=TestRedirectHelperProcess -- original 3>&1 1>" + out + " 2>&3\n"

	status := rt.RunScript(context.Background(), command)

	if status != 0 {
		t.Fatalf("status: %d", status)
	}
	assertFileText(t, out, "")
	if got, want := stdout.String(), "original\n"; got != want {
		t.Fatalf("stdout: got %q want %q", got, want)
	}
}

func TestRuntime_dupFailurePreventsCommandExecution(t *testing.T) {
	out := filepath.ToSlash(filepath.Join(t.TempDir(), "probe.txt"))
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{})

	status := rt.RunScript(context.Background(), "echo ran 9>&8 >"+out+"\n")

	if status == 0 {
		t.Fatal("expected redirect failure")
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("probe command executed or file created: %v", err)
	}
}

func TestRuntime_dupCapturesDescriptionBeforeSourceRebind(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("executable: %v", err)
	}
	t.Setenv("NEMOSH_REDIRECT_HELPER_PROCESS", "1")
	tmp := t.TempDir()
	first := filepath.ToSlash(filepath.Join(tmp, "first.txt"))
	second := filepath.ToSlash(filepath.Join(tmp, "second.txt"))
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{})
	command := filepath.ToSlash(exe) + " -test.run=TestRedirectHelperProcess -- alias 3>" + first + " 4>&3 3>" + second + " 2>&4\n"

	status := rt.RunScript(context.Background(), command)

	if status != 0 {
		t.Fatalf("status: %d", status)
	}
	assertFileText(t, first, "alias\n")
	assertFileText(t, second, "")
}

func TestRuntime_closeAndReopenFollowLexicalOrder(t *testing.T) {
	tmp := t.TempDir()
	reopened := filepath.ToSlash(filepath.Join(tmp, "reopened.txt"))
	closed := filepath.ToSlash(filepath.Join(tmp, "closed.txt"))
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{})

	firstStatus := rt.RunScript(context.Background(), "echo ok 3>&- 3>"+reopened+" 1>&3\n")
	secondStatus := rt.RunScript(context.Background(), "echo blocked 3>"+closed+" 3>&- 1>&3\n")

	if firstStatus != 0 {
		t.Fatalf("close then reopen status: %d", firstStatus)
	}
	if secondStatus == 0 {
		t.Fatal("expected reopen then close failure")
	}
	assertFileText(t, reopened, "ok\n")
	assertFileText(t, closed, "")
}

func TestRuntime_commandRedirectOverridesPipelineEndpointWithoutParentLeak(t *testing.T) {
	out := filepath.ToSlash(filepath.Join(t.TempDir(), "pipeline.txt"))
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	status := rt.RunScript(context.Background(), "echo file >"+out+" | cat\necho parent\n")

	if status != 0 {
		t.Fatalf("status: %d", status)
	}
	assertFileText(t, out, "file\n")
	if got, want := stdout.String(), "parent\n"; got != want {
		t.Fatalf("stdout: got %q want %q", got, want)
	}
}

func TestRuntime_partialOpenFailureReleasesEarlierOwnedFile(t *testing.T) {
	tmp := t.TempDir()
	first := filepath.Join(tmp, "first.txt")
	missing := filepath.ToSlash(filepath.Join(tmp, "missing", "second.txt"))
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{})

	status := rt.RunScript(context.Background(), "echo blocked 3>"+filepath.ToSlash(first)+" 4>"+missing+"\n")

	if status == 0 {
		t.Fatal("expected later open failure")
	}
	renamed := filepath.Join(tmp, "renamed.txt")
	if err := os.Rename(first, renamed); err != nil {
		t.Fatalf("earlier owned file remained open: %v", err)
	}
	assertFileText(t, renamed, "")
}

func assertFileText(t *testing.T, path, want string) {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if got := string(contents); got != want {
		t.Fatalf("file %s: got %q want %q", path, got, want)
	}
}
