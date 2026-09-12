package applets_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// **An applet never spawns an OS process.** Launching one belongs to the shell runtime,
// which owns the Windows machinery it needs -- suffix search, `ComSpec` for batch files,
// argument quoting, the long-path fallback, context cancellation, and the job scope a
// background child is registered in. All of that lives in
// internal/shell/runtime/external*.go and none of it is duplicated here.
//
// It was true before this test existed and written down nowhere. `xargs` draws the line
// by dispatching only to registered applets (xargs.go:139 -- `DefaultRegistry.Lookup`,
// then `commandNotFound`), but that reads as one applet's choice rather than as the rule
// it actually is, and the rule had to be rediscovered by reading that line. A boundary
// nobody states is one somebody crosses: the obvious way to give `awk` its `system()` and
// its `cmd | getline` is an `exec.Command` right here, and it would work, and it would
// quietly put a second process launcher in the tree beside the one that already handles
// every hard case.
//
// So the rule is a test rather than a comment. An applet that needs to run something
// looks it up in the registry and refuses otherwise, which is what `awk` does.
//
// Tests are exempt, and pgrep_test.go uses that exemption for a good reason: it starts a
// process so that `pgrep` has something real to find.

func TestApplets_doNotSpawnProcesses(t *testing.T) {
	// `os/exec` is the import that means "spawn", and the only one.
	//
	// The first version of this test also banned `syscall` and `golang.org/x/sys/windows`
	// on the theory that they were the way around it. The test immediately reported eight
	// files and was right to: this package uses both constantly for *file metadata and
	// process inspection* -- file_details_windows.go, id_windows.go, process_view.go --
	// which is reading, not launching. Banning the packages would have banned the wrong
	// thing and taught the next person to work around the guard. The calls that actually
	// start a process are named separately below.
	forbidden := map[string]string{
		"os/exec": "launching a process belongs to internal/shell/runtime; look the command up in DefaultRegistry and refuse otherwise, as xargs.go does",
	}
	// And the routes that do not need that import. Matched as text against the source,
	// because an applet reaching for one of these is crossing the same boundary by a
	// longer road, and there is no legitimate use of any of them here.
	spawnCalls := []string{"os.StartProcess", "syscall.CreateProcess", "windows.CreateProcess"}

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		source, err := os.ReadFile(filepath.Join(".", name))
		if err != nil {
			t.Fatal(err)
		}
		for _, call := range spawnCalls {
			if strings.Contains(string(source), call) {
				t.Errorf("%s calls %s -- launching a process belongs to internal/shell/runtime", name, call)
			}
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(".", name), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		checked++
		for _, imported := range file.Imports {
			path, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			if because, banned := forbidden[path]; banned {
				t.Errorf("%s imports %q -- %s", name, path, because)
			}
		}
	}
	// A guard on the guard: if the walk ever stops finding files, this test would pass
	// by checking nothing at all, which is the quiet way a rule stops being enforced.
	if checked < 50 {
		t.Fatalf("only %d production files were checked; the walk is not finding the package", checked)
	}
}
