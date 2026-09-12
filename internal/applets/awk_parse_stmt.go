package applets

import "fmt"

// Statements.
//
// The one genuinely awkward corner is `print`: a `>` in its argument list is a
// **redirect**, not a comparison, so `print 1 > "f"` writes to a file while
// `print (1 > 0)` prints `1`. Both references agree, and the only difference is the
// parenthesis -- which is why awkGroupExpr survives parsing and why the argument list is
// read with awkExprNoGT set.

func (p *awkParser) parseStatement() (awkStmt, error) {
	token := p.peek()
	if token.kind == awkTokenOperator && token.text == "{" {
		body, err := p.parseBlock()
		return awkBlockStmt{body: body}, err
	}
	if token.kind == awkTokenKeyword {
		switch token.text {
		case "if":
			return p.parseIf()
		case "while":
			return p.parseWhile()
		case "do":
			return p.parseDo()
		case "for":
			return p.parseFor()
		case "print", "printf":
			return p.parsePrint()
		case "delete":
			return p.parseDelete()
		case "next":
			p.advance()
			return awkNextStmt{}, nil
		case "nextfile":
			p.advance()
			return awkNextFileStmt{}, nil
		case "break":
			p.advance()
			return awkBreakStmt{}, nil
		case "continue":
			p.advance()
			return awkContinueStmt{}, nil
		case "exit":
			p.advance()
			return awkExitStmt{status: p.parseOptionalExpression()}, nil
		case "return":
			p.advance()
			return awkReturnStmt{value: p.parseOptionalExpression()}, nil
		}
	}
	expr, err := p.parseExpression(awkExprNormal)
	if err != nil {
		return nil, err
	}
	return awkExprStmt{expr: expr}, nil
}

// parseOptionalExpression reads the value after `exit` or `return`, if there is one.
func (p *awkParser) parseOptionalExpression() awkExpr {
	if p.endsStatement() {
		return nil
	}
	expr, err := p.parseExpression(awkExprNormal)
	if err != nil {
		return nil
	}
	return expr
}

// endsStatement reports whether the current token closes a statement.
func (p *awkParser) endsStatement() bool {
	token := p.peek()
	switch token.kind {
	case awkTokenEOF, awkTokenNewline:
		return true
	case awkTokenOperator:
		return token.text == ";" || token.text == "}"
	}
	return false
}

func (p *awkParser) parseIf() (awkStmt, error) {
	p.advance()
	condition, err := p.parseParenthesised("if")
	if err != nil {
		return nil, err
	}
	p.skipOptionalNewlines()
	then, err := p.parseStatement()
	if err != nil {
		return nil, err
	}
	statement := awkIfStmt{condition: condition, then: then}
	// The `else` may be separated by a terminator, which is why this looks past one.
	saved := p.offset
	p.skipNewlines()
	if !p.atKeyword("else") {
		p.offset = saved
		return statement, nil
	}
	p.advance()
	p.skipOptionalNewlines()
	otherwise, err := p.parseStatement()
	if err != nil {
		return nil, err
	}
	statement.otherwise = otherwise
	return statement, nil
}

func (p *awkParser) parseWhile() (awkStmt, error) {
	p.advance()
	condition, err := p.parseParenthesised("while")
	if err != nil {
		return nil, err
	}
	p.skipOptionalNewlines()
	body, err := p.parseStatement()
	return awkWhileStmt{condition: condition, body: body}, err
}

func (p *awkParser) parseDo() (awkStmt, error) {
	p.advance()
	p.skipOptionalNewlines()
	body, err := p.parseStatement()
	if err != nil {
		return nil, err
	}
	p.skipNewlines()
	if !p.atKeyword("while") {
		return nil, fmt.Errorf("line %d: expected while after a do body", p.peek().line)
	}
	p.advance()
	condition, err := p.parseParenthesised("while")
	return awkDoStmt{body: body, condition: condition}, err
}

// parseParenthesised reads the `( expr )` a keyword requires.
func (p *awkParser) parseParenthesised(keyword string) (awkExpr, error) {
	if err := p.expectOperator("(", "after "+keyword); err != nil {
		return nil, err
	}
	p.skipOptionalNewlines()
	condition, err := p.parseExpression(awkExprNormal)
	if err != nil {
		return nil, err
	}
	p.skipOptionalNewlines()
	if err := p.expectOperator(")", "to close "+keyword+"'s condition"); err != nil {
		return nil, err
	}
	return condition, nil
}

func (p *awkParser) parseDelete() (awkStmt, error) {
	p.advance()
	name, err := p.expectName("an array name after delete")
	if err != nil {
		return nil, err
	}
	if !p.atOperator("[") {
		// `delete a` empties the whole array. Later than the rest of POSIX, but both
		// references accept it.
		return awkDeleteStmt{name: name}, nil
	}
	p.advance()
	index, err := p.parseSubscripts(awkExprNormal)
	return awkDeleteStmt{name: name, index: index}, err
}
