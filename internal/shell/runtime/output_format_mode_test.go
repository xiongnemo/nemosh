package runtime

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// The `set -o` and `set +o` listings in bash mode are bash's: its names in the order of their
// names, and `set -o`'s rows `%-15s\t%s`; busybox-w32's own glob options, which bash has not got,
// are left out of them. nemosh and sh list busybox's table, in its order. Measured 2026-11-21.
func TestSetListing_bashModeIsBashs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
	rt.EnterBashMode()
	rt.RunScript(context.Background(), "set -o | head -3; set +o | head -2; set -o | grep -c glob; set -o | grep -c noclobber\n")
	if want := "allexport      \toff\nbraceexpand    \ton\nemacs          \toff\nset +o allexport\nset -o braceexpand\n1\n1\n"; stdout.String() != want {
		t.Errorf("bash mode: %q, want %q; stderr %q", stdout.String(), want, stderr.String())
	}
	stdout.Reset()
	rt = New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
	rt.RunScript(context.Background(), "set -o | head -1; set +o | head -1\n")
	if want := "errexit         off\nset +o errexit\n"; stdout.String() != want {
		t.Errorf("as nemosh: %q, want %q", stdout.String(), want)
	}
}

// alias lists an alias in bash mode as bash's does, `alias a=...` with a single quote in the value
// escaped by a backslash between quoted parts, whether all or one is asked for; busybox's has no
// `alias ` and puts the quote in double quotes, which nemosh and sh keep. Measured 2026-11-21.
func TestAliasListing_bashModeIsBashs(t *testing.T) {
	script := "alias a=\"it's\" b=plain; alias a; alias\n"
	for _, test := range []struct {
		bash bool
		want string
	}{
		{bash: true, want: "alias a='it'\\''s'\nalias a='it'\\''s'\nalias b='plain'\n"},
		{bash: false, want: "a='it'\"'\"'s'\na='it'\"'\"'s'\nb='plain'\n"},
	} {
		var stdout, stderr bytes.Buffer
		rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
		if test.bash {
			rt.EnterBashMode()
		}
		rt.RunScript(context.Background(), script)
		if stdout.String() != test.want {
			t.Errorf("bash mode %v: %q, want %q; stderr %q", test.bash, stdout.String(), test.want, stderr.String())
		}
	}
}

// `$-` in bash mode is bash's: every short option that is on, bash's own -h and -B among them --
// on from the start -- in bash's order, abefhikmnptuvxBCEHPT. busybox's lists only its own, in
// its own order backwards, which nemosh and sh keep. Measured 2026-11-21 with Git for Windows'
// bash 5.3 in a script file, where bash adds no c.
func TestDollarDash_bashModeIsBashs(t *testing.T) {
	script := "echo $-; set -eu; echo $-; set -f -C -a +eu; echo $-; set +B +h; echo $-\n"
	for _, test := range []struct {
		bash bool
		want string
	}{
		{bash: true, want: "hB\nehuB\nafhBC\nafC\n"},
		{bash: false, want: "\nue\naCf\naCf\n"},
	} {
		var stdout, stderr bytes.Buffer
		rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
		if test.bash {
			rt.EnterBashMode()
		}
		rt.RunScript(context.Background(), script)
		if stdout.String() != test.want {
			t.Errorf("bash mode %v: $- went %q, want %q; stderr %q", test.bash, stdout.String(), test.want, stderr.String())
		}
	}
}
