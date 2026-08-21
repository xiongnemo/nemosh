package main

import (
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/xiongnemo/nemosh/internal/capability"
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

// completeCommand offers every name this shell can run: its builtins and
// applets, this session's aliases and functions, and everything on PATH.
//
// PATH used to be left out on the grounds that walking it -- dozens of
// directories and thousands of files on Windows -- would be felt on every Tab.
// That reasoning was right and is now obsolete: the same index the suggestion
// engine uses is already built, in the background, once per PATH. Reading it is
// a map lookup, so the objection is gone and `gi` can finish to `git` like it
// finishes to anything else.
//
// Matching follows the platform's own rule, so on Windows `WS` finds `wsl` for
// the same reason completing a filename there ignores case.
func completeCommand(prefix string, candidates []string) []string {
	var matches []string
	for _, name := range candidates {
		if completionMatches(name, prefix) {
			matches = append(matches, name)
		}
	}
	sortCandidates(matches)
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
// The second return says which kind came back, and the caller needs it: a
// dash-leading *path* is rewritten to `./name` so the command does not read it as
// options, and doing that to an actual option turns `--color` into `./--color`.
// One rule for making a name usable, applied to the one kind of word it is about.
func completeOperand(workingDirectory, home, command, prefix string) (matches []string, areOptions bool) {
	if strings.HasPrefix(prefix, "-") {
		if options := completeOption(command, prefix); len(options) > 0 {
			return options, true
		}
	}
	return completePathsFrom(workingDirectory, home, prefix, completesDirectoriesOnly(command)), false
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
//
// No home directory, so a tilde is not completed through this door. Only tests use it; the live
// path is completeOperand, which is handed the home the editor cached.
func completeFile(workingDirectory, prefix string) []string {
	return completePaths(workingDirectory, prefix, false)
}

// completePaths does the reading. A directory comes back with a trailing slash,
// so a second Tab descends into it rather than stopping at a name that needs a
// separator typed by hand.
func completePaths(workingDirectory, prefix string, directoriesOnly bool) []string {
	return completePathsFrom(workingDirectory, "", prefix, directoriesOnly)
}

// completePathsFrom is the same with the home directory named, so `~/` can be read.
//
// Tilde expansion belongs to *word* expansion -- Runtime.expandHomeTilde -- and completion works on
// the text as typed, so it never saw an expanded one. `~/` was split into a directory called `~`,
// joined onto the working directory, and read; the read failed and this returned nil. Silently,
// which is why Tab looked like a dead key rather than a failure.
//
// The home directory is passed in rather than looked up, for the same reason the working directory
// already is: the editor caches both from the runtime, and neither this function nor its callers
// hold a Runtime to ask.
func completePathsFrom(workingDirectory, home, prefix string, directoriesOnly bool) []string {
	if prefix == "~" && home != "" {
		// The one candidate is the home directory itself, which is what bash offers: `~`
		// names a directory unambiguously, so Tab adds the separator and a second Tab
		// lists what is inside. Listing the contents from one keypress would skip a step
		// the person has not taken.
		return []string{"~/"}
	}
	searchPrefix, spelling, ok := expandCompletionTilde(prefix, home)
	if !ok {
		return nil
	}
	directory, stem := path.Split(filepath.ToSlash(searchPrefix))
	searchIn := workingDirectory
	if directory != "" {
		searchIn = filepath.Join(workingDirectory, filepath.FromSlash(directory))
	}
	if filepath.IsAbs(directory) {
		// An expanded `~/` is absolute, and joining an absolute path onto the working
		// directory would bury the home directory inside it.
		searchIn = filepath.FromSlash(directory)
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
		matches = append(matches, spelling+name)
	}
	sort.Strings(matches)
	return matches
}

// expandCompletionTilde turns a typed prefix into one to read and one to insert.
//
// Two values because they differ, and the difference is the point: the read needs the home
// directory, and what Tab puts in the buffer has to keep the `~/` the person typed. Rewriting
// `~/Do` into `C:/Users/nemo/Documents/` on a keypress is not what bash does, and it turns a short
// line into a long one for nothing.
//
// The third result is false when the word looks like a tilde this cannot resolve, and the caller
// then offers nothing. Two cases:
//
//   - `~user`, which needs another account's profile directory. `echo ~root` prints `~root` today
//     and docs/design/v1-scope.md defers resolving it, so guessing here would invent an answer the
//     rest of the shell does not give.
//   - no home at all, where completing against the working directory would offer one directory's
//     contents under another's spelling.
func expandCompletionTilde(prefix, home string) (search, spelling string, ok bool) {
	if !strings.HasPrefix(prefix, "~") {
		directory, _ := path.Split(filepath.ToSlash(prefix))
		return prefix, directory, true
	}
	if home == "" {
		return "", "", false
	}
	root := strings.TrimRight(filepath.ToSlash(home), "/")
	switch {
	case prefix == "~":
		// Completing to the home directory itself, so a second Tab descends into it. The
		// stem is empty and the spelling is `~/`, which together offer exactly `~/`.
		return root + "/", "~/", true
	case strings.HasPrefix(prefix, "~/"):
		rest := strings.TrimPrefix(prefix, "~/")
		directory, _ := path.Split(rest)
		return root + "/" + rest, "~/" + directory, true
	}
	return "", "", false
}

// longestSharedPrefix is what Tab inserts when several candidates match: it
// takes the user as far as the choice actually is, and no further.
//
// "The same" means what completionMatches means by it, which is why the
// comparison runs over foldForCompletion. It used to compare bytes while
// matching folded case, so on Windows one candidate spelled `WhoUses` reduced
// the prefix of eight `wh` matches to nothing and Tab appeared to do nothing at
// all.
//
// The spelling comes from the first candidate rather than from what was typed,
// following busybox-w32, which truncates its chosen match and replaces the typed
// prefix outright -- its comment reads "replace match prefix to allow for
// altered case" (libbb/lineedit.c:1483, 1531-1537). So `PROG` + Tab can come
// back as `Program`, showing the name as it is really spelled.
func longestSharedPrefix(matches []string) string {
	if len(matches) == 0 {
		return ""
	}
	// Folded and unfolded are indexed together, so the length agreed in folded
	// runes can be taken from the original spelling. unicode.ToLower is
	// per-rune, so the two have the same rune count even where they differ in
	// bytes.
	spelling := []rune(matches[0])
	shared := []rune(foldForCompletion(matches[0]))
	for _, match := range matches[1:] {
		candidate := []rune(foldForCompletion(match))
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
	return string(spelling[:len(shared)])
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
