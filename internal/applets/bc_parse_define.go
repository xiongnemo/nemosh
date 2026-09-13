package applets

import "fmt"

// Parsing a `define`, and the parameter and auto lists it carries.
//
// Split from bc_parse_stmt.go for the 250-line ceiling. params and autos are read by the
// same code because they have the same shape -- a name, or `name[]` for an array -- and are
// kept apart only because they are *filled* differently: a parameter takes the caller's
// value and an auto starts at zero.

// parseDefine reads a function.
func (p *bcParser) parseDefine() (bcStmt, error) {
	p.advance()
	if p.peek().kind != bcTokenName {
		return nil, fmt.Errorf("line %d: define needs a name", p.peek().line)
	}
	function := &bcFunction{name: p.peek().text}
	p.advance()
	params, err := p.parseParameterList()
	if err != nil {
		return nil, err
	}
	function.params = params
	p.skipNewlines()
	if err := p.expectOperator("{"); err != nil {
		return nil, err
	}
	p.skipSeparators()
	if p.atKeyword("auto") {
		p.advance()
		autos, err := p.parseAutoList()
		if err != nil {
			return nil, err
		}
		function.autos = autos
	}
	for {
		p.skipSeparators()
		if p.atOperator("}") {
			p.advance()
			return bcDefineStmt{function: function}, nil
		}
		if p.peek().kind == bcTokenEOF {
			return nil, p.incompleteAt("line %d: unclosed define", p.peek().line)
		}
		statement, err := p.parseStatement()
		if err != nil {
			return nil, err
		}
		function.body = append(function.body, statement)
	}
}

func (p *bcParser) parseParameterList() ([]bcParameter, error) {
	if err := p.expectOperator("("); err != nil {
		return nil, err
	}
	var params []bcParameter
	if p.atOperator(")") {
		p.advance()
		return params, nil
	}
	for {
		parameter, err := p.parseParameter()
		if err != nil {
			return nil, err
		}
		params = append(params, parameter)
		if p.atOperator(",") {
			p.advance()
			continue
		}
		return params, p.expectOperator(")")
	}
}

// parseAutoList reads the locals, which end at a newline or a semicolon.
func (p *bcParser) parseAutoList() ([]bcParameter, error) {
	var autos []bcParameter
	for {
		parameter, err := p.parseParameter()
		if err != nil {
			return nil, err
		}
		autos = append(autos, parameter)
		if p.atOperator(",") {
			p.advance()
			continue
		}
		return autos, nil
	}
}

func (p *bcParser) parseParameter() (bcParameter, error) {
	if p.peek().kind != bcTokenName {
		return bcParameter{}, fmt.Errorf("line %d: expected a name", p.peek().line)
	}
	name := p.peek().text
	p.advance()
	if p.atOperator("[") {
		p.advance()
		if err := p.expectOperator("]"); err != nil {
			return bcParameter{}, err
		}
		return bcParameter{name: name, isArray: true}, nil
	}
	return bcParameter{name: name}, nil
}
