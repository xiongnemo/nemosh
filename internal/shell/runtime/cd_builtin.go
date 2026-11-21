package runtime

import (
	"errors"
	"fmt"
	"os"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/pathmodel"
)

// cd follows busybox ash's cdcmd (shell/ash.c:11808): no operand goes to HOME,
// a lone `-` goes to OLDPWD and prints where it landed, and both PWD and OLDPWD
// are updated and exported afterwards (setpwd, shell/ash.c:3571,3591). Nemosh
// kept none of that: bare `cd` stayed put, `-` was looked up as a directory
// name, and $PWD held whatever the process started with -- a stale value that
// was then handed to every child.
func (r Runtime) cd(args []string) int { return r.changeDirectory("cd", args) }

// changeDirectory is cd's body, with the name to blame handed in.
//
// pushd and popd move the shell by calling this, and before it took a name they
// reported their failures as `cd:` -- so `pushd nosuchdir` blamed a builtin the user had
// not typed. bash attributes it to the one that was run, and so does this.
func (r Runtime) changeDirectory(as string, args []string) int {
	physical, args, ok := r.directoryOptions(as, args)
	if !ok {
		return 2
	}
	// bash takes one operand, and refuses two; busybox goes to the first.
	if as == "cd" && len(args) > 1 && r.BashMode() {
		fmt.Fprintln(r.streams.Stderr, r.diagnosticPrefix()+"cd: too many arguments")
		return 2
	}
	target, printResult, ok := r.cdTarget(as, args)
	if !ok {
		return 1
	}
	// CDPATH: a relative operand is looked for under each entry, the current directory
	// last. Only the *last* candidate reports a failure, so `cd nosuch` still says what
	// it always did rather than complaining once per entry.
	candidates := r.cdPathTargets(target)
	for _, candidate := range candidates[:len(candidates)-1] {
		// A directory an entry of CDPATH supplied is printed, as POSIX has it and both
		// references do, so `cd sub` says which sub it went to; one an empty entry found,
		// which is the current directory's, is not.
		if status := r.tryChangeDirectory(as, candidate, printResult || candidate != target, true, physical); status == 0 {
			return 0
		}
	}
	target = candidates[len(candidates)-1]
	return r.tryChangeDirectory(as, target, printResult, false, physical)
}

// reportCD prints a cd diagnostic unless this attempt is a silent CDPATH probe.
func reportCD(r Runtime, quiet bool, format string, args ...any) {
	if quiet {
		return
	}
	fmt.Fprintf(r.streams.Stderr, format, args...)
}

// cdFailure says why the shell could not go to target, and answers the status for it: busybox's
// words, but "cannot" for its "can't", and its 2 for cd, from the error cdcmd raises (`can't cd
// to %s`), which leaves 2 as any regular builtin's error does. pushd and popd are bash's, and
// say bash's, with 1.
func (r Runtime) cdFailure(as, target, reason string, quiet bool) int {
	if as != "cd" {
		reportCD(r, quiet, "%s%s: %s: %s\n", r.diagnosticPrefix(), as, target, reason)
		return 1
	}
	// bash mode's are bash's words and status, `cd: DIR: No such file or directory` and 1.
	status := 2
	if r.BashMode() {
		reportCD(r, quiet, "%scd: %s: %s\n", r.diagnosticPrefix(), target, reason)
		status = 1
	} else {
		reportCD(r, quiet, "%scd: cannot cd to %s: %s\n", r.diagnosticPrefix(), target, reason)
	}
	// `cd C:\Users` unquoted reaches cd as C:Users, the backslash taken as an escape, which is a
	// drive-relative path and goes somewhere else or nowhere. It was refused outright before
	// drive-relative paths worked, which at least said something was odd. At a prompt only, as
	// the other hints are: busybox-w32 says nothing, and a script's diagnostics stay its own.
	if r.interactive.session && !quiet && looksLikeEatenBackslash(target) {
		reportCD(r, quiet, "hint: a backslash outside quotes is an escape, so %s may have been typed %s:\\%s; quote it, or write %s:/%s\n",
			target, target[:1], target[2:], target[:1], target[2:])
	}
	return status
}

// looksLikeEatenBackslash reports a drive-relative path with something after the drive, which is
// what an unquoted `C:\Users` becomes.
func looksLikeEatenBackslash(target string) bool {
	return len(target) > 2 && isDriveLetter(rune(target[0])) && target[1] == ':' && target[2] != '/' && target[2] != '\\'
}

// tryChangeDirectory is one attempt. quiet suppresses the diagnostics, which is what lets
// the CDPATH search try several places without narrating each miss.
func (r Runtime) tryChangeDirectory(as, target string, printResult, quiet, physical bool) int {
	// bash's directory before a `..` has to be there; see cd_dotdot_bash.go.
	if r.bashDotDotMissing(target) {
		return r.cdFailure(as, target, "No such file or directory", quiet)
	}
	resolved, err := r.ResolveNemoshPath(target)
	if err != nil {
		// A host-only UNC path is not a missing directory, it is not a
		// directory at all, so the failure reads as one line of the ordinary
		// shape plus a line naming what to type instead. The hint comes from
		// the path model rather than from the operand, so `//server/` advises
		// `//server/share` and not `//server//share`, and a hostless `//` --
		// which the model calls malformed, not host-only -- gets no share
		// suggestion it cannot complete.
		var hostOnly pathmodel.HostOnlyUNCError
		if errors.As(err, &hostOnly) {
			status := r.cdFailure(as, target, "No such file or directory", quiet)
			reportCD(r, quiet, "hint: %v\n", hostOnly)
			return status
		}
		return r.cdFailure(as, target, err.Error(), quiet)
	}
	if resolved.Device {
		// Refused, and not as "not a directory" -- `test -d /dev` is true, and a shell
		// contradicting its own answer is worse than a refusal. The reason is that a working
		// directory needs a native form: launching a child process sets one, and /dev has
		// none. A cd that succeeded would leave every external command running in the
		// directory the shell was in before while `pwd` said /dev, which is a silent
		// disagreement rather than an error. /tmp is the contrast that makes this a rule
		// rather than an inconsistency: `cd /tmp` works because /tmp has a native mapping.
		return r.cdFailure(as, target, "a device directory cannot be a working directory", quiet)
	}
	info, err := os.Stat(resolved.Native)
	if err != nil {
		// Described rather than dumped. The raw error names a Win32 call the user did
		// not make -- `GetFileAttributesEx C:\...: The system cannot find the file
		// specified.` -- where every shell says `No such file or directory`.
		// applets.CauseText is the mapping every applet diagnostic already uses, so a
		// missing directory reads the same whether an applet or a builtin found it.
		return r.cdFailure(as, target, applets.CauseText(err), quiet)
	}
	// Not folded into the branch above: with a nil error there was nothing to
	// format, so a `cd` onto a regular file reported the literal text <nil> as
	// its reason.
	if !info.IsDir() {
		return r.cdFailure(as, target, "Not a directory", quiet)
	}
	if physical {
		resolved = r.physicalPath(resolved)
	}
	previous := r.WorkingDirectory()
	// The directory is stored under the spelling on disk, so `pwd` and every
	// diagnostic below it answer with the real case rather than with whatever
	// this operand happened to be typed as (windows-path-model.md, "Path case").
	r.paths.setWorkingDirectory(r.withRealCase(resolved))
	r.setDirectoryVariable("OLDPWD", previous)
	r.setDirectoryVariable("PWD", r.WorkingDirectory())
	if printResult {
		fmt.Fprintln(r.streams.Stdout, r.WorkingDirectory())
	}
	return 0
}

// PWD and OLDPWD are exported the way busybox sets them, because their whole
// use is to be read by something else -- a prompt, a subshell, a child process.
func (r Runtime) setDirectoryVariable(name, value string) {
	r.vars[name] = value
	r.env.Set(name, value)
	r.markVarMutation(name)
}
