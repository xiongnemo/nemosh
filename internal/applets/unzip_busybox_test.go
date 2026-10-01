package applets_test

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/applets"
)

type zipEntry struct {
	name, data    string
	method, flags uint16
	modified      time.Time
}

// zipOf is an archive of entries in order. A name ending in a slash is a directory.
func zipOf(t *testing.T, entries ...zipEntry) string {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, entry := range entries {
		header := &zip.FileHeader{Name: entry.name, Method: entry.method, Flags: entry.flags, Modified: entry.modified}
		file, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write([]byte(entry.data)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.String()
}

// unzipIn runs unzip in a directory of its own holding d.zip and y.zip, and gives what it
// wrote, its failure as the shell prints one, and the directory.
func unzipIn(t *testing.T, prepare func(dir string), args ...string) (string, string, string) {
	t.Helper()
	stamp, later := time.Date(2020, 1, 2, 3, 4, 6, 0, time.UTC), time.Date(2021, 12, 31, 23, 59, 58, 0, time.UTC)
	dir := writeSmallFixture(t, map[string]string{
		"d.zip": zipOf(t, zipEntry{name: "dir/", modified: stamp},
			zipEntry{name: "dir/f.txt", data: "in dir\n", modified: stamp},
			zipEntry{name: "s.txt", data: "stored\n", modified: later}),
		"y.zip": zipOf(t, zipEntry{name: "x.txt", data: strings.Repeat("hello ", 100), method: zip.Deflate, flags: 2, modified: stamp}),
	})
	if prepare != nil {
		prepare(dir)
	}
	stdout, stderr, err := runSmall(t, dir, "", "unzip", args...)
	if _, quiet := applets.StatusCode(err); err != nil && !quiet {
		stderr += "unzip: " + err.Error() + "\n"
	}
	return stdout, stderr, dir
}

// unzip prints what busybox's prints: `Archive:  NAME`, then `   creating:` and `  inflating:`
// on standard output, and -l's and -v's tables to the column, with the archive's DOS dates
// month first. -q leaves out the Archive line, -qq the heads and the total as well. Each
// table was measured against busybox-w32's on the same entries, but its total, which counts
// every entry where this counts the ones listed.
func TestUnzip_printsWhatBusyboxsPrints(t *testing.T) {
	const listing = "  Length      Date    Time    Name\n" +
		"---------  ---------- -----   ----\n" +
		"        0  01-02-2020 03:04   dir/\n" +
		"        7  01-02-2020 03:04   dir/f.txt\n" +
		"        7  12-31-2021 23:59   s.txt\n"
	const total = " --------                     -------\n" +
		"       14                     3 files\n"
	for _, test := range []struct {
		args []string
		want string
	}{
		{args: []string{"d.zip"}, want: "Archive:  d.zip\n   creating: dir/\n  inflating: dir/f.txt\n  inflating: s.txt\n"},
		{args: []string{"-q", "d.zip"}, want: ""},
		{args: []string{"-l", "d.zip"}, want: "Archive:  d.zip\n" + listing + total},
		{args: []string{"-lq", "d.zip"}, want: listing + total},
		{args: []string{"-lqq", "d.zip"}, want: strings.SplitN(listing, "\n", 3)[2]},
		{args: []string{"-v", "d.zip"}, want: "Archive:  d.zip\n" +
			" Length   Method    Size  Cmpr    Date    Time   CRC-32   Name\n" +
			"--------  ------  ------- ---- ---------- ----- --------  ----\n" +
			"       0  Stored        0   0% 01-02-2020 03:04 00000000  dir/\n" +
			"       7  Stored        7   0% 01-02-2020 03:04 7ee3f5f7  dir/f.txt\n" +
			"       7  Stored        7   0% 12-31-2021 23:59 a5539ce2  s.txt\n" +
			"--------          ------- ----                            ----\n" +
			"      14               14   0%                            3 files\n"},
		{args: []string{"d"}, want: "Archive:  d.zip\n   creating: dir/\n  inflating: dir/f.txt\n  inflating: s.txt\n"},
	} {
		if got, err, _ := unzipIn(t, nil, test.args...); got != test.want || err != "" {
			t.Errorf("unzip %q = %q, %q; want %q", test.args, got, err, test.want)
		}
	}
	if got, _, _ := unzipIn(t, nil, "-v", "y.zip"); !strings.Contains(got, "  Defl:X ") {
		t.Errorf("unzip -v of a maximum deflate = %q; want Defl:X", got)
	}
}

// unzip reads its arguments as busybox's does: options anywhere, the first operand the
// archive, and the operands after -x the members left out; -x took the next argument alone.
// A pattern is fnmatch's, which a `*` crosses a slash in. -j makes no directory. Each answer
// was measured against busybox-w32.
func TestUnzip_takesBusyboxsArguments(t *testing.T) {
	for _, test := range []struct {
		args       []string
		want, fail string
	}{
		{args: []string{"-lqq", "-x", "d.zip", "dir/f.txt", "dir/"}, want: "        7  12-31-2021 23:59   s.txt\n"},
		{args: []string{"-lqq", "d.zip", "*.txt"}, want: "        7  01-02-2020 03:04   dir/f.txt\n        7  12-31-2021 23:59   s.txt\n"},
		{args: []string{"nope"}, fail: "unzip: cannot open nope[.zip]\n"},
		{args: []string{"-d", "new/deep", "d.zip"}, fail: "unzip: cannot change directory to 'new/deep': No such file or directory\n"},
	} {
		if got, err, _ := unzipIn(t, nil, test.args...); got != test.want || err != test.fail {
			t.Errorf("unzip %q = %q, %q; want %q, %q", test.args, got, err, test.want, test.fail)
		}
	}
	if _, err, dir := unzipIn(t, nil, "-qj", "d.zip"); err != "" {
		t.Errorf("unzip -j: %q", err)
	} else if _, err := os.Stat(filepath.Join(dir, "dir")); err == nil {
		t.Error("unzip -j made dir")
	} else if _, err := os.Stat(filepath.Join(dir, "f.txt")); err != nil {
		t.Errorf("unzip -j did not write f.txt: %v", err)
	}
	if _, err, dir := unzipIn(t, nil, "-q", "-d", "new", "d.zip"); err != "" {
		t.Errorf("unzip -d new: %q", err)
	} else if _, err := os.Stat(filepath.Join(dir, "new", "dir", "f.txt")); err != nil {
		t.Errorf("unzip -d new did not write new/dir/f.txt: %v", err)
	}
	inTheWay := func(dir string) {
		if err := os.Mkdir(filepath.Join(dir, "s.txt"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if _, err, _ := unzipIn(t, inTheWay, "-q", "d.zip"); err != "unzip: 's.txt' exists but is not a regular file\n" {
		t.Errorf("unzip over a directory named s.txt = %q", err)
	}
}
