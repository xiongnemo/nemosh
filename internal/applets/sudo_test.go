package applets

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"
)

// sudoRunner is a ProgramRunner whose programs are those named, each keeping what it was asked;
// nothing is launched.
type sudoRunner struct {
	programs []string
	asked    *string
	args     *[]string
}

func (r sudoRunner) Program(ctx context.Context, name string) (Applet, bool) {
	for _, program := range r.programs {
		if program == name {
			return simpleApplet{name: name, runContext: func(_ context.Context, args []string, _ io.Reader, _, _ io.Writer) error {
				*r.asked, *r.args = name, args
				return nil
			}}, true
		}
	}
	return nil, false
}

func runSudo(t *testing.T, environ, programs []string, args ...string) (string, []string, string, error) {
	t.Helper()
	var asked string
	var ran []string
	var stderr bytes.Buffer
	ctx := WithProgramRunner(WithProcessView(context.Background(), hostProcessView{cwd: t.TempDir(), environ: environ}),
		sudoRunner{programs: programs, asked: &asked, args: &ran})
	applet, ok := DefaultRegistry.Lookup("sudo")
	if !ok {
		t.Fatal("no sudo applet")
	}
	err := applet.Run(ctx, args, strings.NewReader(""), io.Discard, &stderr)
	return asked, ran, stderr.String(), err
}

// Where no SSH session lends elevation, sudo is the PATH's, Windows's own sudo.exe, given every
// argument as it came.
func TestSudo_handsEverythingToThePathsSudoWhereNothingLends(t *testing.T) {
	// When
	asked, ran, _, err := runSudo(t, nil, []string{"sudo"}, "--inline", "-Z", "cmd")

	// Then
	if err != nil || asked != "sudo" || strings.Join(ran, " ") != "--inline -Z cmd" {
		t.Fatalf("ran %q %q, %v; want the PATH's sudo with every argument", asked, ran, err)
	}
}

func TestSudo_saysWhenNothingLendsAndThereIsNoSudo(t *testing.T) {
	// When
	_, _, _, err := runSudo(t, nil, nil, "id")

	// Then
	if status, ok := StatusCode(err); !ok || status != 1 || !strings.Contains(err.Error(), "SSH session") {
		t.Fatalf("sudo with nothing to ask: %v", err)
	}
}

// Inside the shell sudo's streams are the shell's, not handles another process can take, so it runs
// again as a process of its own, whose are.
func TestSudo_runsAgainAsAProcessInsideTheShell(t *testing.T) {
	// Given
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	// When
	asked, ran, _, err := runSudo(t, []string{"NEMOSH_SUDO=" + `\\.\pipe\x`}, []string{self}, "-E", "id", "-u")

	// Then
	if err != nil || asked != self || strings.Join(ran, " ") != "sudo -E id -u" {
		t.Fatalf("ran %q %q, %v; want this program run as sudo with the same arguments", asked, ran, err)
	}
}

// It runs itself once at most. Its streams are always wrapped, and one version that missed that
// started itself until the machine ran out of memory.
func TestSudo_runsItselfOnceAtMost(t *testing.T) {
	// Given
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	// When
	asked, _, _, err := runSudo(t, []string{"NEMOSH_SUDO=" + `\\.\pipe\x`, sudoProcessVariable + "=1"}, []string{self}, "id")

	// Then
	if err == nil || asked != "" || !strings.Contains(err.Error(), "cannot take this process's streams") {
		t.Fatalf("asked %q, %v; want no second run and the reason", asked, err)
	}
}

func TestSudo_refusesAnotherUserAndAnUnknownOption(t *testing.T) {
	for _, args := range [][]string{{"-u", "alice", "id"}, {"-Z", "id"}} {
		// When
		_, _, _, err := runSudo(t, []string{"NEMOSH_SUDO=" + `\\.\pipe\x`}, nil, args...)

		// Then
		if err == nil {
			t.Errorf("sudo %q went ahead", args)
		}
	}
	if _, _, _, err := runSudo(t, []string{"NEMOSH_SUDO=" + `\\.\pipe\x`}, nil, "-u", "root", "id"); err == nil || strings.Contains(err.Error(), "user") {
		t.Errorf("sudo -u root was refused for its user: %v", err)
	}
}
