package applets

import (
	"os"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// `nano +LINE[,COLUMN] FILE` opens at that line and column, as nano does; see
// editor_position.go.

func TestEditorOperands_takeThePlusOperandOutOfTheFiles(t *testing.T) {
	for _, test := range []struct {
		operands []string
		want     editorPosition
		files    string
	}{
		{operands: []string{"+42", "f"}, want: editorPosition{line: 42}, files: "f"},
		{operands: []string{"+3,7", "f"}, want: editorPosition{line: 3, column: 7}, files: "f"},
		{operands: []string{"+,7", "f"}, want: editorPosition{column: 7}, files: "f"},
		{operands: []string{"f"}, files: "f"},
		{operands: []string{"+", "f"}, files: "+ f"},
	} {
		position, files, err := editorOperands(test.operands)
		if err != nil || position != test.want || strings.Join(files, " ") != test.files {
			t.Errorf("editorOperands(%q) = %+v, %q, %v; want %+v, %q", test.operands, position, files, err, test.want, test.files)
		}
	}
	for _, refused := range []string{"+/needle", "+?needle", "+x", "+0", "+3,y"} {
		if _, _, err := editorOperands([]string{refused, "f"}); err == nil {
			t.Errorf("%s was taken", refused)
		}
	}
}

// Opened at line 5, ^K cuts line 5: the cursor was there.
func TestEditor_opensAtTheLineAPlusOperandNames(t *testing.T) {
	path := writeNumberedFile(t, "numbered.txt", 10)
	harness := startEditorAt(t, "nano", path, editorPosition{line: 5})
	defer harness.stop(t)

	// When
	harness.press(t, tcell.KeyCtrlK, 0)
	harness.waitForScreen(t, "Cut one line")
	harness.writeOut(t)

	// Then: nine lines written. Without one line less, the file emptied by the write before it
	// is written lacked line 5 as well, and a macOS runner read it there.
	written := waitForFile(t, path, func(text string) bool { return strings.Count(text, "\n") == 9 })
	if strings.Contains(written, "number 5\n") || !strings.Contains(written, "number 4\n") || !strings.Contains(written, "number 6\n") {
		t.Fatalf("cut the wrong line:\n%s", written)
	}
}

// A line past the end is the last, as nano has it, and a column past a line's end is its end.
func TestEditor_aPositionPastTheEndIsTheEnd(t *testing.T) {
	path := writeNumberedFile(t, "numbered.txt", 3)
	harness := startEditorAt(t, "nano", path, editorPosition{line: 99, column: 99})
	defer harness.stop(t)

	// When: typed where the cursor is
	harness.type_(t, "!")
	harness.writeOut(t)

	// Then: at the end of the last line
	written := waitForFile(t, path, func(text string) bool { return strings.Contains(text, "!") })
	lines := strings.Split(strings.TrimSuffix(written, "\n"), "\n")
	if len(lines) != 3 || !strings.HasSuffix(lines[2], "!") {
		data, _ := os.ReadFile(path)
		t.Fatalf("typed at the wrong place:\n%s", data)
	}
}
