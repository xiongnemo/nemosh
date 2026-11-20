package runtime

import (
	"errors"
	"fmt"
	"path/filepath"
)

// externalCommandPath is where name is run from, which the shell's table remembers; see
// command_hash.go. Running a command, `command -v`, `type` and `hash NAME` look it up so, as
// busybox's find_command does for them.
func (r Runtime) externalCommandPath(name string) (string, error) {
	return r.commandPath(name, true)
}

// knownCommandPath is the same answer without adding to the table, for what asks PATH as a
// program of its own would -- busybox's which, xargs and env are applets that search it
// themselves -- and for the checks dispatch makes before it runs anything.
func (r Runtime) knownCommandPath(name string) (string, error) {
	return r.commandPath(name, false)
}

// programPath is commandPath without the shell's own names: the file name names, or PATH's
// program by it.
func (r Runtime) programPath(name string, remember bool) (string, error) {
	// An empty name is no command anywhere. Joined to a PATH directory it named the directory,
	// and `""` was refused as not executable, 126, where both references say not found.
	if name == "" {
		return "", errExternalNotFound
	}
	if hasPathSeparator(name) {
		resolved, err := r.ResolveNemoshPath(name)
		if err != nil {
			return "", err
		}
		if resolved.Device {
			return "", fmt.Errorf("%s is %w: %w", resolved.Canonical, errExternalIsDevice, errExternalNotExecutable)
		}
		return executableCandidate(resolved.Native)
	}
	pathValue, present := r.vars["PATH"]
	// `command -p` and the like search a PATH of their own, which the table is not about:
	// busybox's DO_ALTPATH neither reads it nor adds to it.
	alternate := r.searchPath != ""
	if alternate {
		pathValue, present = r.searchPath, true
	}
	if !present || pathValue == "" {
		return "", errExternalNotFound
	}
	stat := r.commands.statter()
	if alternate || r.commands == nil {
		found, _, err := r.searchCommand(stat, pathValue, name, hashedCommand{index: -1})
		return found, err
	}
	previous, known := r.commands.entry(pathValue, name)
	if !known {
		previous.index = -1
	} else if !previous.rehash {
		if stillAProgram(stat, previous.native) {
			return previous.native, nil
		}
		previous.index = -1
	}
	found, index, err := r.searchCommand(stat, pathValue, name, previous)
	switch {
	case !remember:
	case err != nil:
		r.commands.forget(pathValue, name)
	case index >= 0:
		r.commands.remember(pathValue, name, found, index)
	}
	return found, err
}

// searchCommand looks for name along pathValue, and answers where it found it and in which of
// the directories. previous is an entry a `cd` asked to be checked again, which busybox's
// find_command does not search for from the start: a directory of PATH that is absolute, before
// the one the entry was found in, held nothing then and is not asked again, and the entry's own
// is asked only whether the program is still there. A relative one is somewhere else now.
func (r Runtime) searchCommand(stat statFunc, pathValue, name string, previous hashedCommand) (string, int, error) {
	var firstCandidateErr error
	for index, dir := range filepath.SplitList(pathValue) {
		if index <= previous.index && !relativeSearchDirectory(dir) {
			if index < previous.index {
				continue
			}
			if stillAProgram(stat, previous.native) {
				return previous.native, index, nil
			}
		}
		if dir == "" {
			dir = "."
		}
		resolved, err := r.ResolveNemoshPath(dir)
		if err != nil {
			if firstCandidateErr == nil {
				firstCandidateErr = err
			}
			continue
		}
		if resolved.Device {
			continue
		}
		found, candidateErr := executableCandidateWith(stat, filepath.Join(resolved.Native, name))
		if candidateErr == nil {
			return found, index, nil
		}
		if !errors.Is(candidateErr, errExternalNotFound) && firstCandidateErr == nil {
			firstCandidateErr = candidateErr
		}
	}
	if firstCandidateErr != nil {
		return "", -1, firstCandidateErr
	}
	return "", -1, errExternalNotFound
}

// everyCommandPath is each program name names along PATH, in its order, the one that would run
// first: `which -a`. A name with a separator in it names one file, or none. The table is neither
// read nor added to, since it remembers only the first. This binary comes first for a name the
// shell answers to.
func (r Runtime) everyCommandPath(name string) []string {
	if name == "" || hasPathSeparator(name) {
		if found, err := r.knownCommandPath(name); err == nil {
			return []string{found}
		}
		return nil
	}
	var found []string
	if self, err := jobExecutable(); err == nil && r.shellAnswers(name) {
		found = append(found, self)
	}
	pathValue := r.vars["PATH"]
	if r.searchPath != "" {
		pathValue = r.searchPath
	}
	if pathValue == "" {
		return found
	}
	stat := r.commands.statter()
	for _, dir := range filepath.SplitList(pathValue) {
		if dir == "" {
			dir = "."
		}
		resolved, err := r.ResolveNemoshPath(dir)
		if err != nil || resolved.Device {
			continue
		}
		if program, err := executableCandidateWith(stat, filepath.Join(resolved.Native, name)); err == nil {
			found = append(found, program)
		}
	}
	return found
}

// stillAProgram is whether what was found at native can still be run from there: it is there,
// and it is no directory.
func stillAProgram(stat statFunc, native string) bool {
	info, err := stat(native)
	return err == nil && !info.IsDir()
}

// relativeSearchDirectory is busybox's is_relative_path for a directory of PATH: one that names
// a different place after a `cd`. Empty is the current directory.
func relativeSearchDirectory(dir string) bool {
	if dir == "" {
		return true
	}
	if dir[0] == '/' || dir[0] == '\\' {
		return false
	}
	drive := len(dir) >= 3 && dir[1] == ':' && (dir[2] == '/' || dir[2] == '\\')
	return !drive
}
