package runtime

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// Where the shell found a command is remembered, as busybox's ash remembers it, rather than
// searched for again on every run. With 31 directories on PATH, 1000 runs of `command -v git`
// took 4.6 s here, where busybox took 0.18 s and bash 0.26 s: each run tried every directory
// before git's, seven ways each on Windows.

// statCounter is the stat a test's lookups go through, counted.
type statCounter struct{ calls int }

func (c *statCounter) stat(path string) (os.FileInfo, error) {
	c.calls++
	return os.Stat(path)
}

// hashProbeFile is the program's name on disk: Windows finds `probe` as probe.exe.
func hashProbeFile() string {
	return "probe.exe"
}

func writeHashProbe(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, hashProbeFile())
	if err := os.WriteFile(path, []byte("probe"), 0o755); err != nil {
		t.Fatalf("write probe: %v", err)
	}
	return path
}

// hashRuntime is a shell whose PATH is count directories with the program in the last, and
// whose lookups stat through the counter it returns.
func hashRuntime(t *testing.T, count int) (Runtime, *statCounter, []string, *bytes.Buffer) {
	t.Helper()
	dirs := make([]string, count)
	for index := range dirs {
		dirs[index] = t.TempDir()
	}
	writeHashProbe(t, dirs[count-1])
	var stdout, stderr bytes.Buffer
	rt := NewWithState(applets.DefaultRegistry, Streams{Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr}, State{
		Cwd: WorkingDirectory(t.TempDir()),
		Env: NewEnvironment([]string{"PATH=" + strings.Join(dirs, string(os.PathListSeparator))}),
	})
	counter := &statCounter{}
	rt.commands.stat = counter.stat
	return rt, counter, dirs, &stdout
}

func runHashScript(t *testing.T, rt Runtime, script string) {
	t.Helper()
	rt.RunScript(context.Background(), script+"\n")
}

// Once found, a command costs one stat to run again -- that it is still there -- whichever of
// PATH's directories it is in.
func TestCommandHash_aFoundCommandCostsOneStatAfterward(t *testing.T) {
	rt, counter, _, _ := hashRuntime(t, 5)
	if _, err := rt.externalCommandPath("probe"); err != nil {
		t.Fatalf("first lookup: %v", err)
	}

	// When
	counter.calls = 0
	for range 100 {
		if _, err := rt.externalCommandPath("probe"); err != nil {
			t.Fatalf("lookup: %v", err)
		}
	}

	// Then
	if counter.calls != 100 {
		t.Fatalf("100 lookups of a found command made %d stats, want 100", counter.calls)
	}
}

// A name that is nowhere costs each directory one stat for each way it may be spelt there:
// as written, and on Windows with each executable suffix. Windows also stat'd it as written a
// second time, to tell a file that is no program from no file.
func TestCommandHash_aMissCostsEachDirectoryOneStatPerSpelling(t *testing.T) {
	rt, counter, dirs, _ := hashRuntime(t, 5)

	// When
	_, err := rt.externalCommandPath("nosuch")

	// Then
	if !errors.Is(err, errExternalNotFound) {
		t.Fatalf("lookup of a missing name: %v, want not found", err)
	}
	if want := len(dirs) * (1 + len(ExecutableSuffixes())); counter.calls != want {
		t.Fatalf("a miss made %d stats, want %d", counter.calls, want)
	}
}

// Any assignment to PATH forgets the table, the same value too, as busybox's changepath does,
// and so does `hash -r`; `export PATH` assigns nothing. A PATH given to a program is the
// program's: busybox puts it in the child's environment and looks the program up without the
// table, so the shell's next run is still one stat. Given to a builtin it is assigned for the
// builtin's while, and that forgets the table as any assignment does.
func TestCommandHash_PATHAndHashRForgetAndAProgramsPrefixDoesNot(t *testing.T) {
	separator := string(os.PathListSeparator)
	for _, test := range []struct {
		script  string
		forgets bool
	}{
		{"hash -r", true},
		{"PATH=$PATH", true},
		{"PATH=$PATH" + separator + "/zz", true},
		{"export PATH", false},
		{"PATH=/x probe 2>/dev/null", false},
		{"PATH=/x ls / >/dev/null", false},
		{"PATH=/x command -v probe >/dev/null", true},
	} {
		rt, counter, _, _ := hashRuntime(t, 3)
		if _, err := rt.externalCommandPath("probe"); err != nil {
			t.Fatalf("first lookup: %v", err)
		}
		searched := counter.calls

		// When
		runHashScript(t, rt, test.script)
		counter.calls = 0
		if _, err := rt.externalCommandPath("probe"); err != nil {
			t.Fatalf("%s: lookup after: %v", test.script, err)
		}

		// Then
		want := 1
		if test.forgets {
			want = searched
		}
		if counter.calls != want {
			t.Errorf("after %q a lookup made %d stats, want %d", test.script, counter.calls, want)
		}
	}
}

// A program that has gone since it was found is looked for again, and found where PATH has it
// next, or not found.
func TestCommandHash_aRememberedProgramThatIsGoneIsLookedForAgain(t *testing.T) {
	rt, _, dirs, _ := hashRuntime(t, 3)
	first := writeHashProbe(t, dirs[1])
	later := filepath.Join(dirs[2], hashProbeFile())
	if got, err := rt.externalCommandPath("probe"); err != nil || got != first {
		t.Fatalf("first lookup: %q, %v; want %q", got, err, first)
	}

	// When
	if err := os.Remove(first); err != nil {
		t.Fatal(err)
	}
	got, err := rt.externalCommandPath("probe")

	// Then
	if err != nil || got != later {
		t.Fatalf("after the first was removed: %q, %v; want %q", got, err, later)
	}
	if err := os.Remove(later); err != nil {
		t.Fatal(err)
	}
	if _, err := rt.externalCommandPath("probe"); !errors.Is(err, errExternalNotFound) {
		t.Fatalf("after both were removed: %v, want not found", err)
	}
}

// `hash` lists what was found, a path a line as `command -v` prints it; after a `cd` each
// entry is starred until it is looked up again, as busybox's hashcd marks them.
func TestCommandHash_hashListsWhatWasFoundAndStarsItAfterCd(t *testing.T) {
	rt, _, dirs, stdout := hashRuntime(t, 2)
	path := filepath.ToSlash(filepath.Join(dirs[1], hashProbeFile()))

	// When
	runHashScript(t, rt, "hash; command -v probe >/dev/null; hash; cd ..; hash; command -v probe >/dev/null; hash")

	// Then
	if want := path + "\n" + path + "*\n" + path + "\n"; stdout.String() != want {
		t.Fatalf("hash printed %q, want %q", stdout.String(), want)
	}
}

// A subshell starts with the shell's table and changes only its own, as busybox's forked one
// does: what it finds or forgets is not the shell's.
func TestCommandHash_aSubshellHasTheShellsTableAndKeepsItsOwn(t *testing.T) {
	for _, test := range []struct {
		script string
		listed bool
	}{
		{"command -v probe >/dev/null; (hash -r); hash", true},
		{"command -v probe >/dev/null; echo \"$(hash)\"", true},
		{"(command -v probe >/dev/null); hash", false},
		{"x=$(command -v probe); hash", false},
	} {
		rt, _, dirs, stdout := hashRuntime(t, 2)
		path := filepath.ToSlash(filepath.Join(dirs[1], hashProbeFile()))

		// When
		runHashScript(t, rt, test.script)

		// Then
		if listed := strings.Contains(stdout.String(), path); listed != test.listed {
			t.Errorf("%s printed %q; listed %v, want %v", test.script, stdout.String(), listed, test.listed)
		}
	}
}

// Found in a relative directory of PATH, a command is looked for there again after `cd`: the
// directory is somewhere else now.
func TestCommandHash_aCommandFoundInARelativeDirectoryIsLookedForAgainAfterCd(t *testing.T) {
	here, there, empty := t.TempDir(), t.TempDir(), t.TempDir()
	writeHashProbe(t, here)
	writeHashProbe(t, there)
	var stdout, stderr bytes.Buffer
	rt := NewWithState(applets.DefaultRegistry, Streams{Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr}, State{
		Cwd: WorkingDirectory(here),
		Env: NewEnvironment([]string{"PATH=."}),
	})
	if got, err := rt.externalCommandPath("probe"); err != nil || !sameFile(t, got, filepath.Join(here, hashProbeFile())) {
		t.Fatalf("lookup in the first directory: %q, %v", got, err)
	}

	// When
	runHashScript(t, rt, "cd '"+filepath.ToSlash(there)+"'")
	got, err := rt.externalCommandPath("probe")

	// Then
	if err != nil || !sameFile(t, got, filepath.Join(there, hashProbeFile())) {
		t.Fatalf("after cd: %q, %v; want the one in %s", got, err, there)
	}
	runHashScript(t, rt, "cd '"+filepath.ToSlash(empty)+"'")
	if _, err := rt.externalCommandPath("probe"); !errors.Is(err, errExternalNotFound) {
		t.Fatalf("after cd to a directory without it: %v, want not found", err)
	}
}

func sameFile(t *testing.T, a, b string) bool {
	t.Helper()
	left, err := os.Stat(a)
	if err != nil {
		return false
	}
	right, err := os.Stat(b)
	return err == nil && os.SameFile(left, right)
}

// Names are kept as written, as busybox keeps them: on Windows `PROBE` finds probe.exe and is
// an entry of its own beside `probe`.
func TestCommandHash_namesAreKeptAsWritten(t *testing.T) {
	rt, _, _, stdout := hashRuntime(t, 1)

	// When
	runHashScript(t, rt, "command -v probe >/dev/null; command -v PROBE >/dev/null; hash")

	// Then
	if lines := strings.Count(stdout.String(), "\n"); lines != 2 {
		t.Fatalf("hash printed %q, want two entries", stdout.String())
	}
}

// `which` is an applet and asks PATH itself, as busybox's does: it adds nothing to the table.
func TestCommandHash_whichAddsNothing(t *testing.T) {
	rt, _, _, stdout := hashRuntime(t, 1)

	// When
	runHashScript(t, rt, "which probe >/dev/null; hash")

	// Then
	if stdout.String() != "" {
		t.Fatalf("hash after which printed %q, want nothing", stdout.String())
	}
}
