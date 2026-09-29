package applets

import (
	"math"
	"reflect"
	"testing"
)

// parseCutList reads LIST as busybox's cut does: from 0, N- to the end, sorted by where each
// range begins unless -D keeps them as given, and a bad one in busybox's words. A range that
// begins inside the one before is folded into it, and one that only touches it is not.
func TestParseCutList_isBusyboxs(t *testing.T) {
	for _, test := range []struct {
		list      string
		keepOrder bool
		want      []cutRange
	}{
		{"2-,5,9-10", false, []cutRange{{1, math.MaxInt}}},
		{"5,-2,3-4", false, []cutRange{{0, 1}, {2, 3}, {4, 4}}},
		{"5,-2,3-4", true, []cutRange{{4, 4}, {0, 1}, {2, 3}}},
		{"1,1,2-3,3-5", false, []cutRange{{0, 0}, {1, 4}}},
		{"2,1,2", true, []cutRange{{1, 1}, {0, 0}, {1, 1}}},
	} {
		got, err := parseCutList(test.list, test.keepOrder)
		if err != nil || !reflect.DeepEqual(got, test.want) {
			t.Errorf("parseCutList(%q, %v) = %v, %v; want %v", test.list, test.keepOrder, got, err, test.want)
		}
	}
	for list, want := range map[string]string{
		"0": "invalid range 0-0", "3-1": "invalid range 3-1", "0-": "invalid range 0-",
		"-0": "invalid range -0", "x": "invalid number 'x'", "1-2-3": "invalid number '2-3'",
		"+1": "invalid number '+1'", "-": "invalid range -", "1,,2": "invalid range 1,,2",
		"": "missing list of positions",
	} {
		if _, err := parseCutList(list, false); err == nil || err.Error() != want {
			t.Errorf("parseCutList(%q): %v, want %q", list, err, want)
		}
	}
}
