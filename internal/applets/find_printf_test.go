package applets_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// find -printf is GNU's: %p %f %h %P %H %d %y %s and the rest, a width and a precision on each
// as on a string, so %-8.1s of 5000 is 5; the times as %t, %T@ and %Tk, where %T@, %TS, %TT and
// %T+ carry ten digits of fraction; GNU's escapes, \c ending the entry, and an unknown escape or
// directive written as it is, with a warning. Each answer is GNU findutils 4.11.0's, measured,
// but for what is the platform's own -- the owner, the mode, the identity -- which is held to
// what ls and stat say here. It was `unrecognized: -printf`.
func TestFind_printfAsGnuDoes(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{"d/a.txt": "hello", "d/sub/b": "", "d/big": strings.Repeat("x", 5000)})
	old := time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC)
	for _, name := range []string{"d/a.txt", "d/sub/b", "d/big", "d/sub", "d"} {
		if err := os.Chtimes(filepath.Join(dir, name), old.Add(time.Second), old); err != nil {
			t.Fatal(err)
		}
	}
	local := old.Local()
	clock := local.Format("15:04:05") + ".0000000000"
	for _, test := range []struct {
		args       []string
		want, says string
	}{
		{[]string{"d", "-printf", `%p|%f|%h|%P|%H|%d|%y\n`},
			"d|d|.||d|0|d\nd/a.txt|a.txt|d|a.txt|d|1|f\nd/big|big|d|big|d|1|f\nd/sub|sub|d|sub|d|1|d\nd/sub/b|b|d/sub|sub/b|d|2|f\n", ""},
		{[]string{"d", "-type", "f", "-printf", `[%10s][%-10f][%5d][%-3y]\n`},
			"[         5][a.txt     ][    1][f  ]\n[      5000][big       ][    1][f  ]\n[         0][b         ][    2][f  ]\n", ""},
		{[]string{"d/a.txt", "-printf", `%t|%T@|%A@\n`},
			local.Format("Mon Jan _2 15:04:05") + ".0000000000 2001|981173106.0000000000|981173107.0000000000\n", ""},
		{[]string{"d/a.txt", "-printf", `%TY-%Tm-%Td %TH:%TM:%TS|%TT|%TD|%T+|%Tj|%Ts|%Ta %Tb\n`},
			local.Format("2006-01-02 ") + clock + "|" + clock + "|" + local.Format("01/02/06") + "|" + local.Format("2006-01-02+") + clock + fmt.Sprintf("|%03d|981173106|", local.YearDay()) + local.Format("Mon Jan") + "\n", ""},
		{[]string{"d/a.txt", "-printf", `%%|\t|\\|\101|\x|%Q\n`}, "%|\t|\\|A|\\x|%Q\n",
			"find: warning: unrecognized escape `\\x'\nfind: warning: unrecognized format directive `%Q'\n"},
		{[]string{"d", "-printf", `a\cb\n`}, "aaaaa", ""},
		{[]string{"d/big", "-printf", `%.3f|%10.2p|%-8.1s|\n`}, "big|        d/|5       |\n", ""},
	} {
		stdout, stderr, err := runSmall(t, dir, "", "find", test.args...)
		if err != nil || stdout != test.want || stderr != test.says {
			t.Errorf("find %q = %q, %q, %v; want %q, %q", test.args, stdout, stderr, err, test.want, test.says)
		}
	}

	field := func(applet string, index int, args ...string) string {
		out, _, err := runSmall(t, dir, "", applet, args...)
		if fields := strings.Fields(out); err == nil && index < len(fields) {
			return fields[index]
		}
		t.Fatalf("%s %q = %q, %v", applet, args, out, err)
		return ""
	}
	stats := strings.Join([]string{field("ls", 2, "-ld", "d/a.txt"), field("ls", 3, "-ld", "d/a.txt"), field("ls", 2, "-ldn", "d/a.txt"),
		field("stat", 0, "-c", "%h", "d/a.txt"), field("stat", 0, "-c", "%A", "d/a.txt"), field("stat", 0, "-c", "%a", "d/a.txt"),
		field("stat", 0, "-c", "%i", "d/a.txt"), field("stat", 0, "-c", "%b", "d/a.txt")}, " ")
	if stdout, _, err := runSmall(t, dir, "", "find", "d/a.txt", "-printf", `%u %g %U %n %M %m %i %b\n`); err != nil || stdout != stats+"\n" {
		t.Errorf("find -printf '%%u %%g %%U %%n %%M %%m %%i %%b' = %q, %v; want %q, as ls and stat say", stdout, err, stats)
	}
	for _, verb := range []string{"%F", "%S", "%Z"} {
		if stdout, _, err := runSmall(t, dir, "", "find", "d", "-printf", verb); err == nil || !strings.Contains(err.Error(), verb) || stdout != "" {
			t.Errorf("find -printf %s = %q, %v; want it refused by name, before the walk", verb, stdout, err)
		}
	}
}
