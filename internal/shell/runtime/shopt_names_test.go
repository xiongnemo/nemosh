package runtime_test

import (
	"strings"
	"testing"
)

// shopt knows every name bash 5.3's has, so a strict preamble does not end the script on its
// first line: `set -e; shopt -s checkwinsize histappend` was status 1 at the first name and
// the script stopped there. The answers are bash's; busybox has no shopt.
func TestShopt_knowsEveryBashName(t *testing.T) {
	// When
	status, stdout, stderr := runSetScript(t, "shopt -p\n")

	// Then
	if status != 0 || stderr != "" {
		t.Fatalf("status = %d, stderr = %q", status, stderr)
	}
	lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
	if len(lines) != 60 {
		t.Errorf("shopt -p printed %d lines, want bash 5.3's 60", len(lines))
	}
	for _, name := range []string{"array_expand_once", "checkwinsize", "completion_strip_exe",
		"expand_aliases", "globskipdots", "lastpipe", "login_shell", "sourcepath", "xpg_echo"} {
		if !strings.Contains(stdout, " "+name+"\n") {
			t.Errorf("shopt -p has no %s: %q", name, stdout)
		}
	}
}

func TestShopt_answersAsBashDoes(t *testing.T) {
	tests := []struct {
		name, script, stdout string
		status               int
	}{
		{
			name:   "a strict preamble survives",
			script: "set -e\nshopt -s nullglob globstar extglob checkwinsize histappend progcomp\necho ok\n",
			stdout: "ok\n",
		},
		{
			// What -p prints is what reads back, so a saved state can be put back.
			name:   "-p output reads back",
			script: "saved=$(shopt -p)\nshopt -s nullglob histappend\neval \"$saved\"\necho \"st=$?\"\nshopt -p nullglob histappend\n",
			stdout: "st=0\nshopt -u nullglob\nshopt -u histappend\n", status: 1,
		},
		{
			name:   "-p of a name off is status 1, of one on 0",
			script: "shopt -p nullglob; echo \"st=$?\"; shopt -s nullglob; shopt -p nullglob; echo \"st=$?\"\n",
			stdout: "shopt -u nullglob\nst=1\nshopt -s nullglob\nst=0\n",
		},
		{
			name:   "-q is 0 only when every name is on",
			script: "shopt -s nullglob; shopt -q nullglob dotglob; echo \"st=$?\"; shopt -s dotglob; shopt -q nullglob dotglob; echo \"st=$?\"\n",
			stdout: "st=1\nst=0\n",
		},
		{
			name:   "-q alone prints nothing",
			script: "shopt -q; echo \"st=$?\"\n",
			stdout: "st=0\n",
		},
		{
			name:   "an unknown name leaves the others set",
			script: "shopt -s nullglob nosuch 2>/dev/null; echo \"st=$?\"; shopt -p nullglob\n",
			stdout: "st=1\nshopt -s nullglob\n",
		},
		{
			name:   "-s with no names lists the ones on",
			script: "shopt -s nullglob\nshopt -s | grep off || echo none-off\nshopt -s | grep nullglob\n",
			stdout: "none-off\nnullglob            \ton\n",
		},
		{
			name:   "an interactive-only name is recorded",
			script: "shopt -s histappend; shopt -u checkwinsize; shopt -p histappend checkwinsize\n",
			stdout: "shopt -s histappend\nshopt -u checkwinsize\n", status: 1,
		},
		{
			name:   "a subshell sees what was recorded",
			script: "shopt -s histappend; (shopt -q histappend && echo inherited)\n",
			stdout: "inherited\n",
		},
		{
			name:   "a fixed name accepts the value it has",
			script: "shopt -s extglob globskipdots; shopt -u extquote gnu_errfmt; echo \"st=$?\"\n",
			stdout: "st=0\n",
		},
		{
			// It reports what the shell does: & in a replacement is itself, as in busybox.
			name:   "a fixed name tells the truth",
			script: "shopt -p patsub_replacement\nx=abc\necho \"${x/b/[&]}\"\n",
			stdout: "shopt -u patsub_replacement\na[&]c\n",
		},
		{
			name:   "and refuses the other",
			script: "shopt -s extquote 2>/dev/null; echo \"st=$?\"\n",
			stdout: "st=1\n",
		},
		{
			// bash accepts a request to change these and changes nothing.
			name:   "login_shell reports how the shell started",
			script: "shopt -s login_shell restricted_shell; echo \"st=$?\"; shopt -p login_shell restricted_shell\n",
			stdout: "st=0\nshopt -u login_shell\nshopt -u restricted_shell\n", status: 1,
		},
		{
			name:   "-o sets a set -o option",
			script: "shopt -so pipefail; shopt -qo pipefail; echo \"st=$?\"; set +o pipefail; shopt -po pipefail errexit\n",
			stdout: "st=0\nset +o pipefail\nset +o errexit\n", status: 1,
		},
		{
			// bash's order, by name; `set +o` keeps busybox's, where vi comes before emacs.
			name:   "-o lists by name",
			script: "shopt -po | grep -E 'emacs$|vi$'; set +o | grep -E 'emacs$|vi$'\n",
			stdout: "set +o emacs\nset +o vi\nset +o vi\nset +o emacs\n",
		},
		{
			name:   "-o with an unknown name",
			script: "shopt -o nosuch 2>&1; echo \"st=$?\"\n",
			stdout: "shopt: nosuch: invalid option name\nst=1\n",
		},
		{
			name:   "an unknown flag",
			script: "shopt -x 2>&1; echo \"st=$?\"\n",
			stdout: "shopt: -x: invalid option\nshopt: usage: shopt [-pqsu] [-o] [optname ...]\nst=2\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, stdout, stderr := runSetScript(t, test.script)

			// Then
			if stdout != test.stdout || status != test.status {
				t.Fatalf("got %q/%d, want %q/%d; stderr = %q", stdout, status, test.stdout, test.status, stderr)
			}
		})
	}
}
