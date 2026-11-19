package applets_test

import (
	"slices"
	"strings"
	"testing"
)

// sort -R is GNU's random sort: an order that differs from run to run, lines with the same key
// together. It was `unknown option -- R`.
func TestSort_randomGroupsEqualKeys(t *testing.T) {
	input := "a 1\nb 2\nc 3\na 1\nb 2\nc 3\nd 4\ne 5\n"
	seen := map[string]bool{}
	for range 30 {
		stdout, stderr, err := runAppletWithInput(t, input, "sort", "-R")
		if err != nil {
			t.Fatalf("sort -R: %v (%s)", err, stderr)
		}
		lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
		sorted := slices.Clone(lines)
		slices.Sort(sorted)
		want := strings.Split(strings.TrimSuffix(input, "\n"), "\n")
		slices.Sort(want)
		if !slices.Equal(sorted, want) {
			t.Fatalf("sort -R lost or made lines: %q", stdout)
		}
		for index := 1; index < len(lines); index++ {
			if lines[index] != lines[index-1] && slices.Contains(lines[:index-1], lines[index]) {
				t.Fatalf("sort -R parted two equal lines: %q", stdout)
			}
		}
		seen[stdout] = true
	}
	if len(seen) < 2 {
		t.Fatalf("thirty runs of sort -R gave one order")
	}
	// A key: lines whose second field is the same are together, whatever else they hold.
	stdout, _, err := runAppletWithInput(t, "x 1\ny 2\nz 1\nw 2\n", "sort", "-k2,2R")
	lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
	if err != nil || len(lines) != 4 || lines[0][2] != lines[1][2] || lines[2][2] != lines[3][2] {
		t.Fatalf("sort -k2,2R = %q, %v; want the 1s together and the 2s together", stdout, err)
	}
	if _, _, err := runAppletWithInput(t, "a\n", "sort", "-nR"); err == nil {
		t.Fatal("sort -nR asked for two orderings and was not refused")
	}
}
