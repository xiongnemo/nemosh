package applets

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// **Ask the stream, not the value.**
//
// The shell hands an applet wrapped streams -- a descriptorWriter for output, a
// contextReader for input -- so `stdout.(*os.File)` is false for every applet run from a
// prompt. An applet that asks the value directly is not asking a harder question, it is
// getting a wrong answer, and always the same wrong answer, and only inside the shell.
//
// This has now cost five bugs, each found by hand rather than by a test:
//
//   - `ls` laid out no columns on a terminal, and `--color=auto` coloured nothing. Fixed
//     by TerminalFile on descriptorWriter, which is where fd_stream.go's comment comes
//     from.
//   - synchronizedWriter then had to forward it, because the chain is two hops deep even
//     for the top-level shell -- which is why terminalFileOf walks rather than checks.
//   - `top` never drew: it asks for the console so tcell can read keys, and the wrapper
//     did not know how to pass that on. LeaseStdinFile exists for it.
//   - Ctrl-C could not reach an applet blocked on a read, because the cancellation was
//     put in at the wrong hop and the chain ended somewhere else entirely.
//   - `less` never paged, and `[ -t 1 ]` never said yes on a real terminal.
//
// Five is enough to stop writing comments about it. Every assertion to *os.File in this
// package is listed below with the reason it is allowed, and the only allowed reason is
// "this is the function that does the walking".
//
// The helpers to use instead: stdoutFile for a writer, leaseTopStdin for standard input.

func TestStreams_areAskedThroughTheHelpersRatherThanAsserted(t *testing.T) {
	allowed := map[string]string{
		"LeaseStdinFile":   "the walker for standard input: it is what the others call",
		"stdoutFile":       "the walker for a writer, and the first hop of it",
		"leaseTopStdin":    "borrows the console for tcell, and walks to it",
		"readWithContext":  "needs the file itself to cancel a read that has already blocked",
		"openProcessInput": "opens an operand, which really is a file rather than a stream",
	}

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fileSet := token.NewFileSet()
	checked := 0
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(fileSet, name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		checked++
		for _, declaration := range parsed.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok {
				continue
			}
			ast.Inspect(function.Body, func(node ast.Node) bool {
				assertion, ok := node.(*ast.TypeAssertExpr)
				if !ok || !isOsFileType(assertion.Type) {
					return true
				}
				if _, permitted := allowed[function.Name.Name]; permitted {
					return true
				}
				t.Errorf("%s:%s asserts a stream to *os.File. Inside the shell that is false "+
					"for every applet, so the answer is not 'no', it is wrong -- ask stdoutFile "+
					"for a writer or leaseTopStdin for standard input. See fd_stream.go; this "+
					"has cost five bugs so far.", name, function.Name.Name)
				return true
			})
		}
	}
	// A scan that matched nothing would pass in silence, which is a failure this project
	// has paid for before.
	if checked < 50 {
		t.Fatalf("only %d files were scanned, so this proves nothing", checked)
	}
}

// isOsFileType reports whether an expression is the type *os.File.
func isOsFileType(expression ast.Expr) bool {
	pointer, ok := expression.(*ast.StarExpr)
	if !ok {
		return false
	}
	selector, ok := pointer.X.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "File" {
		return false
	}
	identifier, ok := selector.X.(*ast.Ident)
	return ok && identifier.Name == "os"
}
