package runtime

// openSubstitution is a `$(` the scan is inside: body is where in logical its body begins,
// and depth how many of the body's own parentheses are open. One that is arithmetic, `$((` or
// a `((` command the line does not close, holds an expression, where a `#` is the
// expression's and no comment.
type openSubstitution struct {
	body, depth int
	arithmetic  bool
}

// stepArithmeticExpansion steps over a `$((` that closes on this line, whole, before the
// command substitution branch can claim its first `(`. Otherwise the `))` that closes it is
// counted as one substitution close and one group close, and the group close is matched
// against whatever is really open: `{ echo $((1+2)); }` failed with `unexpected ), expected }`.
func (scanner *syntaxScanner) stepArithmeticExpansion(line string, index int) (int, bool) {
	if line[index] != '$' || index+2 >= len(line) || line[index+1] != '(' || line[index+2] != '(' || scanner.quote() == '\'' {
		return index, false
	}
	end, ok := arithmeticExpansionEnd(line, index+3)
	if !ok {
		return index, false
	}
	scanner.logical.WriteString(line[index : end+1])
	return end, true
}

// openArithmeticCommand opens a `((` command the line does not close as an arithmetic span, as
// a `$((` that goes on to later lines is one: `(( a = 3 + 4  # x` then `))` is a syntax error in
// bash, which has the command, and was 7 here, the # taken for a comment. The first `(` opens
// the span and the second is its own parenthesis, so `))` closes both.
func (scanner *syntaxScanner) openArithmeticCommand(line string, index int) bool {
	if index+1 >= len(line) || line[index+1] != '(' {
		return false
	}
	scanner.quotes = append(scanner.quotes, 0)
	scanner.substitutions = append(scanner.substitutions, openSubstitution{body: scanner.logical.Len() + 1, arithmetic: true})
	scanner.logical.WriteByte('(')
	return true
}

// inArithmetic is whether the scan is inside an arithmetic span, the innermost one open: a
// command substitution inside it is a script again, with comments of its own.
func (scanner *syntaxScanner) inArithmetic() bool {
	return len(scanner.substitutions) > 0 && scanner.substitutions[len(scanner.substitutions)-1].arithmetic
}
