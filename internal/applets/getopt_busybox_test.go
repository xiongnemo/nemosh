package applets_test

import "testing"

// getopt reads a long option as getopt_long reads one: by a prefix too, `--lon` for --longer,
// the first of several a prefix fits when they take an argument alike; a value given with `=`
// to one that takes none is refused; and an option whose argument is missing is said and left
// out of what is printed, short or long. Each was measured against busybox-w32 and MSYS's
// getopt, which agree but for the contraction in "doesn't".
func TestGetopt_readsLongOptionsAsGetoptLongDoes(t *testing.T) {
	for _, test := range []struct {
		args        []string
		out, stderr string
		fails       bool
	}{
		{args: []string{"-o", "a", "-l", "longer", "--", "--lon"}, out: " --longer --\n"},
		{args: []string{"-o", "a", "-l", "long,longer", "--", "--lon"}, out: " --long --\n"},
		{args: []string{"-o", "a", "-l", "flag", "--", "--flag=x"}, out: " --\n", stderr: "getopt: option does not take an argument -- flag\n", fails: true},
		{args: []string{"-o", "b:c", "--", "-c", "-b"}, out: " -c --\n", stderr: "getopt: option requires an argument -- b\n", fails: true},
		{args: []string{"-o", "c", "-l", "lng:", "--", "-c", "--lng"}, out: " -c --\n", stderr: "getopt: option requires an argument -- lng\n", fails: true},
		{args: []string{"-o", "a", "-l", "long:,lone", "--", "--lon"}, out: " --\n", stderr: "getopt: ambiguous option -- lon\n", fails: true},
	} {
		out, stderr, err := runSmall(t, t.TempDir(), "", "getopt", test.args...)
		if out != test.out || stderr != test.stderr || (err != nil) != test.fails {
			t.Errorf("getopt %q = %q, %q, %v; want %q, %q", test.args, out, stderr, err, test.out, test.stderr)
		}
	}
}
