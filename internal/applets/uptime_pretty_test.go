package applets

import (
	"testing"
	"time"
)

// uptime -p is procps-ng's pretty form, which Ubuntu's uptime prints: what has passed in decades,
// years, weeks, days, hours and minutes, each counted as procps' sprint_uptime counts it -- weeks
// within a year of 52 and years within a decade -- and minutes said when there are none only in
// the first minute. It was `unknown option -- p`.
func TestUptimePretty_isProcps(t *testing.T) {
	day := 24 * time.Hour
	for _, test := range []struct {
		up   time.Duration
		want string
	}{
		{0, "up 0 minutes"},
		{59 * time.Second, "up 0 minutes"},
		{time.Minute, "up 1 minute"},
		{3*time.Hour + 5*time.Minute, "up 3 hours, 5 minutes"},
		{time.Hour, "up 1 hour"},
		{day, "up 1 day"},
		{17*day + 22*time.Hour + 15*time.Minute, "up 2 weeks, 3 days, 22 hours, 15 minutes"},
		{400 * day, "up 1 year, 5 weeks, 1 day"},
		{3660 * day, "up 1 decade, 2 weeks, 6 days"},
	} {
		if got := uptimePretty(test.up); got != test.want {
			t.Errorf("%v: %q, want %q", test.up, got, test.want)
		}
	}
}
