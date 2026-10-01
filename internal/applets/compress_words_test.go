package applets_test

import (
	"bytes"
	"compress/gzip"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// compressSays is what NAME shows at a prompt: its standard output, its standard error with
// an error it returns after its name, as the shell prints one, and its status.
func compressSays(t *testing.T, dir, stdin, name string, args ...string) (string, string, int) {
	t.Helper()
	stdout, stderr, err := runSmall(t, dir, stdin, name, args...)
	status, ok := applets.StatusCode(err)
	if err != nil && !ok {
		stderr, status = stderr+name+": "+err.Error()+"\n", 1
	}
	return stdout, stderr, status
}

// gunzip, zcat, bunzip2 and bzcat say what busybox's say of data they cannot read, each in
// its own name, and without the FILE's, which busybox gives to a file it cannot open alone.
// zcat reads bzip2 as well, by the data's first bytes, and what follows a member or a stream
// is passed over. Each answer was measured against busybox-w32, but the early end of a
// bzip2 stream, where busybox's bunzip2 says `bunzip error -3`, its decoder's return code.
func TestGunzip_saysWhatBusyboxSaysOfBadData(t *testing.T) {
	var member bytes.Buffer
	writer := gzip.NewWriter(&member)
	if _, err := writer.Write([]byte("hello world\n")); err != nil || writer.Close() != nil {
		t.Fatal(err)
	}
	good := member.String()
	dir := writeSmallFixture(t, map[string]string{
		"good.gz": good, "good.bz2": string(bzip2Fixture), "h.txt": "hello world\n",
		"n.gz": "garbage here\n", "n.bz2": "not bz\n", "trunc.gz": good[:12],
		"tail.gz": good + "garbage", "tail.bz2": string(bzip2Fixture) + "garbage",
	})
	for _, test := range []struct {
		stdin, applet string
		args          []string
		out, err      string
	}{
		{applet: "gunzip", err: "gunzip: invalid magic\n"},
		{stdin: "x", applet: "gunzip", err: "gunzip: invalid magic\n"},
		{stdin: good[:3], applet: "gunzip", err: "gunzip: corrupted data\n"},
		{applet: "gunzip", args: []string{"-c", "n.gz"}, err: "gunzip: invalid magic\n"},
		{applet: "gunzip", args: []string{"-t", "trunc.gz"}, err: "gunzip: unexpected end of file\n"},
		{applet: "gzip", args: []string{"-dc", "n.gz"}, err: "gzip: invalid magic\n"},
		{applet: "gunzip", args: []string{"h.txt"}, err: "gunzip: h.txt: unknown suffix - ignored\n"},
		{applet: "gunzip", args: []string{"-c", "tail.gz"}, out: "hello world\n"},
		{applet: "zcat", err: "zcat: short read\n"},
		{stdin: "garbage", applet: "zcat", err: "zcat: no gzip/bzip2/xz magic\n"},
		{applet: "zcat", args: []string{"nope.gz"}, err: "zcat: nope.gz: No such file or directory\n"},
		{applet: "zcat", args: []string{"good.gz", "good.bz2"}, out: "hello world\n" + bzip2Plain},
		{applet: "bzcat", args: []string{"n.bz2"}, err: "bzcat: invalid magic\n"},
		{applet: "bzcat", args: []string{"good.gz"}, err: "bzcat: invalid magic\n"},
		{applet: "bunzip2", err: "bunzip2: invalid magic\n"},
		{stdin: "BZh", applet: "bunzip2", err: "bunzip2: unexpected end of file\n"},
		{applet: "bzcat", args: []string{"tail.bz2"}, out: bzip2Plain},
	} {
		want := 0
		if test.err != "" {
			want = 1
		}
		out, err, status := compressSays(t, dir, test.stdin, test.applet, test.args...)
		if out != test.out || err != test.err || status != want {
			t.Errorf("%s %q of %q = %q, %q, %d; want %q, %q, %d",
				test.applet, test.args, test.stdin, out, err, status, test.out, test.err, want)
		}
	}
}
