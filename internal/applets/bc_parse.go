package applets

import "fmt"

// bc's expression grammar, lowest precedence first:
//
//	assignment -> or -> and -> not -> relational -> additive -> multiplicative
//	-> unary minus -> power -> increment -> primary
//
// Two rungs are worth pointing at, because both catch people:
//
//   - **`^` binds tighter than unary minus**, so `-2^2` is -4 and not 4. It is also
//     **right-associative**, so `2^3^2` is 2^9.
//   - **Assignment is right-associative and is an expression**, so `a = b = 5` sets both --
//     and `(x = 5)` as a statement prints 5 where a bare `x = 5` prints nothing.

type bcParser struct {
	tokens []bcToken
	offset int
}

func (p *bcParser) peek() bcToken {
	if p.offset >= len(p.tokens) {
		return bcToken{kind: bcTokenEOF}
	}
	return p.tokens[p.offset]
}

func (p *bcParser) advance() { p.offset++ }

func (p *bcParser) at(kind bcTokenKind, text string) bool {
	token := p.peek()
	return token.kind == kind && token.text == text
}

func (p *bcParser) atOperator(text string) bool { return p.at(bcTokenOperator, text) }
func (p *bcParser) atKeyword(text string) bool  { return p.at(bcTokenKeyword, text) }

func (p *bcParser) expectOperator(text string) error {
	if !p.atOperator(text) {
		return fmt.Errorf("line %d: expected %s", p.peek().line, text)
	}
	p.advance()
	return nil
}

func (p *bcParser) parseExpression() (bcExpr, error) { return p.parseAssignment() }

var bcAssignOperators = map[string]bool{
	"=": true, "+=": true, "-=": true, "*=": true, "/=": true, "%=": true, "^=": true,
}

func (p *bcParser) parseAssignment() (bcExpr, error) {
	left, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	token := p.peek()
	if token.kind != bcTokenOperator || !bcAssignOperators[token.text] {
		return left, nil
	}
	if !bcIsLvalue(left) {
		return nil, fmt.Errorf("line %d: cannot assign to this", token.line)
	}
	p.advance()
	// Right-associative, so `a = b = 5` parses as `a = (b = 5)`.
	value, err := p.parseAssignment()
	if err != nil {
		return nil, err
	}
	return bcAssignExpr{operator: token.text, target: left, value: value}, nil
}

func bcIsLvalue(expr bcExpr) bool {
	switch expr.(type) {
	case bcNameExpr, bcIndexExpr:
		return true
	}
	return false
}

func (p *bcParser) parseOr() (bcExpr, error) {
	return p.parseLeftAssociative([]string{"||"}, (*bcParser).parseAnd)
}

func (p *bcParser) parseAnd() (bcExpr, error) {
	return p.parseLeftAssociative([]string{"&&"}, (*bcParser).parseNot)
}

func (p *bcParser) parseNot() (bcExpr, error) {
	if p.atOperator("!") {
		p.advance()
		operand, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		return bcUnaryExpr{operator: "!", operand: operand}, nil
	}
	return p.parseRelational()
}

// parseRelational is non-associative: `1 < 2 < 3` is refused rather than answered.
//
// Chaining is almost always a mistake, and `(1<2)<3` -- which is what a left-associative
// reading would give -- is a number nobody meant.
func (p *bcParser) parseRelational() (bcExpr, error) {
	left, err := p.parseAdditive()
	if err != nil {
		return nil, err
	}
	token := p.peek()
	if token.kind != bcTokenOperator || !bcRelational[token.text] {
		return left, nil
	}
	p.advance()
	right, err := p.parseAdditive()
	if err != nil {
		return nil, err
	}
	if next := p.peek(); next.kind == bcTokenOperator && bcRelational[next.text] {
		return nil, fmt.Errorf("line %d: %s cannot be chained after %s; parenthesise one of them",
			next.line, next.text, token.text)
	}
	return bcBinaryExpr{operator: token.text, left: left, right: right}, nil
}

var bcRelational = map[string]bool{
	"==": true, "!=": true, "<": true, "<=": true, ">": true, ">=": true,
}

func (p *bcParser) parseAdditive() (bcExpr, error) {
	return p.parseLeftAssociative([]string{"+", "-"}, (*bcParser).parseMultiplicative)
}

func (p *bcParser) parseMultiplicative() (bcExpr, error) {
	return p.parseLeftAssociative([]string{"*", "/", "%"}, (*bcParser).parseUnary)
}

func (p *bcParser) parseUnary() (bcExpr, error) {
	if p.atOperator("-") {
		p.advance()
		// The operand is a *power*, not another unary: `-2^2` is -(2^2).
		operand, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return bcUnaryExpr{operator: "-", operand: operand}, nil
	}
	return p.parsePower()
}

// parsePower is right-associative, so `2^3^2` is `2^(3^2)`.
func (p *bcParser) parsePower() (bcExpr, error) {
	left, err := p.parseIncrement()
	if err != nil {
		return nil, err
	}
	if !p.atOperator("^") {
		return left, nil
	}
	p.advance()
	// The right side goes through parseUnary so that `2^-1` reads, which every bc accepts.
	right, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	return bcBinaryExpr{operator: "^", left: left, right: right}, nil
}

func (p *bcParser) parseIncrement() (bcExpr, error) {
	if p.atOperator("++") || p.atOperator("--") {
		operator := p.peek().text
		line := p.peek().line
		p.advance()
		target, err := p.parseIncrement()
		if err != nil {
			return nil, err
		}
		if !bcIsLvalue(target) {
			return nil, fmt.Errorf("line %d: %s needs a variable", line, operator)
		}
		return bcIncDecExpr{operator: operator, prefix: true, target: target}, nil
	}
	left, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	for p.atOperator("++") || p.atOperator("--") {
		operator := p.peek().text
		if !bcIsLvalue(left) {
			return nil, fmt.Errorf("line %d: %s needs a variable", p.peek().line, operator)
		}
		p.advance()
		left = bcIncDecExpr{operator: operator, target: left}
	}
	return left, nil
}

func (p *bcParser) parseLeftAssociative(operators []string,
	next func(*bcParser) (bcExpr, error)) (bcExpr, error) {
	left, err := next(p)
	if err != nil {
		return nil, err
	}
	for {
		token := p.peek()
		matched := false
		for _, operator := range operators {
			if token.kind == bcTokenOperator && token.text == operator {
				matched = true
				break
			}
		}
		if !matched {
			return left, nil
		}
		p.advance()
		right, err := next(p)
		if err != nil {
			return nil, err
		}
		left = bcBinaryExpr{operator: token.text, left: left, right: right}
	}
}
