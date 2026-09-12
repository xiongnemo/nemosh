package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/pathmodel"
)

func (r Runtime) pwd() int {
	_, err := fmt.Fprintln(r.streams.Stdout, r.WorkingDirectory())
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "pwd: %v\n", err)
		return 1
	}
	return 0
}

func (r Runtime) export(args []string) int {
	// No operands, or -p, lists what is exported. POSIX specifies the -p form;
	// busybox lists for both, and a shell that prints nothing leaves a user no
	// way to see the environment at all (nemosh issue #10).
	if len(args) == 0 || (len(args) == 1 && args[0] == "-p") {
		return r.listExported()
	}
	for _, arg := range args {
		name, value, hasValue := strings.Cut(arg, "=")
		if name == "" {
			return 2
		}
		if !hasValue {
			value = r.vars[name]
		} else if r.isReadonly(name) {
			fmt.Fprintf(r.streams.Stderr, "export: %s: readonly variable\n", name)
			return 1
		}
		r.vars[name] = value
		r.env.Set(name, value)
		r.markVarMutation(name)
	}
	return 0
}

// unset removes variables, and array elements when the operand carries a subscript.
//
// `unset a[1]` used to return 0 and do nothing at all -- the subscript was never parsed,
// so the whole `a[1]` was looked up as a variable name, found not to exist, and deleting
// a name that is not there succeeds. A script that removed an element and carried on was
// silently wrong, which is the failure mode AGENTS.md singles out: a capability that is
// absent must fail loudly. This one was absent and silent.
func (r Runtime) unset(ctx context.Context, args []string) int {
	for _, name := range args {
		base, subscript, hasSubscript := splitSubscriptedName(name)
		if r.isReadonly(base) {
			fmt.Fprintf(r.streams.Stderr, "unset: %s: readonly variable\n", base)
			return 1
		}
		if !hasSubscript {
			delete(r.vars, name)
			r.arrays.unset(name)
			r.env.Unset(name)
			r.markVarMutation(name)
			continue
		}
		// `a[@]` and `a[*]` remove the whole array, as bash does. Spelling it this way
		// is rare but it is what a script that builds the name does.
		if subscript == "@" || subscript == "*" {
			delete(r.vars, base)
			r.arrays.unset(base)
			r.env.Unset(base)
			r.markVarMutation(base)
			continue
		}
		// An associative name is answered by *key*, so the subscript is a word and not
		// arithmetic: `m[k]` removes the key `k`, where `a[k]` on an indexed array
		// evaluates `k` as a variable and removes that index. Checked before the
		// arithmetic path because `m[0]` is a perfectly good key and would otherwise be
		// read as an index into an array that has none.
		if array, ok := r.arrays.associative[base]; ok {
			array.remove(subscript)
			r.markVarMutation(base)
			continue
		}
		index, err := r.resolveSubscript(ctx, subscript)
		if err != nil {
			fmt.Fprintf(r.streams.Stderr, "unset: %s: %v\n", name, err)
			return 1
		}
		r.arrays.unsetElement(base, index)
		r.markVarMutation(base)
	}
	return 0
}

// splitSubscriptedName separates `a[1]` into its name and its subscript.
//
// The closing bracket must be the last character, so `a[1]x` is a name rather than a
// subscript -- bash refuses that as a bad substitution, and treating it as `a[1]` would
// unset something the writer did not name.
func splitSubscriptedName(text string) (name, subscript string, ok bool) {
	if !strings.HasSuffix(text, "]") {
		return text, "", false
	}
	open := strings.Index(text, "[")
	if open <= 0 {
		return text, "", false
	}
	return text[:open], text[open+1 : len(text)-1], true
}

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
	target, printResult, ok := r.cdTarget(as, args)
	if !ok {
		return 1
	}
	// CDPATH: a relative operand is looked for under each entry, the current directory
	// last. Only the *last* candidate reports a failure, so `cd nosuch` still says what
	// it always did rather than complaining once per entry.
	candidates := r.cdPathTargets(target)
	for _, candidate := range candidates[:len(candidates)-1] {
		if status := r.tryChangeDirectory(as, candidate, printResult, true); status == 0 {
			return 0
		}
	}
	target = candidates[len(candidates)-1]
	return r.tryChangeDirectory(as, target, printResult, false)
}

// reportCD prints a cd diagnostic unless this attempt is a silent CDPATH probe.
func reportCD(r Runtime, quiet bool, format string, args ...any) {
	if quiet {
		return
	}
	fmt.Fprintf(r.streams.Stderr, format, args...)
}

// tryChangeDirectory is one attempt. quiet suppresses the diagnostics, which is what lets
// the CDPATH search try several places without narrating each miss.
func (r Runtime) tryChangeDirectory(as, target string, printResult, quiet bool) int {
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
			reportCD(r, quiet, "%s: %s: No such file or directory\n", as, target)
			reportCD(r, quiet, "hint: %v\n", hostOnly)
			return 1
		}
		reportCD(r, quiet, "%s: %s: %v\n", as, target, err)
		return 1
	}
	if resolved.Device {
		// Refused, and not as "not a directory" -- `test -d /dev` is true, and a shell
		// contradicting its own answer is worse than a refusal. The reason is that a working
		// directory needs a native form: launching a child process sets one, and /dev has
		// none. A cd that succeeded would leave every external command running in the
		// directory the shell was in before while `pwd` said /dev, which is a silent
		// disagreement rather than an error. /tmp is the contrast that makes this a rule
		// rather than an inconsistency: `cd /tmp` works because /tmp has a native mapping.
		reportCD(r, quiet, "%s: %s: a device directory cannot be a working directory\n", as, target)
		return 1
	}
	info, err := os.Stat(resolved.Native)
	if err != nil {
		// Described rather than dumped. The raw error names a Win32 call the user did
		// not make -- `GetFileAttributesEx C:\...: The system cannot find the file
		// specified.` -- where every shell says `No such file or directory`.
		// applets.CauseText is the mapping every applet diagnostic already uses, so a
		// missing directory reads the same whether an applet or a builtin found it.
		reportCD(r, quiet, "%s: %s: %s\n", as, target, applets.CauseText(err))
		return 1
	}
	// Not folded into the branch above: with a nil error there was nothing to
	// format, so a `cd` onto a regular file reported the literal text <nil> as
	// its reason.
	if !info.IsDir() {
		reportCD(r, quiet, "%s: %s: Not a directory\n", as, target)
		return 1
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

// cdTarget answers where to go, whether to print it on arrival, and whether the
// operands made sense at all.
func (r Runtime) cdTarget(as string, args []string) (string, bool, bool) {
	if len(args) == 0 {
		home := r.vars["HOME"]
		if home == "" {
			fmt.Fprintln(r.streams.Stderr, fmt.Sprintf("%s: HOME not set", as))
			return "", false, false
		}
		return home, false, true
	}
	if args[0] != "-" {
		return args[0], false, true
	}
	previous := r.vars["OLDPWD"]
	if previous == "" {
		fmt.Fprintln(r.streams.Stderr, fmt.Sprintf("%s: OLDPWD not set", as))
		return "", false, false
	}
	return previous, true, true
}

// PWD and OLDPWD are exported the way busybox sets them, because their whole
// use is to be read by something else -- a prompt, a subshell, a child process.
func (r Runtime) setDirectoryVariable(name, value string) {
	r.vars[name] = value
	r.env.Set(name, value)
	r.markVarMutation(name)
}

// listExported writes every exported variable as `export NAME='value'`.
//
// Quoted for reuse, which is what POSIX means by writing them "in a form that
// can be reused as input" -- singleQuoteForReuse is the same helper `alias`
// uses, so a value carrying a quote comes back correct from either.
//
// Sorted by name, so two runs of the same script produce the same output and a
// difference between them means something.
func (r Runtime) listExported() int {
	entries := r.env.Environ()
	sort.Strings(entries)
	for _, entry := range entries {
		name, value, found := strings.Cut(entry, "=")
		if !found || name == "" {
			continue
		}
		fmt.Fprintf(r.streams.Stdout, "export %s=%s\n", name, singleQuoteForReuse(value))
	}
	return 0
}
