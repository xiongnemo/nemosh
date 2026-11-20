package applets

import (
	"os"
	"path/filepath"
	"testing"
)

// The stream that writes GNU's first \e[0m names the file it writes to, so that ls, asking it
// whether it is a terminal and how wide, is answered by the terminal: wrapped, `ls` in a session
// on a console laid out one name a line, and `?` stood for nothing it could not show.
func TestLsFirstColor_namesTheFileItWritesTo(t *testing.T) {
	file, err := os.Create(filepath.Join(t.TempDir(), "out"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	if got := stdoutFile(&lsFirstColor{Writer: file}); got != file {
		t.Fatalf("stdoutFile(lsFirstColor over %s) = %v, want the file", file.Name(), got)
	}
}
