package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// The readline functions a macro or a bind reaches that the editor had no key for: set-mark and
// exchange-point-and-mark (C-@ or M-SPC, C-x C-x), shell-expand-line (M-C-e), vi-editing-mode
// (M-C-j), each on bash's keys. And in bash mode C-h is backward-delete-char, as in Git's bash.
func TestLineEditor_readlineFunctionsTheToolsReach(t *testing.T) {
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: new(bytes.Buffer), Stderr: new(bytes.Buffer)})
	ctx := context.Background()
	read := func(t *testing.T, keys string, bash bool) (string, *lineEditor) {
		editor := newLineEditor(strings.NewReader(keys), new(bytes.Buffer), t.TempDir())
		editor.bindings = rt.MatchKeyBinding
		editor.expandLine = func(line string) (string, bool) { return rt.ExpandLine(ctx, line) }
		editor.bashMode = bash
		line, err := editor.readLine(ctx, "$ ")
		if err != nil {
			t.Fatal(err)
		}
		return line, editor
	}
	if line, _ := read(t, "abc\x00\x01\x18\x18d\r", false); line != "abcd" {
		t.Errorf("set-mark and exchange-point-and-mark: %q", line)
	}
	if line, _ := read(t, "echo $(echo hi)\x1b\x05\r", false); line != "echo hi" {
		t.Errorf("shell-expand-line: %q", line)
	}
	if line, _ := read(t, "abc def\x08\r", false); line != "abc " {
		t.Errorf("C-h as nemosh: %q, want a word deleted", line)
	}
	if line, _ := read(t, "abc def\x08\r", true); line != "abc de" {
		t.Errorf("C-h in bash mode: %q, want a character deleted", line)
	}
	if _, editor := read(t, "x\x1b\nab\r", false); !editor.vi.on {
		t.Error("M-C-j left vi mode off")
	}
}

// fzf's Alt-C, its own macro (key-bindings.bash), run as bash runs it: what is typed is put
// aside, `__fzf_cd__` in backquotes is expanded by M-C-e into the cd it answers, the line is
// accepted, and the rest of the macro is left to read for the next line.
func TestBind_fzfsAltC(t *testing.T) {
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: new(bytes.Buffer), Stderr: new(bytes.Buffer)})
	ctx := context.Background()
	script := "__fzf_cd__() { echo \"builtin cd -- 'C:/picked'\"; }\n" +
		"bind -m emacs-standard '\"\\C-\\e(\": redraw-current-line'\n" +
		"bind -m emacs-standard '\"\\ec\": \" \\C-b\\C-k \\C-u`__fzf_cd__`\\e\\C-e\\C-\\e(\\C-m\\C-y\\C-h\\e \\C-y\\ey\\C-x\\C-x\\C-d\\C-y\\ey\\C-_\"'\n"
	if status := rt.RunScript(ctx, script); status != 0 {
		t.Fatalf("fzf's bindings exited %d", status)
	}
	editor := newLineEditor(strings.NewReader("ls -l\x1bc"), new(bytes.Buffer), t.TempDir())
	editor.bindings = rt.MatchKeyBinding
	editor.expandLine = func(line string) (string, bool) { return rt.ExpandLine(ctx, line) }
	editor.bashMode = true
	line, err := editor.readLine(ctx, "$ ")
	if err != nil || line != "builtin cd -- 'C:/picked'" {
		t.Fatalf("Alt-C accepted %q, %v", line, err)
	}
	if len(editor.pending) == 0 {
		t.Error("nothing of the macro was left for the next line")
	}
}
