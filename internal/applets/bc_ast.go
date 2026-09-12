package applets

// The shape of a parsed bc program.
//
// The one node that looks redundant and is not is **bcGroupExpr**. `x = 5` as a statement
// prints nothing, because bc suppresses the value of a top-level assignment; `(x = 5)`
// prints 5, because the parenthesis makes it an expression that happens to assign. The only
// way to tell them apart later is to remember that the parenthesis was there.

type bcExpr interface{ bcExprNode() }

type (
	// bcNumberExpr keeps the digits, not the value: ibase may have changed since the
	// program was read. See bc_lex.go.
	bcNumberExpr struct{ digits string }
	bcNameExpr   struct{ name string }
	bcIndexExpr  struct {
		name  string
		index bcExpr
	}
	bcBinaryExpr struct {
		operator    string
		left, right bcExpr
	}
	bcUnaryExpr struct {
		operator string
		operand  bcExpr
	}
	bcAssignExpr struct {
		operator string
		target   bcExpr
		value    bcExpr
	}
	bcIncDecExpr struct {
		operator string
		prefix   bool
		target   bcExpr
	}
	bcCallExpr struct {
		name string
		args []bcExpr
	}
	// bcBuiltinExpr is length, scale, sqrt or read.
	bcBuiltinExpr struct {
		name string
		args []bcExpr
	}
	bcGroupExpr struct{ inner bcExpr }
	// bcStringExpr appears only in a `print` list, which is the one place bc mixes
	// strings and numbers.
	bcStringExpr struct{ text string }
)

func (bcNumberExpr) bcExprNode()  {}
func (bcNameExpr) bcExprNode()    {}
func (bcIndexExpr) bcExprNode()   {}
func (bcBinaryExpr) bcExprNode()  {}
func (bcUnaryExpr) bcExprNode()   {}
func (bcAssignExpr) bcExprNode()  {}
func (bcIncDecExpr) bcExprNode()  {}
func (bcCallExpr) bcExprNode()    {}
func (bcBuiltinExpr) bcExprNode() {}
func (bcGroupExpr) bcExprNode()   {}
func (bcStringExpr) bcExprNode()  {}

type bcStmt interface{ bcStmtNode() }

type (
	bcExprStmt   struct{ expr bcExpr }
	bcStringStmt struct{ text string }
	// bcPrintStmt takes a mixed list: `print "x is ", x, "\n"`.
	bcPrintStmt struct{ items []bcExpr }
	bcBlockStmt struct{ body []bcStmt }
	bcIfStmt    struct {
		condition bcExpr
		then      bcStmt
		otherwise bcStmt
	}
	bcWhileStmt struct {
		condition bcExpr
		body      bcStmt
	}
	bcForStmt struct {
		initialise bcExpr
		condition  bcExpr
		update     bcExpr
		body       bcStmt
	}
	bcBreakStmt    struct{}
	bcContinueStmt struct{}
	bcReturnStmt   struct{ value bcExpr }
	bcHaltStmt     struct{}
	bcDefineStmt   struct{ function *bcFunction }
)

func (bcExprStmt) bcStmtNode()     {}
func (bcStringStmt) bcStmtNode()   {}
func (bcPrintStmt) bcStmtNode()    {}
func (bcBlockStmt) bcStmtNode()    {}
func (bcIfStmt) bcStmtNode()       {}
func (bcWhileStmt) bcStmtNode()    {}
func (bcForStmt) bcStmtNode()      {}
func (bcBreakStmt) bcStmtNode()    {}
func (bcContinueStmt) bcStmtNode() {}
func (bcReturnStmt) bcStmtNode()   {}
func (bcHaltStmt) bcStmtNode()     {}
func (bcDefineStmt) bcStmtNode()   {}

// bcFunction is a `define`.
//
// params and autos are kept apart because they are filled differently -- a parameter takes
// the caller's value and an auto starts at zero -- but both are **local**, and both are
// restored when the call returns. That is what makes bc's recursion work at all.
type bcFunction struct {
	name   string
	params []bcParameter
	autos  []bcParameter
	body   []bcStmt
}

// bcParameter is a name, and whether it was written as `name[]` -- an array.
type bcParameter struct {
	name    string
	isArray bool
}
