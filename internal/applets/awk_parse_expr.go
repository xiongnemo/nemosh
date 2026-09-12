package applets

import "fmt"

// Parsing an awk expression.
//
// The precedence ladder, lowest first, every rung measured against gawk 5.4.1 and
// busybox-w32 1.38.0 rather than copied from a grammar:
//
//	assignment   = += -= *= /= %= ^=        right
//	ternary      ?:                          right
//	or           ||
//	and          &&
//	match        ~ !~
//	comparison   < <= > >= != ==             NON-associative
//	in
//	concatenation                            (no symbol)
//	additive     + -
//	multiplicative * / %
//	unary        ! - +
//	power        ^ **                        right
//	increment    ++ --                       prefix and postfix
//	field        $
//
// The four places the references disagree are all busybox being permissive, and all four
// follow POSIX and gawk here -- because accepting them means running a program that every
// other awk refuses, which is the quiet divergence this project avoids:
//
//	2^3^2       512 (gawk, right-assoc) not 64 (busybox, left)
//	1 < 2 < 3   a syntax error (gawk) not 1 (busybox)
//	1 in a == 1 1 (gawk, `in` binds tighter) not 0 (busybox)
//	x =\n1      a syntax error (gawk) not an assignment (busybox)
//
// Two rungs catch people and are worth stating. **Concatenation binds tighter than
// comparison**, so `1 2 < 3` is `"12" < 3`. And **binary minus binds tighter than
// concatenation**, so `1 " " -1` is `1` joined to `(" " - 1)` and prints `1-1` rather than
// `1 -1`; both references agree, and it is the single most surprising thing in the
// language.

// parseExpression is the entry point; noIn and noGT suppress rungs that would otherwise
// swallow a `for(x in a)` header or a `print`'s redirect.
func (p *awkParser) parseExpression(flags awkExprFlags) (awkExpr, error) {
	return p.parseAssignment(flags)
}

// awkExprFlags marks the two contexts where an operator means something else.
type awkExprFlags uint8

const (
	awkExprNormal awkExprFlags = 0
	// awkExprNoIn is set inside a `for (` header, where `in` belongs to the loop
	// rather than to the expression.
	awkExprNoIn awkExprFlags = 1 << iota
	// awkExprNoGT is set in a `print` argument list, where `>` starts a redirect.
	// `print (1 > 0)` is a comparison because the parenthesis takes it out of the
	// argument list, which is why awkGroupExpr survives parsing.
	awkExprNoGT
)

func (f awkExprFlags) has(flag awkExprFlags) bool { return f&flag != 0 }

func (p *awkParser) parseAssignment(flags awkExprFlags) (awkExpr, error) {
	left, err := p.parseTernary(flags)
	if err != nil {
		return nil, err
	}
	operator := p.peek().text
	if p.peek().kind != awkTokenOperator || !awkAssignOperators[operator] {
		return left, nil
	}
	if !awkIsLvalue(left) {
		return nil, fmt.Errorf("line %d: %s needs a variable, a field or an array element on the left", p.peek().line, operator)
	}
	p.advance()
	// Right-associative: `x = y = 3` sets both, which both references confirm.
	value, err := p.parseAssignment(flags)
	if err != nil {
		return nil, err
	}
	return awkAssignExpr{operator: operator, target: left, value: value}, nil
}

var awkAssignOperators = map[string]bool{
	"=": true, "+=": true, "-=": true, "*=": true, "/=": true, "%=": true, "^=": true, "**=": true,
}

// awkIsLvalue reports whether something may be assigned to.
//
// Checked in the parser rather than at evaluation so that `1 = 2` is refused where it is
// written, with a line number, instead of failing halfway through a run.
func awkIsLvalue(expr awkExpr) bool {
	switch expr.(type) {
	case awkVarExpr, awkFieldExpr, awkIndexExpr:
		return true
	}
	return false
}

func (p *awkParser) parseTernary(flags awkExprFlags) (awkExpr, error) {
	condition, err := p.parseOr(flags)
	if err != nil {
		return nil, err
	}
	if !p.atOperator("?") {
		return condition, nil
	}
	p.advance()
	yes, err := p.parseTernary(flags)
	if err != nil {
		return nil, err
	}
	if !p.atOperator(":") {
		return nil, fmt.Errorf("line %d: expected : to close a ?:", p.peek().line)
	}
	p.advance()
	no, err := p.parseTernary(flags)
	if err != nil {
		return nil, err
	}
	return awkTernaryExpr{condition: condition, yes: yes, no: no}, nil
}

func (p *awkParser) parseOr(flags awkExprFlags) (awkExpr, error) {
	return p.parseLeftAssociative(flags, []string{"||"}, (*awkParser).parseAnd)
}

func (p *awkParser) parseAnd(flags awkExprFlags) (awkExpr, error) {
	return p.parseLeftAssociative(flags, []string{"&&"}, (*awkParser).parseMatch)
}

// parseIn handles `x in a`, which binds **tighter than comparison**.
//
// gawk's placement, measured: `1 in a == 1` is `(1 in a) == 1` and answers 1, where
// busybox answers 0. Binding tighter means sitting *deeper* in this chain, which the
// first version got backwards -- it called parseMatch from parseAnd and left the `==`
// with nothing to attach to, so the whole expression was refused as "unexpected ==".
func (p *awkParser) parseIn(flags awkExprFlags) (awkExpr, error) {
	left, err := p.parseConcat(flags)
	if err != nil {
		return nil, err
	}
	for !flags.has(awkExprNoIn) && p.peek().kind == awkTokenKeyword && p.peek().text == "in" {
		p.advance()
		array, err := p.expectName("an array name after `in`")
		if err != nil {
			return nil, err
		}
		left = awkInExpr{index: []awkExpr{left}, array: array}
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

// parseConcat joins adjacent expressions with no operator between them.
//
// The test is whether the next token *could begin an operand*. That is the whole of
// concatenation's grammar, and getting the set wrong is how `1 " " -1` ends up as
// `1 " " (-1)` -- giving `1 -1` -- instead of `1 (" " - 1)`, which is what both
// references print. A leading `-` or `+` is therefore **not** in the set: it binds as
// the binary operator it is.
func (p *awkParser) parseConcat(flags awkExprFlags) (awkExpr, error) {
	first, err := p.parseAdditive(flags)
	if err != nil {
		return nil, err
	}
	if !p.startsOperand(flags) {
		return first, nil
	}
	parts := []awkExpr{first}
	for p.startsOperand(flags) {
		next, err := p.parseAdditive(flags)
		if err != nil {
			return nil, err
		}
		parts = append(parts, next)
	}
	return awkConcatExpr{parts: parts}, nil
}

// startsOperand reports whether the next token could begin a concatenated operand.
func (p *awkParser) startsOperand(flags awkExprFlags) bool {
	token := p.peek()
	switch token.kind {
	case awkTokenNumber, awkTokenString, awkTokenRegex, awkTokenName,
		awkTokenFuncName, awkTokenBuiltin:
		return true
	case awkTokenKeyword:
		// `in` and the statement keywords end an expression rather than continuing it.
		return false
	case awkTokenOperator:
		switch token.text {
		case "$", "(", "!", "++", "--":
			return true
		}
	}
	return false
}

func (p *awkParser) parseAdditive(flags awkExprFlags) (awkExpr, error) {
	return p.parseLeftAssociative(flags, []string{"+", "-"}, (*awkParser).parseMultiplicative)
}

func (p *awkParser) parseMultiplicative(flags awkExprFlags) (awkExpr, error) {
	return p.parseLeftAssociative(flags, []string{"*", "/", "%"}, (*awkParser).parseUnary)
}

// parseUnary handles `!`, `-` and `+`.
//
// Below `^`, which is what makes `-2^2` equal -4 rather than 4 -- both references agree,
// and it is the arithmetic surprise people meet first.
func (p *awkParser) parseUnary(flags awkExprFlags) (awkExpr, error) {
	token := p.peek()
	if token.kind == awkTokenOperator {
		switch token.text {
		case "!", "-", "+":
			p.advance()
			operand, err := p.parseUnary(flags)
			if err != nil {
				return nil, err
			}
			return awkUnaryExpr{operator: token.text, operand: operand}, nil
		}
	}
	return p.parsePower(flags)
}

// parsePower is `^` and its synonym `**`, **right-associative**: `2^3^2` is 512.
//
// gawk's answer; busybox says 64, which is left-associative and contradicts POSIX.
//
// The right operand is parsed as a *unary* expression so that `2^-1` works, which both
// references accept.
func (p *awkParser) parsePower(flags awkExprFlags) (awkExpr, error) {
	left, err := p.parsePostfix(flags)
	if err != nil {
		return nil, err
	}
	if !p.atOperator("^") && !p.atOperator("**") {
		return left, nil
	}
	p.advance()
	right, err := p.parseUnary(flags)
	if err != nil {
		return nil, err
	}
	return awkBinaryExpr{operator: "^", left: left, right: right}, nil
}

func (p *awkParser) parsePostfix(flags awkExprFlags) (awkExpr, error) {
	operand, err := p.parsePrimary(flags)
	if err != nil {
		return nil, err
	}
	for (p.atOperator("++") || p.atOperator("--")) && awkIsLvalue(operand) {
		operator := p.peek().text
		p.advance()
		operand = awkIncDecExpr{operator: operator, prefix: false, target: operand}
	}
	return operand, nil
}

// parseLeftAssociative is the shape every left-associative rung shares.
func (p *awkParser) parseLeftAssociative(flags awkExprFlags, operators []string,
	next func(*awkParser, awkExprFlags) (awkExpr, error)) (awkExpr, error) {
	left, err := next(p, flags)
	if err != nil {
		return nil, err
	}
	for {
		token := p.peek()
		if token.kind != awkTokenOperator {
			return left, nil
		}
		matched := ""
		for _, operator := range operators {
			if token.text == operator {
				matched = operator
				break
			}
		}
		if matched == "" {
			return left, nil
		}
		p.advance()
		right, err := next(p, flags)
		if err != nil {
			return nil, err
		}
		left = awkBinaryExpr{operator: matched, left: left, right: right}
	}
}
