package runtime

import (
	"bytes"
	"context"
	"strings"
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

// type and command -V describe an alias and a function in bash mode as bash's do: "ll is aliased
// to `ls -l'", and a function's definition after `is a function`, as declare -f prints it.
// busybox's are `ll is an alias for ls -l` and the bare `is a function`, which nemosh and sh keep.
// Measured 2026-11-21 with Git for Windows' bash 5.3; Oils' builtin-meta.test.sh, command -V.
func TestTypeDescription_bashModeIsBashs(t *testing.T) {
	script := "alias ll='ls -l'; myfunc() { echo x; }\ntype ll; type myfunc; command -V ll\n"
	for _, test := range []struct {
		bash bool
		want string
	}{
		{bash: true, want: "ll is aliased to `ls -l'\nmyfunc is a function\nmyfunc () \n{ \n    echo x\n}\nll is aliased to `ls -l'\n"},
		{bash: false, want: "ll is an alias for ls -l\nmyfunc is a function\nll is an alias for ls -l\n"},
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

// type calls a special builtin "a shell builtin" in bash mode, as bash's does outside its POSIX
// mode, and a POSIX special builtin "a special shell builtin" in it, local not among them.
// busybox calls POSIX's and its own local and times special, which nemosh and sh keep. Measured
// 2026-11-22 with Git for Windows' bash 5.3; Oils' builtin-type.test.sh.
func TestTypeSpecialBuiltin_bashModeIsBashs(t *testing.T) {
	script := "type eval : local cd; set -o posix; type eval local\n"
	for _, test := range []struct {
		bash bool
		want string
	}{
		{bash: true, want: "eval is a shell builtin\n: is a shell builtin\nlocal is a shell builtin\ncd is a shell builtin\neval is a special shell builtin\nlocal is a shell builtin\n"},
		{bash: false, want: "eval is a special shell builtin\n: is a special shell builtin\nlocal is a special shell builtin\ncd is a shell builtin\n"},
	} {
		var stdout, stderr bytes.Buffer
		rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
		if test.bash {
			rt.EnterBashMode()
		}
		rt.RunScript(context.Background(), script)
		if !strings.HasPrefix(stdout.String(), test.want) {
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
