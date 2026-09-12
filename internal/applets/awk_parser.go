package applets

import "fmt"

// The parser's scaffolding: a cursor over the token slice, and the little helpers the
// grammar files share.
//
// The whole token stream is scanned before parsing begins (see scanAwkTokens), so
// lookahead is a slice index rather than a buffer. awk programs are a one-liner or a few
// hundred lines, and nothing here needs to stream.

type awkParser struct {
	tokens []awkToken
	offset int
}

func newAwkParser(tokens []awkToken) *awkParser { return &awkParser{tokens: tokens} }

func (p *awkParser) peek() awkToken {
	if p.offset >= len(p.tokens) {
		return awkToken{kind: awkTokenEOF}
	}
	return p.tokens[p.offset]
}

// peekAhead looks past the current token, which the statement parser needs to tell
// `getline < file` from a comparison.
func (p *awkParser) peekAhead(distance int) awkToken {
	if p.offset+distance >= len(p.tokens) {
		return awkToken{kind: awkTokenEOF}
	}
	return p.tokens[p.offset+distance]
}

func (p *awkParser) advance() { p.offset++ }

func (p *awkParser) atOperator(text string) bool {
	token := p.peek()
	return token.kind == awkTokenOperator && token.text == text
}

func (p *awkParser) atKeyword(text string) bool {
	token := p.peek()
	return token.kind == awkTokenKeyword && token.text == text
}

// expectOperator consumes a required punctuation token, or says what was wanted.
func (p *awkParser) expectOperator(text, because string) error {
	if !p.atOperator(text) {
		return fmt.Errorf("line %d: expected %s %s, found %s",
			p.peek().line, text, because, awkDescribeToken(p.peek()))
	}
	p.advance()
	return nil
}

// expectName consumes a plain name.
//
// A name is also what an array parameter looks like, so this is what `in` and `delete`
// use; whether the name holds an array is a question for evaluation.
func (p *awkParser) expectName(because string) (string, error) {
	token := p.peek()
	if token.kind != awkTokenName {
		return "", fmt.Errorf("line %d: expected %s, found %s", token.line, because, awkDescribeToken(token))
	}
	p.advance()
	return token.text, nil
}

// skipNewlines consumes statement terminators where the grammar allows blank lines --
// between items, and after an opening brace.
func (p *awkParser) skipNewlines() {
	for p.peek().kind == awkTokenNewline || p.atOperator(";") {
		p.advance()
	}
}

// skipOptionalNewlines consumes newlines only, leaving a `;` for a caller that counts it.
func (p *awkParser) skipOptionalNewlines() {
	for p.peek().kind == awkTokenNewline {
		p.advance()
	}
}

// parseGetline reads the plain and file forms; the piped form is assembled by the
// statement parser, which is the only place that can see the `|` before the keyword.
//
// The six forms differ in what they set as well as where they read, which is why the
// mode is recorded rather than inferred later: plain `getline` updates NR, NF and `$0`,
// while `getline < file` updates only `$0` and NF, and `getline var < file` updates
// neither.
func (p *awkParser) parseGetline(flags awkExprFlags) (awkExpr, error) {
	p.advance()
	expr := awkGetlineExpr{mode: awkGetlinePlain}
	// An optional target, which must be somewhere a value can be stored.
	if p.startsGetlineTarget() {
		target, err := p.parsePostfix(flags)
		if err != nil {
			return nil, err
		}
		if !awkIsLvalue(target) {
			return nil, fmt.Errorf("line %d: getline needs a variable, a field or an array element", p.peek().line)
		}
		expr.target = target
	}
	if p.atOperator("<") {
		p.advance()
		// The file name binds tightly: `getline < "a" "b"` reads from `a` and then
		// concatenates, which is what both references do, so this takes a
		// concatenation-free operand.
		source, err := p.parseAdditive(flags)
		if err != nil {
			return nil, err
		}
		expr.mode, expr.source = awkGetlineFile, source
	}
	return expr, nil
}

// startsGetlineTarget reports whether what follows `getline` is a place to store into
// rather than the rest of the statement.
func (p *awkParser) startsGetlineTarget() bool {
	token := p.peek()
	switch token.kind {
	case awkTokenName:
		return true
	case awkTokenOperator:
		return token.text == "$"
	}
	return false
}

// parseAwkProgram is the entry point: tokens in, a program out.
func parseAwkProgram(source string) (*awkProgram, error) {
	tokens, err := scanAwkTokens(source)
	if err != nil {
		return nil, err
	}
	parser := newAwkParser(tokens)
	program, err := parser.parseProgram()
	if err != nil {
		return nil, err
	}
	// Which parameters are arrays can only be settled once every function is parsed,
	// since one may pass a parameter straight on to another. See awk_paramtypes.go.
	inferAwkArrayParameters(program)
	return program, nil
}
