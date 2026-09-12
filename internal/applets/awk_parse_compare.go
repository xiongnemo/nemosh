package applets

import "fmt"

// The middle of the expression grammar: matching, comparison, and the pipe into `getline`.
//
// Split from awk_parse_expr.go only because that file reached this project's 250-line
// ceiling; the precedence ladder reads top to bottom across the two, and the chain is
//
//	assignment -> ternary -> or -> and -> match -> comparison -> in -> pipe-getline
//	-> concat -> additive -> multiplicative -> unary -> power -> postfix -> primary
//
// Binding *tighter* means sitting *deeper* in that chain, which is the thing that is easy
// to get backwards: `in` binds tighter than comparison, so it is called by comparison.

// parsePipeGetline reads `command | getline` and `command | getline var`.
//
// It sits **below comparison and above concatenation**, which is what makes
// `"echo " name | getline line` read from the concatenated command rather than piping only
// the last word of it. The result is a number, so a program almost always writes
// `while (("cmd" | getline line) > 0)` -- and the parentheses there are doing real work,
// because without them the `> 0` would be read before the pipe.
func (p *awkParser) parsePipeGetline(flags awkExprFlags) (awkExpr, error) {
	left, err := p.parseConcat(flags)
	if err != nil {
		return nil, err
	}
	for !flags.has(awkExprNoPipe) && p.atOperator("|") && p.peekAhead(1).text == "getline" {
		p.advance()
		getline, err := p.parseGetline(flags)
		if err != nil {
			return nil, err
		}
		node := getline.(awkGetlineExpr)
		node.mode, node.source = awkGetlineCommand, left
		left = node
	}
	return left, nil
}

func (p *awkParser) parseMatch(flags awkExprFlags) (awkExpr, error) {
	left, err := p.parseComparison(flags)
	if err != nil {
		return nil, err
	}
	for p.peek().kind == awkTokenOperator && (p.peek().text == "~" || p.peek().text == "!~") {
		negated := p.peek().text == "!~"
		p.advance()
		right, err := p.parseComparison(flags)
		if err != nil {
			return nil, err
		}
		left = awkMatchExpr{negated: negated, left: left, right: right}
	}
	return left, nil
}

// parseComparison is **non-associative**: `1 < 2 < 3` is refused.
//
// gawk refuses it and busybox answers 1. Refusing is followed because a chained
// comparison is almost always a mistake, and answering `(1<2)<3` silently gives a number
// nobody meant.
func (p *awkParser) parseComparison(flags awkExprFlags) (awkExpr, error) {
	left, err := p.parseIn(flags)
	if err != nil {
		return nil, err
	}
	operator, ok := p.comparisonOperator(flags)
	if !ok {
		return left, nil
	}
	p.advance()
	right, err := p.parseIn(flags)
	if err != nil {
		return nil, err
	}
	if next, chained := p.comparisonOperator(flags); chained {
		return nil, fmt.Errorf("line %d: %s cannot be chained after %s; parenthesise one of them",
			p.peek().line, next, operator)
	}
	return awkBinaryExpr{operator: operator, left: left, right: right}, nil
}

func (p *awkParser) comparisonOperator(flags awkExprFlags) (string, bool) {
	token := p.peek()
	if token.kind != awkTokenOperator {
		return "", false
	}
	switch token.text {
	case "<", "<=", "==", "!=", ">=":
		return token.text, true
	case ">":
		// In a print argument list a `>` is a redirect, not a comparison.
		if flags.has(awkExprNoGT) {
			return "", false
		}
		return token.text, true
	}
	return "", false
}
