package applets_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Copying a directory was the one thing cp could not do, and nothing else in the
// bundle could do it either -- no combination of applets copies a tree.
//
// The shape is busybox's, measured: without -r a directory answers
// `cp: omitting directory 'src'` and exits 1; with -r and a destination that
// does not exist, dst becomes the copy; with a destination that already exists
// as a directory, the copy lands at dst/src.
func TestCp_copiesATree(t *testing.T) {
	seed := func(t *testing.T) (string, string) {
		t.Helper()
		root := t.TempDir()
		source := filepath.Join(root, "src")
		if err := os.MkdirAll(filepath.Join(source, "sub"), 0o755); err != nil {
			t.Fatal(err)
		}
		for path, body := range map[string]string{
			filepath.Join(source, "a.txt"):        "hi\n",
			filepath.Join(source, "sub", "b.txt"): "deep\n",
		} {
			if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return root, source
	}

	t.Run("a directory needs -r", func(t *testing.T) {
		// Given / When
		root, source := seed(t)
		_, _, err := runAppletWithInput(t, "", "cp", source, filepath.Join(root, "dst"))

		// Then
		if err == nil || !strings.Contains(err.Error(), "omitting directory") {
			t.Fatalf("err = %v, want it to refuse the directory by name", err)
		}
		if _, statErr := os.Stat(filepath.Join(root, "dst")); statErr == nil {
			t.Fatal("it copied something anyway")
		}
	})

	t.Run("into a destination that does not exist", func(t *testing.T) {
		// Given / When
		root, source := seed(t)
		destination := filepath.Join(root, "dst")
		if _, _, err := runAppletWithInput(t, "", "cp", "-r", source, destination); err != nil {
			t.Fatalf("cp -r = %v", err)
		}

		// Then: dst *is* the copy, so the file is at dst/a.txt.
		assertFile(t, filepath.Join(destination, "a.txt"), "hi\n")
		assertFile(t, filepath.Join(destination, "sub", "b.txt"), "deep\n")
	})

	t.Run("into a destination that already exists", func(t *testing.T) {
		// Given
		root, source := seed(t)
		destination := filepath.Join(root, "dst")
		if err := os.Mkdir(destination, 0o755); err != nil {
			t.Fatal(err)
		}

		// When
		if _, _, err := runAppletWithInput(t, "", "cp", "-r", source, destination); err != nil {
			t.Fatalf("cp -r = %v", err)
		}

		// Then: the copy lands under it, at dst/src.
		assertFile(t, filepath.Join(destination, "src", "a.txt"), "hi\n")
		assertFile(t, filepath.Join(destination, "src", "sub", "b.txt"), "deep\n")
	})

	t.Run("-R means the same thing", func(t *testing.T) {
		// Given / When
		root, source := seed(t)
		destination := filepath.Join(root, "dst")
		if _, _, err := runAppletWithInput(t, "", "cp", "-R", source, destination); err != nil {
			t.Fatalf("cp -R = %v", err)
		}

		// Then
		assertFile(t, filepath.Join(destination, "a.txt"), "hi\n")
	})

	t.Run("a plain file still copies without -r", func(t *testing.T) {
		// Given / When
		root, source := seed(t)
		destination := filepath.Join(root, "copy.txt")
		if _, _, err := runAppletWithInput(t, "", "cp", filepath.Join(source, "a.txt"), destination); err != nil {
			t.Fatalf("cp = %v", err)
		}

		// Then
		assertFile(t, destination, "hi\n")
	})
}

// `cat -n` numbers across operands rather than restarting per file, which is
// what makes `cat -n a b` read as one document.
func TestCat_numbersLines(t *testing.T) {
	// Given
	directory := t.TempDir()
	for name, body := range map[string]string{"a.txt": "one\ntwo\n", "b.txt": "three\n"} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// When
	stdout, _, err := runAppletWithInput(t, "", "cat", "-n",
		filepath.Join(directory, "a.txt"), filepath.Join(directory, "b.txt"))

	// Then
	if err != nil {
		t.Fatalf("cat -n = %v", err)
	}
	want := "     1\tone\n     2\ttwo\n     3\tthree\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}

	// And without it, the stream is copied untouched.
	plain, _, err := runAppletWithInput(t, "", "cat", filepath.Join(directory, "a.txt"))
	if err != nil || plain != "one\ntwo\n" {
		t.Fatalf("cat = %q, %v; want the file unchanged", plain, err)
	}
}

// `head -c` counts bytes, which is what a script reaches for to take the first
// part of something that has no lines worth speaking of.
func TestHead_takesBytes(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{name: "bytes", args: []string{"-c", "5"}, want: "hello"},
		{name: "more bytes than there are", args: []string{"-c", "500"}, want: "hello\nworld\n"},
		{name: "lines still work", args: []string{"-n", "1"}, want: "hello\n"},
		{name: "the last one given wins", args: []string{"-c", "5", "-n", "1"}, want: "hello\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			// When
			stdout, _, err := runAppletWithInput(t, "hello\nworld\n", "head", test.args...)

			// Then
			if err != nil {
				t.Fatalf("head %v = %v", test.args, err)
			}
			if stdout != test.want {
				t.Fatalf("stdout = %q, want %q", stdout, test.want)
			}
		})
	}
}

// `uniq -c` is the tally half of `sort | uniq -c | sort -rn`.
func TestUniq_countsRuns(t *testing.T) {
	// When
	stdout, _, err := runAppletWithInput(t, "a\na\nb\na\n", "uniq", "-c")

	// Then
	if err != nil {
		t.Fatalf("uniq -c = %v", err)
	}
	want := "      2 a\n      1 b\n      1 a\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

// `basename -a` changes what the operands mean: every one is a path, where
// without it the second is a suffix to strip from the first.
func TestBasename_takesSeveralOperands(t *testing.T) {
	// When
	several, _, err := runAppletWithInput(t, "", "basename", "-a", "/a/one.txt", "/b/two.txt")
	if err != nil {
		t.Fatalf("basename -a = %v", err)
	}
	suffix, _, err := runAppletWithInput(t, "", "basename", "/a/one.txt", ".txt")
	if err != nil {
		t.Fatalf("basename = %v", err)
	}

	// Then
	if want := "one.txt\ntwo.txt\n"; several != want {
		t.Fatalf("basename -a = %q, want %q", several, want)
	}
	if want := "one\n"; suffix != want {
		t.Fatalf("basename with a suffix = %q, want %q", suffix, want)
	}
}

// `mv -f` is accepted and changes nothing, because nothing here prompts: this mv
// overwrites either way, so -f asks for what is already in force. Scripts carry
// it constantly and were failing at a request that had already been granted.
func TestMv_acceptsForce(t *testing.T) {
	// Given
	directory := t.TempDir()
	source := filepath.Join(directory, "a.txt")
	destination := filepath.Join(directory, "b.txt")
	for path, body := range map[string]string{source: "new\n", destination: "old\n"} {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// When
	if _, _, err := runAppletWithInput(t, "", "mv", "-f", source, destination); err != nil {
		t.Fatalf("mv -f = %v", err)
	}

	// Then
	assertFile(t, destination, "new\n")
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatalf("the source survived the move (%v)", err)
	}
}

func assertFile(t *testing.T, path, want string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if string(body) != want {
		t.Fatalf("%s holds %q, want %q", path, body, want)
	}
}
