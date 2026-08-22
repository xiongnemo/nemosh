package applets

import (
	"fmt"
	"strings"
)

// The expression grammar, which is POSIX's and GNU's:
//
//	expr    := and ( ('-o' | '-or') and )*
//	and     := unary ( ('-a' | '-and')? unary )*
//	unary   := ('!' | '-not') unary | primary
//	primary := '(' expr ')' | predicate
//
// So -a binds tighter than -o, ! binds tighter than both, and adjacency is -a.
// The whole expression is parsed before any directory is read, which is what
// keeps a malformed expression from writing paths a caller would then act on.

// findOperators are the words that cannot be a path operand. Path collection
// stops at the first of them, which is the fix for `find . ! -name x` reporting
// `!: No such file or directory` -- the operator was being collected as a path
// because it does not begin with a dash.
var findOperators = map[string]bool{"!": true, "(": true, ")": true}

// parseFindArguments splits paths from the expression and validates the whole
// expression before any walking starts.
func parseFindArguments(args []string, view ProcessView) ([]string, findExpression, error) {
	var paths []string
	index := 0
	for ; index < len(args); index++ {
		if strings.HasPrefix(args[index], "-") || findOperators[args[index]] {
			break
		}
		paths = append(paths, args[index])
	}
	if len(paths) == 0 {
		paths = []string{"."}
	}

	parser := &findParser{args: args[index:], view: view, expression: findExpression{maxDepth: -1}}
	expression, err := parser.parse()
	if err != nil {
		return nil, findExpression{}, err
	}
	return paths, expression, nil
}

type findParser struct {
	args       []string
	index      int
	view       ProcessView
	expression findExpression
	// hasAction records whether the expression wrote anything itself. POSIX
	// applies an implicit -print only when it does not, which is what stops
	// `find . -name x -print` printing twice.
	hasAction bool
}

func (p *findParser) parse() (findExpression, error) {
	if len(p.args) == 0 {
		p.expression.root = findPrint{terminator: '\n'}
		return p.expression, nil
	}
	root, err := p.parseOr()
	if err != nil {
		return findExpression{}, err
	}
	if p.index < len(p.args) {
		// The only token that can stop the parse without being consumed.
		return findExpression{}, fmt.Errorf("unpaired %q", p.args[p.index])
	}
	if !p.hasAction {
		root = findAnd{left: root, right: findPrint{terminator: '\n'}}
	}
	p.expression.root = root
	return p.expression, nil
}

func (p *findParser) parseOr() (findNode, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.peek() == "-o" || p.peek() == "-or" {
		operator := p.next()
		if !p.startsPrimary() {
			return nil, fmt.Errorf("%s: missing an expression after it", operator)
		}
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = findOr{left: left, right: right}
	}
	return left, nil
}

func (p *findParser) parseAnd() (findNode, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for {
		explicit := p.peek() == "-a" || p.peek() == "-and"
		if explicit {
			operator := p.next()
			if !p.startsPrimary() {
				return nil, fmt.Errorf("%s: missing an expression after it", operator)
			}
		} else if !p.startsPrimary() {
			return left, nil
		}
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		left = findAnd{left: left, right: right}
	}
}

func (p *findParser) parseUnary() (findNode, error) {
	if token := p.peek(); token == "!" || token == "-not" {
		p.next()
		if !p.startsPrimary() {
			return nil, fmt.Errorf("%s: missing an expression after it", token)
		}
		inner, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return findNot{inner: inner}, nil
	}
	return p.parsePrimary()
}

func (p *findParser) parsePrimary() (findNode, error) {
	token := p.peek()
	switch token {
	case "":
		return nil, fmt.Errorf("expected an expression")
	case "(":
		p.next()
		inner, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		if p.peek() != ")" {
			return nil, fmt.Errorf("unpaired %q", "(")
		}
		p.next()
		return inner, nil
	case ")":
		// Reached where a test was required: either `find . )` or an empty
		// group. busybox takes the first as a path operand and prints the whole
		// tree before complaining; naming the operator is the honest answer.
		return nil, fmt.Errorf("unpaired %q", ")")
	case "-o", "-or", "-a", "-and":
		return nil, fmt.Errorf("%s: missing an expression before it", token)
	}
	return p.parsePredicate()
}

// startsPrimary reports whether the next token could begin a test, which is how
// adjacency is recognised as an implicit -a and how a dangling operator is
// caught before it becomes a confusing predicate error.
func (p *findParser) startsPrimary() bool {
	switch p.peek() {
	case "", ")", "-o", "-or", "-a", "-and":
		return false
	}
	return true
}

func (p *findParser) peek() string {
	if p.index >= len(p.args) {
		return ""
	}
	return p.args[p.index]
}

func (p *findParser) next() string {
	token := p.peek()
	p.index++
	return token
}

// argument takes the operand a test requires, naming the test when it is absent
// rather than reporting the end of the arguments.
func (p *findParser) argument(operand string) (string, error) {
	if p.index >= len(p.args) {
		return "", fmt.Errorf("%s: requires an argument", operand)
	}
	return p.next(), nil
}
