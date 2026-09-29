package applets_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// od and hexdump write a buffer at a time, and write out what they hold before each FILE is
// opened, so a FILE that cannot be is named after the lines before it, as busybox's are on a
// terminal: `od -c -w2 one missing two 2>&1` puts its first line before the error.
func TestDump_namesAMissingFileAfterTheLinesBeforeIt(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{"one": "abc", "two": "def"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"od", "-c", "-w2", "one", "missing", "two"}, "0000000   a   b\nod: missing: No such file or directory\n0000002   c   d\n0000004   e   f\n0000006\n"},
		{[]string{"hexdump", "-e", `2/1 "%02x" "\n"`, "one", "missing", "two"}, "6162\nhexdump: missing: No such file or directory\n6364\n6566\n"},
	} {
		applet, _ := applets.DefaultRegistry.Lookup(test.args[0])
		var both bytes.Buffer
		ctx := applets.WithProcessView(context.Background(), permuteTestView{cwd: dir})
		err := applet.Run(ctx, test.args[1:], strings.NewReader(""), &both, &both)
		if both.String() != test.want || err == nil {
			t.Errorf("%q 2>&1: %q, %v; want %q and status 1", test.args, both.String(), err, test.want)
		}
	}
}

// With -N or -n stdin is read no further than the dump goes, as busybox's od reads it with
// its buffering off, so that `{ od -N 4; cat; } < f` leaves cat the rest.
func TestDump_readsNoMoreOfStdinThanItDumps(t *testing.T) {
	for _, test := range []struct {
		args []string
		left string
	}{
		{[]string{"od", "-N", "4", "-c"}, "efgh"},
		{[]string{"hexdump", "-n", "2", "-C"}, "cdefgh"},
	} {
		applet, _ := applets.DefaultRegistry.Lookup(test.args[0])
		stdin := strings.NewReader("abcdefgh")
		if err := applet.Run(context.Background(), test.args[1:], stdin, io.Discard, io.Discard); err != nil {
			t.Fatalf("%q: %v", test.args, err)
		}
		if left, _ := io.ReadAll(stdin); string(left) != test.left {
			t.Errorf("%q left %q of stdin, want %q", test.args, left, test.left)
		}
	}
}
