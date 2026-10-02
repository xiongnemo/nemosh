package runtime_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// An option nobody takes is named in getopt's words, as both references say them on Windows:
// busybox-w32's getopt is the mingw runtime's and MSYS's GNU tools have Cygwin's, and both are
// NetBSD's. `unknown option -- x`, a long one named as it was typed, with any value it was
// given; `option requires an argument -- x`; and `option does not take an argument -- name`,
// where both say "doesn't". nemosh said glibc's, which neither says here, `invalid option --
// 'x'` and `unrecognized option '--x'`, and some applets their own, `unsupported ls option:
// -x`; awk named itself twice. Each was measured against busybox-w32 and MSYS's tools.
func TestRuntime_namesABadOptionInGetoptsWords(t *testing.T) {
	for _, test := range []struct{ script, stderr string }{
		{"cat -Y", "cat: unknown option -- Y\n"},
		{"ls --bogus", "ls: unknown option -- bogus\n"},
		{"sort --foo=bar", "sort: unknown option -- foo=bar\n"},
		{"sed -Y p", "sed: unknown option -- Y\n"},
		{"grep -Y x", "grep: unknown option -- Y\n"},
		{"uname --bogus", "uname: unknown option -- bogus\n"},
		{"date -x", "date: unknown option -- x\n"},
		{"awk -Y 1", "awk: unknown option -- Y\n"},
		{"cut -f", "cut: option requires an argument -- f\n"},
		{"head -n", "head: option requires an argument -- n\n"},
		{"awk -f", "awk: option requires an argument -- f\n"},
		{"sed --expression", "sed: option requires an argument -- expression\n"},
		{"sed --quiet=1 p", "sed: option does not take an argument -- quiet\n"},
		{"touch --no-create=1 f", "touch: option does not take an argument -- no-create\n"},
		{"timeout -Y 1 true", "timeout: unknown option -- Y\n"},
		{"timeout --bogus 1 true", "timeout: unknown option -- bogus\n"},
		{"time -o", "time: option requires an argument -- o\n"},
		{"time --bogus true", "time: unknown option -- bogus\n"},
	} {
		var stderr bytes.Buffer
		rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: new(bytes.Buffer), Stderr: &stderr})
		status := rt.RunScript(context.Background(), test.script+"\n")
		rt.CloseBatch(status)
		if stderr.String() != test.stderr || status == 0 {
			t.Errorf("%s: %q, status %d; want %q", test.script, stderr.String(), status, test.stderr)
		}
	}
}
