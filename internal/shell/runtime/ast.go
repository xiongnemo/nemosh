package runtime

type Script struct {
	program []programNode
}

type programNode interface{ programNode() }

type listNode struct{ value list }

func (listNode) programNode() {}

type backgroundNode struct{ value programNode }

func (backgroundNode) programNode() {}

type functionName struct{ value string }

func (n functionName) String() string { return n.value }

type functionDefinition struct {
	name functionName
	body commandNode
}

func (functionDefinition) programNode() {}

type ifNode struct {
	condition list
	thenBody  []programNode
	elseBody  []programNode
}

func (ifNode) programNode() {}

type loopKind uint8

const (
	loopFor loopKind = iota
	loopWhile
	loopUntil
	// loopArithmetic is `for ((init; condition; step))`, whose three parts are
	// arithmetic expressions rather than a name and a word list.
	loopArithmetic
)

// arithmeticLoop is the three expressions of a C-style for. Any of them may be
// empty: `for ((;;))` is a loop forever, which is what an empty condition means.
type arithmeticLoop struct {
	initialize string
	condition  string
	step       string
}

type loopNode struct {
	kind      loopKind
	condition list
	name      string
	values    []word
	body      []programNode
	// arith is set only for loopArithmetic, where name and values are unused.
	arith arithmeticLoop
	// overArguments is `for name` with no `in`: the list is the positional
	// parameters, which POSIX 2.9.4.2 specifies.
	overArguments bool
}

func (loopNode) programNode() {}

type caseNode struct {
	word word
	arms []caseArmNode
}

func (caseNode) programNode() {}

type caseArmNode struct {
	patterns []word
	body     []programNode
	// terminator decides what happens after the body runs: `;;` stops, `;;&` goes on
	// testing the patterns below, and `;&` runs the next arm's body without testing
	// it. Measured against bash; see execute_case.go.
	terminator string
}

type list struct {
	items []listItem
}

type listItem struct {
	value      andOr
	background bool
}

type andOr struct {
	pipelines []pipeline
	operators []tokenKind
}

type pipeline struct {
	commands []commandNode
	negated  bool
}

type commandNode interface{ commandNode() }

type simpleCommand struct {
	words     []word
	redirects []redirectOperation
}

func (simpleCommand) commandNode() {}

type braceGroup struct {
	body      Script
	redirects []redirectOperation
}

func (braceGroup) commandNode() {}

type subshellCommand struct {
	body      Script
	redirects []redirectOperation
}

func (subshellCommand) commandNode() {}
