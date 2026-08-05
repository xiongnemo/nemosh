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
}
