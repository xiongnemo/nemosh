package applets

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"
)

// dropRunner is a ProgramRunner that has every program, and keeps what it was asked for and how;
// nothing is launched.
type dropRunner struct{ seen *dropSeen }

type dropSeen struct {
	asked, user string
	args        []string
	unelevated  bool
}

func (r dropRunner) Program(ctx context.Context, name string) (Applet, bool) {
	r.seen.asked = name
	return simpleApplet{name: name, runContext: func(ctx context.Context, args []string, _ io.Reader, _, _ io.Writer) error {
		r.seen.args, r.seen.unelevated = args, UnelevatedLaunch(ctx)
		r.seen.user, _ = ProcessViewFromContext(ctx).LookupEnv("USER")
		return nil
	}}, true
}

func runDrop(t *testing.T, name string, environ []string, args ...string) (dropSeen, error) {
	t.Helper()
	seen := dropSeen{}
	ctx := WithProgramRunner(WithProcessView(context.Background(), hostProcessView{cwd: t.TempDir(), environ: environ}), dropRunner{seen: &seen})
	applet, ok := DefaultRegistry.Lookup(name)
	if !ok {
		t.Fatalf("no %s applet", name)
	}
	err := applet.Run(ctx, args, strings.NewReader(""), io.Discard, io.Discard)
	return seen, err
}

func TestDrop_runsWhatItIsGivenWithoutElevation(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		applet string
		args   []string
		asked  string
		want   []string
	}{
		{"drop", []string{"prog", "a", "-Z"}, "prog", []string{"a", "-Z"}},
		{"drop", nil, self, nil},
		{"drop", []string{"-c", "echo hi", "x"}, self, []string{"-c", "echo hi", "x"}},
		{"drop", []string{"sleep", "1"}, self, []string{"sleep", "1"}},
		{"drop", []string{"-s", `C:\Windows\System32\cmd.exe`, "-c", "dir"}, `C:\Windows\System32\cmd.exe`, []string{"/c", "dir"}},
		{"drop", []string{"-s", "pwsh", "-c", "gci"}, "pwsh", []string{"-c", "gci"}},
		{"cdrop", nil, "cmd.exe", nil},
		{"cdrop", []string{"-c", "ver"}, "cmd.exe", []string{"/c", "ver"}},
		{"pdrop", []string{"-c", "gci"}, "powershell.exe", []string{"-c", "gci"}},
	} {
		// When
		seen, err := runDrop(t, c.applet, nil, c.args...)

		// Then
		if err != nil {
			t.Fatalf("%s %q: %v", c.applet, c.args, err)
		}
		if seen.asked != c.asked || strings.Join(seen.args, "|") != strings.Join(c.want, "|") || !seen.unelevated {
			t.Errorf("%s %q ran %q %q, unelevated %v; want %q %q, unelevated", c.applet, c.args, seen.asked, seen.args, seen.unelevated, c.asked, c.want)
		}
	}
}

func TestDrop_namesTheUserAsBusyboxDoes(t *testing.T) {
	account := processAccountName()
	for _, c := range []struct {
		environ []string
		want    string
	}{
		{nil, account},
		{[]string{"USER=root"}, account},
		{[]string{"USER=alice"}, "alice"},
	} {
		// When
		seen, err := runDrop(t, "drop", c.environ, "prog")

		// Then
		if err != nil || seen.user != c.want {
			t.Errorf("environment %q gave USER %q, %v; want %q", c.environ, seen.user, err, c.want)
		}
	}
}

func TestDrop_refusesWhatItCannotRun(t *testing.T) {
	ctx := WithProcessView(context.Background(), hostProcessView{cwd: t.TempDir()})
	applet, _ := DefaultRegistry.Lookup("drop")

	// When
	err := applet.Run(ctx, []string{"nosuchprog"}, strings.NewReader(""), io.Discard, io.Discard)

	// Then
	if status, ok := StatusCode(err); !ok || status != 127 || !strings.Contains(err.Error(), "can't find 'nosuchprog'") {
		t.Fatalf("drop nosuchprog: %v, want busybox's 127 and its words", err)
	}
	if _, err := runDrop(t, "cdrop", nil, "-s", "sh"); err == nil {
		t.Fatal("cdrop took -s, which only drop takes")
	}
}
