package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// An interactive shell sources $ENV, which is what POSIX specifies and what
// busybox ash does (shell/ash.c:16801). It is also how a real configuration
// already reaches busybox on this machine -- ENV is a Windows environment
// variable pointing at ~/.ashrc -- so honouring it costs the user no change.
func TestStartupFile_sourcesENV_forAnInteractiveShell(t *testing.T) {
	// Given
	directory := t.TempDir()
	rc := filepath.Join(directory, "rc.sh")
	if err := os.WriteFile(rc, []byte("STARTUP_RAN=yes\nPS1='rc> '\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	rt.SetVariable("ENV", filepath.ToSlash(rc))
	var stderr bytes.Buffer

	// When
	sourceStartupFile(context.Background(), rt, &stderr)

	// Then
	if value, _ := rt.LookupVariable("STARTUP_RAN"); value != "yes" {
		t.Fatalf("STARTUP_RAN = %q, want %q (stderr %q)", value, "yes", stderr.String())
	}
	if value, _ := rt.LookupVariable("PS1"); value != "rc> " {
		t.Fatalf("PS1 = %q, want it set by the startup file", value)
	}
}

// A missing or unset ENV is silent: not every user has one, and a shell that
// complained on every launch would be unusable.
func TestStartupFile_isSilent_whenThereIsNothingToSource(t *testing.T) {
	for _, test := range []struct {
		name  string
		value string
	}{
		{name: "unset", value: ""},
		{name: "points at nothing", value: "C:/nemosh-no-such-startup-file"},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given
			rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
			if test.value != "" {
				rt.SetVariable("ENV", test.value)
			}
			var stderr bytes.Buffer

			// When
			sourceStartupFile(context.Background(), rt, &stderr)

			// Then
			if stderr.String() != "" {
				t.Fatalf("stderr = %q, want silence", stderr.String())
			}
		})
	}
}

// A startup file that fails reports it. Silently ignoring the error would leave
// the user with a shell that is quietly not configured.
func TestStartupFile_reportsAFailingFile(t *testing.T) {
	// Given
	directory := t.TempDir()
	rc := filepath.Join(directory, "bad.sh")
	if err := os.WriteFile(rc, []byte("{echo unbalanced\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	rt.SetVariable("ENV", filepath.ToSlash(rc))
	var stderr bytes.Buffer

	// When
	sourceStartupFile(context.Background(), rt, &stderr)

	// Then
	if !strings.Contains(stderr.String(), "bad.sh") {
		t.Fatalf("stderr = %q, want it to name the startup file", stderr.String())
	}
}

// A prompt is a command context: dash, bash, and busybox ash all expand
// substitutions in PS1 at display time, which is how a prompt shows a git
// branch or an exit code that changes between commands.
func TestPrompt_expandsCommandSubstitution(t *testing.T) {
	// Given
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	rt.SetVariable("PS1", `[$(echo dynamic)] `)

	// When
	rendered := interactivePrompt(rt, false)

	// Then
	if !strings.Contains(rendered, "[dynamic]") {
		t.Fatalf("prompt = %q, want the substitution expanded", rendered)
	}
}

// A variable in the prompt expands too, which is what makes a colour palette
// defined in a startup file usable.
func TestPrompt_expandsVariables(t *testing.T) {
	// Given
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	rt.SetVariable("MARK", "><")
	rt.SetVariable("PS1", `${MARK} `)

	// When
	rendered := interactivePrompt(rt, false)

	// Then
	if !strings.HasPrefix(rendered, "><") {
		t.Fatalf("prompt = %q, want the variable expanded", rendered)
	}
}

// The default prompt carries colour, and every escape is paired with a reset so
// a failed command cannot leave the terminal tinted.
func TestDefaultPrompt_isColouredAndBalanced(t *testing.T) {
	// Given
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})

	// When
	rendered := interactivePrompt(rt, false)

	// Then
	if !strings.Contains(rendered, "\033[") {
		t.Fatalf("default prompt has no colour: %q", rendered)
	}
	if !strings.HasSuffix(strings.TrimSuffix(rendered, " "), "\033[0m") {
		t.Fatalf("default prompt does not end reset: %q", rendered)
	}
	// It still has to say the things the plain one said.
	for _, want := range []string{promptUsername(rt), promptHostname()} {
		if !strings.Contains(rendered, want) {
			t.Errorf("default prompt lost %q", want)
		}
	}
}

// A startup file written for ash or bash spells colour as an octal escape.
// Without these the prompt printed `\033[1;34m` literally.
func TestPrompt_rendersOctalAndEscapeSequences(t *testing.T) {
	for _, test := range []struct {
		name string
		ps1  string
		want string
	}{
		{name: "octal escape", ps1: `\033[1;34mx`, want: "\033[1;34mx"},
		{name: "short octal stops at a non-digit", ps1: `\07x`, want: "\007x"},
		{name: "three digits at most", ps1: `\0333`, want: "\0333"},
		{name: "e is escape too", ps1: `\e[0m`, want: "\033[0m"},
		{name: "bell", ps1: `\a`, want: "\007"},
		{name: "non-printing markers are dropped", ps1: `\[\033[0m\]x`, want: "\033[0mx"},
		{name: "an unknown escape is left alone", ps1: `\q`, want: `\q`},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given
			rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
			rt.SetVariable("PS1", test.ps1)

			// When
			got := interactivePrompt(rt, false)

			// Then
			if got != test.want {
				t.Fatalf("prompt = %q, want %q", got, test.want)
			}
		})
	}
}
