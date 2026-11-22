package runtime

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

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
	// bash expands aliases in a shell that is not interactive only under `shopt -s
	// expand_aliases`, a session from the start; busybox always does. The invocation is set by
	// now (SetInvocationMode), so a session's `i` is in it.
	if !strings.ContainsRune(r.options.invocation, 'i') {
		r.options.expandAliases = false
	}
	// Git for Windows' bash starts with a umask of 0022, busybox-w32 with 0002; see
	// initialFileModeMask. A background job's process is given its shell's mask after this.
	r.mask.set(0o022)
	// bash's unset takes a caller's local away; see dynamic_unset.go.
	r.options.localVarUnset = false
	r.setBashStartupVariables()
}

// setBashStartupVariables sets what bash's start sets and busybox's does not: PPID an integer and
// read-only, `declare -ir PPID="..."`, and `$_` the path that ran the shell, so `set -u` finds it
// set. busybox's PPID takes an assignment, and its `$_` is unset until a command has run, as
// nemosh's and sh's are. Measured against Git for Windows' bash 5.3.
func (r Runtime) setBashStartupVariables() {
	if r.special != nil {
		r.vars["PPID"] = strconv.Itoa(r.special.ppid)
		r.readonly["PPID"] = struct{}{}
		attributes := r.attributes["PPID"]
		attributes.integer = true
		r.attributes["PPID"] = attributes
	}
	if _, set := r.vars["_"]; !set {
		if path, err := os.Executable(); err == nil {
			r.vars["_"] = filepath.ToSlash(path)
		}
	}
}

// BashMode is whether the shell is in bash mode.
func (r Runtime) BashMode() bool {
	return r.options.bashMode
}

// BashBuiltinPrefix is what an applet that is a builtin in bash says a diagnostic after in
// bash mode, as bash's builtins say theirs, `script: line 3: `; false outside bash mode. printf
// asks it, through the view the shell runs an applet with.
func (r Runtime) BashBuiltinPrefix() (string, bool) {
	if !r.BashMode() {
		return "", false
	}
	return r.diagnosticPrefix(), true
}

// announcesJobs is whether a job put in the background is announced, `[1] PID`: at a session's
// own command line. bash says nothing for one in a subshell, a command substitution or a
// pipeline stage, and busybox-w32 says it there too.
func (r Runtime) announcesJobs() bool {
	return r.interactive.session && (!r.BashMode() || r.subshellDepth == 0 && !r.interactive.stage)
}
