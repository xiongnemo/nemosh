package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// What happens when a name is not found, beyond saying so.

// notFoundHandlerKey marks a context as running command_not_found_handle.
type notFoundHandlerKey struct{}

// commandNotFoundHandle runs bash's command_not_found_handle for a name PATH has no program
// for, and reports whether it did: in a subshell, given the name and its arguments, its status
// the command's. A name with a slash is a path rather than a lookup, and is reported as it is.
//
// A name the handler itself cannot find is reported too, rather than handed back to the
// handler. bash hands it back, a subshell deeper each time; measured, it printed about a
// thousand lines before it stopped, which is no answer anyone wanted.
func (r Runtime) commandNotFoundHandle(ctx context.Context, args []string, err error) (int, bool) {
	if !errors.Is(err, errExternalNotFound) || hasPathSeparator(args[0]) || ctx.Value(notFoundHandlerKey{}) != nil {
		return 0, false
	}
	definition, found := r.calledFunction("command_not_found_handle")
	if !found {
		return 0, false
	}
	child, err := r.subshellSnapshot(ctx)
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "%s%v\n", r.diagnosticPrefix(), err)
		return 1, true
	}
	handlerCtx := context.WithValue(ctx, notFoundHandlerKey{}, true)
	status := child.callFunction(handlerCtx, definition, args)
	status = child.runOwnExitTrap(handlerCtx, r.traps[trapExit], status)
	child.jobScope.cancelAndDrain()
	if err := child.fds.closeAll(); err != nil {
		fmt.Fprintf(r.streams.Stderr, "%s%v\n", r.diagnosticPrefix(), err)
	}
	return status, true
}

// powerShellHint says how to run a PowerShell script: one named on PATH as NAME.ps1, or one
// run by its path. This shell does not start PowerShell for a script, as it starts cmd for a
// batch file, because a .ps1 is no program Windows runs, and choosing a PowerShell -- 5 or 7,
// which policy -- is the user's to make.
func (r Runtime) powerShellHint(name string) string {
	script := name
	if !strings.EqualFold(filepath.Ext(name), ".ps1") {
		script = r.powerShellScriptOnPath(name)
	}
	if script == "" {
		return ""
	}
	return fmt.Sprintf("%s is a PowerShell script, which this shell does not run; `powershell -File %s` does", script, script)
}

// powerShellScriptOnPath is NAME.ps1 as found on PATH, or nothing.
func (r Runtime) powerShellScriptOnPath(name string) string {
	if hasPathSeparator(name) {
		return ""
	}
	for _, directory := range filepath.SplitList(r.vars["PATH"]) {
		resolved, err := r.ResolveNemoshPath(currentIfEmptyEntry(directory))
		if err != nil || resolved.Device {
			continue
		}
		if info, err := os.Stat(filepath.Join(resolved.Native, name+".ps1")); err == nil && !info.IsDir() {
			return name + ".ps1"
		}
	}
	return ""
}

func currentIfEmptyEntry(directory string) string {
	if directory == "" {
		return "."
	}
	return directory
}

// nearNames are up to three names this shell can run that the one not found is nearly: one
// edit away, or two for a name longer than four letters, a swap of neighbours counting as
// one. They come from what `compgen -c` lists -- builtins, functions, aliases, applets and
// PATH -- nearest first.
func (r Runtime) nearNames(name string) []string {
	if len([]rune(name)) < 3 {
		return nil
	}
	limit := 1
	if len([]rune(name)) > 4 {
		limit = 2
	}
	type near struct {
		name     string
		distance int
	}
	var found []near
	seen := map[string]bool{}
	for _, candidate := range r.compgenAction("command", "") {
		if seen[candidate] || candidate == name {
			continue
		}
		seen[candidate] = true
		if distance := editDistance(foldName(name), foldName(candidate)); distance <= limit {
			found = append(found, near{candidate, distance})
		}
	}
	sort.Slice(found, func(i, j int) bool {
		if found[i].distance != found[j].distance {
			return found[i].distance < found[j].distance
		}
		return found[i].name < found[j].name
	})
	var names []string
	for index := 0; index < len(found) && index < 3; index++ {
		names = append(names, found[index].name)
	}
	return names
}

// foldName is a command name as Windows compares it, which finds a program whatever its case.
func foldName(name string) string {
	return strings.ToLower(name)
}

// editDistance counts the insertions, deletions, substitutions and swaps of neighbours that
// turn a into b: the optimal string alignment distance.
func editDistance(a, b string) int {
	x, y := []rune(a), []rune(b)
	rows := make([][]int, len(x)+1)
	for i := range rows {
		rows[i] = make([]int, len(y)+1)
		rows[i][0] = i
	}
	for j := range rows[0] {
		rows[0][j] = j
	}
	for i := 1; i <= len(x); i++ {
		for j := 1; j <= len(y); j++ {
			cost := 1
			if x[i-1] == y[j-1] {
				cost = 0
			}
			rows[i][j] = min(rows[i-1][j]+1, rows[i][j-1]+1, rows[i-1][j-1]+cost)
			if i > 1 && j > 1 && x[i-1] == y[j-2] && x[i-2] == y[j-1] {
				rows[i][j] = min(rows[i][j], rows[i-2][j-2]+1)
			}
		}
	}
	return rows[len(x)][len(y)]
}

// didYouMean is the hint for near names, `git` and the rest quoted as a command is.
func didYouMean(names []string) string {
	quoted := make([]string, len(names))
	for index, name := range names {
		quoted[index] = "`" + name + "`"
	}
	if len(quoted) == 1 {
		return "did you mean " + quoted[0] + "?"
	}
	return "did you mean " + strings.Join(quoted[:len(quoted)-1], ", ") + " or " + quoted[len(quoted)-1] + "?"
}
