package applets

import "fmt"

// The bottom of bc's expression grammar: numbers, names, calls and grouping.
//
// Split from bc_parse.go for the 250-line ceiling. The decision that lives here is
// **`scale`**, which is both a function and a variable -- `scale(x)` and `scale = 5` -- and
// which of the two is decided by looking at the next token, because there is nothing else to
// decide it by.

func (p *bcParser) parsePrimary() (bcExpr, error) {
	token := p.peek()
	switch token.kind {
	case bcTokenNumber:
		p.advance()
		return bcNumberExpr{digits: token.text}, nil
	case bcTokenName:
		return p.parseNameOrCall()
	case bcTokenKeyword:
		switch token.text {
		case "scale":
			// `scale` is both: `scale(x)` is the function and `scale = 5` is the variable
			// that every division reads. Which one is decided by the next token, because
			// there is nothing else to decide it by.
			if p.offset+1 < len(p.tokens) && p.tokens[p.offset+1].text == "(" {
				return p.parseBuiltin()
			}
			p.advance()
			return bcNameExpr{name: "scale"}, nil
		case "length", "sqrt", "read":
			return p.parseBuiltin()
		}
	case bcTokenOperator:
		if token.text == "(" {
			p.advance()
			inner, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			if err := p.expectOperator(")"); err != nil {
				return nil, err
			}
			// Kept rather than folded away: see bc_ast.go on why `(x = 5)` prints.
			return bcGroupExpr{inner: inner}, nil
		}
	}
	return nil, fmt.Errorf("line %d: unexpected %q", token.line, token.text)
}

// parseNameOrCall reads a variable, an array element, or a call.
func (p *bcParser) parseNameOrCall() (bcExpr, error) {
	name := p.peek().text
	p.advance()
	switch {
	case p.atOperator("["):
		p.advance()
		index, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		if err := p.expectOperator("]"); err != nil {
			return nil, err
		}
		return bcIndexExpr{name: name, index: index}, nil
	case p.atOperator("("):
		args, err := p.parseArguments()
		if err != nil {
			return nil, err
		}
		return bcCallExpr{name: name, args: args}, nil
	}
	return bcNameExpr{name: name}, nil
}

func (p *bcParser) parseBuiltin() (bcExpr, error) {
	name := p.peek().text
	p.advance()
	args, err := p.parseArguments()
	if err != nil {
		return nil, err
	}
	return bcBuiltinExpr{name: name, args: args}, nil
}

func (p *bcParser) parseArguments() ([]bcExpr, error) {
	if err := p.expectOperator("("); err != nil {
		return nil, err
	}
	args := []bcExpr{}
	if p.atOperator(")") {
		p.advance()
		return args, nil
	}
	for {
		// An array argument is written `name[]`, which is not an expression anywhere else.
		if p.peek().kind == bcTokenName && p.offset+2 < len(p.tokens) &&
			p.tokens[p.offset+1].text == "[" && p.tokens[p.offset+2].text == "]" {
			args = append(args, bcIndexExpr{name: p.peek().text})
			p.offset += 3
		} else {
			argument, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			args = append(args, argument)
		}
		if p.atOperator(",") {
			p.advance()
			continue
		}
		if err := p.expectOperator(")"); err != nil {
			return nil, err
		}
		return args, nil
	}
}
