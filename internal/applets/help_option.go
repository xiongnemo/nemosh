package applets

import (
	"io"
	"strings"

	"github.com/xiongnemo/nemosh/internal/capability"
)

// `--help` for every applet, answered in one place.
//
// Not fifty-eight option parsers each learning a new flag. Every applet already
// rejected `--help` in its own words -- `ls: unsupported ls option: --help`,
// `du: invalid option -- '-'`, `xargs: invalid option -- '-'` -- which is the
// first thing anyone types and the last thing they expect to be told off for. The
// registry wrapper every applet already passes through is the one place that can
// answer for all of them.
//
// The text comes from internal/capability, where the option letters are measured
// and bound to behaviour. See capability/usage.go for why it is generated rather
// than written per applet.

// helpAsDataApplets are the applets that must not answer `--help`, because for them
// an argument is data rather than an option.
//
// Measured against busybox-w32, which is the reference here:
//
//	busybox echo --help    prints `--help`
//	busybox test --help    prints nothing and exits 0 -- it asked whether the
//	                       string `--help` is non-empty, and it is
//	busybox true --help    prints nothing
//	busybox false --help   prints nothing
//
// Everything else in busybox prints usage, including `printf --help` and
// `expr --help`, so those two follow it rather than GNU. It costs the ability to
// print or evaluate the literal string `--help`, which is the trade busybox
// already made.
var helpAsDataApplets = map[string]bool{
	"echo":  true,
	"test":  true,
	"[":     true,
	"true":  true,
	"false": true,
}

// helpRequested reports whether these arguments ask for usage.
//
// Only before `--`, and only as the whole word. `grep -- --help file` is searching
// for the string `--help`, and `grep --help-me` is a mistyped option that its own
// parser should complain about by name rather than have swallowed here.
func helpRequested(name string, args []string) bool {
	if helpAsDataApplets[name] {
		return false
	}
	for _, arg := range args {
		if arg == "--" {
			return false
		}
		if arg == "--help" {
			return true
		}
	}
	return false
}

// writeUsage answers a help request, and reports whether it could.
//
// An applet with no usage entry is not silently given an empty page: the usage test
// in internal/capability requires an entry for every non-builtin row, so a missing
// one is a failure there rather than a blank screen here.
func writeUsage(name string, stdout io.Writer) bool {
	text, ok := capability.UsageFor(name)
	if !ok {
		return false
	}
	// To stdout, and exit 0: `--help` is a request that was satisfied, so it
	// belongs where the answer to a question goes and `ls --help | head` works.
	// GNU coreutils and busybox both do this; only a *rejected* option is an error.
	_, _ = io.WriteString(stdout, strings.TrimRight(text, "\n")+"\n")
	return true
}
