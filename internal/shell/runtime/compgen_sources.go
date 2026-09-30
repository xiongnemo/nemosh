package runtime

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"
	"slices"
	"strings"

	"github.com/xiongnemo/nemosh/internal/shellquote"
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
			fmt.Fprintf(r.streams.Stderr, "compgen: -W: %s: bad substitution\n", piece)
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

// compgenFunction is -F: the function called as bash calls it from compgen, with the command
// `compgen`, the word and an empty previous word, and COMP_WORDS, COMP_CWORD, COMP_LINE and
// COMP_POINT saying there is no line; COMPREPLY is what it answers, and is unset again after,
// as they are. A function that fails with a shell error answers nothing.
func (r Runtime) compgenFunction(ctx context.Context, name, word string, savedStatus int) ([]string, bool) {
	fmt.Fprintln(r.streams.Stderr, "compgen: warning: -F option may not work as you expect")
	definition, found := r.calledFunction(name)
	if !found {
		fmt.Fprintf(r.streams.Stderr, "compgen: function `%s' not found\n", name)
		return nil, false
	}
	r.arrays.set("COMP_WORDS", nil)
	r.vars["COMP_CWORD"], r.vars["COMP_LINE"], r.vars["COMP_POINT"] = "-1", "", "0"
	r.arrays.unset("COMPREPLY")
	delete(r.vars, "COMPREPLY")
	defer func() {
		r.arrays.unset("COMP_WORDS")
		r.arrays.unset("COMPREPLY")
		for _, variable := range []string{"COMP_WORDS", "COMP_CWORD", "COMP_LINE", "COMP_POINT", "COMPREPLY"} {
			delete(r.vars, variable)
		}
	}()
	result := r.callFunctionResult(ctx, definition, []string{"compgen", word, ""}, savedStatus)
	if r.expansion.shellError || result.control == flowAbort {
		r.discardLine()
		return nil, false
	}
	if r.arrays.has("COMPREPLY") {
		var replies []string
		for _, index := range r.arrays.liveIndices("COMPREPLY") {
			value, _ := r.arrays.valueAt("COMPREPLY", index)
			replies = append(replies, value)
		}
		return replies, true
	}
	if value, set := r.vars["COMPREPLY"]; set {
		return []string{value}, true
	}
	return nil, true
}

// compgenCommand is -C: the command run with the command `compgen`, the word and an empty
// previous word, as bash runs it, and each line of its output a completion.
func (r Runtime) compgenCommand(ctx context.Context, command, word string, savedStatus int) []string {
	fmt.Fprintln(r.streams.Stderr, "compgen: warning: -C option may not work as you expect")
	script, err := r.parseHere(command + " compgen " + shellquote.Single(word) + " ''")
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "compgen: %v\n", err)
		return nil
	}
	output := r.commandSubstitutionScript(ctx, script, savedStatus)
	if output == "" {
		return nil
	}
	return strings.Split(output, "\n")
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
