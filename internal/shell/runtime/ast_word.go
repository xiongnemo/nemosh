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
	// wordPartOutputSubstitution is `>(command)`: a path to a pipe the command reads
	// while the consumer writes. See output_substitution.go.
	wordPartOutputSubstitution
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
	// assignmentTilde marks a `name=value` word, whose tilde-prefixes begin after the
	// `=` and after every unquoted `:`. See tilde_expand.go.
	assignmentTilde bool
	// valueTilde marks an assignment's value standing alone -- an array literal's
	// `[k]=v` -- whose tilde-prefixes begin at its start and after every unquoted `:`.
	valueTilde bool
}
