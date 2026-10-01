package applets_test

import "testing"

// cpio takes busybox's long options, by any prefix that names one alone: --list, --extract,
// --create, --file=, --format=, --verbose, --null, and --quiet, which leaves out the count of
// blocks read, and --to-stdout, which writes each file's data to stdout and makes nothing.
// They were unrecognized. A pattern is fnmatch's, which a `*` crosses a slash in, as busybox's
// filter_accept_list matches it. Each answer was measured against busybox-w32.
func TestCpio_takesBusyboxsLongOptions(t *testing.T) {
	archive := buildCpio(t, []cpioTestEntry{
		{name: "a.txt", content: "one\n"},
		{name: "sub/b.txt", content: "two\n"},
		{name: "sub", mode: 0o040755},
	})
	dir := writeSmallFixture(t, map[string]string{"t.cpio": string(archive)})
	for _, test := range []struct {
		args     []string
		out, err string
	}{
		{args: []string{"--list", "--file=t.cpio"}, out: "a.txt\nsub/b.txt\nsub\n", err: "1 blocks\n"},
		{args: []string{"-t", "--quiet", "-F", "t.cpio"}, out: "a.txt\nsub/b.txt\nsub\n"},
		{args: []string{"-t", "--qu", "-F", "t.cpio", "*.txt"}, out: "a.txt\nsub/b.txt\n"},
		{args: []string{"--extract", "--to-stdout", "--quiet", "--file", "t.cpio"}, out: "one\ntwo\n"},
		{args: []string{"-i", "--to-stdout", "-F", "t.cpio", "sub/*"}, out: "two\n", err: "1 blocks\n"},
	} {
		out, err, failure := runSmall(t, dir, "", "cpio", test.args...)
		if out != test.out || err != test.err || failure != nil {
			t.Errorf("cpio %q = %q, %q, %v; want %q, %q", test.args, out, err, failure, test.out, test.err)
		}
	}
}
