package applets

// Which of a function's parameters are arrays, worked out before the program runs.
//
// awk has no declaration, so it decides by **use**: a parameter is an array if the body
// subscripts it, deletes it, walks it with `for (k in a)`, tests membership in it, or hands
// it to `split`. gawk works this out the same way, and it has to be settled statically
// because the call site cannot tell -- `f(x)` looks identical whichever `f` wants.
//
// Binding every not-yet-typed name by reference was tried instead, and is wrong in a way
// worth recording. With
//
//	function max(a, b) { return a > b ? a : b }   { m = max(m, NF) }
//
// the first call sees an unset `m`, makes it an array, and binds it; every later call then
// finds an array and binds that, so the parameter `a` is never the number the program has
// been accumulating. The answer came out as the last record's NF rather than the largest.
// Both references say 3 and it said 2.
//
// A fixpoint, because a function may pass a parameter straight on: `outer(A)` calling
// `inner(A)` makes A an array only once `inner` is known to take one.
func inferAwkArrayParameters(program *awkProgram) {
	for _, function := range program.functions {
		function.arrayParams = map[string]bool{}
	}
	for settled := false; !settled; {
		settled = true
		for _, function := range program.functions {
			for _, name := range collectAwkArrayNames(program, function.body) {
				if !function.arrayParams[name] && awkHasParameter(function, name) {
					function.arrayParams[name] = true
					settled = false
				}
			}
		}
	}
}

func awkHasParameter(function *awkFunction, name string) bool {
	for _, parameter := range function.params {
		if parameter == name {
			return true
		}
	}
	return false
}

// collectAwkArrayNames lists every name a body uses as an array.
func collectAwkArrayNames(program *awkProgram, body []awkStmt) []string {
	var names []string
	note := func(name string) { names = append(names, name) }
	visitor := &awkVisitor{
		stmt: func(statement awkStmt) {
			switch node := statement.(type) {
			case awkForInStmt:
				note(node.array)
			case awkDeleteStmt:
				note(node.name)
			}
		},
		expr: func(expression awkExpr) {
			switch node := expression.(type) {
			case awkIndexExpr:
				note(node.name)
			case awkInExpr:
				note(node.array)
			case awkBuiltinExpr:
				if node.name == "split" && len(node.args) > 1 {
					if bare, ok := node.args[1].(awkVarExpr); ok {
						note(bare.name)
					}
				}
			case awkCallExpr:
				// Passing a bare name to a function that takes an array there makes it
				// one here too. This is the step that needs the fixpoint.
				callee, defined := program.functions[node.name]
				if !defined {
					return
				}
				for index, argument := range node.args {
					if index >= len(callee.params) {
						break
					}
					bare, isName := argument.(awkVarExpr)
					if isName && callee.arrayParams[callee.params[index]] {
						note(bare.name)
					}
				}
			}
		},
	}
	walkAwkBody(body, visitor)
	return names
}

// awkVisitor is called for every statement and expression under a body.
type awkVisitor struct {
	stmt func(awkStmt)
	expr func(awkExpr)
}

func walkAwkBody(body []awkStmt, visit *awkVisitor) {
	for _, statement := range body {
		walkAwkStmt(statement, visit)
	}
}

func walkAwkStmt(statement awkStmt, visit *awkVisitor) {
	if statement == nil {
		return
	}
	if visit.stmt != nil {
		visit.stmt(statement)
	}
	switch node := statement.(type) {
	case awkPrintStmt:
		walkAwkExprs(node.args, visit)
		walkAwkRedirect(node.redirect, visit)
	case awkPrintfStmt:
		walkAwkExprs(node.args, visit)
		walkAwkRedirect(node.redirect, visit)
	case awkExprStmt:
		walkAwkExpr(node.expr, visit)
	case awkBlockStmt:
		walkAwkBody(node.body, visit)
	case awkIfStmt:
		walkAwkExpr(node.condition, visit)
		walkAwkStmt(node.then, visit)
		walkAwkStmt(node.otherwise, visit)
	case awkWhileStmt:
		walkAwkExpr(node.condition, visit)
		walkAwkStmt(node.body, visit)
	case awkDoStmt:
		walkAwkStmt(node.body, visit)
		walkAwkExpr(node.condition, visit)
	case awkForStmt:
		walkAwkStmt(node.initialise, visit)
		walkAwkExpr(node.condition, visit)
		walkAwkStmt(node.step, visit)
		walkAwkStmt(node.body, visit)
	case awkForInStmt:
		walkAwkStmt(node.body, visit)
	case awkDeleteStmt:
		walkAwkExprs(node.index, visit)
	case awkExitStmt:
		walkAwkExpr(node.status, visit)
	case awkReturnStmt:
		walkAwkExpr(node.value, visit)
	}
}

func walkAwkRedirect(redirect *awkRedirect, visit *awkVisitor) {
	if redirect != nil {
		walkAwkExpr(redirect.target, visit)
	}
}

func walkAwkExprs(list []awkExpr, visit *awkVisitor) {
	for _, item := range list {
		walkAwkExpr(item, visit)
	}
}

func walkAwkExpr(expression awkExpr, visit *awkVisitor) {
	if expression == nil {
		return
	}
	if visit.expr != nil {
		visit.expr(expression)
	}
	switch node := expression.(type) {
	case awkFieldExpr:
		walkAwkExpr(node.index, visit)
	case awkIndexExpr:
		walkAwkExprs(node.index, visit)
	case awkBinaryExpr:
		walkAwkExpr(node.left, visit)
		walkAwkExpr(node.right, visit)
	case awkConcatExpr:
		walkAwkExprs(node.parts, visit)
	case awkUnaryExpr:
		walkAwkExpr(node.operand, visit)
	case awkMatchExpr:
		walkAwkExpr(node.left, visit)
		walkAwkExpr(node.right, visit)
	case awkInExpr:
		walkAwkExprs(node.index, visit)
	case awkTernaryExpr:
		walkAwkExpr(node.condition, visit)
		walkAwkExpr(node.yes, visit)
		walkAwkExpr(node.no, visit)
	case awkAssignExpr:
		walkAwkExpr(node.target, visit)
		walkAwkExpr(node.value, visit)
	case awkIncDecExpr:
		walkAwkExpr(node.target, visit)
	case awkCallExpr:
		walkAwkExprs(node.args, visit)
	case awkBuiltinExpr:
		walkAwkExprs(node.args, visit)
	case awkGroupExpr:
		walkAwkExpr(node.inner, visit)
	case awkGroupListExpr:
		walkAwkExprs(node.items, visit)
	case awkGetlineExpr:
		walkAwkExpr(node.target, visit)
		walkAwkExpr(node.source, visit)
	}
}
