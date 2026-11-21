package runtime

// Bash mode is this shell run as bash: named bash, as `nemosh bash`, a nemosh called
// bash.exe, a command named bash and a `#!/bin/bash` script run it, or named nemosh with
// NEMOSH_COMPAT=bash in its environment. Where bash and busybox part, a shell in bash mode
// follows bash, so a script or a tool's init file written for Git for Windows' bash runs as
// it does there; run as nemosh or sh, the shell keeps busybox's way. A difference only bash
// has, or a defect both references agree on, is no question of mode and is not decided here.
// docs/support-matrix.md, Bash mode, lists each place the mode is consulted.

// EnterBashMode is a shell run as bash saying so: BASH_VERSION and BASH_VERSINFO, and the
// mode every place where the two references part consults. A subshell and a background job
// are in it as their shell is.
func (r Runtime) EnterBashMode() {
	r.SetBashVersion()
	r.options.bashMode = true
}

// BashMode is whether the shell is in bash mode.
func (r Runtime) BashMode() bool {
	return r.options.bashMode
}

// announcesJobs is whether a job put in the background is announced, `[1] PID`: at a session's
// own command line. bash says nothing for one in a subshell, a command substitution or a
// pipeline stage, and busybox-w32 says it there too.
func (r Runtime) announcesJobs() bool {
	return r.interactive.session && (!r.BashMode() || r.subshellDepth == 0 && !r.interactive.stage)
}
