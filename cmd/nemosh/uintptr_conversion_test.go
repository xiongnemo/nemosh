package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// A pointer made a uintptr is no pointer to the runtime from then on: the stack it pointed into
// may be moved and what it pointed at collected, and nothing updates the number. The one place
// the conversion keeps what it points at alive and in place is the argument list of the call
// that hands it to Windows -- syscall's functions and LazyProc.Call are marked uintptrescapes for
// exactly that -- and iconv's default-character flag, converted a statement before its call, was
// written by Windows into a stack that had moved under it. So in this tree's production code
// uintptr(unsafe.Pointer(...)) is a call's argument and nothing else.
func TestUnsafe_aPointerBecomesAUintptrOnlyAsACallsArgument(t *testing.T) {
	root := filepath.Join("..", "..")
	files := token.NewFileSet()
	checked := 0
	for _, tree := range []string{"cmd", "internal", "tools"} {
		err := filepath.WalkDir(filepath.Join(root, tree), func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			parsed, err := parser.ParseFile(files, path, nil, parser.SkipObjectResolution)
			if err != nil {
				return err
			}
			checked++
			var parents []ast.Node
			ast.Inspect(parsed, func(node ast.Node) bool {
				if node == nil {
					parents = parents[:len(parents)-1]
					return false
				}
				if isPointerToUintptr(node) && !isCallArgument(node, parents) {
					t.Errorf("%s: a pointer made a uintptr outside the arguments of the call that uses it", files.Position(node.Pos()))
				}
				parents = append(parents, node)
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if checked < 500 {
		t.Fatalf("only %d files were checked, so this proves nothing", checked)
	}
}

// isPointerToUintptr is uintptr(unsafe.Pointer(...)).
func isPointerToUintptr(node ast.Node) bool {
	outer, ok := node.(*ast.CallExpr)
	if !ok || len(outer.Args) != 1 {
		return false
	}
	if name, ok := outer.Fun.(*ast.Ident); !ok || name.Name != "uintptr" {
		return false
	}
	inner, ok := outer.Args[0].(*ast.CallExpr)
	if !ok {
		return false
	}
	selector, ok := inner.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Pointer" {
		return false
	}
	pkg, ok := selector.X.(*ast.Ident)
	return ok && pkg.Name == "unsafe"
}

// isCallArgument is whether node is one of the arguments of the call it is in.
func isCallArgument(node ast.Node, parents []ast.Node) bool {
	if len(parents) == 0 {
		return false
	}
	call, ok := parents[len(parents)-1].(*ast.CallExpr)
	if !ok {
		return false
	}
	for _, argument := range call.Args {
		if argument == node {
			return true
		}
	}
	return false
}
