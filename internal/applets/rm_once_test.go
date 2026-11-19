package applets_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// rm -I asks once, before removing more than three files or removing recursively, as GNU's does:
// `rm: remove 4 arguments? `. It was `unknown option -- I`. Anything but yes removes nothing.
func TestRm_asksOnceUnderDashI(t *testing.T) {
	for _, test := range []struct {
		args       []string
		answer     string
		prompt     string
		removedAll bool
	}{
		{args: []string{"-I", "a", "b", "c", "d"}, answer: "n\n", prompt: "rm: remove 4 arguments? "},
		{args: []string{"-I", "a", "b", "c", "d"}, answer: "y\n", prompt: "rm: remove 4 arguments? ", removedAll: true},
		{args: []string{"-I", "-r", "dir"}, answer: "n\n", prompt: "rm: remove 1 argument recursively? "},
		{args: []string{"-Ir", "dir", "a"}, answer: "yes\n", prompt: "rm: remove 2 arguments recursively? ", removedAll: true},
		// Three files or fewer, not recursively, is asked nothing; -f after -I asks nothing either.
		{args: []string{"-I", "a", "b"}, removedAll: true},
		{args: []string{"-If", "a", "b", "c", "d"}, removedAll: true},
	} {
		dir := t.TempDir()
		var named []string
		for _, name := range []string{"a", "b", "c", "d", "dir/inside"} {
			path := filepath.Join(dir, filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, nil, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		for _, arg := range test.args {
			if !strings.HasPrefix(arg, "-") {
				named = append(named, arg)
			}
		}
		_, stderr, err := runSmall(t, dir, test.answer, "rm", test.args...)
		if err != nil || stderr != test.prompt {
			t.Errorf("rm %q answering %q: %v, stderr %q; want %q", test.args, test.answer, err, stderr, test.prompt)
		}
		for _, name := range named {
			_, statErr := os.Stat(filepath.Join(dir, name))
			if removed := statErr != nil; removed != test.removedAll {
				t.Errorf("rm %q answering %q: %s removed %v, want %v", test.args, test.answer, name, removed, test.removedAll)
			}
		}
	}
}
