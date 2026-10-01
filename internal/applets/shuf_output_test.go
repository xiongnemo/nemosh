package applets_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// -o writes to a FILE, opened once the input is read, so the FILE may be the input, as
// busybox's is; it was refused as an invalid option.
func TestShuf_writesToTheFileOfMinusO(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{"in.txt": "a\nb\nc\n"})
	for _, output := range []string{"out.txt", "in.txt"} {
		stdout, stderr, err := runSmall(t, dir, "", "shuf", "-o", output, "in.txt")
		if stdout != "" || stderr != "" || err != nil {
			t.Fatalf("shuf -o %s in.txt = %q, %q, %v; want nothing said", output, stdout, stderr, err)
		}
		data, err := os.ReadFile(filepath.Join(dir, output))
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
		slices.Sort(lines)
		if !slices.Equal(lines, []string{"a", "b", "c"}) {
			t.Errorf("shuf -o %s wrote %q; want a, b and c in some order", output, data)
		}
	}
	if _, _, err := runSmall(t, dir, "", "shuf", "-o", "no/such/out.txt", "in.txt"); err == nil ||
		!strings.HasPrefix(err.Error(), "cannot open 'no/such/out.txt': ") {
		t.Errorf("shuf -o into a missing directory = %v; want cannot open", err)
	}
}
