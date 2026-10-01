package applets_test

import (
	"strconv"
	"strings"
	"testing"
)

// A count takes b, k or m after it, for 512, 1024 or 1048576, under -n and -c and in the -N
// form, as busybox's head and tail read one with bkm_suffixes. Each answer is busybox-w32's,
// measured on seq 1 2000, which is 8893 bytes; `head -c 1k` was "invalid number '1k'".
func TestHeadTail_countTakesASuffix(t *testing.T) {
	var numbers strings.Builder
	for n := 1; n <= 2000; n++ {
		numbers.WriteString(strconv.Itoa(n) + "\n")
	}
	dir := writeSmallFixture(t, map[string]string{"big": numbers.String()})
	for _, test := range []struct {
		applet string
		args   []string
		// lines is whether want counts lines rather than bytes.
		lines bool
		want  int
	}{
		{applet: "head", args: []string{"-c", "1k", "big"}, want: 1024},
		{applet: "head", args: []string{"-c", "2b", "big"}, want: 1024},
		{applet: "head", args: []string{"-c", "1m", "big"}, want: 8893},
		{applet: "head", args: []string{"-c", "-1k", "big"}, want: 7869},
		{applet: "head", args: []string{"-n", "1k", "big"}, lines: true, want: 1024},
		{applet: "head", args: []string{"-1k", "big"}, lines: true, want: 1024},
		{applet: "tail", args: []string{"-c", "1k", "big"}, want: 1024},
		{applet: "tail", args: []string{"-c", "+1k", "big"}, want: 7870},
		{applet: "tail", args: []string{"-1k", "big"}, lines: true, want: 1024},
		{applet: "tail", args: []string{"-n", "+1k", "big"}, lines: true, want: 977},
	} {
		got, _, err := runSmall(t, dir, "", test.applet, test.args...)
		measured := len(got)
		if test.lines {
			measured = strings.Count(got, "\n")
		}
		if err != nil || measured != test.want {
			t.Errorf("%s %q gave %d (lines %v), %v; want %d", test.applet, test.args, measured, test.lines, err, test.want)
		}
	}
}

// Only those three, and only alone: busybox says `invalid number '1K'` for a K, and the same
// for kb or a bare k. A count the suffix takes past what a count holds is out of range, in
// the shape busybox says it.
func TestHeadTail_refusesAnotherSuffix(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{"a": "x\n"})
	for _, test := range []struct {
		applet string
		args   []string
		want   string
	}{
		{applet: "head", args: []string{"-c", "1K", "a"}, want: "invalid number '1K'"},
		{applet: "head", args: []string{"-c", "1kb", "a"}, want: "invalid number '1kb'"},
		{applet: "head", args: []string{"-c", "k", "a"}, want: "invalid number 'k'"},
		{applet: "tail", args: []string{"-c", "1B", "a"}, want: "invalid number '1B'"},
		{applet: "head", args: []string{"-c", "9007199254740992k", "a"}, want: "number 9007199254740992k is not in 0.."},
	} {
		stdout, stderr, err := runSmall(t, dir, "", test.applet, test.args...)
		if err == nil || stdout != "" || !strings.Contains(stderr+err.Error(), test.want) {
			t.Errorf("%s %q = %q, %q, %v; want a refusal saying %q", test.applet, test.args, stdout, stderr, err, test.want)
		}
	}
}
