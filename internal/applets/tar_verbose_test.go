package applets_test

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeTarArchive writes an archive of the headers given, each file holding its name's last
// letter as many times as its size says, at path.
func writeTarArchive(t *testing.T, path string, headers ...*tar.Header) {
	t.Helper()
	var buffer bytes.Buffer
	archive := tar.NewWriter(&buffer)
	for _, header := range headers {
		if err := archive.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if header.Typeflag == tar.TypeReg {
			if _, err := archive.Write(bytes.Repeat([]byte{header.Name[len(header.Name)-1]}, int(header.Size))); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buffer.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
}

// verboseFixture is an archive with an owner named and a group by number, at a time of day
// that reads the same wherever the test runs.
func verboseFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	when := time.Date(2020, 1, 2, 3, 4, 5, 0, time.Local)
	writeTarArchive(t, filepath.Join(root, "a.tar"),
		&tar.Header{Name: "src/", Typeflag: tar.TypeDir, Mode: 0o755, Uid: 1000, Gid: 100, Uname: "nemo", ModTime: when},
		&tar.Header{Name: "src/a.txt", Typeflag: tar.TypeReg, Mode: 0o644, Size: 2, Uid: 1000, Gid: 100, Uname: "nemo", ModTime: when},
		&tar.Header{Name: "src/l", Typeflag: tar.TypeSymlink, Linkname: "a.txt", Mode: 0o777, Uid: 1000, Gid: 100, ModTime: when},
	)
	return root
}

// -t names each entry, and -tv, or a second -v, gives busybox's long line: the mode as ls
// has it, the owner and group by name or else by number, the size, the local time, the name,
// and where a link points. It was Go's mode, the size and the name.
func TestTar_listsAsBusyboxDoes(t *testing.T) {
	root := verboseFixture(t)
	long := "drwxr-xr-x nemo/100         0 2020-01-02 03:04:05 src/\n" +
		"-rw-r--r-- nemo/100         2 2020-01-02 03:04:05 src/a.txt\n" +
		"lrwxrwxrwx 1000/100         0 2020-01-02 03:04:05 src/l -> a.txt\n"
	for _, test := range []struct {
		args []string
		want string
	}{
		{args: []string{"tf", "a.tar"}, want: "src/\nsrc/a.txt\nsrc/l\n"},
		{args: []string{"tvf", "a.tar"}, want: long},
		{args: []string{"-t", "-t", "-f", "a.tar"}, want: long},
	} {
		if stdout, stderr, err := runSmall(t, root, "", "tar", test.args...); stdout != test.want || stderr != "" || err != nil {
			t.Errorf("tar %q = %q, %q, %v; want %q", test.args, stdout, stderr, err, test.want)
		}
	}
}

// Extracting, -v names each entry on stdout, and -vv gives the long line; with -O the names go
// to stderr, which busybox does not do: it mixes them into the data.
func TestTar_namesWhatItExtractsOnStdout(t *testing.T) {
	root := verboseFixture(t)
	target := filepath.Join(root, "o")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	stdout, _, err := runSmall(t, root, "", "tar", "xvf", "a.tar", "-C", "o", "src/a.txt")
	if stdout != "src/a.txt\n" || err != nil {
		t.Errorf("tar xvf = %q, %v; want src/a.txt named", stdout, err)
	}
	stdout, _, err = runSmall(t, root, "", "tar", "xvvf", "a.tar", "-C", "o", "src/a.txt")
	if stdout != "-rw-r--r-- nemo/100         2 2020-01-02 03:04:05 src/a.txt\n" || err != nil {
		t.Errorf("tar xvvf = %q, %v; want the long line", stdout, err)
	}
	stdout, stderr, err := runSmall(t, root, "", "tar", "xvOf", "a.tar", "src/a.txt")
	if stdout != "tt" || stderr != "src/a.txt\n" || err != nil {
		t.Errorf("tar xvOf = %q, %q, %v; want the data alone on stdout", stdout, stderr, err)
	}
}

// Creating, -v names each entry on stdout, but on stderr when the archive goes to stdout, as
// busybox puts them.
func TestTar_namesWhatItArchives(t *testing.T) {
	root := writeSmallFixture(t, map[string]string{"src/a.txt": "a\n"})
	stdout, stderr, err := runSmall(t, root, "", "tar", "cvf", "b.tar", "src")
	if stdout != "src/\nsrc/a.txt\n" || stderr != "" || err != nil {
		t.Errorf("tar cvf b.tar = %q, %q, %v; want the names on stdout", stdout, stderr, err)
	}
	stdout, stderr, err = runSmall(t, root, "", "tar", "cvf", "-", "src")
	if stderr != "src/\nsrc/a.txt\n" || len(stdout) < 1024 || err != nil {
		t.Errorf("tar cvf - = %d bytes, %q, %v; want the names on stderr", len(stdout), stderr, err)
	}
}
