package applets_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// tarFixture is busybox's measurement fixture: src/ holding a.txt, c.txt and sub/b.log, with
// a.txt's time set in 2020, archived as t.tar in the same directory.
func tarFixture(t *testing.T) string {
	t.Helper()
	root := writeSmallFixture(t, map[string]string{"src/a.txt": "a\n", "src/c.txt": "c\n", "src/sub/b.log": "b\n"})
	if err := os.Chtimes(filepath.Join(root, "src", "a.txt"), tarFixtureTime, tarFixtureTime); err != nil {
		t.Fatal(err)
	}
	if _, stderr, err := runSmall(t, root, "", "tar", "cf", "t.tar", "src"); err != nil {
		t.Fatalf("tar cf: %v (stderr %q)", err, stderr)
	}
	return root
}

var tarFixtureTime = time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)

// extractedNames is every name under dir, slash-separated and sorted, as `find . | sort` has it.
func extractedNames(t *testing.T, dir string) []string {
	t.Helper()
	var names []string
	err := filepath.WalkDir(dir, func(current string, _ os.DirEntry, err error) error {
		if err != nil || current == dir {
			return err
		}
		relative, err := filepath.Rel(dir, current)
		names = append(names, filepath.ToSlash(relative))
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(names)
	return names
}

// An empty directory beside the fixture's archive, to extract into.
func tarTarget(t *testing.T, root, name string) string {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// Listing and extracting take the names given, each a pattern matched against as many leading
// components as it has, and what is under them. A name that took nothing is said, and the
// status is 1. Every answer but the last is busybox-w32's, measured: busybox fails `src/*.txt`
// as not found having listed what it matched, because it asks whether an entry it took is
// spelled like the name.
func TestTar_takesTheNamesGiven(t *testing.T) {
	root := tarFixture(t)
	for _, test := range []struct {
		args           []string
		stdout, stderr string
		status         int
	}{
		{args: []string{"tf", "t.tar", "src/sub"}, stdout: "src/sub/\nsrc/sub/b.log\n"},
		{args: []string{"tf", "t.tar", "src/sub/"}, stdout: "src/sub/\nsrc/sub/b.log\n"},
		{args: []string{"tf", "t.tar", "src/nope", "src/c.txt"}, stdout: "src/c.txt\n",
			stderr: "tar: src/nope: not found in archive\n", status: 1},
		{args: []string{"tf", "t.tar", "--exclude", "src/sub"}, stdout: "src/\nsrc/a.txt\nsrc/c.txt\n"},
		// An exclusion is matched against a first component, so *.log leaves nothing out.
		{args: []string{"tf", "t.tar", "--exclude=*.log"}, stdout: "src/\nsrc/a.txt\nsrc/c.txt\nsrc/sub/\nsrc/sub/b.log\n"},
		// A name an exclusion matches took nothing, and that is no failure.
		{args: []string{"tf", "t.tar", "--exclude", "src/c.txt", "src/c.txt", "src/a.txt"}, stdout: "src/a.txt\n"},
		// Listing shows the names whole: --strip-components shortens them as they are written.
		{args: []string{"tf", "t.tar", "--strip-components", "1", "src/a.txt"}, stdout: "src/a.txt\n"},
		{args: []string{"tf", "t.tar", "src/*.txt"}, stdout: "src/a.txt\nsrc/c.txt\n"},
	} {
		stdout, stderr, err := runSmall(t, root, "", "tar", test.args...)
		status, ok := applets.StatusCode(err)
		if err != nil && !ok {
			status = -1
		}
		if stdout != test.stdout || stderr != test.stderr || status != test.status {
			t.Errorf("tar %q = %q, %q, %v; want %q, %q, status %d", test.args, stdout, stderr, err,
				test.stdout, test.stderr, test.status)
		}
	}
}

// -T reads the names from a file, or from stdin for `-`, one a line, a trailing slash off each.
// An empty line is a name too, one that is not there, as it is to busybox.
func TestTar_readsNamesFromAFile(t *testing.T) {
	root := tarFixture(t)
	if err := os.WriteFile(filepath.Join(root, "list"), []byte("src/a.txt\n\nsrc/sub/\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, err := runSmall(t, root, "", "tar", "tf", "t.tar", "-T", "list")
	if status, _ := applets.StatusCode(err); stdout != "src/a.txt\nsrc/sub/\nsrc/sub/b.log\n" ||
		stderr != "tar: : not found in archive\n" || status != 1 {
		t.Errorf("tar tf -T list = %q, %q, %v", stdout, stderr, err)
	}
	stdout, _, err = runSmall(t, root, "src/c.txt\r\n", "tar", "--list", "--file=t.tar", "--files-from", "-")
	if stdout != "src/c.txt\n" || err != nil {
		t.Errorf("tar --list --files-from - = %q, %v; want src/c.txt", stdout, err)
	}
}

// Extracting a name takes it alone, its directories made for it.
func TestTar_extractsTheNameGivenAlone(t *testing.T) {
	root := tarFixture(t)
	target := tarTarget(t, root, "o")
	if _, stderr, err := runSmall(t, target, "", "tar", "xf", "../t.tar", "src/sub/b.log"); err != nil {
		t.Fatalf("tar xf src/sub/b.log: %v (stderr %q)", err, stderr)
	}
	if got := extractedNames(t, target); !slices.Equal(got, []string{"src", "src/sub", "src/sub/b.log"}) {
		t.Errorf("extracted %q; want src/sub/b.log and its directories", got)
	}
}

// --strip-components takes leading components off each name as it is written, an entry that is
// all leading components passed over; the names given select by the names the archive holds.
func TestTar_stripsComponentsAsItExtracts(t *testing.T) {
	root := tarFixture(t)
	all := tarTarget(t, root, "all")
	if _, stderr, err := runSmall(t, all, "", "tar", "xf", "../t.tar", "--strip-components", "1"); err != nil {
		t.Fatalf("tar xf --strip-components 1: %v (stderr %q)", err, stderr)
	}
	if got := extractedNames(t, all); !slices.Equal(got, []string{"a.txt", "c.txt", "sub", "sub/b.log"}) {
		t.Errorf("extracted %q; want the four names without src/", got)
	}
	one := tarTarget(t, root, "one")
	if _, _, err := runSmall(t, one, "", "tar", "xf", "../t.tar", "--strip-components=1", "src/a.txt"); err != nil {
		t.Fatalf("tar xf --strip-components=1 src/a.txt: %v", err)
	}
	if got := extractedNames(t, one); !slices.Equal(got, []string{"a.txt"}) {
		t.Errorf("extracted %q; want a.txt", got)
	}
	_, stderr, err := runSmall(t, one, "", "tar", "xf", "../t.tar", "--strip-components", "1", "c.txt")
	if status, _ := applets.StatusCode(err); stderr != "tar: c.txt: not found in archive\n" || status != 1 {
		t.Errorf("tar xf --strip-components 1 c.txt = %q, %v; want c.txt not found", stderr, err)
	}
	if _, _, err := runSmall(t, one, "", "tar", "xf", "../t.tar", "--strip-components", "x"); err == nil ||
		err.Error() != "invalid number 'x'" {
		t.Errorf("tar --strip-components x = %v; want invalid number 'x'", err)
	}
}

// Creating leaves out what an exclusion matches at the start of any component, from -X's file
// or --exclude, and --no-recursion stores a directory without what is in it.
func TestTar_leavesOutWhatIsExcludedWhenCreating(t *testing.T) {
	root := tarFixture(t)
	if err := os.WriteFile(filepath.Join(root, "ex"), []byte("*.txt\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		args []string
		want string
	}{
		{args: []string{"cf", "c.tar", "-X", "ex", "src"}, want: "src/\nsrc/sub/\nsrc/sub/b.log\n"},
		{args: []string{"cf", "c.tar", "--exclude", "sub", "src"}, want: "src/\nsrc/a.txt\nsrc/c.txt\n"},
		{args: []string{"cf", "c.tar", "--exclude=src/sub", "src"}, want: "src/\nsrc/a.txt\nsrc/c.txt\n"},
		{args: []string{"cf", "c.tar", "--exclude-from", "ex", "--exclude", "*.log", "src"}, want: "src/\nsrc/sub/\n"},
		{args: []string{"cf", "c.tar", "--no-recursion", "src", "src/a.txt"}, want: "src/\nsrc/a.txt\n"},
	} {
		if _, stderr, err := runSmall(t, root, "", "tar", test.args...); err != nil {
			t.Fatalf("tar %q: %v (stderr %q)", test.args, err, stderr)
		}
		if stdout, _, err := runSmall(t, root, "", "tar", "tf", "c.tar"); stdout != test.want || err != nil {
			t.Errorf("tar %q stored %q, %v; want %q", test.args, stdout, err, test.want)
		}
	}
}

// What is there already is replaced, so a hard link to it keeps what it held; -k ends the
// extraction at it instead, and --keep-old --overwrite writes all the same. A directory there
// is not removed. Each answer is busybox's, but for the directory, where busybox-w32 says
// Permission denied for the EISDIR busybox gives elsewhere.
func TestTar_replacesOrKeepsWhatIsThere(t *testing.T) {
	root := tarFixture(t)
	target := tarTarget(t, root, "o")
	existing := filepath.Join(target, "src", "a.txt")
	if err := os.MkdirAll(filepath.Dir(existing), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(existing, []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, err := runSmall(t, target, "", "tar", "xkf", "../t.tar")
	if data, _ := os.ReadFile(existing); err == nil || err.Error() != "cannot open 'src/a.txt': File exists" || string(data) != "old\n" {
		t.Errorf("tar xkf over src/a.txt = %v, left %q; want File exists and the old file", err, data)
	}
	link := filepath.Join(root, "link")
	linked := os.Link(existing, link) == nil
	if _, _, err := runSmall(t, target, "", "tar", "xf", "../t.tar"); err != nil {
		t.Fatalf("tar xf over src/a.txt: %v", err)
	}
	if data, _ := os.ReadFile(existing); string(data) != "a\n" {
		t.Errorf("tar xf left src/a.txt holding %q; want what the archive holds", data)
	}
	if data, _ := os.ReadFile(link); linked && string(data) != "old\n" {
		t.Errorf("tar xf wrote into a hard link to src/a.txt (%q); it replaces the file", data)
	}
	if _, _, err := runSmall(t, target, "", "tar", "--keep-old", "--overwrite", "-xf", "../t.tar"); err != nil {
		t.Errorf("tar --keep-old --overwrite: %v; --overwrite wins", err)
	}
	if err := os.Remove(existing); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(existing, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runSmall(t, target, "", "tar", "xf", "../t.tar"); err == nil ||
		err.Error() != "cannot remove old file src/a.txt: Is a directory" {
		t.Errorf("tar xf over a directory = %v; want cannot remove old file src/a.txt: Is a directory", err)
	}
}

// --overwrite writes into the file a hard link shares, where replacing leaves the link alone.
func TestTar_overwriteWritesIntoALinkedFile(t *testing.T) {
	root := tarFixture(t)
	target := tarTarget(t, root, "o")
	existing := filepath.Join(target, "src", "a.txt")
	if err := os.MkdirAll(filepath.Dir(existing), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(existing, []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Link(existing, link); err != nil {
		t.Skipf("no hard link here: %v", err)
	}
	if _, _, err := runSmall(t, target, "", "tar", "xf", "../t.tar", "--overwrite"); err != nil {
		t.Fatalf("tar xf --overwrite: %v", err)
	}
	if data, _ := os.ReadFile(link); string(data) != "a\n" {
		t.Errorf("tar xf --overwrite left the link holding %q; want what the archive holds", data)
	}
}

// A file extracted has the time the archive gives it, and -m leaves it the time it was made.
func TestTar_restoresModificationTimes(t *testing.T) {
	root := tarFixture(t)
	for _, test := range []struct {
		options  string
		restored bool
	}{{options: "xf", restored: true}, {options: "xmf"}} {
		target := tarTarget(t, root, test.options)
		if _, stderr, err := runSmall(t, target, "", "tar", test.options, "../t.tar"); err != nil {
			t.Fatalf("tar %s: %v (stderr %q)", test.options, err, stderr)
		}
		info, err := os.Stat(filepath.Join(target, "src", "a.txt"))
		if err != nil {
			t.Fatal(err)
		}
		if restored := info.ModTime().Equal(tarFixtureTime); restored != test.restored {
			t.Errorf("tar %s made src/a.txt at %v; restored should be %v", test.options, info.ModTime(), test.restored)
		}
	}
}

// A symbolic link is stored as one, with its target; -h stores what it points at, a
// directory's contents too.
func TestTar_storesALinkOrWhatItPointsAt(t *testing.T) {
	root := writeSmallFixture(t, map[string]string{"d/inner/f.txt": "x\n"})
	if err := os.Symlink("inner", filepath.Join(root, "d", "link")); err != nil {
		t.Skipf("no symbolic link here: %v", err)
	}
	for _, test := range []struct {
		options, want string
	}{
		{options: "cf", want: "d/\nd/inner/\nd/inner/f.txt\nd/link\n"},
		{options: "chf", want: "d/\nd/inner/\nd/inner/f.txt\nd/link/\nd/link/f.txt\n"},
	} {
		if _, stderr, err := runSmall(t, root, "", "tar", test.options, "a.tar", "d"); err != nil {
			t.Fatalf("tar %s: %v (stderr %q)", test.options, err, stderr)
		}
		if stdout, _, err := runSmall(t, root, "", "tar", "tf", "a.tar"); stdout != test.want || err != nil {
			t.Errorf("tar %s stored %q, %v; want %q", test.options, stdout, err, test.want)
		}
	}
}

// The long options are busybox's, each of a letter but for its own few.
func TestTar_takesBusyboxsLongOptions(t *testing.T) {
	root := tarFixture(t)
	target := tarTarget(t, root, "o")
	if _, stderr, err := runSmall(t, root, "", "tar", "--extract", "--file", "t.tar", "--directory=o", "--touch",
		"--no-same-owner", "--numeric-owner", "--no-same-permissions", "src/c.txt"); err != nil {
		t.Fatalf("tar --extract: %v (stderr %q)", err, stderr)
	}
	if got := extractedNames(t, target); !slices.Equal(got, []string{"src", "src/c.txt"}) {
		t.Errorf("extracted %q; want src/c.txt", got)
	}
	for _, test := range []struct {
		args []string
		want string
	}{
		{args: []string{"--bogus"}, want: "unknown option -- bogus"},
		{args: []string{"--list=1"}, want: "unknown option -- list=1"},
		{args: []string{"-t", "--exclude"}, want: "option '--exclude' requires an argument"},
	} {
		if _, _, err := runSmall(t, root, "", "tar", test.args...); err == nil || err.Error() != test.want {
			t.Errorf("tar %q = %v; want %s", test.args, err, test.want)
		}
	}
}
