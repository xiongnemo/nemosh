package runtime

import "strings"

// compgenSplitWords splits a -W list at the characters of ifs that are not quoted, escaped or
// inside a substitution, as bash's split_at_delims does. The rest of each piece is one word: a
// blank or an operator that would end a word in a command is escaped, since in a word list it
// is the word's own.
func compgenSplitWords(list, ifs string) []string {
	var pieces []string
	var piece strings.Builder
	var open []byte // what each open quote or substitution waits for, innermost last
	for index := 0; index < len(list); index++ {
		char := list[index]
		top := byte(0)
		if len(open) > 0 {
			top = open[len(open)-1]
		}
		opener := char == '$' && index+1 < len(list) && (list[index+1] == '(' || list[index+1] == '{')
		switch {
		case top == '\'':
			if char == '\'' {
				open = open[:len(open)-1]
			}
			piece.WriteByte(char)
		case char == '\\' && index+1 < len(list):
			piece.WriteString(list[index : index+2])
			index++
		case top != 0 && char == top:
			open = open[:len(open)-1]
			piece.WriteByte(char)
		case opener && top != '`':
			open = append(open, map[byte]byte{'(': ')', '{': '}'}[list[index+1]])
			piece.WriteString(list[index : index+2])
			index++
		case char == '`' || (char == '"' || char == '\'') && top != '"' && top != '`':
			open = append(open, char)
			piece.WriteByte(char)
		case char == '(' && top == ')':
			open = append(open, ')')
			piece.WriteByte(char)
		case top == 0 && strings.IndexByte(ifs, char) >= 0:
			if piece.Len() > 0 {
				pieces = append(pieces, piece.String())
				piece.Reset()
			}
		case top == 0 && strings.IndexByte(" \t\n|&;<>()", char) >= 0:
			piece.WriteByte('\\')
			piece.WriteByte(char)
		default:
			piece.WriteByte(char)
		}
	}
	if piece.Len() > 0 {
		pieces = append(pieces, piece.String())
	}
	return pieces
}

// compgenWord parses text that is one word, as a command's word would be read.
func compgenWord(text string) (word, bool) {
	tokens, err := scanShellTokens(text)
	if err != nil || len(tokens) != 1 || tokens[0].kind != tokenWord || tokens[0].parsed == nil {
		return word{}, false
	}
	return parseTypedWord(*tokens[0].parsed), true
}
