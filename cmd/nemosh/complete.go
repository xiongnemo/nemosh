package main

import (
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/capability"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// completesCommand reports whether the word being typed is a command name
// rather than an argument. Completing the wrong kind is worse than completing
// nothing, because the user has to notice it and undo it.
//
// A command name starts a line, or follows an operator that starts a new one.
// This reads the text before the word rather than parsing it: the line is
// half-typed and usually will not parse at all.
func completesCommand(line string) bool {
	// The word being typed starts after the last blank. A trailing blank is not
	// trimmed first: `cat ` means a new word has already begun, and trimming
	// would make it indistinguishable from `cat`.
	start := strings.LastIndexAny(line, " \t") + 1
	before := strings.TrimRight(line[:start], " \t")
	if before == "" {
		return true
	}
	switch before[len(before)-1] {
	case '|', '&', ';', '(', '{':
		return true
	}
	return false
}

// completeCommand offers the names this shell can run without consulting PATH:
// its builtins and its applets. PATH is deliberately not walked -- on Windows
// that is dozens of directories and thousands of files, and the pause would be
// felt on every Tab.
func completeCommand(prefix string) []string {
	var matches []string
	for _, name := range runtime.BuiltinNames() {
		if strings.HasPrefix(name, prefix) {
			matches = append(matches, name)
		}
	}
	for _, name := range applets.DefaultRegistry.Names() {
		if strings.HasPrefix(name, prefix) {
			matches = append(matches, name)
		}
	}
	sort.Strings(matches)
	return slicesCompact(matches)
}

// completeOperand offers what the command in progress can actually take.
//
// A word that begins with a dash asks for an option, so options are offered
// first -- and only if none of them matches does this fall back to paths. That
// fallback is bash's idea (`-o bashdefault`: when the specification produces
// nothing, try the ordinary thing) and it is what keeps a file genuinely named
// `-1.18-windows.xml` reachable: no option matches `-1.1`, so the file is
// offered instead.
func completeOperand(workingDirectory, command, prefix string) []string {
	if strings.HasPrefix(prefix, "-") {
		if options := completeOption(command, prefix); len(options) > 0 {
			return options
		}
	}
	return completePaths(workingDirectory, prefix, completesDirectoriesOnly(command))
}

// completeOption offers the options the command accepts, from the capability
// table that a test holds against the applets' real behaviour.
func completeOption(command, prefix string) []string {
	var matches []string
	for _, option := range capability.Options(command) {
		if strings.HasPrefix(option, prefix) {
			matches = append(matches, option)
		}
	}
	sort.Strings(matches)
	return matches
}

// completeFile offers any path under the shell's working directory.
func completeFile(workingDirectory, prefix string) []string {
	return completePaths(workingDirectory, prefix, false)
}

// completePaths does the reading. A directory comes back with a trailing slash,
// so a second Tab descends into it rather than stopping at a name that needs a
// separator typed by hand.
func completePaths(workingDirectory, prefix string, directoriesOnly bool) []string {
	directory, stem := path.Split(filepath.ToSlash(prefix))
	searchIn := workingDirectory
	if directory != "" {
		searchIn = filepath.Join(workingDirectory, filepath.FromSlash(directory))
	}
	entries, err := os.ReadDir(searchIn)
	if err != nil {
		return nil
	}
	var matches []string
	for _, entry := range entries {
		name := entry.Name()
		if !completionMatches(name, stem) {
			continue
		}
		if entry.IsDir() {
			name += "/"
		} else if directoriesOnly {
			continue
		}
		matches = append(matches, directory+name)
	}
	sort.Strings(matches)
	return matches
}

// longestSharedPrefix is what Tab inserts when several candidates match: it
// takes the user as far as the choice actually is, and no further.
func longestSharedPrefix(matches []string) string {
	if len(matches) == 0 {
		return ""
	}
	shared := []rune(matches[0])
	for _, match := range matches[1:] {
		candidate := []rune(match)
		if len(candidate) < len(shared) {
			shared = shared[:len(candidate)]
		}
		for index := range shared {
			if candidate[index] != shared[index] {
				shared = shared[:index]
				break
			}
		}
	}
	return string(shared)
}

func slicesCompact(sorted []string) []string {
	if len(sorted) < 2 {
		return sorted
	}
	unique := sorted[:1]
	for _, value := range sorted[1:] {
		if value != unique[len(unique)-1] {
			unique = append(unique, value)
		}
	}
	return unique
}
