package main

import "strings"

// Cutting a line into the pieces a reader sees, which is not the same as cutting it at
// blanks.
//
// The highlighter used to split on spaces alone, and `a() { case a in a) echo bingo;; *)
// echo hmm;; esac; }` showed what that costs: `bingo;;` is one word to a space-splitter, so
// the `;;` inside it was invisible, the command position never came back, and **neither
// `echo` was drawn as a command**. Everything after the first pattern was painted as one
// long argument list.
//
// So this splits where the shell's lexer does. It is still a lexer and not a parser: it
// runs on every keystroke, over a line that is usually half-written, and the keystroke
// budget is measured in perf_keystroke_test.go. What it must get right is where one word
// ends and the next begins.

// highlightToken is one piece of a line: a run of blanks, an operator, or a word.
type highlightToken struct {
	text     string
	operator bool
	blank    bool
}

// shellOperators are the operators that end a word, longest first so that `;;` is not read
// as two `;` and `&&` not as two `&`.
//
// `)` and `}` are here as well as their openers. They were absent before, which is half of
// why a case arm's body was mistaken for arguments: `a)` ended no word.
var shellOperators = [...]string{";;&", ";;", ";&", "&&", "||", "<<-", "<<", ">>", "<>", ">|", "&>", ";", "&", "|", "(", ")", "{", "}", "<", ">"}

// splitHighlightTokens cuts a line into blanks, operators and words.
//
// Quoting is respected, because `echo ";"` is one word and a semicolon in it separates
// nothing. An unterminated quote runs to the end of the line, which is what a half-typed
// line looks like and is not an error here.
func splitHighlightTokens(line string) []highlightToken {
	var tokens []highlightToken
	runes := []rune(line)
	index := 0
	for index < len(runes) {
		if runes[index] == ' ' || runes[index] == '\t' {
			start := index
			for index < len(runes) && (runes[index] == ' ' || runes[index] == '\t') {
				index++
			}
			tokens = append(tokens, highlightToken{text: string(runes[start:index]), blank: true})
			continue
		}
		if operator, width := operatorAt(runes, index); width > 0 {
			tokens = append(tokens, highlightToken{text: operator, operator: true})
			index += width
			continue
		}
		start := index
		quote := rune(0)
		for index < len(runes) {
			char := runes[index]
			if char == '\\' && index+1 < len(runes) {
				index += 2
				continue
			}
			if quote != 0 {
				if char == quote {
					quote = 0
				}
				index++
				continue
			}
			if char == '\'' || char == '"' {
				quote = char
				index++
				continue
			}
			if char == ' ' || char == '\t' {
				break
			}
			if _, width := operatorAt(runes, index); width > 0 {
				break
			}
			index++
		}
		if index > len(runes) {
			index = len(runes)
		}
		tokens = append(tokens, highlightToken{text: string(runes[start:index])})
	}
	return tokens
}

// operatorAt reports the operator starting at index, and how many runes it takes.
//
// A `{` or `}` only counts as an operator when it stands alone as a word -- `echo {a,b}` is
// a brace expansion and `${x}` is a parameter, and drawing either as punctuation would be
// wrong about what the shell will do with it. The other operators need no such test: none
// of them can appear inside a word unquoted.
func operatorAt(runes []rune, index int) (string, int) {
	for _, operator := range shellOperators {
		candidate := []rune(operator)
		if index+len(candidate) > len(runes) || string(runes[index:index+len(candidate)]) != operator {
			continue
		}
		if (operator == "{" || operator == "}") && !braceStandsAlone(runes, index) {
			continue
		}
		return operator, len(candidate)
	}
	return "", 0
}

// braceStandsAlone reports whether the brace at index is a word of its own rather than part
// of one.
func braceStandsAlone(runes []rune, index int) bool {
	if index > 0 && !isHighlightBoundary(runes[index-1]) {
		return false
	}
	return index+1 >= len(runes) || isHighlightBoundary(runes[index+1])
}

func isHighlightBoundary(char rune) bool {
	return char == ' ' || char == '\t' || strings.ContainsRune(";&|()", char)
}
