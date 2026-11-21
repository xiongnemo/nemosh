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

// /dev/tty is the console the shell is attached to: CONIN$ read and CONOUT$ written, each the
// console's own handle, so a program given `</dev/tty` -- fzf, zoxide's interactive pick -- has
// the terminal and not a pipe. With no console, a service's or a CI runner's, it is Git's bash's
// answer, No such device or address. It was "nonexistent directory". A file stands in for each
// console name here, since whether a test has a console is the machine's to say.
func TestDevice_ttyIsTheConsole(t *testing.T) {
	dir := t.TempDir()
	conout, conin := filepath.Join(dir, "conout"), filepath.Join(dir, "conin")
	if err := os.WriteFile(conin, []byte("typed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	restore := openConsoleFile
	t.Cleanup(func() { openConsoleFile = restore })
	openConsoleFile = func(name string) (*os.File, error) {
		switch name {
		case "CONOUT$":
			return os.OpenFile(conout, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o644)
		case "CONIN$":
			return os.Open(conin)
		}
		return nil, errors.New("not a console name: " + name)
	}
	var stdout bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: new(bytes.Buffer)})
	rt.RunScript(context.Background(), "echo to-tty > /dev/tty\nread x < /dev/tty; echo \"read $x\"\ntest -c /dev/tty && echo char\n")
	if stdout.String() != "read typed\nchar\n" {
		t.Errorf("stdout %q", stdout.String())
	}
	if data, _ := os.ReadFile(conout); string(data) != "to-tty\n" {
		t.Errorf("the console was written %q", data)
	}

	openConsoleFile = func(string) (*os.File, error) { return nil, os.ErrNotExist }
	var stderr bytes.Buffer
	rt = New(applets.DefaultRegistry, Streams{Stdout: new(bytes.Buffer), Stderr: &stderr})
	if status := rt.RunScript(context.Background(), "echo x > /dev/tty\n"); status == 0 || !strings.Contains(stderr.String(), "/dev/tty: No such device or address") {
		t.Errorf("with no console: status %d, stderr %q", status, stderr.String())
	}
}
