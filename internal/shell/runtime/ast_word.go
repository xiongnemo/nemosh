package runtime

type quoteContext uint8

const (
	quoteUnquoted quoteContext = iota
	quoteSingle
	quoteDouble
)

type wordPartKind uint8

const (
	wordPartLiteral wordPartKind = iota
	wordPartParameter
	wordPartCommandSubstitution
	wordPartEscaped
	wordPartArithmetic
	// wordPartProcessSubstitution is `<(command)`: it expands to a path holding the
	// command's output. See process_substitution.go.
	wordPartProcessSubstitution
)

type wordPart struct {
	kind   wordPartKind
	text   string
	quote  quoteContext
	script *Script
}

type word struct {
	parts       []wordPart
	quotedEmpty bool
	expandTilde bool
	// assignmentTilde marks a `name=~` word, where the tilde to expand is after
	// the `=` rather than at the start. See assignment_expand.go.
	assignmentTilde bool
}
