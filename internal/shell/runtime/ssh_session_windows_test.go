//go:build windows

package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/sudo"
	"golang.org/x/sys/windows"
)

func TestSSHSessionDrops_onlyAnElevatedSessionNotKeptElevated(t *testing.T) {
	for _, c := range []struct {
		env      map[string]string
		elevated bool
		want     bool
	}{
		{map[string]string{"SSH_CONNECTION": "::1 5 ::1 22"}, true, true},
		{map[string]string{"SSH_CLIENT": "::1 5 22"}, true, true},
		{map[string]string{"SSH_CONNECTION": "::1 5 ::1 22"}, false, false},
		{map[string]string{}, true, false},
		{map[string]string{"SSH_CONNECTION": "::1 5 ::1 22", sudo.KeepVariable: "keep"}, true, false},
		{map[string]string{"SSH_CONNECTION": "::1 5 ::1 22", sudo.KeepVariable: "no"}, true, true},
		{map[string]string{"SSH_CONNECTION": "::1 5 ::1 22", sudo.PipeVariable: `\\.\pipe\x`}, true, false},
	} {
		getenv := func(name string) string { return c.env[name] }
		if got := sshSessionDrops(getenv, func() bool { return c.elevated }); got != c.want {
			t.Errorf("%v, elevated %v: drops %v, want %v", c.env, c.elevated, got, c.want)
		}
	}
}

func TestSSHSessionIsInteractive_onlyAShellReadingATerminal(t *testing.T) {
	for _, c := range []struct {
		args []string
		want bool
	}{
		{nil, true},
		{[]string{"-l"}, true},
		{[]string{"-i"}, true},
		{[]string{"-c", "echo"}, false},
		{[]string{"-lc", "echo"}, false},
		{[]string{"script.sh"}, false},
		{[]string{"--login"}, true},
	} {
		if got := sshSessionIsInteractive(c.args, true); got != c.want {
			t.Errorf("%q: interactive %v, want %v", c.args, got, c.want)
		}
	}
	if sshSessionIsInteractive(nil, false) {
		t.Error("a shell reading no terminal was taken for an interactive one")
	}
}

// The lender takes the asker's streams from it and gives them to what it runs, and answers with
// that status.
func TestSudoLender_runsTheRequestWithTheAskersStreams(t *testing.T) {
	// Given
	name := fmt.Sprintf(`\\.\pipe\nemosh-sudo-test-%d-%d`, os.Getpid(), time.Now().UnixNano())
	var seen []string
	lender, err := listenForSudo(name, func(request sudo.Request, stdio [3]*os.File, _ windows.Handle) (int, string) {
		seen = request.Args
		if stdio[0] != nil || stdio[2] != nil || stdio[1] == nil {
			return 1, "streams the asker did not give, or not the one it did"
		}
		fmt.Fprintln(stdio[1], "lent")
		return 5, ""
	})
	if err != nil {
		t.Fatal(err)
	}
	go lender.serve()
	output, err := os.Create(filepath.Join(t.TempDir(), "out"))
	if err != nil {
		t.Fatal(err)
	}
	defer output.Close()

	// When
	reply, err := sudo.Ask(name, sudo.Request{Handles: [3]uint64{0, uint64(output.Fd()), 0}, Args: []string{"id", "-u"}})

	// Then
	if err != nil || reply.Status != 5 || reply.Message != "" {
		t.Fatalf("reply %+v, %v; want status 5", reply, err)
	}
	written, _ := os.ReadFile(output.Name())
	if string(written) != "lent\n" || strings.Join(seen, " ") != "id -u" {
		t.Fatalf("wrote %q for %q", written, seen)
	}
}

// An elevated shell that an SSH session starts runs the session's shell unelevated, and lends it
// the elevation back through sudo. Only an elevated test can see it: a CI runner is elevated.
func TestSSHSession_runsUnelevatedAndLendsSudo(t *testing.T) {
	binary := os.Getenv(jobBinaryVariable)
	if binary == "" || !processIsElevated() {
		t.Skip("needs an elevated test and a built nemosh")
	}
	shell := exec.Command(binary, "-c", `id -u; sudo id -u; sudo sh -c 'exit 7'; echo "$?"; sudo -s <<'EOF'
id -u
EOF`)
	shell.Env = append(os.Environ(), "SSH_CONNECTION=::1 50000 ::1 22", sudo.KeepVariable+"=")

	// When
	output, err := shell.CombinedOutput()

	// Then
	if err != nil || string(output) != "4095\n0\n7\n0\n" {
		t.Fatalf("output %q, %v; want the session unelevated and sudo elevated", output, err)
	}
}
