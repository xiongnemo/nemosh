package applets_test

import (
	"os"
	"path/filepath"
	"testing"
)

// A first argument with no dash is tar's options, as every tar reads it and busybox's does:
// `tar cf a.tar s`, `tar czf`, `tar xzf a.tgz -C o`, and f's value is the next argument even
// when letters follow f, so `tar fx a.tar` extracts. Each was an operand, and tar said that one
// of -c, -t or -x was required. Each answer is busybox-w32's, measured.
func TestTar_readsAFirstArgumentWithoutADashAsItsOptions(t *testing.T) {
	root := writeSmallFixture(t, map[string]string{"s/f": "x\n"})
	for _, args := range [][]string{
		{"cf", "a.tar", "s"},
		{"czf", "a.tgz", "s"},
	} {
		if _, stderr, err := runSmall(t, root, "", "tar", args...); err != nil {
			t.Fatalf("tar %q: %v (stderr %q)", args, err, stderr)
		}
	}
	if stdout, _, err := runSmall(t, root, "", "tar", "tf", "a.tar"); err != nil || stdout != "s/\ns/f\n" {
		t.Errorf("tar tf a.tar = %q, %v; want the two names", stdout, err)
	}
	if err := os.Mkdir(filepath.Join(root, "o"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, stderr, err := runSmall(t, root, "", "tar", "xzf", "a.tgz", "-C", "o"); err != nil {
		t.Fatalf("tar xzf: %v (stderr %q)", err, stderr)
	}
	if _, stderr, err := runSmall(t, filepath.Join(root, "o"), "", "tar", "fx", "../a.tar"); err != nil {
		t.Fatalf("tar fx: %v (stderr %q)", err, stderr)
	}
	if data, err := os.ReadFile(filepath.Join(root, "o", "s", "f")); err != nil || string(data) != "x\n" {
		t.Errorf("o/s/f holds %q, %v; want x", data, err)
	}
}
