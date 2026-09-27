package applets_test

import "testing"

// With -A n there is no address, so there is no final line either, which is the length as
// an address: busybox ends `od -A n` at the last row. This printed an empty line there, and
// every `... | od -A n -c` carried it into its reader.
func TestOd_noAddressMeansNoFinalLine(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{"a.txt": "abc", "empty.txt": ""})
	for _, test := range []struct {
		args []string
		want string
	}{
		{args: []string{"-A", "n", "-t", "x1", "a.txt"}, want: " 61 62 63\n"},
		{args: []string{"-A", "n", "-c", "a.txt"}, want: "   a   b   c\n"},
		{args: []string{"-A", "n", "-c", "empty.txt"}, want: ""},
		{args: []string{"-c", "a.txt"}, want: "0000000   a   b   c\n0000003\n"},
	} {
		got, stderr, err := runSmall(t, dir, "", "od", test.args...)
		if err != nil {
			t.Fatalf("od %v: %v (%s)", test.args, err, stderr)
		}
		if got != test.want {
			t.Fatalf("od %v = %q, want %q", test.args, got, test.want)
		}
	}
}
