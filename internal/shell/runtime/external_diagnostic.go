package runtime

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// reportLookupFailure turns a failed executable lookup into the layered
// diagnostic of P1.1 and the status SUSv3 gives it: 127 when nothing was found,
// 126 when something was and could not be run.
//
// The first line is what a script greps and does not move. The hint is what a
// person needs and is only offered where there is something to say. The detail
// -- which directories were searched, which suffixes were tried -- is on the
// `exec` channel, because printing the search path every time would bury the
// one line that matters and would leak host paths into output that behaviour
// cases compare byte for byte.
func (r Runtime) reportLookupFailure(name string, err error) int {
	switch {
	case errors.Is(err, errExternalNotFound):
		r.report(name, shellDiagnostic{
			message: "not found",
			hint:    r.notFoundHint(name),
			channel: debugExec,
			details: r.debugDetails(debugExec, func() []string { return r.lookupDetails(name) }),
		})
		return 127
	case errors.Is(err, errExternalNotExecutable):
		r.report(name, shellDiagnostic{
			message: err.Error(),
			hint:    notExecutableHint(name),
			channel: debugExec,
			details: r.debugDetails(debugExec, func() []string { return r.lookupDetails(name) }),
		})
		return 126
	default:
		r.report(name, shellDiagnostic{
			message: err.Error(),
			channel: debugPath,
			details: r.debugDetails(debugPath, func() []string { return r.pathDetails(name) }),
		})
		return 1
	}
}

// notFoundHint says the useful thing rather than restating the failure. A name
// with a separator in it was meant as a path, so PATH is not the answer; a bare
// name with no PATH to search is a different problem again.
func (r Runtime) notFoundHint(name string) string {
	if hasPathSeparator(name) {
		return fmt.Sprintf("%s was read as a path, not a PATH lookup; check the spelling and that the file exists", name)
	}
	if value, ok := r.vars["PATH"]; !ok || value == "" {
		return "PATH is empty, so no directory was searched"
	}
	if isRuntimeBuiltin(name) {
		return name + " is a shell builtin here, so this was a lookup for an external program of the same name"
	}
	return fmt.Sprintf("no directory on PATH holds %s; `command -v %s` answers the same question", name, name)
}

func notExecutableHint(name string) string {
	if strings.HasSuffix(strings.ToLower(name), ".dll") {
		return "a DLL is loaded by a program, not launched as one"
	}
	return "the file is there but is not something Windows will start; check that it is a program and not data"
}

// lookupDetails is the `exec` channel: what lookup actually did. It names the
// directories in the order they were searched and the suffixes appended to a
// bare name, which is the pair of facts that explain almost every miss.
func (r Runtime) lookupDetails(name string) []string {
	details := []string{fmt.Sprintf("name %q", name)}
	if hasPathSeparator(name) {
		return append(details, "contains a separator, so PATH was not searched")
	}
	value := r.vars["PATH"]
	entries := filepath.SplitList(value)
	details = append(details, fmt.Sprintf("PATH has %d entries", len(entries)))
	for _, entry := range entries {
		if entry == "" {
			entry = "."
		}
		details = append(details, "searched "+filepath.ToSlash(entry))
	}
	return append(details, "suffixes tried "+strings.Join(windowsExecutableSuffixes[:], " "))
}

// pathDetails is the `path` channel: how an operand became a host path, which
// is the question behind every "no such file" that names a spelling the user
// believes in.
func (r Runtime) pathDetails(operand string) []string {
	details := []string{
		fmt.Sprintf("operand %q", operand),
		"working directory " + r.WorkingDirectory(),
	}
	resolved, err := r.ResolveNemoshPath(operand)
	if err != nil {
		return append(details, "resolution failed: "+err.Error())
	}
	details = append(details, "canonical "+string(resolved.Canonical))
	if resolved.Device {
		return append(details, "resolved to a device, which has no host path")
	}
	return append(details, "native "+filepath.ToSlash(resolved.Native))
}

// reportRedirectFailure is the layered contract for a redirection that could not
// be applied. The first line keeps the shape callers already print, so nothing
// that greps it moves; the hint names the way out where there is one, and the
// detail is on the `fd` channel because a descriptor problem is answered by
// knowing which descriptor and which path.
func (r Runtime) reportRedirectFailure(err error) {
	r.report("nemosh", shellDiagnostic{
		message: err.Error(),
		hint:    redirectHint(err),
		channel: debugFD,
		details: r.debugDetails(debugFD, func() []string {
			return []string{
				"working directory " + r.WorkingDirectory(),
				"open descriptors " + r.fds.describe(),
			}
		}),
	})
}

func redirectHint(err error) string {
	text := err.Error()
	switch {
	case strings.Contains(text, "cannot overwrite existing file"):
		return "`set -C` is on; write with `>|` to truncate anyway, or `>>` to append"
	case errors.Is(err, errAmbiguousRedirect):
		return "the operand expanded to more or fewer than one word; quote it to keep it whole"
	case strings.Contains(text, "unsupported device"):
		return "this device is readable or writable but not both; see docs/design/windows-path-model.md for the v0 set"
	case strings.Contains(text, "The system cannot find the path"),
		strings.Contains(text, "no such file or directory"),
		strings.Contains(text, "No such file or directory"):
		return "the parent directory has to exist already; a redirection creates the file and not the path to it"
	}
	return ""
}
