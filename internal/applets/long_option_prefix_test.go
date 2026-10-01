package applets_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A long option is taken by any prefix that names it alone, as getopt_long takes one in every
// busybox applet that has long options: `mkdir --par`, `cut --output-d=-`. Only the whole name
// was taken. A prefix two options share is no option. Each answer is busybox-w32's, measured.
func TestLongOptions_takeAPrefixThatNamesOneAlone(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{"a.txt": "a:b\n", "g": "x\n"})
	for _, test := range []struct {
		applet string
		args   []string
		stdin  string
		want   string
	}{
		{applet: "cut", args: []string{"-d:", "-f1,2", "--output-d=-", "a.txt"}, want: "a-b\n"},
		{applet: "cmp", args: []string{"--sil", "g", "g"}, want: ""},
		{applet: "nl", args: []string{"--body-n=a", "g"}, want: "     1\tx\n"},
		{applet: "od", args: []string{"--address-r=n", "-c", "g"}, want: "   x  \\n\n"},
		{applet: "env", args: []string{"--ignore-e", "A=1", "env"}, want: "A=1\n"},
	} {
		if got, _, err := runSmall(t, dir, test.stdin, test.applet, test.args...); got != test.want || err != nil {
			t.Errorf("%s %q = %q, %v; want %q", test.applet, test.args, got, err, test.want)
		}
	}
	if _, stderr, err := runSmall(t, dir, "", "mkdir", "--par", "p/q"); err != nil {
		t.Errorf("mkdir --par p/q: %q, %v", stderr, err)
	}
	if info, err := os.Stat(filepath.Join(dir, "p", "q")); err != nil || !info.IsDir() {
		t.Errorf("mkdir --par made no p/q: %v", err)
	}
	if _, stderr, err := runSmall(t, dir, "", "od", "--s", "2", "g"); err == nil || !strings.Contains(stderr+err.Error(), "--s") {
		t.Errorf("od --s = %q, %v; want a refusal, --s being --skip-bytes and --strings both", stderr, err)
	}
}
