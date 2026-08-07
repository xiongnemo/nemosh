package behavior

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// A reference is a local shell a golden case can be compared against. The
// comparison exists to catch the cases where Nemosh and the reference disagree
// about something the case itself did not think to pin -- a case author writes
// down what they expect, and what they expect is exactly where a wrong
// assumption hides.
type reference struct {
	// name is the token a case puts in its `references` list.
	name string
	// candidates are the program names to look for, in order of preference.
	candidates []string
	// argv builds the command for a script, given the resolved program.
	argv func(program, script string) []string
	// platform, when set, is the only GOOS this reference can be resolved on.
	//
	// `busybox-w32` needs it: the name means the Windows port, and a `busybox`
	// on a Linux PATH is a different program that answers Windows questions
	// wrongly. Measured -- on ubuntu-latest it reported `one\r\r\ntwo\r\n` for
	// the lone-carriage-return case, because a Linux busybox normalizes no line
	// endings at all, which says nothing about whether Nemosh matches the port
	// it is meant to match.
	platform string
}

// The references Nemosh's cases already name. busybox-w32 is the primary
// Windows behaviour reference (AGENTS.md); dash is the closest thing to a plain
// POSIX shell; bash --posix is what most readers actually have.
var references = []reference{
	{
		name:       "busybox-w32",
		candidates: []string{"busybox", "busybox.exe"},
		argv:       func(program, script string) []string { return []string{program, "sh", "-c", script} },
		platform:   "windows",
	},
	{
		name:       "busybox-ash",
		candidates: []string{"busybox", "busybox.exe"},
		argv:       func(program, script string) []string { return []string{program, "ash", "-c", script} },
	},
	{
		name:       "dash",
		candidates: []string{"dash", "dash.exe"},
		argv:       func(program, script string) []string { return []string{program, "-c", script} },
	},
	{
		name:       "bash-posix",
		candidates: []string{"bash", "bash.exe"},
		argv:       func(program, script string) []string { return []string{program, "--posix", "-c", script} },
	},
	{
		name:       "bash",
		candidates: []string{"bash", "bash.exe"},
		argv:       func(program, script string) []string { return []string{program, "-c", script} },
	},
	{
		name:       "posix",
		candidates: []string{"dash", "dash.exe", "bash", "bash.exe"},
		argv:       func(program, script string) []string { return []string{program, "-c", script} },
	},
}

// ReferenceExecutor resolves a reference by name and returns an executor for it,
// or a reason it is unavailable. An absent reference is a skip and not a
// failure: this machine's toolbox is not the contract.
func ReferenceExecutor(name string) (ScriptExecutor, string) {
	for _, candidate := range references {
		if candidate.name != name {
			continue
		}
		if candidate.platform != "" && candidate.platform != runtime.GOOS {
			return nil, fmt.Sprintf("reference %s exists only on %s", name, candidate.platform)
		}
		program, found := firstOnPath(candidate.candidates)
		if !found {
			return nil, fmt.Sprintf("reference %s is not installed (looked for %s)",
				name, strings.Join(candidate.candidates, ", "))
		}
		return referenceScriptExecutor(program, candidate), ""
	}
	return nil, fmt.Sprintf("reference %s has no adapter", name)
}

func firstOnPath(candidates []string) (string, bool) {
	for _, candidate := range candidates {
		if resolved, err := exec.LookPath(candidate); err == nil {
			return resolved, true
		}
	}
	return "", false
}

func referenceScriptExecutor(program string, ref reference) ScriptExecutor {
	return func(ctx context.Context, request ScriptRequest) (ProcessResult, error) {
		argv := ref.argv(program, request.Script)
		command := exec.CommandContext(ctx, argv[0], argv[1:]...)
		command.Dir = request.Dir
		command.Stdin = strings.NewReader(request.Stdin)
		// The reference gets the same wholesale environment replacement the
		// Nemosh executor uses, so neither inherits something the other does
		// not. PATH survives, because a reference shell needs its own tools.
		command.Env = append([]string{"PATH=" + os.Getenv("PATH")}, request.Env...)
		var stdout, stderr strings.Builder
		command.Stdout = &stdout
		command.Stderr = &stderr
		err := command.Run()
		status := 0
		if exitErr, ok := err.(*exec.ExitError); ok {
			status = exitErr.ExitCode()
		} else if err != nil {
			return ProcessResult{}, err
		}
		return ProcessResult{Status: status, Stdout: stdout.String(), Stderr: stderr.String()}, nil
	}
}

// Divergence is one disagreement between Nemosh and a reference.
type Divergence struct {
	Reference string
	Field     string
	Nemosh    string
	Expected  string
}

func (d Divergence) String() string {
	return fmt.Sprintf("%s: %s: nemosh %q, %s %q", d.Reference, d.Field, d.Nemosh, d.Reference, d.Expected)
}

// CompareWithReference reports where a reference disagrees with the case's own
// expectations.
//
// Status and stdout are compared; stderr is not. Diagnostic wording is where
// shells differ most and where they are least required to agree -- POSIX
// specifies that a message is written, not what it says -- so comparing it would
// bury real divergences under noise. The cases that care about Nemosh's own
// wording pin it directly in `expect.stderr`.
func CompareWithReference(name string, expect Expect, actual ProcessResult) []Divergence {
	var divergences []Divergence
	if expect.Status != actual.Status {
		divergences = append(divergences, Divergence{
			Reference: name, Field: "status",
			Nemosh: fmt.Sprint(expect.Status), Expected: fmt.Sprint(actual.Status),
		})
	}
	if expect.Stdout != actual.Stdout {
		divergences = append(divergences, Divergence{
			Reference: name, Field: "stdout",
			Nemosh: expect.Stdout, Expected: actual.Stdout,
		})
	}
	return divergences
}

// ComparableReferences returns the references a case names that this build knows
// how to drive. `nemosh` and `busybox` metadata that names no adapter is left
// out rather than reported, because a case is allowed to cite a reference for
// provenance without asking to be run against it.
func ComparableReferences(c Case) []string {
	var names []string
	for _, name := range c.References {
		for _, known := range references {
			if known.name == name {
				names = append(names, name)
				break
			}
		}
	}
	return names
}

// provenanceOnlyReferences are the names a case may cite to say where a rule
// came from, without asking to be run against anything. `posix` and `busybox`
// name documents and source trees; `nemosh` names this shell's own extension.
var provenanceOnlyReferences = map[string]bool{
	"posix":           false, // has an adapter; listed here to say the omission is deliberate
	"busybox":         true,
	"busybox-w32-ash": true,
	"nemosh":          true,
	"dash-posix":      true,
}

// ProvenanceOnlyReference reports whether a cited reference is documentation
// rather than something to execute.
func ProvenanceOnlyReference(name string) bool {
	return provenanceOnlyReferences[name]
}
