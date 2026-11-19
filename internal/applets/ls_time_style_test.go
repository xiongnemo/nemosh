package applets_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ls takes GNU's --time-style: full-iso, long-iso, iso, locale and +FORMAT, which Debian users
// set in scripts so a listing reads the same everywhere. It was `unknown option`.
func TestLs_timeStyle(t *testing.T) {
	dir := t.TempDir()
	old := time.Date(2026, time.January, 2, 3, 4, 5, 123456700, time.Local)
	recent := time.Now().Add(-time.Hour).Truncate(time.Second)
	for name, when := range map[string]time.Time{"old": old, "new": recent} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, when, when); err != nil {
			t.Fatal(err)
		}
	}
	for _, test := range []struct {
		args []string
		want string
	}{
		{args: []string{"--time-style=long-iso", "old"}, want: " 2026-01-02 03:04 old"},
		{args: []string{"--time-style", "long-iso", "old"}, want: " 2026-01-02 03:04 old"},
		{args: []string{"--time-style=full-iso", "old"}, want: " 2026-01-02 03:04:05.123456700 " + old.Format("-0700") + " old"},
		{args: []string{"--time-style=iso", "old"}, want: " 2026-01-02  old"},
		{args: []string{"--time-style=iso", "new"}, want: " " + recent.Format("01-02 15:04") + " new"},
		{args: []string{"--time-style=+%Y/%m/%d", "old"}, want: " 2026/01/02 old"},
		// A newline parts the format for an old file from the one for a recent file, as GNU's.
		{args: []string{"--time-style=+%Y\n%H:%M", "old"}, want: " 2026 old"},
		{args: []string{"--time-style=+%Y\n%H:%M", "new"}, want: " " + recent.Format("15:04") + " new"},
		{args: []string{"--time-style=posix-long-iso", "old"}, want: " 2026-01-02 03:04 old"},
	} {
		stdout, stderr, err := runSmall(t, dir, "", "ls", append([]string{"-l"}, test.args...)...)
		if err != nil || !strings.HasSuffix(strings.TrimSuffix(stdout, "\n"), test.want) {
			t.Errorf("ls -l %q = %q, %v (%s); want it to end %q", test.args, stdout, err, stderr, test.want)
		}
	}
	if _, stderr, err := runSmall(t, dir, "", "ls", "-l", "--time-style=bogus", "old"); err == nil ||
		!strings.Contains(stderr+err.Error(), "invalid argument 'bogus' for '--time-style'") {
		t.Errorf("ls --time-style=bogus: %v, %s", err, stderr)
	}
}
