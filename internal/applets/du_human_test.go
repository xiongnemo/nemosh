package applets

import "testing"

// humanReadable is busybox's make_human_readable_str: a fixed unit rounds to the nearest, and
// the scaled form keeps one decimal, carrying a ten into the next whole number.
func TestHumanReadable_isBusyboxs(t *testing.T) {
	for _, test := range []struct {
		value, blockSize, unit uint64
		want                   string
	}{
		{0, 512, 0, "0"},
		{0, 512, 1024, "0"},
		{8, 512, 0, "4.0K"},
		{24, 512, 0, "12.0K"},
		{3000, 512, 0, "1.5M"},
		{5000, 1, 0, "4.9K"},
		{1023, 1, 0, "1023"},
		{2047, 1, 0, "2.0K"},
		{8, 512, 1024, "4"},
		{1, 512, 1024, "1"},
		{5000, 1, 1, "5000"},
		{2048, 512, 1024 * 1024, "1"},
	} {
		if got := humanReadable(test.value, test.blockSize, test.unit); got != test.want {
			t.Errorf("humanReadable(%d, %d, %d) = %q, want %q", test.value, test.blockSize, test.unit, got, test.want)
		}
	}
}
