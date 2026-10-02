package runtime

import (
	"fmt"
	"strings"
)

// arrayLiteralEnd finds the `)` that closes the array literal whose `(` is at open, and
// refuses what bash refuses between the two.
//
// The elements are words, so an operator among them is a syntax error: `a=(1 & 2)`,
// `a=(x | y)`, `a=(p; q)`, and a literal written one element a line with a stray `&` on
// one of them. The operator was dropped without a word, so the literal meant something
// other than what was written and the script went on. It is refused as the script is
// parsed, as every other syntax error is here, so none of the script runs; bash, which
// reads a script a line at a time, runs what came before it.
//
// The elements are lexed again at run time, by compoundElements. Lexing them here as well
// is what lets a mistake among them stop the script before it starts. A budget of their
// own, because counting them twice against the script's would halve the size of the
// largest literal it can hold.
func arrayLiteralEnd(line string, open, depth int) (int, error) {
	end, ok := matchingParenthesis(line, open)
	if !ok {
		return 0, fmt.Errorf("%w: missing ) for array assignment", ErrIncompleteScript)
	}
	tokens, err := scanShellTokensWithBudget(line[open+1:end], &parseBudget{}, depth)
	if err != nil {
		return 0, err
	}
	for _, token := range tokens {
		if token.kind != tokenWord {
			return 0, fmt.Errorf("syntax error: unexpected %s in an array assignment", token.value)
		}
		if hasUnquotedSemicolon(*token.parsed) {
			return 0, fmt.Errorf("syntax error: unexpected ; in an array assignment")
		}
		if hasUnquotedParenthesis(*token.parsed) {
			return 0, fmt.Errorf("syntax error: unexpected ( in an array assignment")
		}
	}
	return end, nil
}

// hasUnquotedParenthesis reports a word holding a `(` that nothing quotes and no extended
// pattern or process substitution opens. `a=( inside=() )` and `a=( x (y) )` are syntax
// errors in busybox-w32 and bash, where the parenthesis was kept in an element.
func hasUnquotedParenthesis(item word) bool {
	for _, part := range item.parts {
		if part.kind != wordPartLiteral || part.quote != quoteUnquoted {
			continue
		}
		for index := 0; index < len(part.text); index++ {
			if part.text[index] == '(' && (index == 0 || strings.IndexByte("@!*+?<>", part.text[index-1]) < 0) {
				return true
			}
		}
	}
	return false
}

// hasUnquotedSemicolon reports whether a word holds a `;` that nothing quotes. The lexer
// leaves `;` in a word, because the line was cut at every separator before the lexer saw
// it; one inside an array literal survives that, being inside its parentheses.
func hasUnquotedSemicolon(item word) bool {
	for _, part := range item.parts {
		if part.kind == wordPartLiteral && part.quote == quoteUnquoted && strings.Contains(part.text, ";") {
			return true
		}
	}
	return false
}

// An array literal is an assignment's value and nothing else: `echo a=(1 2)`, `for x in a=()`
// and `case a=() in` are syntax errors in busybox-w32 and bash, where the literal was taken
// as a word and its text used. It may stand in front of a command, as any assignment may,
// and after a declaration utility, which assigns its operands. `let x=( 1 )` is arithmetic,
// and the operand of `[[ x =~ a=(x) ]]` a regular expression, so both keep theirs. So does
// eval's, which bash reads as it reads a declaration's: `eval a=( ${list[@]} )` hands eval the
// literal to run, and was refused, so the script never began.

// refuseMisplacedArrayLiteral refuses an array literal after a command name that is not a
// declaration utility, let, eval, or `[[`.
func refuseMisplacedArrayLiteral(words []word) error {
	command := 0
	for command < len(words) && isAssignmentWord(words[command]) {
		command++
	}
	if command >= len(words) || isDeclarationUtility(words[command]) {
		return nil
	}
	if name := soleLiteralText(words[command]); name == "let" || name == "[[" || name == "eval" {
		return nil
	}
	for _, item := range words[command+1:] {
		if err := refuseArrayLiteral(item); err != nil {
			return err
		}
	}
	return nil
}

// refuseArrayLiteral refuses a word that is an array literal. An element assignment,
// `b[0]=2`, is an ordinary word after a command name, as `echo b[0]=2` prints it.
func refuseArrayLiteral(item word) error {
	if assignment, literal := parseArrayAssignmentWord(item); literal && assignment.list {
		return fmt.Errorf("syntax error: unexpected ( in %s", soleLiteralText(item))
	}
	return nil
}

// checkLoopName refuses a loop variable that cannot be a name, `for i.j in`, as busybox-w32
// does; bash runs such a loop as if nothing were wrong with it. A quoted one is refused when
// the loop runs, as it was.
func checkLoopName(tokens []shellToken) error {
	if len(tokens) == 0 || tokens[0].parsed == nil || !isUnquotedLiteralWord(*tokens[0].parsed) {
		return nil
	}
	if !isValidVariableName(tokens[0].value) {
		return fmt.Errorf("syntax error: bad for loop variable %s", tokens[0].value)
	}
	return nil
}
