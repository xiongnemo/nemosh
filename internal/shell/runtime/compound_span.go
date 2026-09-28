package runtime

type compoundKind uint8

const (
	compoundIf compoundKind = iota
	compoundLoop
	compoundCase
)

// suffix is what followed the closer -- a redirection, or a pipe into another
// command. Empty for the ordinary case.
type compoundSpan struct {
	kind       compoundKind
	background bool
	start      int
	thenIndex  int
	elseIndex  int
	doIndex    int
	end        int
	caseArms   []caseArmSpan
	// suffix is what followed the closer -- a redirection, or a pipe into another
	// command. Empty for the ordinary case; see splitCompoundCloser.
	suffix string
	// prefix is what stood before the compound on its line: the words before the `|`
	// in `cmd | while read ...`, or the `&&` in `cmd && if ...`. prefixOperator is
	// that operator, and `!` with an empty prefix for a negated compound. Both empty
	// for the ordinary case; see parser_operator_compound.go.
	prefix         string
	prefixOperator string
	// afterCompound marks a compound whose prefix is the compound that closed on its opening
	// line, joined to it by prefixOperator: the `while` in `done | while read l`. See
	// parser_compound_chain.go.
	afterCompound bool
	// header is the compound's own header when the line held something before it, so
	// the readers see `while read -r l` rather than `cmd | while read -r l`. Empty
	// means the whole line is the header.
	header string
}

type caseArmSpan struct {
	patternIndex int
	bodyStart    int
	bodyEnd      int
	// terminator is `;;`, `;;&` or `;&`, which decides what happens after the body
	// runs. See caseArmNode.
	terminator string
}

type compoundFrame struct {
	span           compoundSpan
	casePattern    int
	casePatternSet bool
}
