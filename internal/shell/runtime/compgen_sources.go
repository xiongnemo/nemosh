package runtime

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"
	"slices"
	"strings"
)

// compgenPaths is -f, and -d with dirsOnly: the names in word's directory that begin with the
// rest of it, with that directory in front, as readline completes a filename. Hidden ones too,
// as readline's match-hidden-files is on unless it is turned off. The word is a path as written,
// with no quoting to remove.
func (r Runtime) compgenPaths(word string, dirsOnly bool) []string {
	directory, base := "", word
	if slash := strings.LastIndexByte(word, '/'); slash >= 0 {
		directory, base = word[:slash+1], word[slash+1:]
	}
	listed := directory
	if listed == "" {
		listed = "."
	}
	resolved, err := r.ResolveNemoshPath(listed)
	if err != nil || resolved.Device {
		return nil
	}
	entries, err := os.ReadDir(resolved.Native)
	if err != nil {
		return nil
	}
	var names []string
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), base) {
			continue
		}
		if dirsOnly {
			if info, err := os.Stat(filepath.Join(resolved.Native, entry.Name())); err != nil || !info.IsDir() {
				continue
			}
		}
		names = append(names, directory+entry.Name())
	}
	slices.Sort(names)
	return names
}

// pathCommandNames are the commands on PATH that begin with word: on Windows the files with an
// executable suffix, named without it, as they are run; elsewhere the files with an execute bit.
func (r Runtime) pathCommandNames(word string) []string {
	var names []string
	for _, directory := range filepath.SplitList(r.vars["PATH"]) {
		if directory == "" {
			directory = "."
		}
		resolved, err := r.ResolveNemoshPath(directory)
		if err != nil || resolved.Device {
			continue
		}
		entries, err := os.ReadDir(resolved.Native)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() {
				continue
			}
			if goruntime.GOOS == "windows" {
				if !hasWindowsExecutableSuffix(name) {
					continue
				}
				name = strings.TrimSuffix(name, filepath.Ext(name))
			} else if info, err := entry.Info(); err != nil || info.Mode().Perm()&0o111 == 0 {
				continue
			}
			if strings.HasPrefix(name, word) {
				names = append(names, name)
			}
		}
	}
	slices.Sort(names)
	return slices.Compact(names)
}

// compgenGlob is -G: the pathnames the pattern matches, none when it matches none.
func (r Runtime) compgenGlob(ctx context.Context, pattern string, savedStatus int) []string {
	parsed, ok := compgenWord(pattern)
	if !ok {
		return nil
	}
	var matches []string
	for _, field := range r.expandWord(ctx, parsed, savedStatus) {
		matches = append(matches, r.expandPathnames(field)...)
	}
	return matches
}

// compgenWords is -W: the list split at IFS's characters, as bash's split_at_delims splits it,
// outside quotes and substitutions and not where a backslash escapes one, and then each piece
// expanded as a word is, apart from pathname expansion. An expansion that fails fails compgen.
func (r Runtime) compgenWords(ctx context.Context, list string, savedStatus int) ([]string, bool) {
	ifs, set := r.vars["IFS"]
	if !set {
		ifs = " \t\n"
	}
	var words []string
	for _, piece := range compgenSplitWords(list, ifs) {
		parsed, ok := compgenWord(piece)
		if !ok {
			fmt.Fprintf(r.streams.Stderr, "%scompgen: -W: %s: bad substitution\n", r.diagnosticPrefix(), piece)
			return nil, false
		}
		words = append(words, r.expandWord(ctx, parsed, savedStatus)...)
		if r.expansion.shellError {
			r.discardLine()
			return nil, false
		}
	}
	return words, true
}

// discardLine makes a shell error in compgen's words or function what bash makes of it there:
// compgen fails, and the rest of the line with it, and the script goes on.
func (r Runtime) discardLine() {
	r.expansion.shellError, r.expansion.discard = true, true
}

// compgenFilter is -X: each completion the pattern matches removed, or with a leading ! each
// one it does not match; an & in it is the word, and \& an &.
func (r Runtime) compgenFilter(matches []string, pattern, word string) []string {
	negated := strings.HasPrefix(pattern, "!")
	pattern = strings.TrimPrefix(pattern, "!")
	var expanded strings.Builder
	for index := 0; index < len(pattern); index++ {
		switch {
		case pattern[index] == '\\' && index+1 < len(pattern) && pattern[index+1] == '&':
			expanded.WriteString(`\&`)
			index++
		case pattern[index] == '&':
			expanded.WriteString(escapeGlob(word))
		default:
			expanded.WriteByte(pattern[index])
		}
	}
	var kept []string
	for _, match := range matches {
		if matchShellPattern(expanded.String(), match) == negated {
			kept = append(kept, match)
		}
	}
	return kept
}
