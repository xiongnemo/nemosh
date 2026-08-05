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

func TestRuntime_braceGroup_persistsStateAndPropagatesStatus(t *testing.T) {
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})
	status := rt.RunScript(context.Background(), "{ value=grouped; false; }\necho $value\n")
	if status != 0 || stdout.String() != "grouped\n" {
		t.Fatalf("status = %d, stdout = %q", status, stdout.String())
	}
}

func TestRuntime_subshell_isolatesStateAndExit(t *testing.T) {
	var stdout bytes.Buffer
	temp := filepath.ToSlash(t.TempDir())
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})
	source := "value=parent\nset -- parent\npwd\n(value=child; cd " + temp + "; set -- child; set -o pipefail; exit 7)\necho $value $1\npwd\nfalse | true\n"
	status := rt.RunScript(context.Background(), source)
	if status != 0 {
		t.Fatalf("status = %d, want 0", status)
	}
	lines := bytes.Split(stdout.Bytes(), []byte("\n"))
	if len(lines) != 4 || string(lines[1]) != "parent parent" || !bytes.Equal(lines[0], lines[2]) {
		t.Fatalf("stdout = %q, want matching cwd around parent state", stdout.String())
	}
}

func TestRuntime_compoundRedirect_wrapsWholeBodyAndRestoresParentFDs(t *testing.T) {
	var stdout bytes.Buffer
	output := filepath.ToSlash(filepath.Join(t.TempDir(), "group.txt"))
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})
	status := rt.RunScript(context.Background(), "{ echo one; echo two; } >"+output+"\necho parent\n")
	contents, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if status != 0 || string(contents) != "one\ntwo\n" || stdout.String() != "parent\n" {
		t.Fatalf("status = %d, file = %q, stdout = %q", status, contents, stdout.String())
	}
}

func TestRuntime_groupsInPipelines_isolateStateAndHonorPipefail(t *testing.T) {
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})
	source := "value=parent\n{ value=group; echo data; } | (cat; false)\necho $value\nset -o pipefail\n(true) | (exit 7) | true\n"
	status := rt.RunScript(context.Background(), source)
	if status != 7 || stdout.String() != "data\nparent\n" {
		t.Fatalf("status = %d, stdout = %q", status, stdout.String())
	}
}

func TestRuntime_nestedSubshell_returnsBodyStatus(t *testing.T) {
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{})
	if status := rt.RunScript(context.Background(), "((false))\n"); status != 1 {
		t.Fatalf("status = %d, want 1", status)
	}
}

func TestRuntime_braceGroup_preservesQuotedSemicolonAsWordContent(t *testing.T) {
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})
	if status := rt.RunScript(context.Background(), "{ echo 'left;right'; }\n"); status != 0 {
		t.Fatalf("status = %d, want 0", status)
	}
	if stdout.String() != "left;right\n" {
		t.Fatalf("stdout = %q, want quoted semicolon", stdout.String())
	}
}
