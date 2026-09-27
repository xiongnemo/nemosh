package runtime_test

import (
	"fmt"
	"path/filepath"
	"testing"
)

// `<>` opens a file for reading and writing, and the descriptor reads as well as writes. It
// was bound as a writer alone, so `exec 3<> f; read <&3` and `while read l; do ...; done <> f`
// answered "file descriptor is not readable". A read and a write through it share one
// offset: the `.` goes after the byte read. busybox-w32 and bash agree on all of it.
func TestRuntime_readWriteRedirectReads(t *testing.T) {
	script := fmt.Sprintf("cd '%s'\n", filepath.ToSlash(t.TempDir())) +
		"echo foo > f\nexec 3<> f\nread -n 1 x <&3\necho -n . >&3\nexec 3>&-\ncat f\n" +
		"printf 'a\\nb\\n' > g\nwhile read l; do echo \"<$l>\"; done <> g\n" +
		"exec 4<> h\necho new >&4\nexec 4>&-\ncat h\n"
	want := "f.o\n<a>\n<b>\nnew\n"
	if stdout, status := runScriptCapturing(script); stdout != want || status != 0 {
		t.Errorf("got %q/%d, want %q/0, as both references answer", stdout, status, want)
	}
}
