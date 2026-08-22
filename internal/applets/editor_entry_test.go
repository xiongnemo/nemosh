package applets

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The editor applet's own entry points, which the harness bypasses: it calls
// openEditorSession and runEditorApplication directly so it can hand in a
// simulation screen, so -H, the operand checks, the non-terminal refusal and
// save's own refusals were all uncovered.

// -H states the subset, in the manner of `busybox vi -H`. It writes to *stdout*
// and needs no terminal, because it is an answer to a question rather than a
// session.
func TestEditor_dashHNeedsNoTerminal(t *testing.T) {
	for _, name := range []string{"nano", "micro"} {
		t.Run(name, func(t *testing.T) {
			var stdout, stderr strings.Builder
			// Empty stdin: not a terminal, which -H must not care about.
			if err := newEditorApplet(name).Run(t.Context(), []string{"-H"},
				strings.NewReader(""), &stdout, &stderr); err != nil {
				t.Fatalf("%s -H: %v (%s)", name, err, stderr.String())
			}
			written := stdout.String()
			if !strings.Contains(written, name+" implements these features") {
				t.Fatalf("-H does not name itself: %q", written)
			}
			// Every binding this name has is described, which is the property that
			// keeps the list from claiming a key the editor does not bind.
			for _, binding := range editorKeyMapFor(name).bindings {
				if !strings.Contains(written, binding.describes) {
					t.Errorf("-H omits %q", binding.describes)
				}
			}
			// And what is absent is named, because an editor that silently lacks
			// undo is worse than one that says so.
			for _, absence := range []string{"No syntax highlighting", "No multiple buffers", "No mouse"} {
				if !strings.Contains(written, absence) {
					t.Errorf("-H does not admit %q", absence)
				}
			}
			// Nothing on stderr: -H succeeded, so there is nothing to report.
			if stderr.String() != "" {
				t.Errorf("-H wrote to stderr: %q", stderr.String())
			}
		})
	}
}

// Without a terminal there is no useful non-interactive form, so the editor
// refuses and says why rather than drawing itself and waiting for keys that never
// arrive. `nano file < /dev/null` is the case that hung: /dev/null *is* a file, so
// leasing it succeeds and only IsTerminal tells the truth.
func TestEditor_refusesWithoutATerminal(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "a.txt")
	if err := os.WriteFile(path, []byte("content\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, stdin := range []string{"", "some piped text\n"} {
		var stdout, stderr strings.Builder
		err := newEditorApplet("nano").Run(t.Context(), []string{path},
			strings.NewReader(stdin), &stdout, &stderr)
		if err == nil {
			t.Fatal("the editor started without a terminal")
		}
		if !strings.Contains(err.Error(), "terminal") {
			t.Fatalf("the refusal does not mention a terminal: %v", err)
		}
	}
	// And the file is untouched: refusing must not truncate what it would not edit.
	if got, err := os.ReadFile(path); err != nil || string(got) != "content\n" {
		t.Fatalf("the file changed during a refusal: %q (%v)", got, err)
	}
}

// The operand rules, and the option it does not have.
func TestEditor_operandAndOptionRules(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"a.txt", "b.txt"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, test := range []struct {
		name    string
		args    []string
		because string
	}{
		// There are no multiple buffers, so two files is refused rather than one
		// being silently ignored.
		{name: "two files", args: []string{"a.txt", "b.txt"}, because: "one file"},
		{name: "three files", args: []string{"a.txt", "b.txt", "a.txt"}, because: "one file"},
		{name: "an option it does not have", args: []string{"-Z", "a.txt"}, because: "invalid option"},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := WithProcessView(t.Context(), hostProcessView{cwd: root})
			var stdout, stderr strings.Builder
			err := newEditorApplet("nano").Run(ctx, test.args, strings.NewReader(""), &stdout, &stderr)
			if err == nil {
				t.Fatalf("%v was accepted", test.args)
			}
			if !strings.Contains(err.Error(), test.because) {
				t.Fatalf("%v said %q, which does not mention %q", test.args, err, test.because)
			}
		})
	}
}

// A directory is not a file to edit. Opening one would read a directory as text,
// which fails with an error from the read rather than from the operand.
func TestEditor_refusesToOpenADirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "adir"), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx := WithProcessView(t.Context(), hostProcessView{cwd: root})
	if _, err := openEditorSession(ctx, "nano", []string{"adir"}, false); err == nil {
		t.Fatal("a directory was opened as a file")
	}
}

// save's own refusals, reached directly because the harness cannot type into a
// read-only buffer and there is no key that saves a nameless one.
func TestEditorSession_saveRefusals(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "a.txt")
	if err := os.WriteFile(path, []byte("original\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx := WithProcessView(t.Context(), hostProcessView{cwd: root})

	// Read-only refuses, and leaves the file as it was.
	readOnly, err := openEditorSession(ctx, "nano", []string{"a.txt"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := readOnly.save("replaced\n"); err == nil {
		t.Fatal("a read-only session saved")
	}
	if got, _ := os.ReadFile(path); string(got) != "original\n" {
		t.Fatalf("the read-only refusal wrote anyway: %q", got)
	}

	// No name at all: the editor started with no operand has nowhere to write.
	nameless, err := openEditorSession(ctx, "nano", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	err = nameless.save("text\n")
	if err == nil {
		t.Fatal("a nameless session saved")
	}
	if !strings.Contains(err.Error(), "name") {
		t.Fatalf("the refusal does not mention the missing name: %v", err)
	}

	// A writable session saves, and keeps the file's own mode rather than imposing
	// 0644 on a file that had something else.
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	writable, err := openEditorSession(ctx, "nano", []string{"a.txt"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := writable.save("replaced\n"); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); string(got) != "replaced\n" {
		t.Fatalf("the save wrote %q", got)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	// Windows reports only the read-only bit, so the assertion is that the mode did
	// not become 0644 rather than that it is exactly 0600.
	if info.Mode().Perm() == 0o644 {
		t.Errorf("the save imposed 0644 on a file that was 0600")
	}
}

// A new file is marked as new, so the status line can say so rather than showing
// an empty buffer with no explanation -- and saving it creates it.
func TestEditorSession_marksAndCreatesANewFile(t *testing.T) {
	root := t.TempDir()
	ctx := WithProcessView(t.Context(), hostProcessView{cwd: root})
	session, err := openEditorSession(ctx, "nano", []string{"fresh.txt"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !session.isNew {
		t.Fatal("a file that does not exist was not marked new")
	}
	if session.text != "" {
		t.Fatalf("a new file started with %q", session.text)
	}
	if err := session.save("created\n"); err != nil {
		t.Fatal(err)
	}
	if session.isNew {
		t.Fatal("the file is still marked new after being written")
	}
	if got, err := os.ReadFile(filepath.Join(root, "fresh.txt")); err != nil || string(got) != "created\n" {
		t.Fatalf("the new file holds %q (%v)", got, err)
	}
}

// editorLines, the line counting that was off by one for every file ending in a
// newline. Unit-tested as well as driven through the editor, because the rule is
// small and its cases are exactly enumerable.
func TestEditorLines_countsLinesAsAReaderWould(t *testing.T) {
	for _, test := range []struct {
		text string
		want int
	}{
		// The case that was wrong: a terminating newline does not add a line.
		{text: "a\nb\n", want: 2},
		{text: "one\n", want: 1},
		// No terminator is still a line.
		{text: "a\nb", want: 2},
		{text: "a", want: 1},
		// An empty buffer is one empty line, not zero -- zero would divide by
		// len(lines) in the search wrap.
		{text: "", want: 1},
		// A blank line before the terminator *is* a line.
		{text: "a\n\n", want: 2},
		{text: "a\n\n\n", want: 3},
		{text: "\n", want: 1},
		{text: "\n\n", want: 2},
	} {
		t.Run(strings.ReplaceAll(test.text, "\n", "|"), func(t *testing.T) {
			if got := len(editorLines(test.text)); got != test.want {
				t.Fatalf("editorLines(%q) counted %d lines, want %d", test.text, got, test.want)
			}
		})
	}
}
