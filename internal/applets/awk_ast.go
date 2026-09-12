package applets

// The shape of a parsed awk program.
//
// Two things in this tree are unusual enough to say out loud, and both are measured:
//
//   - **Concatenation is an operator with no symbol**, and it binds tighter than
//     comparison and looser than `+`. So `1 2 < 3` is `("12") < 3` -- which both
//     references answer 1 for -- and `1 " " -1` is `1` joined to `(" " - 1)`, giving
//     `1-1` rather than `1 -1`. That second one catches everybody.
//   - **`$` binds tighter than `++`.** `$x++` increments the *field*, not the subscript:
//     with `x=1` and `$0="a b c"` both references print `0 1`, so `$1` was read, coerced
//     to 0 and incremented, and `x` never moved.

// awkExpr is any expression.
type awkExpr interface{ awkExprNode() }

type (
	// awkNumberExpr and awkStringExpr are literals. A string literal is a *string*
	// forever, never a strnum -- which is why `x = "10"; x < 9` is true.
	awkNumberExpr struct{ value float64 }
	awkStringExpr struct{ value string }

	// awkRegexExpr is `/re/` standing alone, which means `$0 ~ /re/`. As the right
	// operand of `~` it is the pattern itself rather than a match, which is why the
	// node survives to evaluation instead of being rewritten here.
	awkRegexExpr struct{ pattern string }

	awkVarExpr   struct{ name string }
	awkFieldExpr struct{ index awkExpr }

	// awkIndexExpr is `a[i]` and `a[i,j]`; the second joins its parts with SUBSEP.
	awkIndexExpr struct {
		name  string
		index []awkExpr
	}

	awkBinaryExpr struct {
		operator    string
		left, right awkExpr
	}
	// awkConcatExpr is juxtaposition. A list rather than nested pairs, because
	// `a b c d` is one operation and rendering it as three would make the evaluator
	// build three intermediate strings.
	awkConcatExpr struct{ parts []awkExpr }

	awkUnaryExpr struct {
		operator string
		operand  awkExpr
	}
	// awkMatchExpr is `~` and `!~`.
	awkMatchExpr struct {
		negated     bool
		left, right awkExpr
	}
	// awkInExpr is `(i) in a` and `(i,j) in a`.
	awkInExpr struct {
		index []awkExpr
		array string
	}
	awkTernaryExpr struct{ condition, yes, no awkExpr }
	// awkAssignExpr covers `=` and the compound forms, whose operator is kept whole:
	// `+=` rather than a `+` and a flag.
	awkAssignExpr struct {
		operator string
		target   awkExpr
		value    awkExpr
	}
	awkIncDecExpr struct {
		operator string
		prefix   bool
		target   awkExpr
	}
	awkCallExpr struct {
		name string
		args []awkExpr
	}
	// awkBuiltinExpr is a call to one of the language's own functions, kept apart from
	// awkCallExpr because several take an array or an lvalue rather than a value --
	// `split(s, a)` fills `a`, and `sub(re, x)` rewrites `$0`.
	awkBuiltinExpr struct {
		name string
		args []awkExpr
	}
	// awkGroupExpr is an expression in parentheses, kept rather than folded away
	// because `(1 > 0)` is a comparison while a bare `1 > 0` after `print` is a
	// redirect. The parser needs to remember which it saw.
	awkGroupExpr struct{ inner awkExpr }
	// awkGroupListExpr is `(a, b)` with no `in` after it, which is legal in exactly two
	// places: as the whole argument list of `print` or `printf`. Written as an
	// expression anywhere else it is an error, and evaluating one says so.
	awkGroupListExpr struct{ items []awkExpr }

	// awkGetlineExpr is every form of getline. Which one is decided by which fields
	// are set: a source command or file, an optional target, and whether the input
	// came from a pipe.
	awkGetlineExpr struct {
		target awkExpr
		source awkExpr
		mode   awkGetlineMode
	}
)

// awkGetlineMode names the six forms, because they differ in what they set as well as
// where they read: plain `getline` updates NR, NF and $0, while `getline < file` updates
// only $0 and NF.
type awkGetlineMode uint8

const (
	awkGetlinePlain awkGetlineMode = iota
	awkGetlineFile
	awkGetlineCommand
)

func (awkNumberExpr) awkExprNode()    {}
func (awkStringExpr) awkExprNode()    {}
func (awkRegexExpr) awkExprNode()     {}
func (awkVarExpr) awkExprNode()       {}
func (awkFieldExpr) awkExprNode()     {}
func (awkIndexExpr) awkExprNode()     {}
func (awkBinaryExpr) awkExprNode()    {}
func (awkConcatExpr) awkExprNode()    {}
func (awkUnaryExpr) awkExprNode()     {}
func (awkMatchExpr) awkExprNode()     {}
func (awkInExpr) awkExprNode()        {}
func (awkTernaryExpr) awkExprNode()   {}
func (awkAssignExpr) awkExprNode()    {}
func (awkIncDecExpr) awkExprNode()    {}
func (awkCallExpr) awkExprNode()      {}
func (awkBuiltinExpr) awkExprNode()   {}
func (awkGroupExpr) awkExprNode()     {}
func (awkGroupListExpr) awkExprNode() {}
func (awkGetlineExpr) awkExprNode()   {}

// awkStmt is any statement.
type awkStmt interface{ awkStmtNode() }

// awkRedirect is where a `print` sends its output: `>`, `>>` or `|`.
type awkRedirect struct {
	operator string
	target   awkExpr
}

type (
	awkPrintStmt struct {
		args     []awkExpr
		redirect *awkRedirect
	}
	awkPrintfStmt struct {
		args     []awkExpr
		redirect *awkRedirect
	}
	awkExprStmt  struct{ expr awkExpr }
	awkBlockStmt struct{ body []awkStmt }
	awkIfStmt    struct {
		condition awkExpr
		then      awkStmt
		otherwise awkStmt
	}
	awkWhileStmt struct {
		condition awkExpr
		body      awkStmt
	}
	awkDoStmt struct {
		body      awkStmt
		condition awkExpr
	}
	awkForStmt struct {
		initialise awkStmt
		condition  awkExpr
		step       awkStmt
		body       awkStmt
	}
	awkForInStmt struct {
		name  string
		array string
		body  awkStmt
	}
	// awkDeleteStmt with no index removes the whole array, which POSIX added later and
	// both references accept.
	awkDeleteStmt struct {
		name  string
		index []awkExpr
	}
	awkNextStmt     struct{}
	awkNextFileStmt struct{}
	awkExitStmt     struct{ status awkExpr }
	awkReturnStmt   struct{ value awkExpr }
	awkBreakStmt    struct{}
	awkContinueStmt struct{}
)

func (awkPrintStmt) awkStmtNode()    {}
func (awkPrintfStmt) awkStmtNode()   {}
func (awkExprStmt) awkStmtNode()     {}
func (awkBlockStmt) awkStmtNode()    {}
func (awkIfStmt) awkStmtNode()       {}
func (awkWhileStmt) awkStmtNode()    {}
func (awkDoStmt) awkStmtNode()       {}
func (awkForStmt) awkStmtNode()      {}
func (awkForInStmt) awkStmtNode()    {}
func (awkDeleteStmt) awkStmtNode()   {}
func (awkNextStmt) awkStmtNode()     {}
func (awkNextFileStmt) awkStmtNode() {}
func (awkExitStmt) awkStmtNode()     {}
func (awkReturnStmt) awkStmtNode()   {}
func (awkBreakStmt) awkStmtNode()    {}
func (awkContinueStmt) awkStmtNode() {}

// awkItemKind is which sort of rule an item is.
type awkItemKind uint8

const (
	// awkItemAlways has no pattern, so it runs for every record.
	awkItemAlways awkItemKind = iota
	awkItemBegin
	awkItemEnd
	awkItemPattern
	// awkItemRange is `pat1, pat2`, which is on from the record matching the first
	// until the record matching the second.
	awkItemRange
)

// awkItem is one `pattern { action }` rule.
type awkItem struct {
	kind awkItemKind
	// pattern and until are the one or two patterns; until is set only for a range.
	pattern awkExpr
	until   awkExpr
	// action is nil when the rule had none, which means `{ print }`.
	action []awkStmt
	// active carries a range's state between records. On the item because that is what
	// a range *is* -- a rule that remembers.
	active bool
}

// awkFunction is a user-defined function.
type awkFunction struct {
	name string
	// params are both the parameters and the locals: awk has no other way to declare
	// one, so a function takes extra parameters the caller does not pass.
	params []string
	body   []awkStmt
}

// awkProgram is a whole program.
type awkProgram struct {
	items     []awkItem
	functions map[string]*awkFunction
}
