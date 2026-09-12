package applets

import "fmt"

// The bottom of the expression grammar: literals, names, fields, calls and grouping.
//
// Two decisions here carry weight further up.
//
// **`$` binds tighter than `++`.** `$x++` is `($x)++`, which both references confirm:
// with `x=1` and `$0="a b c"` they print `0 1`, so the field was read, coerced to 0 and
// incremented while `x` never moved. So `$` takes a *postfix-free* operand and the
// increment wraps the field afterwards.
//
// **A parenthesised expression keeps its node.** `print (1 > 0)` is a comparison and
// `print 1 > "f"` is a redirect, and the only difference is the parenthesis -- so it has
// to survive parsing rather than being folded away.

func (p *awkParser) parsePrimary(flags awkExprFlags) (awkExpr, error) {
	token := p.peek()
	switch token.kind {
	case awkTokenNumber:
		p.advance()
		return awkNumberExpr{value: token.number}, nil
	case awkTokenString:
		p.advance()
		return awkStringExpr{value: token.text}, nil
	case awkTokenRegex:
		p.advance()
		return awkRegexExpr{pattern: token.text}, nil
	case awkTokenFuncName:
		return p.parseCall()
	case awkTokenBuiltin:
		return p.parseBuiltin(flags)
	case awkTokenName:
		return p.parseNameOrIndex(flags)
	case awkTokenKeyword:
		if token.text == "getline" {
			return p.parseGetline(flags)
		}
	case awkTokenOperator:
		switch token.text {
		case "$":
			p.advance()
			// A field's subscript is itself a primary, so `$NF` and `$(i+1)` both
			// work while `$i++` increments the field rather than the subscript.
			index, err := p.parsePrimaryForField(flags)
			if err != nil {
				return nil, err
			}
			return awkFieldExpr{index: index}, nil
		case "++", "--":
			p.advance()
			operand, err := p.parseUnary(flags)
			if err != nil {
				return nil, err
			}
			if !awkIsLvalue(operand) {
				return nil, fmt.Errorf("line %d: %s needs a variable, a field or an array element", token.line, token.text)
			}
			return awkIncDecExpr{operator: token.text, prefix: true, target: operand}, nil
		case "(":
			return p.parseGrouping(flags)
		}
	}
	return nil, fmt.Errorf("line %d: unexpected %s", token.line, awkDescribeToken(token))
}

// parsePrimaryForField reads what follows a `$`, stopping before any postfix increment.
func (p *awkParser) parsePrimaryForField(flags awkExprFlags) (awkExpr, error) {
	token := p.peek()
	if token.kind == awkTokenOperator {
		switch token.text {
		case "-", "+", "!":
			// `$-1` and `$!x` are legal, and the operand is again postfix-free.
			p.advance()
			operand, err := p.parsePrimaryForField(flags)
			if err != nil {
				return nil, err
			}
			return awkUnaryExpr{operator: token.text, operand: operand}, nil
		case "++", "--":
			// `$++x` increments the subscript, which both references confirm prints
			// the *second* field for `x=1`.
			return p.parsePrimary(flags)
		}
	}
	return p.parsePrimary(flags)
}

// parseGrouping reads `( ... )`, which is either one expression or the `(i,j) in a` form.
func (p *awkParser) parseGrouping(flags awkExprFlags) (awkExpr, error) {
	line := p.peek().line
	p.advance()
	// Inside parentheses a `>` is a comparison again, whatever the enclosing print
	// statement wanted.
	inner, err := p.parseExpression(flags &^ awkExprNoGT)
	if err != nil {
		return nil, err
	}
	if p.atOperator(",") {
		return p.parseParenthesisedIn(inner, flags, line)
	}
	if !p.atOperator(")") {
		return nil, fmt.Errorf("line %d: expected ) to close a group", p.peek().line)
	}
	p.advance()
	return awkGroupExpr{inner: inner}, nil
}

// parseParenthesisedIn reads `(i, j) in a`, the multi-subscript membership test.
func (p *awkParser) parseParenthesisedIn(first awkExpr, flags awkExprFlags, line int) (awkExpr, error) {
	index := []awkExpr{first}
	for p.atOperator(",") {
		p.advance()
		next, err := p.parseExpression(flags &^ awkExprNoGT)
		if err != nil {
			return nil, err
		}
		index = append(index, next)
	}
	if !p.atOperator(")") {
		return nil, fmt.Errorf("line %d: expected ) to close a group", p.peek().line)
	}
	p.advance()
	if p.peek().kind != awkTokenKeyword || p.peek().text != "in" {
		// The other legal home for a parenthesised list is `print (a, b)` and
		// `printf("%s\n", x)`, where it is the whole argument list. The parser cannot
		// tell that from here -- it does not know what statement it is inside -- so the
		// list is carried up and the print statement unwraps it. Anywhere else,
		// evaluating the node is what refuses.
		return awkGroupListExpr{items: index}, nil
	}
	p.advance()
	array, err := p.expectName("an array name after `in`")
	if err != nil {
		return nil, err
	}
	return awkInExpr{index: index, array: array}, nil
}

// parseNameOrIndex reads a bare name or `a[i]`.
func (p *awkParser) parseNameOrIndex(flags awkExprFlags) (awkExpr, error) {
	name := p.peek().text
	p.advance()
	if !p.atOperator("[") {
		return awkVarExpr{name: name}, nil
	}
	p.advance()
	index, err := p.parseSubscripts(flags)
	if err != nil {
		return nil, err
	}
	return awkIndexExpr{name: name, index: index}, nil
}

// parseSubscripts reads the inside of `[...]`, which may be a comma-separated list.
func (p *awkParser) parseSubscripts(flags awkExprFlags) ([]awkExpr, error) {
	index := []awkExpr{}
	for {
		// A subscript is a whole expression, and `in` and `>` mean what they usually
		// do inside brackets.
		item, err := p.parseExpression(flags &^ (awkExprNoGT | awkExprNoIn))
		if err != nil {
			return nil, err
		}
		index = append(index, item)
		if p.atOperator(",") {
			p.advance()
			continue
		}
		if !p.atOperator("]") {
			return nil, fmt.Errorf("line %d: expected ] to close a subscript", p.peek().line)
		}
		p.advance()
		return index, nil
	}
}

// parseCall reads `name(args)` for a user-defined function.
func (p *awkParser) parseCall() (awkExpr, error) {
	name := p.peek().text
	p.advance()
	args, err := p.parseArguments()
	if err != nil {
		return nil, err
	}
	return awkCallExpr{name: name, args: args}, nil
}

// parseBuiltin reads a call to one of the language's own functions.
//
// `length` is the one that may be written without parentheses, which both references
// accept and which is why it needs a case of its own.
func (p *awkParser) parseBuiltin(flags awkExprFlags) (awkExpr, error) {
	name := p.peek().text
	p.advance()
	if !p.atOperator("(") {
		if name != "length" {
			return nil, fmt.Errorf("line %d: %s needs its arguments in parentheses", p.peek().line, name)
		}
		return awkBuiltinExpr{name: name}, nil
	}
	args, err := p.parseArguments()
	if err != nil {
		return nil, err
	}
	return awkBuiltinExpr{name: name, args: args}, nil
}

// parseArguments reads `( a, b, c )`, the parenthesis already ahead.
func (p *awkParser) parseArguments() ([]awkExpr, error) {
	if !p.atOperator("(") {
		return nil, fmt.Errorf("line %d: expected ( to open an argument list", p.peek().line)
	}
	p.advance()
	args := []awkExpr{}
	if p.atOperator(")") {
		p.advance()
		return args, nil
	}
	for {
		// Arguments are full expressions, so a `>` inside one is a comparison.
		arg, err := p.parseExpression(awkExprNormal)
		if err != nil {
			return nil, err
		}
		args = append(args, arg)
		if p.atOperator(",") {
			p.advance()
			continue
		}
		if !p.atOperator(")") {
			return nil, fmt.Errorf("line %d: expected , or ) in an argument list", p.peek().line)
		}
		p.advance()
		return args, nil
	}
}

// awkDescribeToken names a token for an error message, in the words a reader would use.
func awkDescribeToken(token awkToken) string {
	switch token.kind {
	case awkTokenEOF:
		return "end of program"
	case awkTokenNewline:
		return "end of line"
	case awkTokenString:
		return fmt.Sprintf("the string %q", token.text)
	case awkTokenRegex:
		return fmt.Sprintf("the regular expression /%s/", token.text)
	case awkTokenNumber, awkTokenName, awkTokenFuncName, awkTokenBuiltin, awkTokenKeyword:
		return fmt.Sprintf("%q", token.text)
	}
	return fmt.Sprintf("%q", token.text)
}
