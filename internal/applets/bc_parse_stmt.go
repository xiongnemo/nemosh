package applets

// bc's statements, and the `define` that holds them.
//
// A statement ends at a newline or a `;`, and **a newline inside brackets is not a
// terminator** -- which is why `if (x)` on one line and its body on the next works. The
// parser skips newlines wherever a statement plainly cannot have ended: after `if (...)`,
// after `else`, after `{`, and between the statements of a block.

func parseBcProgram(source string) ([]bcStmt, error) {
	tokens, err := scanBcTokens(source)
	if err != nil {
		return nil, err
	}
	parser := &bcParser{tokens: tokens}
	var program []bcStmt
	for {
		parser.skipSeparators()
		if parser.peek().kind == bcTokenEOF {
			return program, nil
		}
		statement, err := parser.parseStatement()
		if err != nil {
			return nil, err
		}
		if statement != nil {
			program = append(program, statement)
		}
	}
}

// skipSeparators passes over the newlines and semicolons between statements.
func (p *bcParser) skipSeparators() {
	for {
		token := p.peek()
		if token.kind == bcTokenNewline || (token.kind == bcTokenOperator && token.text == ";") {
			p.advance()
			continue
		}
		return
	}
}

// skipNewlines passes over newlines only, for the places a statement cannot have ended.
func (p *bcParser) skipNewlines() {
	for p.peek().kind == bcTokenNewline {
		p.advance()
	}
}

func (p *bcParser) parseStatement() (bcStmt, error) {
	token := p.peek()
	switch token.kind {
	case bcTokenString:
		p.advance()
		return bcStringStmt{text: token.text}, nil
	case bcTokenOperator:
		if token.text == "{" {
			return p.parseBlock()
		}
	case bcTokenKeyword:
		switch token.text {
		case "if":
			return p.parseIf()
		case "while":
			return p.parseWhile()
		case "for":
			return p.parseFor()
		case "break":
			p.advance()
			return bcBreakStmt{}, nil
		case "continue":
			p.advance()
			return bcContinueStmt{}, nil
		case "return":
			return p.parseReturn()
		case "halt", "quit":
			// quit leaves at once even inside a function, which is what makes it
			// different from halt in a real bc; both end the program here.
			p.advance()
			return bcHaltStmt{}, nil
		case "print":
			return p.parsePrint()
		case "define":
			return p.parseDefine()
		}
	}
	expr, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	return bcExprStmt{expr: expr}, nil
}

func (p *bcParser) parseBlock() (bcStmt, error) {
	if err := p.expectOperator("{"); err != nil {
		return nil, err
	}
	var body []bcStmt
	for {
		p.skipSeparators()
		if p.atOperator("}") {
			p.advance()
			return bcBlockStmt{body: body}, nil
		}
		if p.peek().kind == bcTokenEOF {
			return nil, p.incompleteAt("line %d: unclosed {", p.peek().line)
		}
		statement, err := p.parseStatement()
		if err != nil {
			return nil, err
		}
		body = append(body, statement)
	}
}

func (p *bcParser) parseIf() (bcStmt, error) {
	p.advance()
	condition, err := p.parseParenthesised()
	if err != nil {
		return nil, err
	}
	p.skipNewlines()
	if p.peek().kind == bcTokenEOF {
		// `if (x)` with its body still to come: keep reading rather than refusing.
		return nil, errBcIncomplete
	}
	then, err := p.parseStatement()
	if err != nil {
		return nil, err
	}
	// The `else` may be on a later line, so newlines are passed over to look for it -- but
	// only if one is really there, since otherwise they end the statement.
	saved := p.offset
	p.skipSeparators()
	if !p.atKeyword("else") {
		p.offset = saved
		return bcIfStmt{condition: condition, then: then}, nil
	}
	p.advance()
	p.skipNewlines()
	if p.peek().kind == bcTokenEOF {
		return nil, errBcIncomplete
	}
	otherwise, err := p.parseStatement()
	if err != nil {
		return nil, err
	}
	return bcIfStmt{condition: condition, then: then, otherwise: otherwise}, nil
}

func (p *bcParser) parseWhile() (bcStmt, error) {
	p.advance()
	condition, err := p.parseParenthesised()
	if err != nil {
		return nil, err
	}
	p.skipNewlines()
	if p.peek().kind == bcTokenEOF {
		return nil, errBcIncomplete
	}
	body, err := p.parseStatement()
	if err != nil {
		return nil, err
	}
	return bcWhileStmt{condition: condition, body: body}, nil
}

// parseFor reads `for (init; condition; update) body`, where each of the three may be empty.
//
// An empty condition is **true**, which is what makes `for (;;)` a loop rather than a
// statement that never runs.
func (p *bcParser) parseFor() (bcStmt, error) {
	p.advance()
	if err := p.expectOperator("("); err != nil {
		return nil, err
	}
	initialise, err := p.parseOptionalExpression(";")
	if err != nil {
		return nil, err
	}
	if err := p.expectOperator(";"); err != nil {
		return nil, err
	}
	condition, err := p.parseOptionalExpression(";")
	if err != nil {
		return nil, err
	}
	if err := p.expectOperator(";"); err != nil {
		return nil, err
	}
	update, err := p.parseOptionalExpression(")")
	if err != nil {
		return nil, err
	}
	if err := p.expectOperator(")"); err != nil {
		return nil, err
	}
	p.skipNewlines()
	if p.peek().kind == bcTokenEOF {
		return nil, errBcIncomplete
	}
	body, err := p.parseStatement()
	if err != nil {
		return nil, err
	}
	return bcForStmt{initialise: initialise, condition: condition, update: update, body: body}, nil
}

func (p *bcParser) parseOptionalExpression(terminator string) (bcExpr, error) {
	if p.atOperator(terminator) {
		return nil, nil
	}
	return p.parseExpression()
}

func (p *bcParser) parseParenthesised() (bcExpr, error) {
	if err := p.expectOperator("("); err != nil {
		return nil, err
	}
	inner, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	return inner, p.expectOperator(")")
}

// parseReturn reads `return`, `return expr` and `return(expr)`, which are all written.
func (p *bcParser) parseReturn() (bcStmt, error) {
	p.advance()
	token := p.peek()
	if token.kind == bcTokenNewline || token.kind == bcTokenEOF ||
		(token.kind == bcTokenOperator && (token.text == ";" || token.text == "}")) {
		return bcReturnStmt{}, nil
	}
	value, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	return bcReturnStmt{value: value}, nil
}

// parsePrint reads a comma-separated list of expressions and strings.
func (p *bcParser) parsePrint() (bcStmt, error) {
	p.advance()
	var items []bcExpr
	for {
		if p.peek().kind == bcTokenString {
			items = append(items, bcStringExpr{text: p.peek().text})
			p.advance()
		} else {
			item, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			items = append(items, item)
		}
		if p.atOperator(",") {
			p.advance()
			continue
		}
		return bcPrintStmt{items: items}, nil
	}
}
