package runtime

import (
	"fmt"
	"strconv"
	"strings"
)

// The bash-completion package's helpers that tool-generated completion scripts call, as
// builtins. cobra's -- gh's, kubectl's, docker's, helm's -- read the line with
// _get_comp_words_by_ref when the package's _init_completion is not there, and complete files
// with _filedir; neither is bash's, and on Windows the package is seldom installed. So those
// completions stopped at their first helper, which was not found. These are the package's 2.12
// _comp_get_words, _comp_initialize, _comp_compgen_filedir and _comp_ltrim_colon_completions,
// under the names the scripts call; a function of the same name, as the package defines, is
// called in their place.

// completionHelper runs one of the helpers.
func (r Runtime) completionHelper(args []string) int {
	switch args[0] {
	case "_get_comp_words_by_ref":
		return r.getCompWordsByRef(args[1:])
	case "_init_completion":
		return r.initCompletion(args[1:])
	case "_filedir":
		return r.filedir(args[1:])
	}
	return r.ltrimColonCompletions(args[1:])
}

// getCompWordsByRef is _comp_get_words: cur, prev, words and cword, each into the variable named
// for it -- by -c, -p, -w and -i, or by its own name given as an operand -- with the characters of
// -n read as no word break, so `-n =:` makes `--repo=cli/c` one word where COMP_WORDS has three.
func (r Runtime) getCompWordsByRef(args []string) int {
	exclude, targets := "", map[string]string{}
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if role, ok := map[string]string{"-c": "cur", "-p": "prev", "-w": "words", "-i": "cword", "-n": "exclude"}[arg]; ok {
			if index+1 == len(args) {
				fmt.Fprintf(r.streams.Stderr, "bash_completion: _get_comp_words_by_ref: usage error\n")
				return 1
			}
			index++
			if role == "exclude" {
				exclude += args[index]
			} else {
				targets[role] = args[index]
			}
			continue
		}
		switch arg {
		case "cur", "prev", "words", "cword":
			targets[arg] = arg
		default:
			fmt.Fprintf(r.streams.Stderr, "bash_completion: _get_comp_words_by_ref: `%s': unknown argument\n", arg)
			return 1
		}
	}
	words, cword, cur := r.completionWordsAtCursor(exclude)
	r.setCompletionWords(targets, words, cword, cur)
	return 0
}

// setCompletionWords sets the variables targets names for each of cur, prev, words and cword.
func (r Runtime) setCompletionWords(targets map[string]string, words []string, cword int, cur string) {
	previous := ""
	if cword >= 1 && cword <= len(words) {
		previous = words[cword-1]
	}
	for role, value := range map[string]string{"cur": cur, "prev": previous, "cword": strconv.Itoa(cword)} {
		if name := targets[role]; name != "" {
			r.assignVar(name, value)
		}
	}
	if name := targets["words"]; name != "" {
		r.arrays.set(name, words)
		delete(r.vars, name)
	}
}

// completionWordsAtCursor is _comp__get_cword_at_cursor: COMP_WORDS with the excluded break
// characters joined back into their words, the index of the cursor's, and the word up to the
// cursor.
func (r Runtime) completionWordsAtCursor(exclude string) ([]string, int, string) {
	compWords := r.arrays.liveValues("COMP_WORDS")
	compCword, _ := strconv.Atoi(r.vars["COMP_CWORD"])
	breaks := r.vars["COMP_WORDBREAKS"]
	excluded := ""
	for _, char := range exclude {
		if strings.ContainsRune(breaks, char) && !strings.ContainsRune(excluded, char) {
			excluded += string(char)
		}
	}
	words, cword := reassembleCompletionWords(compWords, compCword, r.vars["COMP_LINE"], excluded)
	line := []rune(r.vars["COMP_LINE"])
	index, _ := strconv.Atoi(r.vars["COMP_POINT"])
	index = min(max(index, 0), len(line))
	if index == 0 || strings.TrimSpace(string(line[:index])) == "" {
		return words, cword, ""
	}
	cur := line
	for i := 0; i <= cword && i < len(words); i++ {
		word := []rune(words[i])
		for len(cur) >= len(word) && string(cur[:len(word)]) != words[i] {
			cur = cur[1:]
			if index > 0 {
				index--
			}
		}
		if i < cword && len(cur) >= len(word) {
			cur = cur[len(word):]
			index -= len(word)
		}
	}
	if strings.TrimSpace(string(cur)) == "" {
		return words, cword, ""
	}
	return words, cword, string(cur[:min(max(index, 0), len(cur))])
}

// reassembleCompletionWords is _comp__reassemble_words: each word of COMP_WORDS made only of
// excluded break characters joined to the word before it, unless a blank stands between them in
// the line, and to the word after it, unless a blank follows it; and COMP_CWORD moved to match.
func reassembleCompletionWords(compWords []string, compCword int, line, excluded string) ([]string, int) {
	if excluded == "" {
		return append([]string(nil), compWords...), compCword
	}
	var words []string
	add := func(at int, text string) {
		for len(words) <= at {
			words = append(words, "")
		}
		words[at] += text
	}
	onlyExcluded := func(word string) bool { return word != "" && strings.Trim(word, excluded) == "" }
	cword := compCword
	i, j := 0, 0
	for ; i < len(compWords); i, j = i+1, j+1 {
		for i > 0 && onlyExcluded(compWords[i]) {
			if !startsWithBlank(line) && j >= 2 {
				j--
			}
			add(j, compWords[i])
			if i == compCword {
				cword = j
			}
			line = afterFirst(line, compWords[i])
			if i == len(compWords)-1 {
				return words, cword
			}
			i++
			if startsWithBlank(line) {
				j++
			}
		}
		add(j, compWords[i])
		line = afterFirst(line, compWords[i])
		if i == compCword {
			cword = j
		}
	}
	return words, cword
}

func startsWithBlank(text string) bool {
	return text != "" && (text[0] == ' ' || text[0] == '\t')
}

// afterFirst is ${line#*"$word"}: line past the first place word is in it.
func afterFirst(line, word string) string {
	if at := strings.Index(line, word); at >= 0 {
		return line[at+len(word):]
	}
	return line
}

// ltrimColonCompletions is __ltrim_colon_completions: with a colon in the word and in
// COMP_WORDBREAKS, the word up to its last colon taken off the front of each COMPREPLY, since
// the word the line editor replaces begins after it.
func (r Runtime) ltrimColonCompletions(args []string) int {
	cur := ""
	if len(args) > 0 {
		cur = args[0]
	}
	colon := strings.LastIndexByte(cur, ':')
	if colon < 0 || !strings.Contains(r.vars["COMP_WORDBREAKS"], ":") {
		return 0
	}
	replies := r.arrays.liveValues("COMPREPLY")
	for index, reply := range replies {
		replies[index] = strings.TrimPrefix(reply, cur[:colon+1])
	}
	r.arrays.set("COMPREPLY", replies)
	return 0
}
