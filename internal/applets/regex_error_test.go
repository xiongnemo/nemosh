package applets_test

import (
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// A pattern regcomp refuses is refused in busybox's words, `bad regex 'PATTERN': REASON`, and grep
// ends with status 2. An unmatched `[` was taken for a literal and `a\{` for a brace, so each
// matched text where busybox refuses it, and the rest came out in Go's own words. `\{,m\}` is
// GNU's and busybox's at most m. Each answer is busybox-w32's, measured.
func TestRegex_aBadPatternIsRefusedInBusyboxsWords(t *testing.T) {
	for _, test := range []struct{ args, want string }{
		{"[", "Invalid regular expression"},
		{"a[", "Invalid regular expression"},
		{"[a", "Unmatched [ or [^"},
		{"[[:alpha:]", "Unmatched [ or [^"},
		{`a\{`, `Unmatched \{`},
		{`a\{1`, `Unmatched \{`},
		{`a\{x\}`, `Invalid content of \{\}`},
		{`a\{2,1\}`, `Invalid content of \{\}`},
		{`\(a`, `Unmatched ( or \(`},
		{`a\)`, `Unmatched ( or \(`},
		{"[b-a]", "Invalid range end"},
		{`\{1\}`, "Invalid preceding regular expression"},
	} {
		_, _, err := runAppletWithInput(t, "xa\n", "grep", "-e", test.args)
		want := "bad regex '" + test.args + "': " + test.want
		if status, _ := applets.StatusCode(err); err == nil || status != 2 || err.Error() != want {
			t.Errorf("grep -e %q: %v (status %d), want %q and status 2", test.args, err, status, want)
		}
	}
	for _, test := range []struct {
		applet string
		args   []string
		want   string
	}{
		{"grep", []string{"-E", "(a"}, `bad regex '(a': Unmatched ( or \(`},
		{"grep", []string{"-E", "[b-a]"}, "bad regex '[b-a]': Invalid range end"},
		{"sed", []string{"-n", `/\(a/p`}, `bad regex '\(a': Unmatched ( or \(`},
		{"sed", []string{`s/a\{x\}/y/`}, `bad regex 'a\{x\}': Invalid content of \{\}`},
	} {
		if _, _, err := runAppletWithInput(t, "a\n", test.applet, test.args...); err == nil || err.Error() != test.want {
			t.Errorf("%s %q: %v, want %q", test.applet, test.args, err, test.want)
		}
	}
	for _, test := range []struct{ pattern, want string }{
		{`a\{,2\}`, "aa\na\n"},
		{`xa\{,\}`, "xaaa\n"},
		{`x\{1,3\}`, "x\n"},
	} {
		if stdout, _, err := runAppletWithInput(t, "xaaa\n", "grep", "-o", "-e", test.pattern); err != nil || stdout != test.want {
			t.Errorf("grep -o %q = %q, %v; want %q", test.pattern, stdout, err, test.want)
		}
	}
}
