package main

import (
	"context"
	"fmt"

	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// startSession is startShell for a prompt, followed by $ENV. $0 is the shell's name, as
// it is in both references, and `-s ARG...` gives the session its positional parameters.
func (c command) startSession(ctx context.Context, rt runtime.Runtime) error {
	rt.SetArguments(c.scriptName(), c.invocation.operands)
	return c.prepareSession(ctx, rt, "is")
}

// runSessionCommand is `nemosh -i -c COMMAND`, bash's: a session, which reads its rc file, with
// COMMAND in place of the lines a prompt would read, and then done. An error that ends a line at
// a prompt ends one of COMMAND's lines, and the lines after it run; see runtime.MarkSession.
// It was refused, as -i with -c.
func (c command) runSessionCommand(ctx context.Context, controller *interruptController, script string, args []string) error {
	rt := c.newRuntime()
	invocation := commandStringInvocation(c.scriptName(), args)
	rt.SetArguments(invocation.name, invocation.args)
	if err := c.prepareSession(ctx, rt, "ic"); err != nil {
		return err
	}
	rt.MarkSession()
	return c.runScriptWith(ctx, controller, rt, script)
}

// prepareSession is what a session does before its first command: the shell started, a login
// one's profiles read, the session's defaults, the rc files, and what is said of them.
func (c command) prepareSession(ctx context.Context, rt runtime.Runtime, mode string) error {
	term, aside := rt.SetAsideInheritedTERM()
	// The session's defaults come before every startup file, a login shell's profiles too,
	// which startShell reads, so any of them can take one away or define it otherwise. In bash
	// mode what the profile set is told from what it found by these two, for the notice.
	var beforeProfile, afterProfile runtime.Definitions
	defaulted := false
	c.beforeProfiles = func() {
		c.sessionDefaults(rt)
		defaulted = true
		if rt.BashMode() {
			beforeProfile = rt.Definitions()
		}
	}
	if err := c.startShell(ctx, rt, mode); err != nil {
		return err
	}
	if beforeProfile != nil {
		afterProfile = rt.Definitions()
	}
	if !defaulted {
		c.sessionDefaults(rt)
	}
	if status, exited := c.sourceRCFile(ctx, rt, beforeProfile, afterProfile); exited {
		return startupExit(rt, status)
	}
	c.sayTERMSetAside(rt, term, aside)
	setDefaultHistoryFile(rt, c.stderr)
	sayOldSpecs(c.stderr, sessionSpecLookup)
	return nil
}

// sessionDefaults is what a session has before its startup files: Ubuntu's aliases and
// LS_COLORS, and in bash mode readline's inputrc, as readline has read it by the time a bashrc's
// bind runs.
func (c command) sessionDefaults(rt runtime.Runtime) {
	rt.SetSessionDefaults()
	if rt.BashMode() {
		rt.ReadInputrc()
	}
}

// sourceRCFile is a session's rc file: $ENV, or bash's --rcfile FILE in its place, or none for
// --norc. In bash mode an interactive shell that is not a login one reads ~/.bashrc, or the
// --rcfile, first, and then nemosh's, as the user chose: one ~/.bashrc serves Git's bash and
// this shell, and nemosh's has the last word, and says what it changed of bash's. A login one
// has read bash's profile already; before and afterBash are what it found and left.
func (c command) sourceRCFile(ctx context.Context, rt runtime.Runtime, before, afterBash runtime.Definitions) (int, bool) {
	switch {
	case c.invocation.norc:
		return 0, false
	case rt.BashMode():
		if !c.invocation.login {
			before = rt.Definitions()
			if status, exited := c.sourceBashRC(ctx, rt); exited {
				return status, true
			}
			afterBash = rt.Definitions()
		}
		beforeNemosh := rt.Definitions()
		status, exited := sourceStartupFile(ctx, rt, c.stderr)
		if name, read := nemoshRCName(rt); read && !exited && before != nil {
			if names := overridden(before, afterBash, beforeNemosh, rt.Definitions()); len(names) > 0 {
				fmt.Fprint(c.stderr, overriddenNotice(name, c.bashFileName(rt), names))
			}
		}
		return status, exited
	case c.invocation.rcfile != "":
		return sourceProfile(ctx, rt, c.stderr, c.invocation.rcfile, rt.ResolvePath(c.invocation.rcfile))
	}
	return sourceStartupFile(ctx, rt, c.stderr)
}

// sourceBashRC is bash's rc file: --rcfile's, or ~/.bashrc.
func (c command) sourceBashRC(ctx context.Context, rt runtime.Runtime) (int, bool) {
	if c.invocation.rcfile != "" {
		return sourceProfile(ctx, rt, c.stderr, c.invocation.rcfile, rt.ResolvePath(c.invocation.rcfile))
	}
	return sourceHomeFile(ctx, rt, c.stderr, ".bashrc")
}
