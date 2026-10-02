package applets_test

import "testing"

// tsort reads its words as one stream, a pair at a time across lines, as both references read
// them, and an odd one out is refused before anything is written. A cycle is said, `cycle at
// NAME`, and broken at its first item, and the rest written after it, with status 1. A second
// FILE is an extra operand. Each line was paired on its own, a word left over named an item, a
// cycle ended the output, and every FILE was read. Measured against busybox-w32 and MSYS's tsort.
func TestTsort_pairsAcrossLinesAndBreaksACycle(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{"t.txt": "a b\n"})
	for _, test := range []struct {
		args                   []string
		stdin, out, stderr, is string
		fails                  bool
	}{
		{stdin: "a\nb\n", out: "a\nb\n"},
		{stdin: "a b c\n", is: "odd input", fails: true},
		{stdin: "solo\n", is: "odd input", fails: true},
		{stdin: "a b\nb c\nc a\nd e\n", out: "d\ne\na\nb\nc\n", stderr: "tsort: cycle at a\n", fails: true},
		{args: []string{"t.txt", "t.txt"}, is: "extra operand 't.txt'", fails: true},
		{args: []string{"-"}, stdin: "x y\n", out: "x\ny\n"},
	} {
		out, stderr, err := runSmall(t, dir, test.stdin, "tsort", test.args...)
		if out != test.out || stderr != test.stderr || (err != nil) != test.fails || test.is != "" && err.Error() != test.is {
			t.Errorf("tsort %q <<< %q = %q, %q, %v; want %q, %q, %q", test.args, test.stdin, out, stderr, err, test.out, test.stderr, test.is)
		}
	}
}
