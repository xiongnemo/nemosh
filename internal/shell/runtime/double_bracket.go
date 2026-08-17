package runtime

import (
	"context"
	"fmt"
	"strings"
)

// `[[ ... ]]`, the conditional expression.
//
// Not POSIX -- dash has only `[` -- and this follows bash, measured:
//
//	x="a b"; [[ $x == "a b" ]]      true   -- no word splitting inside
//	[[ abc == a* ]]                 true   -- the right side is a pattern
//	[[ "abc" == "a*" ]]             false  -- quoted, so it is a literal
//	[[ abc =~ ^a.c$ ]]              true   -- a regular expression
//	[[ 3 -lt 5 ]]                   true
//	[[ 1 -eq 1 && 2 -eq 2 ]]        true
//	[[ $empty -n ]] with empty unset does not become a syntax error
//
// **The reason it exists is the first and the last of those.** Inside `[[ ]]` a
// word is not split and not globbed, so `[ $x = "a b" ]` -- which becomes
// `[ a b = a b ]` and is a syntax error -- works. That is the whole appeal, and
// it is also why this cannot be an applet: an applet receives words that have
// already been split, and by then the information is gone.
//
// So it is intercepted before expansion, with the word AST still in hand. That
// also supplies the other thing an applet could not know: whether the right-hand
// side of `==` was quoted, which decides pattern against literal.
//
// One limitation, stated rather than hidden: the expression has to be on one
// line. bash allows it to span lines, because there `[[` is a reserved word the
// parser knows; here it is recognised at execution time, and the line has already
// been divided into commands by then.

// isDoubleBracket reports whether this command is a `[[ ]]` conditional.
func isDoubleBracket(command []word) bool {
	if len(command) < 2 {
		return false
	}
	return isUnquotedLiteralWord(command[0]) && soleLiteralText(command[0]) == "[["
}

// runDoubleBracket evaluates the conditional and returns its status: 0 for true,
// 1 for false, 2 for a malformed expression -- which is bash's, and keeps "the
// answer is no" distinguishable from "that was not an expression".
func (r Runtime) runDoubleBracket(ctx context.Context, command []word, savedStatus int) lineResult {
	if soleLiteralText(command[len(command)-1]) != "]]" {
		fmt.Fprintln(r.streams.Stderr, "nemosh: [[: missing ]]")
		return lineResult{status: 2}
	}
	terms := make([]conditionTerm, 0, len(command)-2)
	for _, item := range command[1 : len(command)-1] {
		text, quoted := r.expandConditionWord(ctx, item, savedStatus)
		terms = append(terms, conditionTerm{text: text, quoted: quoted})
	}
	if r.expansionFailed() {
		return unsetParameterResult()
	}
	parser := &conditionParser{terms: terms, runtime: r}
	value, err := parser.parseOr()
	if err == nil && !parser.done() {
		err = fmt.Errorf("unexpected %s", parser.peek().text)
	}
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "nemosh: [[: %v\n", err)
		return lineResult{status: 2}
	}
	if value {
		return lineResult{}
	}
	return lineResult{status: 1}
}

// conditionTerm is one word of the expression, and whether any of it was quoted.
type conditionTerm struct {
	text   string
	quoted bool
}

// expandConditionWord expands one word with neither field splitting nor pathname
// expansion, and reports whether any part of it was quoted.
//
// The quoting matters for exactly one thing and it is not cosmetic: `[[ abc ==
// a* ]]` is a pattern match and `[[ abc == "a*" ]]` is a string comparison.
// Measured -- the second is false in bash.
func (r Runtime) expandConditionWord(ctx context.Context, item word, savedStatus int) (string, bool) {
	quoted := false
	for _, part := range item.parts {
		if part.quote != quoteUnquoted || part.kind == wordPartEscaped {
			quoted = true
			break
		}
	}
	// A single field, joined: expandWord splits on IFS, and inside `[[ ]]` it
	// must not. Joining what it produced restores the word -- the split is the
	// only thing being undone, so a value containing blanks comes back whole.
	fields := r.expandWord(ctx, item, savedStatus)
	return strings.Join(fields, " "), quoted
}

// conditionParser reads the expression: `||` lowest, then `&&`, then `!`, then a
// primary. Same shape as expr's parser and for the same reason -- precedence has
// to come from the grammar, not from a table.
type conditionParser struct {
	terms   []conditionTerm
	at      int
	runtime Runtime
}

func (p *conditionParser) done() bool { return p.at >= len(p.terms) }

func (p *conditionParser) peek() conditionTerm {
	if p.done() {
		return conditionTerm{}
	}
	return p.terms[p.at]
}

func (p *conditionParser) take() conditionTerm {
	term := p.peek()
	p.at++
	return term
}

func (p *conditionParser) parseOr() (bool, error) {
	left, err := p.parseAnd()
	if err != nil {
		return false, err
	}
	for !p.done() && p.peek().text == "||" {
		p.take()
		right, err := p.parseAnd()
		if err != nil {
			return false, err
		}
		left = left || right
	}
	return left, nil
}

func (p *conditionParser) parseAnd() (bool, error) {
	left, err := p.parseNegation()
	if err != nil {
		return false, err
	}
	for !p.done() && p.peek().text == "&&" {
		p.take()
		right, err := p.parseNegation()
		if err != nil {
			return false, err
		}
		left = left && right
	}
	return left, nil
}

func (p *conditionParser) parseNegation() (bool, error) {
	if !p.done() && p.peek().text == "!" && !p.peek().quoted {
		p.take()
		value, err := p.parseNegation()
		return !value, err
	}
	return p.parsePrimary()
}

// soleLiteralText is a word's text when it is one plain literal part, and "" for
// anything else. `[[` has to be recognised as written rather than as expanded: a
// variable holding the string `[[` is an ordinary command word.
func soleLiteralText(item word) string {
	if len(item.parts) != 1 || item.parts[0].kind != wordPartLiteral {
		return ""
	}
	return item.parts[0].text
}
