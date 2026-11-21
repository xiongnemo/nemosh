package main

import (
	"strings"
	"testing"
)

// bash expands aliases in a shell that is not interactive only once `shopt -s expand_aliases`
// asks it to, and a session from the start; busybox always does. So a bash-mode script or -c
// does not, a bash-mode session does, and the nemosh name goes on as busybox. Measured
// 2026-11-21 with Git for Windows' bash 5.3: `bash -c 'alias e=echo<newline>e x'` is
// `e: command not found`, 127.
func TestBashMode_expandsAliasesOnlyInASessionUnlessAsked(t *testing.T) {
	t.Setenv("PS1", "")
	t.Setenv("PS2", "")
	script := "alias e=echo\ne x\n"
	if got := runInvocation(t, "", "bash", "-c", script); got.status != 127 || got.stdout != "" || !strings.Contains(got.stderr, "e: not found") {
		t.Errorf("bash -c: status %d, stdout %q, stderr %q; want e not found, 127", got.status, got.stdout, got.stderr)
	}
	if got := runInvocation(t, "", "bash", "-c", "shopt -s expand_aliases\n"+script); got.stdout != "x\n" {
		t.Errorf("bash -c after shopt -s expand_aliases: stdout %q, stderr %q", got.stdout, got.stderr)
	}
	if got := runInvocation(t, script, "bash", "--norc", "-i"); got.stdout != "x\n" {
		t.Errorf("bash -i: stdout %q, stderr %q; want the alias", got.stdout, got.stderr)
	}
	if got := runInvocation(t, "", "-c", script); got.stdout != "x\n" {
		t.Errorf("nemosh -c: stdout %q, stderr %q; want busybox's, the alias", got.stdout, got.stderr)
	}
}
