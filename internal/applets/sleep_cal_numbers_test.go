package applets_test

import (
	"strings"
	"testing"
)

// sleep and cal say what busybox's say of a number they cannot take: sleep's `invalid number`,
// and cal's `invalid number` for what is no number and `number N is not in LOW..HIGH range` for
// one out of bounds, as xatou_range words them. Each answer is busybox-w32's, measured.
func TestSleepCal_badNumbersInBusyboxsWords(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{})
	for _, test := range []struct {
		applet string
		args   []string
		want   string
	}{
		{applet: "sleep", args: []string{"1x"}, want: "sleep: invalid number '1x'\n"},
		{applet: "sleep", args: []string{""}, want: "sleep: invalid number ''\n"},
		{applet: "sleep", args: []string{"-1"}, want: "sleep: invalid number '-1'\n"},
		{applet: "cal", args: []string{"13", "2020"}, want: "number 13 is not in 1..12 range"},
		{applet: "cal", args: []string{"0", "2020"}, want: "number 0 is not in 1..12 range"},
		{applet: "cal", args: []string{"1", "10000"}, want: "number 10000 is not in 1..9999 range"},
		{applet: "cal", args: []string{"+3", "2020"}, want: "invalid number '+3'"},
		{applet: "cal", args: []string{"x"}, want: "invalid number 'x'"},
	} {
		_, stderr, err := runSmall(t, dir, "", test.applet, test.args...)
		if err == nil || !strings.Contains(stderr+err.Error(), test.want) {
			t.Errorf("%s %q = %q, %v; want a failure saying %q", test.applet, test.args, stderr, err, test.want)
		}
	}
}
