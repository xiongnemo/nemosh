package main

import (
	"context"
	"maps"
	"slices"
	"strings"
	"testing"
)

// `$PA<TAB>` completes a variable's name, as bash does: `$PATH` and a blank, or `$HOME/` when
// the variable holds a directory, which is where the next keystroke was going anyway. It
// completed a file called `$PA`, which is to say nothing, or worse, escaped the dollar sign.
// Braces are kept and closed, a double quote left open stays open with nothing added inside
// it, and in single quotes a dollar sign is only a dollar sign.
func TestLineEditor_completesAVariableName(t *testing.T) {
	home := t.TempDir()
	values := map[string]string{"PATH": "/bin", "HOME": home, "HOSTNAME": "box", "PAGER": "less"}

	for _, test := range []struct{ typed, want string }{
		{"echo $PAT", "echo $PATH "},
		{"echo ${PAT", "echo ${PATH} "},
		{"ls $HOM", "ls $HOME/"},
		{`echo "$PAT`, `echo "$PATH`},
		{"echo $HOST$PAT", "echo $HOST$PATH "},
		{"echo $NOSUCH", "echo $NOSUCH"},
		{"echo '$PAT", "echo '$PAT"},
		{`echo \$PAT`, `echo \$PAT`},
	} {
		t.Run(test.typed, func(t *testing.T) {
			// Given
			editor := newLineEditor(strings.NewReader(test.typed+"\t\r"), newScreenModel(t, 120), t.TempDir())
			editor.width = func() int { return 120 }
			editor.variables = func() []string { return slices.Sorted(maps.Keys(values)) }
			editor.variableValue = func(name string) (string, bool) { value, ok := values[name]; return value, ok }

			// When
			line, err := editor.readLine(context.Background(), "$ ")

			// Then
			if err != nil {
				t.Fatalf("readLine: %v", err)
			}
			if line != test.want {
				t.Fatalf("line = %q, want %q", line, test.want)
			}
		})
	}
}

// Several names are taken as far as they agree and then listed, as names.
func TestLineEditor_listsTheVariablesAPrefixCouldBe(t *testing.T) {
	screen := newScreenModel(t, 120)
	editor := newLineEditor(strings.NewReader("echo $PA\t\r"), screen, t.TempDir())
	editor.width = func() int { return 120 }
	editor.variables = func() []string { return []string{"PAGER", "PATH", "PWD"} }
	editor.variableValue = func(string) (string, bool) { return "", true }

	line, err := editor.readLine(context.Background(), "$ ")

	if err != nil {
		t.Fatalf("readLine: %v", err)
	}
	if line != "echo $PA" {
		t.Fatalf("line = %q, want it left at $PA, all they share", line)
	}
	painted := screenText(screen)
	if !strings.Contains(painted, "$PAGER") || !strings.Contains(painted, "$PATH") || strings.Contains(painted, "$PWD") {
		t.Fatalf("the listing is not the two $PA names:\n%s", painted)
	}
}
