package runtime

// withoutArithmeticQuotes is an arithmetic loop part with its double quotes removed, as bash
// removes them from an expression it reads as though in double quotes: `i<"$n"` is i<$n.
// Only the quotes of the expression itself go -- those in a command substitution, a
// parameter expansion or backquotes are that text's own, and `$(wc -l < "$f")` keeps them.
func withoutArithmeticQuotes(text string) string {
	out := make([]byte, 0, len(text))
	depth := 0
	backquoted := false
	for index := 0; index < len(text); index++ {
		char := text[index]
		switch {
		case char == '\\' && index+1 < len(text):
			out = append(out, char, text[index+1])
			index++
			continue
		case char == '`':
			backquoted = !backquoted
		case backquoted:
		case char == '$' && index+1 < len(text) && (text[index+1] == '(' || text[index+1] == '{'):
			depth++
			out = append(out, char, text[index+1])
			index++
			continue
		case depth > 0 && (char == '(' || char == '{'):
			depth++
		case depth > 0 && (char == ')' || char == '}'):
			depth--
		case char == '"' && depth == 0:
			continue
		}
		out = append(out, char)
	}
	return string(out)
}
