package applets_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// `cp a b c dir/` and `mv a b c dir/`, the POSIX `source_file... target_directory`
// form -- which had no test at all despite being the reason a refusal of a third
// operand was removed.
//
// The shape of the answer was measured against busybox-w32: copy what you can, name
// each source you could not, exit 1 if any failed. A fail-fast loop would leave a
// half-done copy with no record of which half, so the per-operand behaviour is the
// property, not an implementation detail.

func TestCp_copiesManySourcesIntoADirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "into"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{"a.txt": "alpha", "b.txt": "beta", "c.txt": "gamma"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if _, stderr, err := runSmall(t, root, "", "cp", "a.txt", "b.txt", "c.txt", "into"); err != nil {
		t.Fatalf("cp of three sources: %v (%s)", err, stderr)
	}
	for name, want := range map[string]string{"a.txt": "alpha", "b.txt": "beta", "c.txt": "gamma"} {
		got, err := os.ReadFile(filepath.Join(root, "into", name))
		if err != nil {
			t.Fatalf("%s did not arrive: %v", name, err)
		}
		if string(got) != want {
			t.Errorf("into/%s = %q, want %q", name, got, want)
		}
		// cp leaves the source, which is what distinguishes it from mv.
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Errorf("cp removed the source %s", name)
		}
	}
}

func TestMv_movesManySourcesIntoADirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "into"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a.txt", "b.txt"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if _, stderr, err := runSmall(t, root, "", "mv", "a.txt", "b.txt", "into"); err != nil {
		t.Fatalf("mv of two sources: %v (%s)", err, stderr)
	}
	for _, name := range []string{"a.txt", "b.txt"} {
		if got, err := os.ReadFile(filepath.Join(root, "into", name)); err != nil || string(got) != name {
			t.Errorf("into/%s = %q (%v)", name, got, err)
		}
		// mv removes the source, which is what distinguishes it from cp.
		if _, err := os.Stat(filepath.Join(root, name)); err == nil {
			t.Errorf("mv left the source %s behind", name)
		}
	}
}

// The last operand has to *be* a directory already. `cp a b c` with c a plain file
// cannot mean anything, and guessing would overwrite c three times -- so it is
// named as the failure it is rather than as an extra operand, because the operands
// are fine and the target is not.
func TestCp_refusesATargetThatIsNotADirectory(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"a.txt", "b.txt", "plain.txt"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("original "+name), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	for _, applet := range []string{"cp", "mv"} {
		t.Run(applet, func(t *testing.T) {
			_, _, err := runSmall(t, root, "", applet, "a.txt", "b.txt", "plain.txt")
			if err == nil {
				t.Fatalf("%s with three operands and a file target was accepted", applet)
			}
			if !strings.Contains(err.Error(), "not a directory") {
				t.Fatalf("%s said %q, want it named as a target that is not a directory", applet, err)
			}
			// And nothing was written: a refusal that had already overwritten the
			// target once would be worse than the guess it is avoiding.
			if got, _ := os.ReadFile(filepath.Join(root, "plain.txt")); string(got) != "original plain.txt" {
				t.Fatalf("plain.txt was overwritten anyway: %q", got)
			}
		})
	}
	// A target that does not exist at all is the same failure.
	if _, _, err := runSmall(t, root, "", "cp", "a.txt", "b.txt", "nope"); err == nil {
		t.Fatal("cp accepted a target directory that does not exist")
	}
}

// One source that cannot be read must not cost the others. busybox names each
// failure and carries on, exiting 1 at the end -- so a script reading stderr can
// tell what it still has to do.
func TestCp_carriesOnPastOneFailingSource(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "into"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"first.txt", "third.txt"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	// The middle source does not exist.
	_, stderr, err := runSmall(t, root, "", "cp", "first.txt", "missing.txt", "third.txt", "into")
	if err == nil {
		t.Fatal("cp exited 0 with a source it could not copy")
	}
	// The failing source is named, so the caller knows which one.
	if !strings.Contains(stderr, "missing.txt") {
		t.Fatalf("stderr does not name the failing source: %q", stderr)
	}
	// The sources *before and after* it both arrived, which is the whole point of
	// not failing fast: a fail-fast loop would have dropped third.txt.
	for _, name := range []string{"first.txt", "third.txt"} {
		if got, err := os.ReadFile(filepath.Join(root, "into", name)); err != nil || string(got) != name {
			t.Errorf("%s did not arrive despite an earlier failure: %q (%v)", name, got, err)
		}
	}
}

// Copying a directory into a directory needs -r, and without it the failure names
// the directory rather than being silent.
func TestCp_reportsADirectorySourceWithoutRecursion(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "into"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "adir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "adir", "inner.txt"), []byte("deep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("flat"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, stderr, err := runSmall(t, root, "", "cp", "a.txt", "adir", "into")
	if err == nil {
		t.Fatal("cp copied a directory without -r")
	}
	if !strings.Contains(stderr, "adir") {
		t.Fatalf("stderr does not name the directory: %q", stderr)
	}
	// The plain file still arrived.
	if _, err := os.Stat(filepath.Join(root, "into", "a.txt")); err != nil {
		t.Fatalf("the plain source was lost with the directory: %v", err)
	}

	// With -r the directory arrives whole.
	if _, stderr, err := runSmall(t, root, "", "cp", "-r", "adir", "into"); err != nil {
		t.Fatalf("cp -r: %v (%s)", err, stderr)
	}
	if got, err := os.ReadFile(filepath.Join(root, "into", "adir", "inner.txt")); err != nil || string(got) != "deep" {
		t.Fatalf("cp -r did not copy the tree: %q (%v)", got, err)
	}
}

// Two operands is still the ordinary form, and the target may be a name rather
// than a directory -- which is what the many-source path must not have broken.
func TestCp_twoOperandsStillNameTheDestination(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("content"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, stderr, err := runSmall(t, root, "", "cp", "a.txt", "renamed.txt"); err != nil {
		t.Fatalf("cp to a name: %v (%s)", err, stderr)
	}
	if got, err := os.ReadFile(filepath.Join(root, "renamed.txt")); err != nil || string(got) != "content" {
		t.Fatalf("renamed.txt = %q (%v)", got, err)
	}
	// And one operand is still an error, because there is nowhere for it to go.
	if _, _, err := runSmall(t, root, "", "cp", "a.txt"); err == nil {
		t.Fatal("cp with one operand was accepted")
	}
}
