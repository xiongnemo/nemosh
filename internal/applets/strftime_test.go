package applets

import (
	"testing"
	"time"
)

// The one strftime, over a fixed instant in UTC so the answers do not depend on the zone the
// test runs in. Each expected string is what GNU date gives for the same instant, measured;
// `%Z` is left out because a zone's name is the platform's to choose (GMT there, UTC here).
func TestStrftime_rendersTheCConversions(t *testing.T) {
	when := time.Date(1970, time.July, 19, 20, 0, 0, 0, time.UTC) // a Sunday
	for _, test := range []struct{ format, want string }{
		{"%Y-%m-%d %H:%M:%S", "1970-07-19 20:00:00"},
		{"%F %T", "1970-07-19 20:00:00"},
		{"%a %A %b %B %h", "Sun Sunday Jul July Jul"},
		{"%e|%j|%C|%y", "19|200|19|70"},
		{"%I %l %p %P", "08  8 PM pm"},
		{"%k|%R|%r", "20|20:00|08:00:00 PM"},
		{"%u %w", "7 0"},
		{"%U %W %V %G %g", "29 28 29 1970 70"},
		{"%D %x %X", "07/19/70 07/19/70 20:00:00"},
		{"%s", "17265600"},
		{"%z", "+0000"},
		{"100%%", "100%"},
	} {
		if got, err := strftime(when, test.format, true); err != nil || got != test.want {
			t.Errorf("strftime(%q) = %q, %v; want %q", test.format, got, err, test.want)
		}
	}
}

// An unknown conversion is refused when strict, for `date`, and written as it was otherwise,
// for `ts`, where a typo in a stamp is better shown than swallowed.
func TestStrftime_decidesAnUnknownConversionByCaller(t *testing.T) {
	when := time.Unix(0, 0).UTC()
	if _, err := strftime(when, "%Q", true); err == nil {
		t.Error("strict strftime accepted %Q")
	}
	if got, _ := strftime(when, "[%Q]", false); got != "[%Q]" {
		t.Errorf("lenient strftime gave %q, want the conversion as written", got)
	}
}
