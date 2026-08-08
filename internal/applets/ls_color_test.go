package applets_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

func runLS(t *testing.T, dir string, args ...string) (string, string, error) {
	t.Helper()
	applet, ok := applets.DefaultRegistry.Lookup("ls")
	if !ok {
		t.Fatal("ls is not registered")
	}
	var stdout, stderr bytes.Buffer
	ctx := applets.WithProcessView(context.Background(), findTestProcessView{cwd: dir})
	err := applet.Run(ctx, args, strings.NewReader(""), &stdout, &stderr)
	return stdout.String(), stderr.String(), err
}

func lsColorFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "adir"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"plain.txt", "run.exe"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// The colours are busybox-w32's, measured on 2026-08-09: a directory is
// \033[1;34m, an executable \033[1;32m, anything else \033[0;0m, and every name
// is closed with \033[m.
func TestLS_coloursWhenAsked(t *testing.T) {
	// Given
	dir := lsColorFixture(t)

	// When
	stdout, stderr, err := runLS(t, dir, "--color=always")

	// Then
	if err != nil {
		t.Fatalf("ls --color=always: %v (stderr %q)", err, stderr)
	}
	for _, want := range []string{
		"\033[1;34madir\033[m",
		"\033[0;0mplain.txt\033[m",
		"\033[1;32mrun.exe\033[m",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("output %q does not contain %q", stdout, want)
		}
	}
}

// `--color=auto` writing to something that is not a terminal produces no
// escapes, which is the whole reason the alias in a startup file is safe to
// pipe. A test buffer is not a terminal.
func TestLS_autoIsPlainWhenNotATerminal(t *testing.T) {
	// Given
	dir := lsColorFixture(t)

	// When
	stdout, _, err := runLS(t, dir, "--color=auto")

	// Then
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stdout, "\033[") {
		t.Fatalf("output %q carries escapes, want plain text", stdout)
	}
}

func TestLS_colourSpellings(t *testing.T) {
	for _, test := range []struct {
		name    string
		args    []string
		colours bool
	}{
		{name: "bare --color means always", args: []string{"--color"}, colours: true},
		{name: "always", args: []string{"--color=always"}, colours: true},
		{name: "force is a synonym", args: []string{"--color=force"}, colours: true},
		{name: "never", args: []string{"--color=never"}},
		{name: "none is a synonym", args: []string{"--color=none"}},
		{name: "auto, and this is not a terminal", args: []string{"--color=auto"}},
		{name: "alongside a short option", args: []string{"-a", "--color=always"}, colours: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given
			dir := lsColorFixture(t)

			// When
			stdout, stderr, err := runLS(t, dir, test.args...)

			// Then
			if err != nil {
				t.Fatalf("ls %v: %v (stderr %q)", test.args, err, stderr)
			}
			if got := strings.Contains(stdout, "\033["); got != test.colours {
				t.Fatalf("ls %v coloured = %v, want %v (output %q)", test.args, got, test.colours, stdout)
			}
		})
	}
}

// An unknown long option is refused by name, not swallowed, and not reported as
// the bare `--` it used to be.
func TestLS_refusesAnUnknownLongOption(t *testing.T) {
	// Given
	dir := lsColorFixture(t)

	// When
	stdout, stderr, err := runLS(t, dir, "--nosuchoption")

	// Then
	if err == nil {
		t.Fatal("ls --nosuchoption succeeded, want a refusal")
	}
	if stdout != "" {
		t.Fatalf("ls --nosuchoption wrote %q before refusing", stdout)
	}
	message := stderr + err.Error()
	if !strings.Contains(message, "--nosuchoption") {
		t.Fatalf("reported %q, want it to name the whole option", message)
	}
}

func TestLS_refusesAnUnknownColourWhen(t *testing.T) {
	// Given
	dir := lsColorFixture(t)

	// When
	_, stderr, err := runLS(t, dir, "--color=maybe")

	// Then
	if err == nil {
		t.Fatal("ls --color=maybe succeeded, want a refusal")
	}
	if message := stderr + err.Error(); !strings.Contains(message, "maybe") {
		t.Fatalf("reported %q, want it to name the value", message)
	}
}

// `--` still ends option parsing, so a file called `--color` can be listed.
func TestLS_doubleDashStillEndsOptions(t *testing.T) {
	// Given
	dir := lsColorFixture(t)

	// When
	stdout, stderr, err := runLS(t, dir, "--", "plain.txt")

	// Then
	if err != nil {
		t.Fatalf("ls -- plain.txt: %v (stderr %q)", err, stderr)
	}
	if !strings.Contains(stdout, "plain.txt") {
		t.Fatalf("output %q, want plain.txt", stdout)
	}
}
