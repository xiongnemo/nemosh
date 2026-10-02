package applets

import (
	"bytes"
	"context"
	"testing"
	"time"
)

// date takes a long option by any prefix that names one alone, as getopt_long takes them:
// `--da` is --date and `--u` --utc, and `--rfc` is -R, the one letter both rfc names are.
// Only the whole name was taken. A value given one that takes none, or missing from one that
// needs it, is named as it was typed. Each was measured against busybox-w32.
func TestDate_takesALongOptionByItsPrefix(t *testing.T) {
	applet := dateApplet{now: func() time.Time { return time.Date(2026, time.September, 29, 12, 34, 56, 0, time.UTC) }}
	for _, test := range []struct {
		args      []string
		out, fail string
	}{
		{args: []string{"--u", "--da=@0", "+%Y"}, out: "1970\n"},
		{args: []string{"--ut", "--da", "@86400", "+%d"}, out: "02\n"},
		{args: []string{"--rfc", "-u", "-d", "@0"}, out: "Thu, 01 Jan 1970 00:00:00 +0000\n"},
		{args: []string{"--da"}, fail: "date: option requires an argument -- da"},
		{args: []string{"--re"}, fail: "date: option requires an argument -- re"},
		{args: []string{"--utc=1"}, fail: "date: option does not take an argument -- utc"},
		{args: []string{"--bogus=1"}, fail: "date: unknown option -- bogus=1"},
	} {
		var stdout, stderr bytes.Buffer
		err := applet.Run(context.Background(), test.args, &bytes.Buffer{}, &stdout, &stderr)
		want := ""
		if test.fail != "" {
			want = test.fail + "\n"
		}
		if stdout.String() != test.out || stderr.String() != want || (err != nil) != (test.fail != "") {
			t.Errorf("date %q = %q, %q, %v; want %q, %q", test.args, stdout.String(), stderr.String(), err, test.out, test.fail)
		}
	}
}
