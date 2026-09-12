package applets

import "fmt"

// The program level: items, functions and blocks.
//
// An item is `pattern { action }` in any of its shapes -- `BEGIN`, `END`, a bare block, a
// pattern, or a `pat1, pat2` range. A pattern with **no action means `{ print }`**, and
// that is left as a nil action rather than synthesised here: the two run the same, and the
// nil records which one the program actually wrote.

func (p *awkParser) parseProgram() (*awkProgram, error) {
	program := &awkProgram{functions: map[string]*awkFunction{}}
	p.skipNewlines()
	for p.peek().kind != awkTokenEOF {
		if p.atKeyword("function") || p.atKeyword("func") {
			if err := p.parseFunction(program); err != nil {
				return nil, err
			}
			p.skipNewlines()
			continue
		}
		item, err := p.parseItem()
		if err != nil {
			return nil, err
		}
		program.items = append(program.items, item)
		p.skipNewlines()
	}
	return program, nil
}

// parseFunction reads `function name(params) { body }`.
func (p *awkParser) parseFunction(program *awkProgram) error {
	line := p.peek().line
	p.advance()
	token := p.peek()
	if token.kind != awkTokenName && token.kind != awkTokenFuncName {
		return fmt.Errorf("line %d: expected a function name, found %s", token.line, awkDescribeToken(token))
	}
	name := token.text
	if _, defined := program.functions[name]; defined {
		return fmt.Errorf("line %d: function %s is defined twice", line, name)
	}
	p.advance()
	if err := p.expectOperator("(", "to open the parameter list"); err != nil {
		return err
	}
	// Parameters double as the only way to declare a local, so a function often takes
	// more than its callers pass.
	params := []string{}
	for !p.atOperator(")") {
		p.skipOptionalNewlines()
		param, err := p.expectName("a parameter name")
		if err != nil {
			return err
		}
		params = append(params, param)
		if p.atOperator(",") {
			p.advance()
			p.skipOptionalNewlines()
		}
	}
	p.advance()
	p.skipOptionalNewlines()
	body, err := p.parseBlock()
	if err != nil {
		return err
	}
	program.functions[name] = &awkFunction{name: name, params: params, body: body}
	return nil
}

// parseItem reads one `pattern { action }` rule in any of its shapes.
func (p *awkParser) parseItem() (awkItem, error) {
	switch {
	case p.atKeyword("BEGIN"):
		p.advance()
		p.skipOptionalNewlines()
		body, err := p.parseBlock()
		return awkItem{kind: awkItemBegin, action: body}, err
	case p.atKeyword("END"):
		p.advance()
		p.skipOptionalNewlines()
		body, err := p.parseBlock()
		return awkItem{kind: awkItemEnd, action: body}, err
	case p.atOperator("{"):
		body, err := p.parseBlock()
		return awkItem{kind: awkItemAlways, action: body}, err
	}
	pattern, err := p.parseExpression(awkExprNormal)
	if err != nil {
		return awkItem{}, err
	}
	item := awkItem{kind: awkItemPattern, pattern: pattern}
	if p.atOperator(",") {
		// A range: on from the record matching the first pattern until one matches
		// the second.
		p.advance()
		p.skipOptionalNewlines()
		until, err := p.parseExpression(awkExprNormal)
		if err != nil {
			return awkItem{}, err
		}
		item.kind, item.until = awkItemRange, until
	}
	if p.atOperator("{") {
		body, err := p.parseBlock()
		if err != nil {
			return awkItem{}, err
		}
		item.action = body
	}
	// A pattern with no action means `{ print }`, which is left as a nil action for the
	// interpreter to read rather than synthesised here -- the two are the same thing,
	// and the nil says which the program actually wrote.
	return item, nil
}

// parseBlock reads `{ ... }`.
func (p *awkParser) parseBlock() ([]awkStmt, error) {
	if err := p.expectOperator("{", "to open a block"); err != nil {
		return nil, err
	}
	body := []awkStmt{}
	p.skipNewlines()
	for !p.atOperator("}") {
		if p.peek().kind == awkTokenEOF {
			return nil, fmt.Errorf("line %d: expected } to close a block", p.peek().line)
		}
		statement, err := p.parseStatement()
		if err != nil {
			return nil, err
		}
		body = append(body, statement)
		p.skipNewlines()
	}
	p.advance()
	return body, nil
}
