package runtime

import (
	"slices"
	"strings"
)

// In bash mode an alias is substituted as the text is read, before its structure is decided, as
// bash's reader and busybox's substitute one: where a command's name goes, a plain word that is
// an alias becomes its text, and the reading goes on through that text, so an alias for `{`, `(`,
// a reserved word or half a loop is syntax. Its text is read in turn, the first word of it a
// command's name if the alias was in that place; an alias is not substituted again inside its own
// text; and text that ends in a blank makes the next word a candidate too, wherever it is.
//
// nemosh decides a text's structure in passes over its lines before any word is lexed (parser.go),
// so this is one more pass, over the text the heredoc pass has left, and reads only as much of the
// shell's grammar as says where a command's name goes: after an operator or a newline, after the
// reserved words a command follows, after assignments and redirections, inside a `$(...)`; not in
// a case pattern, for's words, a `[[ ]]` or an arithmetic command. As nemosh and sh the pass is
// not made; an alias is substituted as its command runs, from alias_expand.go.

// maxReadAliasSubstitutions bounds the substitutions in one text, so text that substitutes itself
// endlessly through a chain of aliases ends.
const maxReadAliasSubstitutions = 4096

// readMode is the place in a compound command's header the reading is in.
type readMode int

const (
	readPlain        readMode = iota
	readForName               // after for or select: its name
	readForIn                 // after for's name: in, or the body
	readList                  // for's words after in, to a `;` or the line's end
	readCaseWord              // after case: its word
	readCaseIn                // after case's word: in
	readFunctionName          // after function: its name
	readConditional           // inside [[ ]]
)

// readState is where the reading of a text is.
type readState struct {
	command  bool // the next word is where a command's name goes
	eligible bool // the next word follows an alias whose text ended in a blank
	target   bool // the next word is a redirection's
	named    bool // the last word was a command's name, which `()` makes a function's
	header   bool // inside a function header's `()`
	mode     readMode
	patterns []bool // the cases open, innermost last: whether their patterns are being read
}

type aliasReader struct {
	aliases map[string]string
	left    int // substitutions still to be made
	limit   int // the length of output past which none is
}

// readAliases is text with aliases substituted where bash's reader substitutes them.
func readAliases(text string, aliases map[string]string) string {
	reader := aliasReader{aliases: aliases, left: maxReadAliasSubstitutions, limit: max(4*len(text), 1<<20)}
	var out strings.Builder
	reader.read(text, nil, readState{command: true}, &out)
	return out.String()
}

// read writes text to out with its aliases substituted, chain the aliases whose text it is, from
// state, and answers the state it leaves.
func (a *aliasReader) read(text string, chain []string, state readState, out *strings.Builder) readState {
	for index := 0; index < len(text); {
		char := text[index]
		switch {
		case isShellBlank(char):
			out.WriteByte(char)
			index++
		case char == '\n':
			out.WriteByte(char)
			index++
			state = state.lineEnds()
		case char == '#' && (index == 0 || isShellBlank(text[index-1]) || strings.IndexByte("\n;&|()", text[index-1]) >= 0):
			end := strings.IndexByte(text[index:], '\n')
			if end < 0 {
				end = len(text) - index
			}
			out.WriteString(text[index : index+end])
			index += end
		case char == '(' && state.command && strings.HasPrefix(text[index:], "((") && state.mode == readPlain:
			// An arithmetic command, whose words are no command's.
			end := len(text)
			if close, ok := arithmeticExpansionEnd(text, index+2); ok {
				end = close + 1
			}
			out.WriteString(text[index:end])
			index, state.command = end, false
		case (char == '<' || char == '>') && strings.HasPrefix(text[index+1:], "(") && state.mode != readConditional:
			// A process substitution, whose text is a command's, read as a `$(...)`'s is.
			end := len(text)
			if close, ok := commandSubstitutionEnd(text, index+2); ok {
				end = close + 1
			}
			out.WriteString(text[index : index+2])
			a.read(text[index+2:max(end-1, index+2)], nil, readState{command: true}, out)
			out.WriteString(text[max(end-1, index+2):end])
			index, state.eligible = end, false
			if state.target {
				state.target = false
			} else if state.mode == readPlain {
				state.command = false
			}
		case strings.IndexByte(";&|()<>", char) >= 0:
			operator := readOperator(text, index)
			out.WriteString(operator)
			index += len(operator)
			state = state.after(operator, text[index:])
		default:
			end := max(readWordEnd(text, index), index+1)
			word := text[index:end]
			index = end
			state = a.word(word, text[index:], chain, state, out)
		}
	}
	return state
}

// word writes one word, substituted if it is an alias where one is read, and answers the state
// after it.
func (a *aliasReader) word(word, rest string, chain []string, state readState, out *strings.Builder) readState {
	eligible := state.eligible
	state.eligible, state.named = false, false
	if state.target || redirectionPrefix(word, rest) {
		state.target = false
		out.WriteString(a.substitutions(word))
		return state
	}
	// Where a command's name goes, or after a text ending in a blank wherever that is -- for's
	// name and words too, as bash reads them -- but never in a case pattern or a `[[ ]]`.
	where := eligible || state.command && state.mode == readPlain
	if name, ok := a.aliasFor(word, chain); ok && where && !state.inPatterns() && state.mode != readConditional && a.left > 0 && out.Len() < a.limit {
		a.left--
		value := a.aliases[name]
		blank := strings.HasSuffix(value, " ") || strings.HasSuffix(value, "\t")
		state.eligible = blank
		inner := a.read(value, append(slices.Clip(chain), name), state, out)
		inner.eligible = blank
		return inner
	}
	if state.mode != readPlain || state.inPatterns() {
		out.WriteString(a.substitutions(word))
		return state.headerWord(word)
	}
	out.WriteString(a.substitutions(word))
	if !state.command {
		return state
	}
	return state.commandWord(word)
}

// aliasFor is the alias a word read is, if it is one that may be substituted there.
func (a *aliasReader) aliasFor(word string, chain []string) (string, bool) {
	if !plainAliasWord(word) || slices.Contains(chain, word) {
		return "", false
	}
	_, ok := a.aliases[word]
	return word, ok
}

// redirectionPrefix is whether a word is the descriptor of the redirection after it: `2>file`,
// `{fd}>file`.
func redirectionPrefix(word, rest string) bool {
	if rest == "" || rest[0] != '<' && rest[0] != '>' {
		return false
	}
	return isDigits(word) || len(word) > 2 && word[0] == '{' && word[len(word)-1] == '}' && isValidVariableName(word[1:len(word)-1])
}
