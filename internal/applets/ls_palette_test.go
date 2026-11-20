package applets_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// lsColorsView is a directory's view with LS_COLORS set.
type lsColorsView struct {
	findTestProcessView
	colors string
}

func (v lsColorsView) LookupEnv(name string) (string, bool) {
	if name == "LS_COLORS" {
		return v.colors, true
	}
	return "", false
}

// With LS_COLORS set, ls colours as GNU's does with it, which is what Ubuntu's ~/.bashrc sets: a
// type's code, or a suffix's for a file that is no program, matched without regard to case, the
// last given for a suffix given twice; nothing for a file neither names; \e[0m after each, and
// once before the first. An LS_COLORS it
// cannot read is GNU's warning and no colour. Unset, busybox's colours are as they were.
func TestLS_readsLsColors(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "d"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a.tar.gz", "notes.txt", "old~", "run.exe", "x.PNG"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	run := func(colors string) (string, string, error) {
		applet, _ := applets.DefaultRegistry.Lookup("ls")
		var stdout, stderr bytes.Buffer
		ctx := applets.WithProcessView(context.Background(), lsColorsView{findTestProcessView{cwd: dir}, colors})
		err := applet.Run(ctx, []string{"-1", "--color=always"}, strings.NewReader(""), &stdout, &stderr)
		return stdout.String(), stderr.String(), err
	}

	stdout, stderr, err := run("rs=0:di=01;34:ex=01;32:*.gz=01;33:*~=00;90:*.png=01;35:*.gz=01;31:")
	want := "\033[0m\033[01;31ma.tar.gz\033[0m\n\033[01;34md\033[0m\nnotes.txt\n\033[00;90mold~\033[0m\n" +
		"\033[01;32mrun.exe\033[0m\n\033[01;35mx.PNG\033[0m\n"
	if err != nil || stdout != want {
		t.Errorf("ls with LS_COLORS = %q, %q, %v; want %q", stdout, stderr, err, want)
	}

	stdout, stderr, err = run("di")
	if err != nil || stdout != "a.tar.gz\nd\nnotes.txt\nold~\nrun.exe\nx.PNG\n" ||
		stderr != "ls: unparsable value for LS_COLORS environment variable\n" {
		t.Errorf("ls with LS_COLORS=di = %q, %q, %v; want the names plain and the warning", stdout, stderr, err)
	}
}
