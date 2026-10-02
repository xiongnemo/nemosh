package applets_test

import (
	"strings"
	"testing"
)

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

// cpio chooses what to do as busybox's does, with no exclusion among the three: -o creates,
// and needs -H newc to, and otherwise -t lists, though -i came too. A bare -o wrote newc,
// which neither reference writes, and a mixture was refused. -H matters to -o alone: an
// archive read is known by its magic, so -t with another -H still lists it. Each was
// measured against busybox-w32.
func TestCpio_choosesItsModeAsBusyboxDoes(t *testing.T) {
	archive := buildCpio(t, []cpioTestEntry{{name: "a.txt", content: "one\n"}})
	dir := writeSmallFixture(t, map[string]string{"t.cpio": string(archive), "a.txt": "one\n"})
	for _, test := range []struct {
		args         []string
		out, failure string
	}{
		{args: []string{"-o"}, failure: "-o requires -H newc"},
		{args: []string{"-o", "-H", "odc"}, failure: `only -H newc is written; "odc" is not a format this build produces`},
		{args: []string{"-v", "-F", "t.cpio"}, failure: "one of -t, -i or -o is required"},
		{args: []string{"-ti", "-F", "t.cpio"}, out: "a.txt\n"},
		{args: []string{"-it", "-F", "t.cpio"}, out: "a.txt\n"},
		{args: []string{"-t", "-H", "crc", "-F", "t.cpio"}, out: "a.txt\n"},
	} {
		out, _, err := runSmall(t, dir, "a.txt\n", "cpio", test.args...)
		failure := ""
		if err != nil {
			failure = err.Error()
		}
		if out != test.out || failure != test.failure {
			t.Errorf("cpio %q = %q, %q; want %q, %q", test.args, out, failure, test.out, test.failure)
		}
	}
	for _, mixture := range []string{"-ot", "-oi"} {
		out, stderr, err := runSmall(t, dir, "a.txt\n", "cpio", mixture, "-H", "newc")
		if err != nil || !strings.HasPrefix(out, "070701") {
			t.Errorf("cpio %s -H newc wrote %q, %v (%s); want an archive", mixture, out[:min(len(out), 16)], err, stderr)
		}
	}
}
