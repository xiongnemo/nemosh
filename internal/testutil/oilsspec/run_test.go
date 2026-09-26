package oilsspec_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/testutil/oilsspec"
)

// TestMain lets this test binary be the shell a case runs, when the harness's environment
// says so, so that how a case is run can be tested in milliseconds rather than by
// building one.
func TestMain(m *testing.M) {
	if os.Getenv("OILSSPEC_FAKE_SHELL") != "" {
		os.Exit(fakeShell())
	}
	os.Exit(m.Run())
}

// fakeShell prints what a case sees -- its name, directory, stdin and the variables the
// harness gives it -- then does as its stdin's first word says: exit with a status, run
// past any timeout, or leave a process behind, one that would run for a minute, writing
// its pid to the file named.
func fakeShell() int {
	if os.Getenv("OILSSPEC_FAKE_LINGER") != "" {
		time.Sleep(time.Minute)
		return 0
	}
	code, _ := io.ReadAll(os.Stdin)
	wd, _ := os.Getwd()
	fmt.Printf("argv0=%s\ndir=%s\nstdin=%q\n", os.Args[0], filepath.ToSlash(wd), code)
	for _, name := range []string{"SH", "TMP", "HOME", "REPO_ROOT", "LC_ALL", "PATH", "USERPROFILE"} {
		value, set := os.LookupEnv(name)
		fmt.Printf("%s=%s %v\n", name, value, set)
	}
	words := strings.Fields(string(code))
	switch {
	case len(words) == 2 && words[0] == "exit":
		status, _ := strconv.Atoi(words[1])
		return status
	case len(words) == 1 && words[0] == "hang":
		time.Sleep(time.Minute)
	case len(words) == 2 && words[0] == "leave":
		self, _ := os.Executable()
		child := exec.Command(self)
		child.Env = []string{"OILSSPEC_FAKE_SHELL=1", "OILSSPEC_FAKE_LINGER=1"}
		if err := child.Start(); err != nil {
			return 1
		}
		if err := os.WriteFile(words[1], []byte(strconv.Itoa(child.Process.Pid)), 0o644); err != nil {
			return 1
		}
	}
	return 0
}

func fakeSubject(t *testing.T) oilsspec.Subject {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return oilsspec.Subject{
		Label: "bash", Program: self, Name: "bash",
		Path: []string{"helpers", "shell"},
		Env:  []string{"OILSSPEC_FAKE_SHELL=1"},
	}
}

// A case runs as sh_spec.py runs it: its code on stdin, in its own directory, under the
// name $SH gives, with an environment made for it -- TMP and HOME the case's directory --
// and nothing of this process's passed on, such as USERPROFILE.
func TestRunCase_givesTheCaseItsOwnDirectoryAndEnvironment(t *testing.T) {
	// Resolved, since macOS's temporary directory is under /var, a link to /private/var,
	// and the shell reports where it is without the link.
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	run, err := fakeSubject(t).RunCase(context.Background(), "exit 3", dir, "/repo")

	if err != nil {
		t.Fatal(err)
	}
	slashed := filepath.ToSlash(dir)
	want := []string{
		"argv0=bash\n", "dir=" + slashed + "\n", `stdin="exit 3"` + "\n",
		"SH=bash true\n", "TMP=" + slashed + " true\n", "HOME=" + slashed + " true\n",
		"REPO_ROOT=/repo true\n", "LC_ALL=C.UTF-8 true\n",
		"PATH=helpers" + string(os.PathListSeparator) + "shell true\n", "USERPROFILE= false\n",
	}
	for _, line := range want {
		if !strings.Contains(run.Stdout, line) {
			t.Errorf("the case saw\n%s\nwithout %q", run.Stdout, line)
		}
	}
	if run.Status != 3 || run.TimedOut {
		t.Errorf("status %d, timed out %v; want 3, false", run.Status, run.TimedOut)
	}
}

// A case that runs out of time is ended and says so.
func TestRunCase_endsACaseThatRunsOutOfTime(t *testing.T) {
	subject := fakeSubject(t)
	subject.Timeout = 500 * time.Millisecond
	start := time.Now()

	run, err := subject.RunCase(context.Background(), "hang", t.TempDir(), "/repo")

	if err != nil {
		t.Fatal(err)
	}
	if !run.TimedOut || time.Since(start) > 10*time.Second {
		t.Errorf("timed out %v after %v; want a timeout in about half a second", run.TimedOut, time.Since(start))
	}
}

// What a case leaves running is ended with it, so it cannot write into the next case or
// outlive the suite.
func TestRunCase_endsWhatTheCaseLeftRunning(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "pid")

	if _, err := fakeSubject(t).RunCase(context.Background(), "leave "+pidFile, t.TempDir(), "/repo"); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(string(data))
	if err != nil {
		t.Fatal(err)
	}
	if !processEnds(pid, 2*time.Second) {
		t.Fatalf("process %d, which the case left behind, was still running after the case", pid)
	}
}

// $REPO_ROOT is a tree with the Oils checkout's names in it, empty, and the vendored
// spec/testdata and spec/bin whole, with their modes, and the _tmp/spec-tmp cases write to.
func TestRepoRoot_isTheOilsSkeletonWithTheTestdataInIt(t *testing.T) {
	record := readUpstream(t)
	dir := t.TempDir()

	if err := record.RepoRoot(root, dir); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"client", "core", "cpp", "bin", "spec/testdata/bind", "_tmp/spec-tmp"} {
		if info, err := os.Stat(filepath.Join(dir, filepath.FromSlash(name))); err != nil || !info.IsDir() {
			t.Errorf("%s is not a directory: %v", name, err)
		}
	}
	for _, name := range []string{"oils-version.txt", "spec/alias.test.sh", "spec/toysh.test.sh"} {
		if info, err := os.Stat(filepath.Join(dir, filepath.FromSlash(name))); err != nil || info.Size() != 0 {
			t.Errorf("%s is not an empty file: %v", name, err)
		}
	}
	for _, name := range []string{"spec/testdata/echo.sh", "spec/bin/builtins-exec-here-doc-helper.sh"} {
		got, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name)))
		want, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil || string(got) != string(want) || len(want) == 0 {
			t.Errorf("%s is not the vendored file: %v", name, err)
		}
		info, err := os.Stat(filepath.Join(dir, filepath.FromSlash(name)))
		if runtime.GOOS != "windows" && (err != nil || info.Mode()&0o111 == 0) {
			t.Errorf("%s is not executable", name)
		}
	}
}
