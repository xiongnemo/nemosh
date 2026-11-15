package runtime

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// `exec` with only redirections changes the shell's own descriptors, and a function, eval,
// `.` and a brace group run in the shell, so what exec does in one is still in force after
// it: busybox-w32 prints the line into the file in every case here. Each command was given
// a copy of the descriptor table, closed when it ended, so the descriptor was gone and the
// write failed with dup2(3,1): Bad file descriptor.
func TestExecRedirection_madeInsideTheShellOutlastsWhatMadeIt(t *testing.T) {
	for _, test := range []struct {
		name  string
		setup string
	}{
		{name: "a function", setup: "f() { exec 3>'%s'; }; f"},
		{name: "eval", setup: "eval \"exec 3>'%s'\""},
		{name: "a brace group", setup: "{ exec 3>'%s'; }"},
		{name: "a dotted file", setup: ". '%s.sh'"},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given
			path := filepath.ToSlash(filepath.Join(t.TempDir(), "fd3.txt"))
			if err := os.WriteFile(path+".sh", []byte("exec 3>'"+path+"'\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			var stdout, stderr bytes.Buffer
			rt := New(applets.DefaultRegistry, Streams{Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr})
			script := strings.ReplaceAll(test.setup, "%s", path) + "; echo kept >&3; exec 3>&-\n"

			// When
			status := rt.RunScript(context.Background(), script)

			// Then
			written, _ := os.ReadFile(path)
			if status != 0 || string(written) != "kept\n" || stderr.Len() != 0 {
				t.Fatalf("status %d, file %q, stderr %q; want 0, %q and nothing", status, written, stderr.String(), "kept\n")
			}
		})
	}
}

// A subshell is a shell of its own, so what exec does there ends with it, in both references.
func TestExecRedirection_madeInASubshellEndsWithIt(t *testing.T) {
	// Given
	path := filepath.ToSlash(filepath.Join(t.TempDir(), "fd3.txt"))
	var stdout, stderr bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr})

	// When
	status := rt.RunScript(context.Background(), "( exec 3>'"+path+"' ); echo lost >&3\n")

	// Then
	if status == 0 || !strings.Contains(stderr.String(), "Bad file descriptor") {
		t.Fatalf("status %d, stderr %q; the subshell's descriptor outlived it", status, stderr.String())
	}
}
