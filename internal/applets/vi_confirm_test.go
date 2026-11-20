package applets

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// :s's c flag, vim's: each match is asked about, `replace with X (y/n/a/q/l)?`, y replacing it, n
// passing it, a replacing it and the rest, q stopping, l replacing it and stopping; the whole is
// one change.
func TestVi_substituteAsksAtEachMatch(t *testing.T) {
	for _, test := range []struct{ name, keys, want string }{
		{"y n y", ":%s/a/X/gc\ryny", "X a\nX"},
		{"a", ":%s/a/X/gc\ra", "X X\nX"},
		{"q", ":%s/a/X/gc\ryq", "X a\na"},
		{"l", ":%s/a/X/gc\rl", "X a\na"},
		{"Escape", ":%s/a/X/gc\r\x1b", "a a\na"},
		{"one change", ":%s/a/X/gc\rau", "a a\na"},
		{"without g, one a line", ":%s/a/X/c\ryy", "X a\nX"},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("a a\na\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			editor, screen := newTestVi(t, dir, "f.txt", false, 60, 6)
			typeVi(editor, ":%s/a/X/gc\r")
			if got := viRow(t, screen, 5); got != "replace with X (y/n/a/q/l)?" {
				t.Errorf("asked %q", got)
			}
			typeVi(editor, strings.TrimPrefix(strings.TrimPrefix(test.keys, ":%s/a/X/gc\r"), ":%s/a/X/c\r"))
			if test.name == "without g, one a line" {
				editor, _ = newTestVi(t, dir, "f.txt", false, 60, 6)
				typeVi(editor, test.keys)
			}
			if got := viText(editor); got != test.want {
				t.Errorf("the buffer is %q, want %q", got, test.want)
			}
			if editor.confirming != nil {
				t.Error("it is still asking")
			}
		})
	}
}

// view is vi -R, vim's.
func TestVi_viewIsReadOnly(t *testing.T) {
	stdout, _, status := runApplet(t, "view", []string{"-H"}, "")
	if status != 0 || !strings.Contains(stdout, "busybox-w32's vi") {
		t.Errorf("view -H: status %d, %q", status, stdout)
	}
	if applet, _ := DefaultRegistry.Lookup("view"); applet == nil || applet.Name() != "view" {
		t.Fatal("view is not an applet")
	}
}
