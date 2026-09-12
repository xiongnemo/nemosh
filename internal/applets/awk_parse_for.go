package applets

import "fmt"

// `for` and `print`, the two statements whose parsing needs a decision rather than a
// transcription.
//
// `for` is three statements wearing one keyword: the C form, `for (k in a)`, and the C
// form whose initialiser happens to contain `in`. Telling the second from the third is
// the whole difficulty, and it is done by looking ahead for `name in name )` rather than
// by parsing and backtracking.
//
// `print`'s difficulty is that **`>` in its argument list is a redirect**: `print 1 > "f"`
// writes to a file and `print (1 > 0)` prints 1. Both references agree and the only
// difference is the parenthesis.

func (p *awkParser) parseFor() (awkStmt, error) {
	p.advance()
	if err := p.expectOperator("(", "after for"); err != nil {
		return nil, err
	}
	if statement, ok, err := p.parseForIn(); ok || err != nil {
		return statement, err
	}
	// The C form. Each of the three parts may be empty: `for (;;)` loops forever.
	var initialise awkStmt
	if !p.atOperator(";") {
		first, err := p.parseSimpleStatement()
		if err != nil {
			return nil, err
		}
		initialise = first
	}
	if err := p.expectOperator(";", "after a for initialiser"); err != nil {
		return nil, err
	}
	p.skipOptionalNewlines()
	var condition awkExpr
	if !p.atOperator(";") {
		test, err := p.parseExpression(awkExprNormal)
		if err != nil {
			return nil, err
		}
		condition = test
	}
	if err := p.expectOperator(";", "after a for condition"); err != nil {
		return nil, err
	}
	p.skipOptionalNewlines()
	var step awkStmt
	if !p.atOperator(")") {
		third, err := p.parseSimpleStatement()
		if err != nil {
			return nil, err
		}
		step = third
	}
	if err := p.expectOperator(")", "to close a for header"); err != nil {
		return nil, err
	}
	p.skipOptionalNewlines()
	body, err := p.parseStatement()
	return awkForStmt{initialise: initialise, condition: condition, step: step, body: body}, err
}

// parseForIn recognises `for (name in array)` by looking ahead, and answers false when
// the header is the C form instead.
//
// Lookahead rather than parse-and-backtrack, because `for (k in a)` and
// `for (i = (k in a); ...)` both begin with a name and the second is a perfectly good C
// loop whose initialiser tests membership. Three tokens settle it: a name, `in`, a name,
// and then the closing parenthesis.
func (p *awkParser) parseForIn() (awkStmt, bool, error) {
	if p.peek().kind != awkTokenName {
		return nil, false, nil
	}
	if !(p.peekAhead(1).kind == awkTokenKeyword && p.peekAhead(1).text == "in") {
		return nil, false, nil
	}
	if p.peekAhead(2).kind != awkTokenName {
		return nil, false, nil
	}
	if !(p.peekAhead(3).kind == awkTokenOperator && p.peekAhead(3).text == ")") {
		return nil, false, nil
	}
	name := p.peek().text
	array := p.peekAhead(2).text
	p.offset += 4
	p.skipOptionalNewlines()
	body, err := p.parseStatement()
	return awkForInStmt{name: name, array: array, body: body}, true, err
}

// parseSimpleStatement is what a `for` header's first and third parts may hold: an
// expression, and nothing else. `for (print; ;)` is not a thing.
func (p *awkParser) parseSimpleStatement() (awkStmt, error) {
	expr, err := p.parseExpression(awkExprNoIn)
	if err != nil {
		return nil, err
	}
	return awkExprStmt{expr: expr}, nil
}

// parsePrint reads `print` and `printf`, with their optional redirect.
func (p *awkParser) parsePrint() (awkStmt, error) {
	keyword := p.peek().text
	p.advance()
	args, err := p.parsePrintArguments()
	if err != nil {
		return nil, err
	}
	redirect, err := p.parseRedirect()
	if err != nil {
		return nil, err
	}
	if keyword == "printf" {
		if len(args) == 0 {
			return nil, fmt.Errorf("line %d: printf needs a format", p.peek().line)
		}
		return awkPrintfStmt{args: args, redirect: redirect}, nil
	}
	return awkPrintStmt{args: args, redirect: redirect}, nil
}

// parsePrintArguments reads the comma-separated list, with `>` reserved for the redirect.
//
// `print (1, 2)` is the parenthesised form of the whole list rather than one grouped
// expression, which is why a leading `(` that spans the entire argument list is unwrapped
// here.
func (p *awkParser) parsePrintArguments() ([]awkExpr, error) {
	args := []awkExpr{}
	if p.endsStatement() || p.startsRedirect() {
		return args, nil
	}
	for {
		arg, err := p.parseExpression(awkExprNoGT | awkExprNoPipe)
		if err != nil {
			return nil, err
		}
		args = append(args, arg)
		if !p.atOperator(",") {
			break
		}
		p.advance()
		p.skipOptionalNewlines()
	}
	// `print (a, b)` parses as one group holding a list, which the grouping parser
	// refuses on its own -- so the whole-list parenthesis is handled by the caller
	// seeing a single group and the evaluator unwrapping it. Here, a single argument
	// that is a group stays a group; `print (1 > 0)` depends on it.
	return args, nil
}

func (p *awkParser) startsRedirect() bool {
	return p.atOperator(">") || p.atOperator(">>") || p.atOperator("|")
}

// parseRedirect reads `> file`, `>> file` or `| command`, if one is there.
func (p *awkParser) parseRedirect() (*awkRedirect, error) {
	if !p.startsRedirect() {
		return nil, nil
	}
	operator := p.peek().text
	p.advance()
	// The target binds tightly: concatenation is allowed, comparison is not, which is
	// what makes `print > "a" "b"` write to the file `ab`.
	target, err := p.parseConcat(awkExprNoGT)
	if err != nil {
		return nil, err
	}
	return &awkRedirect{operator: operator, target: target}, nil
}
