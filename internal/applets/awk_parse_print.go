package applets

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Rendering a parsed program back as fully parenthesised text.
//
// This exists for the tests, and it earns its place there: precedence is invisible in a
// tree, so asserting `1 " " -1` parses correctly means asserting it comes back as
// `concat(1, (" " - 1))` rather than `concat(1, " ", -1)`. A reprinter turns every
// precedence question into a string comparison, which is the only way to write those
// cases so that a reader can see what they claim.
//
// Not a pretty-printer: it never omits a parenthesis, because the whole point is to show
// the shape the parser chose.

func awkReprintExpr(expr awkExpr) string {
	switch node := expr.(type) {
	case awkNumberExpr:
		return formatAwkNumber(node.value, "%.6g")
	case awkStringExpr:
		return strconv.Quote(node.value)
	case awkRegexExpr:
		return "/" + node.pattern + "/"
	case awkVarExpr:
		return node.name
	case awkFieldExpr:
		return "$(" + awkReprintExpr(node.index) + ")"
	case awkIndexExpr:
		return node.name + "[" + awkReprintList(node.index) + "]"
	case awkBinaryExpr:
		return "(" + awkReprintExpr(node.left) + " " + node.operator + " " + awkReprintExpr(node.right) + ")"
	case awkConcatExpr:
		parts := make([]string, 0, len(node.parts))
		for _, part := range node.parts {
			parts = append(parts, awkReprintExpr(part))
		}
		return "concat(" + strings.Join(parts, ", ") + ")"
	case awkUnaryExpr:
		return "(" + node.operator + awkReprintExpr(node.operand) + ")"
	case awkMatchExpr:
		operator := "~"
		if node.negated {
			operator = "!~"
		}
		return "(" + awkReprintExpr(node.left) + " " + operator + " " + awkReprintExpr(node.right) + ")"
	case awkInExpr:
		return "((" + awkReprintList(node.index) + ") in " + node.array + ")"
	case awkTernaryExpr:
		return "(" + awkReprintExpr(node.condition) + " ? " + awkReprintExpr(node.yes) +
			" : " + awkReprintExpr(node.no) + ")"
	case awkAssignExpr:
		return "(" + awkReprintExpr(node.target) + " " + node.operator + " " + awkReprintExpr(node.value) + ")"
	case awkIncDecExpr:
		if node.prefix {
			return "(" + node.operator + awkReprintExpr(node.target) + ")"
		}
		return "(" + awkReprintExpr(node.target) + node.operator + ")"
	case awkCallExpr:
		return node.name + "(" + awkReprintList(node.args) + ")"
	case awkBuiltinExpr:
		return node.name + "(" + awkReprintList(node.args) + ")"
	case awkGroupExpr:
		return "group(" + awkReprintExpr(node.inner) + ")"
	case awkGetlineExpr:
		return awkReprintGetline(node)
	case nil:
		return "<nil>"
	}
	return fmt.Sprintf("<unknown %T>", expr)
}

func awkReprintGetline(node awkGetlineExpr) string {
	out := "getline"
	if node.target != nil {
		out += " " + awkReprintExpr(node.target)
	}
	switch node.mode {
	case awkGetlineFile:
		out += " < " + awkReprintExpr(node.source)
	case awkGetlineCommand:
		out = awkReprintExpr(node.source) + " | " + out
	}
	return "(" + out + ")"
}

func awkReprintList(items []awkExpr) string {
	parts := make([]string, 0, len(items))
	for _, item := range items {
		parts = append(parts, awkReprintExpr(item))
	}
	return strings.Join(parts, ", ")
}

func awkReprintStmt(statement awkStmt) string {
	switch node := statement.(type) {
	case awkPrintStmt:
		return "print(" + awkReprintList(node.args) + ")" + awkReprintRedirect(node.redirect)
	case awkPrintfStmt:
		return "printf(" + awkReprintList(node.args) + ")" + awkReprintRedirect(node.redirect)
	case awkExprStmt:
		return awkReprintExpr(node.expr)
	case awkBlockStmt:
		return "{" + awkReprintStmts(node.body) + "}"
	case awkIfStmt:
		out := "if " + awkReprintExpr(node.condition) + " " + awkReprintStmt(node.then)
		if node.otherwise != nil {
			out += " else " + awkReprintStmt(node.otherwise)
		}
		return out
	case awkWhileStmt:
		return "while " + awkReprintExpr(node.condition) + " " + awkReprintStmt(node.body)
	case awkDoStmt:
		return "do " + awkReprintStmt(node.body) + " while " + awkReprintExpr(node.condition)
	case awkForStmt:
		return "for (" + awkReprintOptionalStmt(node.initialise) + "; " +
			awkReprintOptionalExpr(node.condition) + "; " +
			awkReprintOptionalStmt(node.step) + ") " + awkReprintStmt(node.body)
	case awkForInStmt:
		return "for (" + node.name + " in " + node.array + ") " + awkReprintStmt(node.body)
	case awkDeleteStmt:
		if len(node.index) == 0 {
			return "delete " + node.name
		}
		return "delete " + node.name + "[" + awkReprintList(node.index) + "]"
	case awkNextStmt:
		return "next"
	case awkNextFileStmt:
		return "nextfile"
	case awkBreakStmt:
		return "break"
	case awkContinueStmt:
		return "continue"
	case awkExitStmt:
		return "exit " + awkReprintOptionalExpr(node.status)
	case awkReturnStmt:
		return "return " + awkReprintOptionalExpr(node.value)
	case nil:
		return ""
	}
	return fmt.Sprintf("<unknown %T>", statement)
}

func awkReprintRedirect(redirect *awkRedirect) string {
	if redirect == nil {
		return ""
	}
	return " " + redirect.operator + " " + awkReprintExpr(redirect.target)
}

func awkReprintOptionalExpr(expr awkExpr) string {
	if expr == nil {
		return ""
	}
	return awkReprintExpr(expr)
}

func awkReprintOptionalStmt(statement awkStmt) string {
	if statement == nil {
		return ""
	}
	return awkReprintStmt(statement)
}

func awkReprintStmts(body []awkStmt) string {
	parts := make([]string, 0, len(body))
	for _, statement := range body {
		parts = append(parts, awkReprintStmt(statement))
	}
	return strings.Join(parts, "; ")
}

// awkReprintProgram renders a whole program, items in order and functions after them.
func awkReprintProgram(program *awkProgram) string {
	parts := make([]string, 0, len(program.items))
	for _, item := range program.items {
		parts = append(parts, awkReprintItem(item))
	}
	names := make([]string, 0, len(program.functions))
	for name := range program.functions {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		function := program.functions[name]
		parts = append(parts, "function "+name+"("+strings.Join(function.params, ", ")+") {"+
			awkReprintStmts(function.body)+"}")
	}
	return strings.Join(parts, " | ")
}

func awkReprintItem(item awkItem) string {
	action := "{" + awkReprintStmts(item.action) + "}"
	if item.action == nil {
		// A nil action is not an empty one: it means the rule wrote no braces and so
		// means `{ print }`.
		action = "<default>"
	}
	switch item.kind {
	case awkItemBegin:
		return "BEGIN " + action
	case awkItemEnd:
		return "END " + action
	case awkItemAlways:
		return action
	case awkItemRange:
		return awkReprintExpr(item.pattern) + ", " + awkReprintExpr(item.until) + " " + action
	}
	return awkReprintExpr(item.pattern) + " " + action
}
