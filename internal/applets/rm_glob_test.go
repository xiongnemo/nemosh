package applets_test

import (
	"testing"
)

// rm -f says nothing of an operand that is not there, a glob that matched nothing too:
// `rm -f *.tmp` with no .tmp left leaves `*.tmp` as the operand. Windows cannot hold a `*` in a
// name, so asking about one fails with ERROR_INVALID_NAME rather than not-found, and rm took
// that for a failure: `cannot stat '*.tmp'` and status 1, where busybox-w32's stat calls it not
// there, as it calls every failure but a few. Without -f the operand is named, as busybox does.
func TestRm_forceIsQuietForAGlobThatMatchedNothing(t *testing.T) {
	view := permuteTestView{cwd: t.TempDir()}
	if _, stderr, err := runPermuted(t, view, "", "rm", "-f", "*.tmp", "x??"); err != nil || stderr != "" {
		t.Errorf("rm -f '*.tmp' 'x??': %q, %v; want silence and status 0", stderr, err)
	}
	_, stderr, err := runPermuted(t, view, "", "rm", "*.tmp")
	if want := "rm: cannot remove '*.tmp': No such file or directory\n"; stderr != want || err == nil {
		t.Errorf("rm '*.tmp': %q, %v; want %q and status 1", stderr, err, want)
	}
}
