package applets

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// touch takes busybox's -d, -t, -r, -a and -m, and their long forms. It took -c alone, so each
// of these was refused as an invalid option.
func TestTouch_setsTheTimesItIsGiven(t *testing.T) {
	dir := t.TempDir()
	view := chmodTestView{cwd: dir}
	run := func(args ...string) (string, error) {
		var stdout, stderr bytes.Buffer
		err := newTouchApplet().Run(WithProcessView(context.Background(), view), args, strings.NewReader(""), &stdout, &stderr)
		return stderr.String(), err
	}
	modified := func(name string) time.Time {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("stat %s: %v", name, err)
		}
		return info.ModTime()
	}
	local := func(year int, month time.Month, day, hour, minute, second int) time.Time {
		return time.Date(year, month, day, hour, minute, second, 0, time.Local)
	}
	for _, test := range []struct {
		args []string
		name string
		want time.Time
	}{
		{[]string{"-d", "2020-01-02 03:04:05", "a"}, "a", local(2020, time.January, 2, 3, 4, 5)},
		{[]string{"-t", "202101020304.05", "b"}, "b", local(2021, time.January, 2, 3, 4, 5)},
		{[]string{"-r", "a", "c"}, "c", local(2020, time.January, 2, 3, 4, 5)},
		{[]string{"--date=2019-06-07 08:09:10", "d"}, "d", local(2019, time.June, 7, 8, 9, 10)},
		{[]string{"--reference=b", "e"}, "e", local(2021, time.January, 2, 3, 4, 5)},
		{[]string{"-d", "@86400", "f"}, "f", time.Unix(86400, 0)},
		// -a alone leaves the modification time.
		{[]string{"-a", "-d", "2018-01-01 00:00:00", "a"}, "a", local(2020, time.January, 2, 3, 4, 5)},
		{[]string{"-m", "-d", "2017-01-01 00:00:00", "a"}, "a", local(2017, time.January, 1, 0, 0, 0)},
	} {
		if stderr, err := run(test.args...); err != nil || stderr != "" {
			t.Errorf("touch %q: %q, %v", test.args, stderr, err)
			continue
		}
		if got := modified(test.name); !got.Equal(test.want) {
			t.Errorf("touch %q: %s modified at %v, want %v", test.args, test.name, got, test.want)
		}
	}
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"-d", "bogus", "g"}, "invalid date 'bogus'"},
		{[]string{"-r", "nosuch", "g"}, "cannot stat 'nosuch': No such file or directory"},
		{[]string{"-r", "a", "-t", "202001010000", "g"}, "-r and -t cannot both be given"},
	} {
		if _, err := run(test.args...); err == nil || err.Error() != test.want {
			t.Errorf("touch %q: %v, want %q", test.args, err, test.want)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "g")); !os.IsNotExist(err) {
		t.Errorf("a refused touch made g: %v", err)
	}
}
