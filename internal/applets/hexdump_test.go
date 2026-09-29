package applets_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func hexdumpFixture(t *testing.T) permuteTestView {
	t.Helper()
	dir := t.TempDir()
	for name, content := range map[string]string{
		"text":    "Hello, world!\nThe quick brown fox\n",
		"bin":     "\x00\x01\x02\x03\x7f\x80\xff\t\n\r abc",
		"three":   "abc",
		"zeros":   strings.Repeat("\x00", 64),
		"alnum":   "abcdefghijklmnopqrstuvwxyz0123456789",
		"ab":      strings.Repeat("a", 16) + strings.Repeat("b", 40),
		"fmtfile": "# a comment\n\"%07_ax\" 16/1 \" %02x\" \"\\n\"\n\n  \"%07_Ax\\n\"\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return permuteTestView{cwd: dir}
}

// hexdump and hd are busybox's, libbb's dump: each case measured against busybox-w32 and its
// output byte for byte. Every option's format is added in the order given, and -e's are
// busybox's units with every conversion it takes. hexdump printed one fixed format, -x the
// default's and -b -c -d -o od's, and had no -e -f -n or -s.
func TestHexdump_dumpsAsBusyboxsDoes(t *testing.T) {
	view := hexdumpFixture(t)
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"hexdump", "text"}, "0000000 6548 6c6c 2c6f 7720 726f 646c 0a21 6854\n0000010 2065 7571 6369 206b 7262 776f 206e 6f66\n0000020 0a78                                   \n0000022\n"},
		{[]string{"hexdump", "-b", "text"}, "0000000 110 145 154 154 157 054 040 167 157 162 154 144 041 012 124 150\n0000010 145 040 161 165 151 143 153 040 142 162 157 167 156 040 146 157\n0000020 170 012                                                        \n0000022\n"},
		{[]string{"hexdump", "-c", "text"}, "0000000   H   e   l   l   o   ,       w   o   r   l   d   !  \\n   T   h\n0000010   e       q   u   i   c   k       b   r   o   w   n       f   o\n0000020   x  \\n                                                        \n0000022\n"},
		{[]string{"hexdump", "-d", "text"}, "0000000   25928   27756   11375   30496   29295   25708   02593   26708\n0000010   08293   30065   25449   08299   29282   30575   08302   28518\n0000020   02680                                                        \n0000022\n"},
		{[]string{"hexdump", "-o", "text"}, "0000000  062510  066154  026157  073440  071157  062154  005041  064124\n0000010  020145  072561  061551  020153  071142  073557  020156  067546\n0000020  005170                                                        \n0000022\n"},
		{[]string{"hexdump", "-x", "text"}, "0000000    6548    6c6c    2c6f    7720    726f    646c    0a21    6854\n0000010    2065    7571    6369    206b    7262    776f    206e    6f66\n0000020    0a78                                                        \n0000022\n"},
		{[]string{"hexdump", "-C", "text"}, "00000000  48 65 6c 6c 6f 2c 20 77  6f 72 6c 64 21 0a 54 68  |Hello, world!.Th|\n00000010  65 20 71 75 69 63 6b 20  62 72 6f 77 6e 20 66 6f  |e quick brown fo|\n00000020  78 0a                                             |x.|\n00000022\n"},
		{[]string{"hexdump", "-c", "bin"}, "0000000  \\0 001 002 003 177 200 377  \\t  \\n  \\r       a   b   c        \n000000e\n"},
		{[]string{"hexdump", "-C", "bin"}, "00000000  00 01 02 03 7f 80 ff 09  0a 0d 20 61 62 63        |.......... abc|\n0000000e\n"},
		{[]string{"hexdump", "-C", "-C", "three"}, "00000000  61 62 63                                          |abc|\n00000000  61 62 63                                          |abc|\n00000003\n"},
		{[]string{"hexdump", "-x", "-C", "-b", "three"}, "0000000    6261    0063                                                \n00000000  61 62 63                                          |abc|\n0000000 141 142 143                                                    \n0000003\n"},
		{[]string{"hexdump", "zeros"}, "0000000 0000 0000 0000 0000 0000 0000 0000 0000\n*\n0000040\n"},
		{[]string{"hexdump", "-v", "zeros"}, "0000000 0000 0000 0000 0000 0000 0000 0000 0000\n0000010 0000 0000 0000 0000 0000 0000 0000 0000\n0000020 0000 0000 0000 0000 0000 0000 0000 0000\n0000030 0000 0000 0000 0000 0000 0000 0000 0000\n0000040\n"},
		{[]string{"hexdump", "-n", "5", "text"}, "0000000 6548 6c6c 006f                         \n0000005\n"},
		{[]string{"hexdump", "-s", "5", "text"}, "0000005 202c 6f77 6c72 2164 540a 6568 7120 6975\n0000015 6b63 6220 6f72 6e77 6620 786f 000a     \n0000022\n"},
		{[]string{"hexdump", "-s", "5", "-n", "4", "-C", "text"}, "00000005  2c 20 77 6f                                       |, wo|\n00000009\n"},
		{[]string{"hexdump", "-s", "100", "text"}, "0000022\n"},
		{[]string{"hexdump", "-n", "0", "-C", "three"}, ""},
		{[]string{"hd", "three"}, "00000000  61 62 63                                          |abc|\n00000003\n"},
		{[]string{"hd", "-C", "three"}, "00000000  61 62 63                                          |abc|\n00000000  61 62 63                                          |abc|\n00000003\n"},
		{[]string{"hd", "-n", "3", "text"}, "00000000  48 65 6c                                          |Hel|\n00000003\n"},
		{[]string{"hexdump", "-e", `16/1 "%02x "`, "three"}, "61 62 63                                       "},
		{[]string{"hexdump", "-e", `4/1 "%3_u" "\n"`, "bin"}, "nulsohstxetx\ndel 80 ff ht\n lf cr     a\n  b  c      \n"},
		{[]string{"hexdump", "-e", `1/4 "%d\n"`, "three"}, "6513249\n"},
		{[]string{"hexdump", "-e", `3/1 "%02x\n"`, "alnum"}, "61\n62\n6364\n65\n6667\n68\n696a\n6b\n6c6d\n6e\n6f70\n71\n7273\n74\n7576\n77\n7879\n7a\n3031\n32\n3334\n35\n3637\n38\n39"},
		{[]string{"hexdump", "-e", `8/1 "%02X" "\n"`, "alnum"}, "6162636465666768\n696A6B6C6D6E6F70\n7172737475767778\n797A303132333435\n36373839        \n"},
		{[]string{"hexdump", "-e", `"%_ad:" 4/1 " %02x" "\n"`, "-n", "8", "alnum"}, "0: 61 62 63 64\n4: 65 66 67 68\n"},
		{[]string{"hexdump", "-e", `"%_ao " 16/1 "%c" "\n"`, "alnum"}, "0 abcdefghijklmnop\n20 qrstuvwxyz012345\n40 6789\n"},
		{[]string{"hexdump", "-e", `2/1 "%02x "`, "-e", `"|\n"`, "three"}, "61 62|\n63   |\n"},
		{[]string{"hexdump", "-e", `"%_Ax\n"`, "-e", `4/1 "%02x" "\n"`, "three"}, "616263  \n3\n"},
		{[]string{"hexdump", "-e", `"%07_ax" 16/1 " %02x" "\n"`, "-e", `"%07_Ax\n"`, "ab"}, "0000000 61 61 61 61 61 61 61 61 61 61 61 61 61 61 61 61\n0000010 62 62 62 62 62 62 62 62 62 62 62 62 62 62 62 62\n*\n0000030 62 62 62 62 62 62 62 62                        \n0000038\n"},
		{[]string{"hexdump", "-e", `1/1 "%c"`, "-C", "three"}, "a00000000  61 62 63                                          |abc|\n00000003\n"},
		{[]string{"hexdump", "-e", `2/1 "%02x" 2/1 "%c"`, "three"}, "6162c"},
		{[]string{"hexdump", "-n", "16", "-e", `1/1 "%-3c|" 1/1 "%+d|"`, "alnum"}, "a  |+98|c  |+100|e  |+102|g  |+104|i  |+106|k  |+108|m  |+110|o  |+112|"},
		{[]string{"hexdump", "-n", "16", "-e", `1/4 "%#x " 1/4 "%#o|"`, "alnum"}, "0x64636261 015031663145|0x6c6b6a69 016033667155|"},
		{[]string{"hexdump", "-n", "16", "-e", `1/8 "%g " 1/8 "%.3e\n"`, "alnum"}, "8.54088e+194 3.904e+233\n"},
		{[]string{"hexdump", "-e", `1/8 "%lld\n"`, "-n", "8", "alnum"}, "7523094288207667809\n"},
		{[]string{"hexdump", "-e", `"%5.2s|"`, "-n", "4", "alnum"}, "   ab|   cd|"},
		{[]string{"hexdump", "-f", "fmtfile", "three"}, "0000000 61 62 63                                       \n0000003\n"},
		// busybox reads %s with a byte count past the datum, to a NUL wherever one is.
		{[]string{"hexdump", "-e", `2/3 "%s|" "\n"`, "-n", "6", "alnum"}, "abc|def|\n"},
		// C's float, where busybox-w32's msvcrt writes three digits of exponent.
		{[]string{"hexdump", "-e", `1/8 "%e\n"`, "-s", "24", "-n", "8", "alnum"}, "2.108977e-52\n"},
		// An OFFSET as long as the FILE skips it, as util-linux's does: busybox's dumps it anyway.
		{[]string{"hexdump", "-s", "3", "three"}, "0000003\n"},
	} {
		stdout, stderr, err := runPermuted(t, view, "", test.args...)
		if err != nil || stderr != "" || stdout != test.want {
			t.Errorf("%q: %q, %q, %v\n  want %q", test.args, stdout, stderr, err, test.want)
		}
	}
	// -s skips a pipe's bytes by reading them, where busybox-w32 seeks and does not move.
	stdout, _, err := runPermuted(t, view, "abcdef", "hexdump", "-s", "2", "-C")
	if want := "00000002  63 64 65 66                                       |cdef|\n00000006\n"; err != nil || stdout != want {
		t.Errorf("hexdump -s 2 -C < abcdef: %q, %v; want %q", stdout, err, want)
	}
}

// Each refusal is busybox's.
func TestHexdump_refusesAsBusyboxDoes(t *testing.T) {
	view := hexdumpFixture(t)
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"-e", "x"}, "bad format {x}"},
		{[]string{"-e", `3/1"%02x"`}, `bad format {3/1"%02x"}`},
		{[]string{"-e", `"%02x`}, `bad format {"%02x}`},
		{[]string{"-e", `"%q"`}, "bad conversion character %q"},
		{[]string{"-e", `"%_ay"`}, "bad conversion character %_ay"},
		{[]string{"-e", `1/2 "%hd"`}, "bad conversion character %hd"},
		{[]string{"-e", `1/3 "%x"`}, "bad byte count for conversion character x"},
		{[]string{"-e", `1/8 "%llx\n"`}, "bad byte count for conversion character x\n"},
		{[]string{"-e", `"%s"`}, "%s needs precision or byte count"},
		{[]string{"-e", `4/1 "%02x %c"`}, "byte count with multiple conversion characters"},
		{[]string{"-n", "1b"}, "invalid number '1b'"},
		{[]string{"-n", "2147483648"}, "number 2147483648 is not in 0..2147483647 range"},
		{[]string{"-s", "-1"}, "invalid number '-1'"},
		{[]string{"-f", "missing"}, "cannot open 'missing': No such file or directory"},
	} {
		_, _, err := runPermuted(t, view, "", append(append([]string{"hexdump"}, test.args...), "three")...)
		if err == nil || err.Error() != test.want {
			t.Errorf("hexdump %q: %v; want %q", test.args, err, test.want)
		}
	}
}
