package applets_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// mv -u moves a file only over one older than it, or where there is none, as GNU's --update: it
// was `unknown option -- u`. One it leaves is said nothing of, even under -v.
func TestMv_updateMovesOnlyOverOlder(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string, when time.Time) {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, when, when); err != nil {
			t.Fatal(err)
		}
	}
	read := func(name string) string {
		content, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return "<none>"
		}
		return string(content)
	}
	now := time.Now()
	write("newer", "newer", now)
	write("older-dest", "old", now.Add(-time.Hour))
	write("older", "older", now.Add(-2*time.Hour))
	write("newer-dest", "new", now.Add(-time.Minute))
	write("lone", "lone", now)

	for _, test := range []struct {
		args       []string
		stdout     string
		source     string
		dest, want string
	}{
		{args: []string{"-u", "newer", "older-dest"}, source: "newer", dest: "older-dest", want: "newer"},
		{args: []string{"-uv", "older", "newer-dest"}, source: "older", dest: "newer-dest", want: "new"},
		{args: []string{"--update", "lone", "fresh"}, source: "lone", dest: "fresh", want: "lone"},
	} {
		stdout, stderr, err := runSmall(t, dir, "", "mv", test.args...)
		if err != nil || stdout != test.stdout {
			t.Errorf("mv %q = %q, %v (%s)", test.args, stdout, err, stderr)
		}
		if got := read(test.dest); got != test.want {
			t.Errorf("mv %q left %s holding %q, want %q", test.args, test.dest, got, test.want)
		}
	}
	if read("older") != "older" {
		t.Error("mv -u moved a file over a newer one")
	}
}
