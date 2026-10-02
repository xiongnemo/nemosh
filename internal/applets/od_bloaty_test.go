package applets_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func odFixture(t *testing.T) permuteTestView {
	t.Helper()
	dir := t.TempDir()
	for name, content := range map[string]string{
		"text":    "Hello, world!\nThe quick brown fox\n",
		"bin":     "\x00\x01\x02\x03\x7f\x80\xff\t\n\r abc",
		"three":   "abc",
		"alpha":   "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ",
		"alnum":   "abcdefghijklmnopqrstuvwxyz0123456789",
		"zeros":   strings.Repeat("\x00", 64),
		"floats":  "\x00\x00\x80\x3f\x00\x00\x00\xc0\x00\x00\x80\x7f\x00\x00\xc0\x7f",
		"doubles": "\x18\x2d\x44\x54\xfb\x21\x09\x40\x00\x00\x00\x00\x00\x00\xf0\x3f",
		"strs":    "one\x00two\x00abcdefg\x00xy\x00longer string\x00tail",
		"s0":      "a\x00\x00bc\x00",
		"s1":      "abcdef",
		"s2":      "xy\x00abcdefgh",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return permuteTestView{cwd: dir}
}

// od is busybox's od_bloaty. Each case but the four marked was measured against busybox-w32 and
// is its output byte for byte: every letter and -t type in its own size, the letters in
// busybox's order whatever order they come in, -A -N -j -w -S and the long forms, and
// --traditional's OFFSET and LABEL. It took a few -t types and six letters, two of them wrong.
func TestOd_dumpsAsBusyboxsOdBloaty(t *testing.T) {
	view := odFixture(t)
	for _, test := range []struct {
		args, want string
	}{
		{"-a bin", "0000000 nul soh stx etx del nul del  ht  nl  cr  sp   a   b   c\n0000016\n"},
		{"-b bin", "0000000 000 001 002 003 177 200 377 011 012 015 040 141 142 143\n0000016\n"},
		{"-c bin", "0000000  \\0 001 002 003 177 200 377  \\t  \\n  \\r       a   b   c\n0000016\n"},
		{"-d bin", "0000000   256   770 32895  2559  3338 24864 25442\n0000016\n"},
		{"-D bin", "0000000   50462976  167739519 1629490442      25442\n0000016\n"},
		{"-i bin", "0000000    50462976   167739519  1629490442       25442\n0000016\n"},
		{"-s bin", "0000000    256    770 -32641   2559   3338  24864  25442\n0000016\n"},
		{"-o bin", "0000000 000400 001402 100177 004777 006412 060440 061542\n0000016\n"},
		{"-O bin", "0000000 00300400400 01177700177 14110006412 00000061542\n0000016\n"},
		{"-x bin", "0000000 0100 0302 807f 09ff 0d0a 6120 6362\n0000016\n"},
		{"-X bin", "0000000 03020100 09ff807f 61200d0a 00006362\n0000016\n"},
		{"-t x1 bin", "0000000 00 01 02 03 7f 80 ff 09 0a 0d 20 61 62 63\n0000016\n"},
		{"-t x8 bin", "0000000 09ff807f03020100 0000636261200d0a\n0000016\n"},
		{"-t d1 bin", "0000000    0    1    2    3  127 -128   -1    9   10   13   32   97   98   99\n0000016\n"},
		{"-t d8 bin", "0000000   720435748402233600      109274187435274\n0000016\n"},
		{"-t u8 bin", "0000000   720435748402233600      109274187435274\n0000016\n"},
		{"-t o8 bin", "0000000 0047774007740300400400 0000003066114110006412\n0000016\n"},
		{"-A x text", "000000 062510 066154 026157 073440 071157 062154 005041 064124\n000010 020145 072561 061551 020153 071142 073557 020156 067546\n000020 005170\n000022\n"},
		{"-A d text", "0000000 062510 066154 026157 073440 071157 062154 005041 064124\n0000016 020145 072561 061551 020153 071142 073557 020156 067546\n0000032 005170\n0000034\n"},
		{"-A n text", " 062510 066154 026157 073440 071157 062154 005041 064124\n 020145 072561 061551 020153 071142 073557 020156 067546\n 005170\n"},
		{"-N 5 text", "0000000 062510 066154 000157\n0000005\n"},
		{"-N 0x10 text", "0000000 062510 066154 026157 073440 071157 062154 005041 064124\n0000020\n"},
		{"-N 010 text", "0000000 062510 066154 026157 073440\n0000010\n"},
		{"-j 5 text", "0000005 020054 067567 066162 020544 052012 062550 070440 064565\n0000025 065543 061040 067562 067167 063040 074157 000012\n0000042\n"},
		{"-j 0x5 -N 4 text", "0000005 020054 067567\n0000011\n"},
		{"-j 6 s1", "0000006\n"},
		{"-j 6 s1 s0", "0000006 000141 061000 000143\n0000014\n"},
		{"-w8 alpha", "0000000 061141 062143 063145 064147\n0000010 065151 066153 067155 070157\n0000020 071161 072163 073165 074167\n0000030 075171 030460 031462 032464\n0000040 033466 034470 041101 042103\n0000050 043105 044107 045111 046113\n0000060 047115 050117 051121 052123\n0000070 053125 054127 055131\n0000076\n"},
		{"-w alpha", "0000000 061141 062143 063145 064147 065151 066153 067155 070157 071161 072163 073165 074167 075171 030460 031462 032464\n0000040 033466 034470 041101 042103 043105 044107 045111 046113 047115 050117 051121 052123 053125 054127 055131\n0000076\n"},
		{"zeros", "0000000 000000 000000 000000 000000 000000 000000 000000 000000\n*\n0000100\n"},
		{"-v zeros", "0000000 000000 000000 000000 000000 000000 000000 000000 000000\n0000020 000000 000000 000000 000000 000000 000000 000000 000000\n0000040 000000 000000 000000 000000 000000 000000 000000 000000\n0000060 000000 000000 000000 000000 000000 000000 000000 000000\n0000100\n"},
		{"-S 3 strs", "0000000 one\n0000004 two\n0000010 abcdefg\n0000023 longer string\n"},
		{"--strings strs", "0000000 one\n0000004 two\n0000010 abcdefg\n0000023 longer string\n"},
		{"--strings=2 strs", "0000000 one\n0000004 two\n0000010 abcdefg\n0000020 xy\n0000023 longer string\n"},
		{"-A n -S 3 strs", "one\ntwo\nabcdefg\nlonger string\n"},
		{"-S 0 s0", "0000000 a\n0000002 \n0000003 bc\n"},
		{"-b -c three", "0000000 141 142 143\n          a   b   c\n0000003\n"},
		{"-c -b three", "0000000 141 142 143\n          a   b   c\n0000003\n"},
		{"--address-radix=x --format=x1 --read-bytes=4 --skip-bytes=2 text", "000002 6c 6c 6f 2c\n000006\n"},
		{"--traditional s1 2", "0000002 062143 063145\n0000006\n"},
		{"--traditional -A n s1 2 100", "(0000002) 062143 063145\n(0000006)\n"},
		{"--traditional -A x s1 2 0x100", "000002 (000100) 062143 063145\n000006 (000104)\n"},
		// C's %e, where busybox-w32's msvcrt prints 1.0000000e+000 and 1.#INF000e+000.
		{"-f floats", "0000000   1.0000000e+00  -2.0000000e+00             inf             nan\n0000020\n"},
		{"-t f8 doubles", "0000000   3.141592653589793e+00   1.000000000000000e+00\n0000020\n"},
		// A size and then another type or z, which busybox refuses as a 4294967295-byte type
		// though its own comment gives d4afL as a string it reads.
		{"-t x1z text", "0000000 48 65 6c 6c 6f 2c 20 77 6f 72 6c 64 21 0a 54 68  >Hello, world!.Th<\n0000020 65 20 71 75 69 63 6b 20 62 72 6f 77 6e 20 66 6f  >e quick brown fo<\n0000040 78 0a                                            >x.<\n0000042\n"},
		// A run -N ends, where it begins: busybox's and GNU's say 0000002, the byte before.
		{"-S 3 -N 8 s2", "0000003 abcde\n"},
		// Several types stand in columns, as GNU od lays them out.
		{"-t d1 -t x1 alnum", "0000000   97   98   99  100  101  102  103  104  105  106  107  108  109  110  111  112\n          61   62   63   64   65   66   67   68   69   6a   6b   6c   6d   6e   6f   70\n0000020  113  114  115  116  117  118  119  120  121  122   48   49   50   51   52   53\n          71   72   73   74   75   76   77   78   79   7a   30   31   32   33   34   35\n0000040   54   55   56   57\n          36   37   38   39\n0000044\n"},
		{"--traditional -c -t x1 s1 1 10", "0000001 (0000010)   b   c   d   e   f\n         62  63  64  65  66\n0000006 (0000015)\n"},
	} {
		stdout, stderr, err := runPermuted(t, view, "", append([]string{"od"}, strings.Fields(test.args)...)...)
		if err != nil || stderr != "" || stdout != test.want {
			t.Errorf("od %s: %q, %q, %v\n  want %q", test.args, stdout, stderr, err, test.want)
		}
	}
	stdout, stderr, err := runPermuted(t, view, "abcdef", "od", "--traditional", "2", "100")
	if want := "0000002 (0000100) 062143 063145\n0000006 (0000104)\n"; err != nil || stderr != "" || stdout != want {
		t.Errorf("od --traditional 2 100 < s1: %q, %q, %v; want %q", stdout, stderr, err, want)
	}
	// A long is four bytes on Windows and eight elsewhere, so -l is -i or -t d8.
	long, _, _ := runPermuted(t, view, "", "od", "-l", "bin")
	want, _, _ := runPermuted(t, view, "", "od", "-t", map[bool]string{true: "d4", false: "d8"}[runtime.GOOS == "windows"], "bin")
	if long != want {
		t.Errorf("od -l bin: %q; want %q", long, want)
	}
}

// Each refusal is busybox's, and a command line with two bad options gets the error of the one
// busybox reads first: -w, -A, -N, -j, the types, -S.
func TestOd_refusesAsBusyboxDoes(t *testing.T) {
	view := odFixture(t)
	for _, test := range []struct {
		args, want string
	}{
		{"-t x3 bin", "invalid type string 'x3'; 3-byte integral type is not supported"},
		{"-t d16 bin", "invalid type string 'd16'; 16-byte integral type is not supported"},
		{"-t q bin", "invalid character 'q' in type string 'q'"},
		{"-t fL bin", "invalid type string 'fL'; 16-byte floating point type is not supported"},
		{"-A z text", "bad output address radix 'z' (must be [doxn])"},
		{"-N 5x text", "invalid number '5x'"},
		{"-N -1 text", "invalid number '-1'"},
		{"-j 100 text", "cannot skip past end of combined input"},
		{"-S 5000000000 s1", "invalid number '5000000000'"},
		{"-S 5000000k s1", "number 5000000k is not in 0..4294967295 range"},
		{"-N 9999999999999999999k s1", "number 9999999999999999999k is not in 0..18446744073709551615 range"},
		{"-N 18446744073709551615 -j 1 s1", "SKIP + SIZE is too large"},
		{"-t q -N x s1", "invalid number 'x'"},
		{"-j y -t q s1", "invalid number 'y'"},
		{"-A z -wx s1", "invalid number 'x'"},
		// -w's WIDTH is only ever in its own word: this is -w and a FILE named x.
		{"-A z -w x s1", "bad output address radix 'z' (must be [doxn])"},
		{"--traditional s1 zz", "invalid second argument 'zz'"},
		{"--traditional s1 1z", "invalid number '1z'"},
		{"--traditional s1 1 zz", "the last two arguments must be offsets"},
		{"--traditional s1 s0 s2 s1", "too many arguments"},
	} {
		_, _, err := runPermuted(t, view, "", append([]string{"od"}, strings.Fields(test.args)...)...)
		if err == nil || err.Error() != test.want {
			t.Errorf("od %s: %v; want %q", test.args, err, test.want)
		}
	}
	// A WIDTH that is no whole number of lines is warned of, once the input is open, and the
	// least one is used.
	stdout, stderr, err := runPermuted(t, view, "", "od", "-w5", "-t", "x2", "three")
	if want := "0000000 6261\n0000002 0063\n0000003\n"; err != nil || stdout != want || stderr != "od: warning: invalid width 5; using 2 instead\n" {
		t.Errorf("od -w5 -t x2 three: %q, %q, %v; want %q and the warning", stdout, stderr, err, want)
	}
}
