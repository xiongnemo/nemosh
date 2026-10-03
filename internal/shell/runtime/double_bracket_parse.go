package runtime

import (
	"errors"
	"fmt"
	"strings"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// `[[ ]]` is read with the script, as bash's parser reads it (parse.y, cond_expr): an operator
// is one by how it is written, before anything is expanded, so `op='=='; [[ a $op a ]]` is a
// syntax error in both, and an expression that is no expression stops the script before any
// of it runs, status 2. It was read when the command came round, from the words once expanded:
// `[[ -z ]]` and `[[ a b ]]` were found out only then, and every operand was expanded whether
// its answer was wanted or not, so `[[ -z $x || $(f) ]]` ran f whatever x held. How the tree
// runs is double_bracket.go's.

// conditionKind is what a node of the expression is.
type conditionKind uint8

const (
	// conditionUnary is an operator and its operand. A word alone is `-n` and the word, as bash
	// reads it, so `[[ $x ]]` asks whether x is empty.
	conditionUnary conditionKind = iota
	// conditionBinary is a word, an operator and a word.
	conditionBinary
	conditionAnd
	conditionOr
	// conditionGroup is a parenthesised expression, held in left.
	conditionGroup
)

// conditionNode is one node of a `[[ ]]` expression.
type conditionNode struct {
	kind     conditionKind
	operator string
	operands []word
	left     *conditionNode
	right    *conditionNode
	// negated is a `!` before the node, as bash's CMD_INVERT_RETURN is: on the group when it
	// stands before one, and so traced only before a test of its own.
	negated bool
}

// conditionReader reads the words between `[[` and `]]` as cond_or, cond_and and cond_term
// read them: `||` lowest, then `&&`, both grouping to the right.
type conditionReader struct {
	words []word
	at    int
}

// parseDoubleBracket reads a `[[ ]]` command, its words `[[` and `]]` included. The first `]]`
// written plainly ends it, as COND_END does there, and nothing but a redirection may follow it.
func parseDoubleBracket(words []word) (*conditionNode, error) {
	end := 1
	for end < len(words) && conditionOperator(words[end]) != "]]" {
		end++
	}
	if end == len(words) {
		return nil, fmt.Errorf("%w: missing ]]", ErrIncompleteScript)
	}
	if end < len(words)-1 {
		return nil, fmt.Errorf("syntax error: unexpected %s", printWord(words[end+1]))
	}
	reader := &conditionReader{words: words[1:end]}
	node, err := reader.or()
	if err == nil && !reader.done() {
		err = fmt.Errorf("syntax error in conditional expression: unexpected token `%s'", reader.text())
	}
	return node, err
}

// conditionOperator is a word's text when it is written plainly, as an operator has to be,
// and "" when any of it is quoted or expanded: `"=="` and `$op` are words.
func conditionOperator(item word) string {
	if !isUnquotedLiteralWord(item) {
		return ""
	}
	var text strings.Builder
	for _, part := range item.parts {
		text.WriteString(part.text)
	}
	return text.String()
}

// isConditionToken reports the operators that are tokens of their own in bash, which no
// operand can be: a parenthesis, `&&`, `||`, `<` and `>`.
func isConditionToken(operator string) bool {
	switch operator {
	case "(", ")", "&&", "||", "<", ">":
		return true
	}
	return false
}

// isConditionUnaryOperator is bash's test_unop: `[`'s unary tests, -a for -e, and -v and -o,
// which ask of the shell.
func isConditionUnaryOperator(operator string) bool {
	return applets.IsUnaryConditionOperator(operator) || operator == "-a" || operator == "-v" || operator == "-o"
}

func (reader *conditionReader) done() bool { return reader.at >= len(reader.words) }

// operator is conditionOperator of the word the reader is at, and "" past the end.
func (reader *conditionReader) operator() string {
	if reader.done() {
		return ""
	}
	return conditionOperator(reader.words[reader.at])
}

// text is the word the reader is at as it was written, and `]]` past the end, which is what
// bash names there.
func (reader *conditionReader) text() string {
	if reader.done() {
		return "]]"
	}
	return printWord(reader.words[reader.at])
}

// operand is the word the reader is at, taken, when it can be an operand: bash's WORD, which
// a token is not.
func (reader *conditionReader) operand() (word, bool) {
	if reader.done() || isConditionToken(reader.operator()) {
		return word{}, false
	}
	reader.at++
	return reader.words[reader.at-1], true
}

func (reader *conditionReader) or() (*conditionNode, error) {
	left, err := reader.and()
	if err != nil || reader.operator() != "||" {
		return left, err
	}
	reader.at++
	right, err := reader.or()
	return &conditionNode{kind: conditionOr, left: left, right: right}, err
}

func (reader *conditionReader) and() (*conditionNode, error) {
	left, err := reader.term()
	if err != nil || reader.operator() != "&&" {
		return left, err
	}
	reader.at++
	right, err := reader.and()
	return &conditionNode{kind: conditionAnd, left: left, right: right}, err
}

// term is cond_term: a group, a negation, a unary test, a binary one, or a word alone.
func (reader *conditionReader) term() (*conditionNode, error) {
	operator := reader.operator()
	switch {
	case reader.done():
		return nil, errors.New("syntax error in conditional expression")
	// A binary operator after the word, with an operand after it, makes a comparison whatever
	// the word is, as busybox reads `[[ -f == -f ]]` and `[[ ! == x ]]`: two strings compared,
	// where bash's parser stops at them. Nothing bash reads otherwise is read differently.
	case reader.binaryFollows():
		reader.at++
		return reader.binary(reader.words[reader.at-1])
	case operator == "(":
		reader.at++
		inner, err := reader.or()
		if err != nil {
			return nil, err
		}
		if reader.operator() != ")" {
			return nil, fmt.Errorf("syntax error: unexpected token `%s', expected `)'", reader.text())
		}
		reader.at++
		return &conditionNode{kind: conditionGroup, left: inner}, nil
	case operator == "!":
		reader.at++
		node, err := reader.term()
		if err == nil {
			node.negated = !node.negated
		}
		return node, err
	case isConditionUnaryOperator(operator):
		reader.at++
		operand, ok := reader.operand()
		if !ok {
			return nil, fmt.Errorf("syntax error: unexpected argument `%s' to conditional unary operator", reader.text())
		}
		return &conditionNode{kind: conditionUnary, operator: operator, operands: []word{operand}}, nil
	case isConditionToken(operator):
		return nil, fmt.Errorf("syntax error: unexpected token `%s' in conditional command", operator)
	}
	left := reader.words[reader.at]
	reader.at++
	if operator = reader.operator(); reader.done() || operator == "&&" || operator == "||" || operator == ")" {
		return &conditionNode{kind: conditionUnary, operator: "-n", operands: []word{left}}, nil
	}
	return reader.binary(left)
}

// binary reads a binary test whose left operand is taken, the reader at its operator.
func (reader *conditionReader) binary(left word) (*conditionNode, error) {
	operator := reader.operator()
	if !doubleBracketBinaryOperators[operator] {
		return nil, fmt.Errorf("syntax error: unexpected token `%s', conditional binary operator expected", reader.text())
	}
	reader.at++
	right, ok := reader.operand()
	if !ok {
		return nil, fmt.Errorf("syntax error: unexpected argument `%s' to conditional binary operator", reader.text())
	}
	return &conditionNode{kind: conditionBinary, operator: operator, operands: []word{left, right}}, nil
}

// binaryFollows reports a binary operator written plainly after the word the reader is at,
// and a word after it that can be its operand.
func (reader *conditionReader) binaryFollows() bool {
	if reader.at+2 >= len(reader.words) {
		return false
	}
	operator, operand := conditionOperator(reader.words[reader.at+1]), conditionOperator(reader.words[reader.at+2])
	return doubleBracketBinaryOperators[operator] && !isConditionToken(operand)
}
