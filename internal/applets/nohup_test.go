package applets

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// nohupRunner is a ProgramRunner that has one program, which writes what it is told to its
// two streams and keeps what it was asked and given; nothing is launched.
type nohupRunner struct {
	name          string
	stdout        string
	stderr        string
	asked         *string
	args          *[]string
	detached      *bool
	stdinFileName *string
}

func (r nohupRunner) Program(ctx context.Context, name string) (Applet, bool) {
	*r.asked = name
	if name != r.name {
		return nil, false
	}
	return simpleApplet{name: name, runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
		*r.args, *r.detached = args, DetachedLaunch(ctx)
		if file, ok := stdin.(*os.File); ok {
			*r.stdinFileName = file.Name()
		}
		if _, err := io.WriteString(stdout, r.stdout); err != nil {
			return err
		}
		_, err := io.WriteString(stderr, r.stderr)
		return err
	}}, true
}

type nohupRun struct {
	asked, stdinFileName string
	args                 []string
	detached             bool
	stdout, stderr       bytes.Buffer
	err                  error
}

// runNohupWith runs nohup in dir with HOME at home, its streams terminals or not as given.
func runNohupWith(t *testing.T, dir, home string, terminals [3]bool, runner nohupRunner, args ...string) *nohupRun {
	t.Helper()
	run := &nohupRun{}
	runner.asked, runner.args, runner.detached, runner.stdinFileName = &run.asked, &run.args, &run.detached, &run.stdinFileName
	saved := nohupTerminals
	nohupTerminals = func(context.Context, io.Reader, io.Writer, io.Writer) [3]bool { return terminals }
	t.Cleanup(func() { nohupTerminals = saved })
	ctx := WithProgramRunner(WithProcessView(context.Background(), hostProcessView{cwd: dir, environ: []string{"HOME=" + home}}), runner)
	run.err = newNohupApplet().Run(ctx, args, strings.NewReader(""), &run.stdout, &run.stderr)
	return run
}

func TestNohup_runsTheCommandOutOfTheSession(t *testing.T) {
	// Given
	runner := nohupRunner{name: "prog", stdout: "out\n", stderr: "err\n"}

	// When
	run := runNohupWith(t, t.TempDir(), t.TempDir(), [3]bool{}, runner, "prog", "a", "b c")

	// Then
	if run.err != nil {
		t.Fatalf("nohup prog: %v", run.err)
	}
	if !run.detached {
		t.Fatal("the program was not asked to run out of the session")
	}
	if strings.Join(run.args, "|") != "a|b c" {
		t.Fatalf("args = %q, want [a, b c]", run.args)
	}
	if run.stdout.String() != "out\n" || run.stderr.String() != "err\n" {
		t.Fatalf("stdout %q, stderr %q: streams that are no terminal are the program's", run.stdout.String(), run.stderr.String())
	}
}

func TestNohup_appendsATerminalsOutputToNohupOut(t *testing.T) {
	// Given
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "nohup.out"), []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := nohupRunner{name: "prog", stdout: "out\n", stderr: "err\n"}

	// When
	run := runNohupWith(t, dir, t.TempDir(), [3]bool{true, true, true}, runner, "prog")

	// Then
	if run.err != nil {
		t.Fatalf("nohup prog: %v", run.err)
	}
	written, err := os.ReadFile(filepath.Join(dir, "nohup.out"))
	if err != nil {
		t.Fatal(err)
	}
	if string(written) != "old\nout\nerr\n" {
		t.Fatalf("nohup.out = %q, want the old text, then stdout and stderr", written)
	}
	if run.stdout.String() != "" || run.stderr.String() != "nohup: appending output to nohup.out\n" {
		t.Fatalf("stdout %q, stderr %q", run.stdout.String(), run.stderr.String())
	}
	if !strings.EqualFold(run.stdinFileName, os.DevNull) {
		t.Fatalf("stdin was %q, want %s in place of the terminal", run.stdinFileName, os.DevNull)
	}
}

func TestNohup_putsNohupOutInHomeWhenTheDirectoryRefusesIt(t *testing.T) {
	// Given
	home := t.TempDir()
	gone := filepath.Join(t.TempDir(), "gone")
	runner := nohupRunner{name: "prog", stdout: "out\n"}

	// When
	run := runNohupWith(t, gone, filepath.ToSlash(home), [3]bool{false, true, false}, runner, "prog")

	// Then
	if run.err != nil {
		t.Fatalf("nohup prog: %v", run.err)
	}
	written, err := os.ReadFile(filepath.Join(home, "nohup.out"))
	if err != nil || string(written) != "out\n" {
		t.Fatalf("$HOME/nohup.out = %q, %v", written, err)
	}
	want := "nohup: appending output to " + filepath.ToSlash(home) + "/nohup.out\n"
	if run.stderr.String() != want {
		t.Fatalf("stderr = %q, want %q", run.stderr.String(), want)
	}
}

func TestNohup_runsAnAppletAsAProcessOfItsOwn(t *testing.T) {
	// Given
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	runner := nohupRunner{name: self}

	// When
	run := runNohupWith(t, t.TempDir(), t.TempDir(), [3]bool{}, runner, "sleep", "5")

	// Then
	if run.err != nil {
		t.Fatalf("nohup sleep: %v", run.err)
	}
	if run.asked != self || strings.Join(run.args, " ") != "sleep 5" || !run.detached {
		t.Fatalf("asked %q for %q, detached %v; want this program for the applet, out of the session", run.asked, run.args, run.detached)
	}
}

func TestNohup_saysWhatItCannotRun(t *testing.T) {
	for _, args := range [][]string{{"nosuchprog"}, {}} {
		// When
		run := runNohupWith(t, t.TempDir(), t.TempDir(), [3]bool{}, nohupRunner{name: "prog"}, args...)

		// Then: 127, busybox's status for both.
		if status, ok := StatusCode(run.err); !ok || status != 127 {
			t.Fatalf("nohup %q: %v, want status 127", args, run.err)
		}
	}
}
