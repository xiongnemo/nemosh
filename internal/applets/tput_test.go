package applets_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// tput is ncurses', with xterm-256color's terminfo, the VT dialect the Windows console speaks:
// setaf and setab in their three ranges, the attributes, cup, the screen's modes; cols and lines
// from COLUMNS and LINES, the terminal, or 80 by 24; colors 256. An unknown capability is status
// 4, as ncurses' tput has it. tty says whether standard input is a terminal: `not a tty` and 1
// when it is not, nothing with -s. Neither was here.
func TestTput_speaksXterm256color(t *testing.T) {
	run := func(env map[string]string, applet string, args ...string) (string, error) {
		found, _ := applets.DefaultRegistry.Lookup(applet)
		var stdout, stderr bytes.Buffer
		ctx := applets.WithProcessView(context.Background(), permuteTestView{cwd: t.TempDir(), env: env})
		err := found.Run(ctx, args, strings.NewReader(""), &stdout, &stderr)
		return stdout.String(), err
	}
	for _, test := range []struct {
		env  map[string]string
		args []string
		want string
	}{
		{nil, []string{"setaf", "1"}, "\x1b[31m"},
		{nil, []string{"setaf", "9"}, "\x1b[91m"},
		{nil, []string{"setaf", "208"}, "\x1b[38;5;208m"},
		{nil, []string{"setab", "2"}, "\x1b[42m"},
		{nil, []string{"setab", "12"}, "\x1b[104m"},
		{nil, []string{"bold"}, "\x1b[1m"},
		{nil, []string{"sgr0"}, "\x1b(B\x1b[m"},
		{nil, []string{"smul"}, "\x1b[4m"},
		{nil, []string{"rev"}, "\x1b[7m"},
		{nil, []string{"cup", "5", "10"}, "\x1b[6;11H"},
		{nil, []string{"civis"}, "\x1b[?25l"},
		{nil, []string{"el"}, "\x1b[K"},
		{nil, []string{"clear"}, "\x1b[H\x1b[2J"},
		{nil, []string{"colors"}, "256\n"},
		{nil, []string{"cols"}, "80\n"},
		{nil, []string{"lines"}, "24\n"},
		{map[string]string{"COLUMNS": "100", "LINES": "30"}, []string{"cols"}, "100\n"},
		{map[string]string{"COLUMNS": "100", "LINES": "30"}, []string{"lines"}, "30\n"},
		{nil, []string{"longname"}, "xterm with 256 colors"},
	} {
		if stdout, err := run(test.env, "tput", test.args...); err != nil || stdout != test.want {
			t.Errorf("tput %q = %q, %v; want %q", test.args, stdout, err, test.want)
		}
	}
	if _, err := run(nil, "tput", "frobnicate"); err == nil || !strings.Contains(err.Error(), "unknown terminfo capability 'frobnicate'") {
		t.Errorf("tput frobnicate = %v", err)
	} else if status, _ := applets.StatusCode(err); status != 4 {
		t.Errorf("tput frobnicate is status %d, want 4", status)
	}

	stdout, err := run(nil, "tty")
	if status, _ := applets.StatusCode(err); stdout != "not a tty\n" || status != 1 {
		t.Errorf("tty from a pipe = %q, %v; want not a tty and 1", stdout, err)
	}
	stdout, err = run(nil, "tty", "-s")
	if status, _ := applets.StatusCode(err); stdout != "" || status != 1 {
		t.Errorf("tty -s from a pipe = %q, %v; want nothing and 1", stdout, err)
	}
}
