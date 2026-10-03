package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// initCompletion is the package's _comp_initialize, as _init_completion: COMPREPLY emptied, and
// cur, prev, words and cword set as _get_comp_words_by_ref sets them, with `<`, `>` and `&` and
// -n's characters no word breaks; with -s an `--opt=value` word split into prev and cur. It
// answers 1 when it has completed the word itself -- a `$` name, or a file after a redirection
// -- or the cursor is on the command, and 0 when the caller has an operand to complete. -e, -o
// and -i, the redirections' file filters, are read and not used.
func (r Runtime) initCompletion(args []string) int {
	exclude, split := "", false
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "-n", "-e", "-o", "-i":
			index++
			if args[index-1] == "-n" && index < len(args) {
				exclude += args[index]
			}
		case "-s":
			split, exclude = true, exclude+"="
		default:
			fmt.Fprintln(r.streams.Stderr, "bash_completion: _init_completion: usage error")
			return 1
		}
	}
	r.arrays.set("COMPREPLY", nil)
	delete(r.vars, "COMPREPLY")
	words, cword, cur := r.completionWordsAtCursor(exclude + "<>&")
	all := map[string]string{"cur": "cur", "prev": "prev", "words": "words", "cword": "cword"}
	if name, ok := strings.CutPrefix(cur, "$"); ok && isVariableNamePrefix(name) {
		r.setCompletionWords(all, words, cword, cur)
		r.arrays.set("COMPREPLY", r.variableNamesFrom(name))
		return 1
	}
	if width := redirectionWidth(cur); width > 0 || cword >= 1 && redirectionWidth(words[cword-1]) == len(words[cword-1]) && words[cword-1] != "" {
		r.setCompletionWords(all, words, cword, cur[width:])
		r.filedir(nil)
		return 1
	}
	// A redirection and the file after it are no operands, so they leave the words.
	for index := 1; index < len(words); {
		width := redirectionWidth(words[index])
		if width <= 0 {
			index++
			continue
		}
		skip := 1
		if width == len(words[index]) && index+1 < len(words) {
			skip = 2
		}
		words = slices.Delete(words, index, index+skip)
		if index <= cword {
			cword -= skip
		}
	}
	r.setCompletionWords(all, words, cword, cur)
	if cword <= 0 {
		return 1
	}
	// _comp__split_longopt: `--opt=value` is prev `--opt` and cur `value`.
	if equals := strings.IndexByte(cur, '='); split && strings.HasPrefix(cur, "--") && equals > 2 {
		r.assignVar("prev", cur[:equals])
		r.assignVar("cur", cur[equals+1:])
		r.assignVar("split", "set")
	}
	return 0
}

// isVariableNamePrefix reports what can begin a variable's name, or nothing yet.
func isVariableNamePrefix(text string) bool {
	return text == "" || isVariableName(text)
}

// variableNamesFrom is `$` and each set variable's name that begins with prefix.
func (r Runtime) variableNamesFrom(prefix string) []string {
	var names []string
	for _, name := range r.compgenAction("variable", prefix) {
		names = append(names, "$"+name)
	}
	return names
}

// redirectionWidth is how much of word a redirection takes at its front -- an optional
// descriptor or {name}, then one of > >> >| >& < <> <& << <<- <<<, or &> or &>> -- and 0 when
// it begins with none.
func redirectionWidth(word string) int {
	at := 0
	for at < len(word) && word[at] >= '0' && word[at] <= '9' {
		at++
	}
	if at == 0 && strings.HasPrefix(word, "{") {
		if end := strings.IndexByte(word, '}'); end > 1 && isVariableName(word[1:end]) {
			at = end + 1
		}
	}
	rest := word[at:]
	for _, operator := range []string{"<<<", "<<-", "&>>", ">>", ">|", ">&", "<>", "<&", "<<", "&>", ">", "<"} {
		if strings.HasPrefix(rest, operator) && (at == 0 || operator[0] != '&') {
			return at + len(operator)
		}
	}
	return 0
}

// filedir is the package's _comp_compgen_filedir, as _filedir: the files the word in cur begins
// -- directories alone with -d; with an extension, `_filedir txt`, the names ending in it, in
// either case, and the directories -- added to COMPREPLY, and -o filenames set for the running
// completion when there are any, so the line editor puts them in as names.
func (r Runtime) filedir(args []string) int {
	cur := r.vars["cur"]
	dirsOnly := len(args) > 0 && args[0] == "-d"
	found := r.compgenPaths(cur, dirsOnly)
	if !dirsOnly && len(args) > 0 && args[0] != "" {
		pattern := "*.@(" + args[0] + "|" + strings.ToUpper(args[0]) + ")"
		found = slices.DeleteFunc(found, func(name string) bool {
			return !matchShellPattern(pattern, name) && !r.namesDirectory(name)
		})
	}
	if len(found) == 0 {
		return 0
	}
	if running := r.completions.running; running != nil {
		running.spec.options["filenames"] = true
	}
	r.arrays.set("COMPREPLY", append(r.arrays.liveValues("COMPREPLY"), found...))
	delete(r.vars, "COMPREPLY")
	return 0
}

// namesDirectory reports whether a path as written names a directory.
func (r Runtime) namesDirectory(name string) bool {
	resolved, err := r.ResolveNemoshPath(name)
	if err != nil || resolved.Device {
		return false
	}
	info, err := os.Stat(filepath.Clean(resolved.Native))
	return err == nil && info.IsDir()
}
