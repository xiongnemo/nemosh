package runtime_test

import (
	"os"
	"path/filepath"
	"testing"
)

// `. name` looks for name on PATH before the working directory, as both references do, and
// `. file args` runs file with args for its positional parameters and gives the caller's back
// afterwards, as busybox does. The working directory's file was read, and the arguments were
// dropped.
func TestDot_searchesPathAndTakesArguments(t *testing.T) {
	directory := t.TempDir()
	bin := filepath.Join(directory, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		filepath.Join(bin, "lib.sh"):         "echo from-path \"$#:$*\"\nset -- changed\n",
		filepath.Join(directory, "lib.sh"):   "echo from-cwd\n",
		filepath.Join(directory, "only.sh"):  "echo only-cwd \"$#:$*\"\n",
		filepath.Join(directory, "shift.sh"): "shift\n",
	}
	for name, content := range files {
		if err := os.WriteFile(name, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	script := "cd '" + filepath.ToSlash(directory) + "'\nPATH='" + filepath.ToSlash(bin) + "'\nset -- p q\n" +
		". lib.sh x y\necho \"after: $#:$*\"\n" +
		". only.sh\n" +
		// Given no arguments, the file shares the caller's parameters.
		". ./shift.sh\necho \"shared: $#:$*\"\n" +
		"shopt -u sourcepath\n. lib.sh\n"

	// When
	status, stdout, stderr := runSetScript(t, script)

	// Then
	want := "from-path 2:x y\nafter: 2:p q\nonly-cwd 2:p q\nshared: 1:q\nfrom-cwd\n"
	if stdout != want || status != 0 {
		t.Fatalf("got %q/%d, want %q/0; stderr = %q", stdout, status, want, stderr)
	}
}
