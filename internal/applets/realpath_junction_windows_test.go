//go:build windows

package applets_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// realpath follows a junction to the directory it leads to, as busybox-w32's does. It
// answered the junction: filepath.EvalSymlinks leaves one as it is.
func TestDefaultRegistry_followsAJunction_whenRealpathRuns(t *testing.T) {
	// Given
	dir := t.TempDir()
	real, link := filepath.Join(dir, "real"), filepath.Join(dir, "link")
	mkdirRealpathFixture(t, real)
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, real).CombinedOutput(); err != nil {
		t.Skipf("no junction can be made here: %v %s", err, out)
	}
	applet := lookupRealpath(t)
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{link, link + string(os.PathSeparator) + "."},
		&bytes.Buffer{}, &stdout, &bytes.Buffer{})

	// Then
	want := slashAbs(t, real) + "\n" + slashAbs(t, real) + "\n"
	if err != nil || stdout.String() != want {
		t.Fatalf("got %q (err %v), want %q", stdout.String(), err, want)
	}
}
