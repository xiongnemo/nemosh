package main

import (
	"bytes"
	"context"
	"regexp"
	"strings"
	"testing"
)

// An OSC ended by ST, ESC \, draws nothing, as one ended by BEL draws nothing: Windows Terminal's
// documentation writes its marks that way. The ESC ended it and the backslash was a column, so a
// prompt holding one put the cursor a cell to the right for each.
func TestPromptColumns_anOSCEndedByTheStringTerminatorTakesNoColumn(t *testing.T) {
	for _, prompt := range []string{"\x1b]0;title\x1b\\$ ", "\x1b]133;A\x1b\\$ \x1b]133;B\x1b\\", "\x1b]0;title\a$ "} {
		if got := promptColumns(prompt); got != 2 {
			t.Errorf("promptColumns(%q) = %d, want 2", prompt, got)
		}
	}
}

// The marks are on in Windows Terminal, which says it is there with WT_SESSION, when the prompt
// goes to a terminal; NEMOSH_SHELL_INTEGRATION=never turns them off there and always on anywhere.
func TestShellMarks_areOnInWindowsTerminalUnlessSaidOtherwise(t *testing.T) {
	for _, test := range []struct {
		name     string
		set      map[string]string
		terminal bool
		want     bool
	}{
		{name: "in Windows Terminal", set: map[string]string{"WT_SESSION": "x"}, terminal: true, want: true},
		{name: "in another terminal", set: map[string]string{}, terminal: true},
		{name: "with the prompt going elsewhere", set: map[string]string{"WT_SESSION": "x"}},
		{name: "never", set: map[string]string{"WT_SESSION": "x", shellIntegrationSetting: "never"}, terminal: true},
		{name: "always", set: map[string]string{shellIntegrationSetting: "always"}, want: true},
	} {
		lookup := func(name string) (string, bool) { value, ok := test.set[name]; return value, ok }
		if got := shellMarksOn(lookup, test.terminal); got != test.want {
			t.Errorf("%s: marks on %v, want %v", test.name, got, test.want)
		}
	}
}

var shellMark = regexp.MustCompile("\x1b\\]133;([A-D](?:;\\d+)?)\a")

// A session in Windows Terminal marks each prompt's start and end, A and B, where the command
// begins to run, C, and where it finished, D with its status -- written before the next prompt --
// or D alone after a line that ran nothing; and says the directory, OSC 9;9, so that a duplicated
// tab or a split pane opens there.
func TestShellMarks_markEachPromptAndCommandOfASession(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv(shellIntegrationSetting, "always")
	dir := t.TempDir()
	t.Chdir(dir)
	var screen, stdout bytes.Buffer
	shell := command{stdin: strings.NewReader(""), stdout: &stdout, stderr: &screen, stderrIsTerminal: true}
	editor := newLineEditor(strings.NewReader("true\rfalse\r\rfor i in 1\rdo :; done\r"), &screen, dir)

	// When
	_ = shell.runInteractiveEdited(context.Background(), &interruptController{}, editor, nil)

	// Then
	var marks []string
	for _, found := range shellMark.FindAllStringSubmatch(screen.String(), -1) {
		marks = append(marks, found[1])
	}
	want := "A B C D;0 A B C D;1 A B D A B C D;0 A B"
	if got := strings.Join(marks, " "); got != want {
		t.Fatalf("marks %q, want %q, in %q", got, want, screen.String())
	}
	if !strings.Contains(screen.String(), "\x1b]9;9;\"") {
		t.Fatalf("the directory was not said: %q", screen.String())
	}
}

// Off, the session writes none of it.
func TestShellMarks_areNotWrittenWhenOff(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv(shellIntegrationSetting, "never")
	t.Setenv("WT_SESSION", "x")
	var screen, stdout bytes.Buffer
	shell := command{stdin: strings.NewReader(""), stdout: &stdout, stderr: &screen, stderrIsTerminal: true}
	editor := newLineEditor(strings.NewReader("true\r"), &screen, t.TempDir())

	// When
	_ = shell.runInteractiveEdited(context.Background(), &interruptController{}, editor, nil)

	// Then
	if strings.Contains(screen.String(), "\x1b]133;") || strings.Contains(screen.String(), "\x1b]9;9;") {
		t.Fatalf("marks were written: %q", screen.String())
	}
}
