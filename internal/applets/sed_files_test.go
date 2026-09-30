package applets_test

import (
	"os"
	"path/filepath"
	"testing"
)

// sed's r, w and s///w are busybox's: r queues a file's lines for the end of the cycle and
// takes one address, w writes the pattern space to a file made empty when the run starts, and
// a FILE runs to the end of the line. They were "unsupported command". Each answer is
// busybox-w32's, measured.
func TestSed_readsAndWritesFilesAsBusyboxDoes(t *testing.T) {
	dir := t.TempDir()
	view := permuteTestView{cwd: dir}
	for name, text := range map[string]string{"kv": "k1=v1\nk2=v2\n", "ins": "INS\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	read := func(name string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return string(data)
	}
	for _, test := range []struct {
		args           []string
		stdout         string
		file, fileText string
	}{
		{[]string{"sed", "/k1/r ins", "kv"}, "k1=v1\nINS\nk2=v2\n", "", ""},
		{[]string{"sed", "$r ins", "kv"}, "k1=v1\nk2=v2\nINS\n", "", ""},
		{[]string{"sed", "/k1/r nosuch", "kv"}, "k1=v1\nk2=v2\n", "", ""},
		{[]string{"sed", "-n", "/k1/w out1", "kv"}, "", "out1", "k1=v1\n"},
		{[]string{"sed", "s/k/K/w out2", "kv"}, "K1=v1\nK2=v2\n", "out2", "K1=v1\nK2=v2\n"},
		{[]string{"sed", "-n", "s/1/ONE/pw out3", "kv"}, "kONE=v1\n", "out3", "kONE=v1\n"},
		{[]string{"sed", "-n", "/zz/w out4", "kv"}, "", "out4", ""},
		{[]string{"sed", "w out5\ns/k/Q/", "kv"}, "Q1=v1\nQ2=v2\n", "out5", "k1=v1\nk2=v2\n"},
	} {
		stdout, stderr, err := runPermuted(t, view, "", test.args...)
		if err != nil || stdout != test.stdout {
			t.Errorf("%q: got %q, %q, %v; want %q", test.args, stdout, stderr, err, test.stdout)
			continue
		}
		if test.file != "" {
			if got := read(test.file); got != test.fileText {
				t.Errorf("%q: %s holds %q, want %q", test.args, test.file, got, test.fileText)
			}
		}
	}
	for _, args := range [][]string{{"sed", "1,2r ins", "kv"}, {"sed", "r", "kv"}} {
		if _, _, err := runPermuted(t, view, "", args...); err == nil {
			t.Errorf("%q: accepted; want it refused, as busybox refuses it", args)
		}
	}
}

// l writes the pattern space unambiguously, as POSIX has it: a backslash doubled, the C
// escapes, octal for a byte that is no printable character, a split with a backslash past 69
// characters, and $ at the end. It was "unsupported command l".
func TestSed_listsThePatternSpaceUnambiguously(t *testing.T) {
	long := ""
	for range 80 {
		long += "x"
	}
	for _, test := range []struct{ input, want string }{
		{"a\tb\\c\x01\n", "a\\tb\\\\c\\001$\n"},
		{"café\n", "café$\n"},
		{long + "\n", long[:69] + "\\\n" + long[69:] + "$\n"},
	} {
		stdout, _, err := runAppletWithInput(t, test.input, "sed", "-n", "l")
		if err != nil || stdout != test.want {
			t.Errorf("sed -n l over %q = %q (err %v), want %q", test.input, stdout, err, test.want)
		}
	}
}
