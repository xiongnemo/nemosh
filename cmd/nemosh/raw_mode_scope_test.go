package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// **Raw mode is borrowed for one line read, never held across a command.**
//
// It used to be entered when the session started and put back with a `defer` that ran when
// the session *ended*, so every command in between ran with the terminal still raw. From the
// prompt, `bc` then showed nothing as it was typed, never saw a line at all -- Enter arrives
// as a carriage return and there is no line discipline to turn it into one -- and could not
// be interrupted, because os.Interrupt on Windows is delivered by the console only while
// ENABLE_PROCESSED_INPUT is set, which raw mode clears. Three symptoms, one cause.
//
// The first version of this guard checked *where* enterRawMode is called from, and passed
// against the broken code: the call site never moved, only the lifetime did. So it checks the
// lifetime instead. `runInteractive` still asks whether the terminal will go raw at all --
// that is how it chooses the edited session over the cooked one -- and must hand it straight
// back rather than deferring, because everything the session does happens inside the call it
// makes next.

// TestRawMode_isNotHeldAcrossTheSession fails if the probe's restore is deferred.
func TestRawMode_isNotHeldAcrossTheSession(t *testing.T) {
	parsed, err := parser.ParseFile(token.NewFileSet(), filepath.Join(".", "session.go"), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	ast.Inspect(parsed, func(node ast.Node) bool {
		function, ok := node.(*ast.FuncDecl)
		if !ok || function.Name.Name != "runInteractive" {
			return true
		}
		found = true
		ast.Inspect(function.Body, func(inner ast.Node) bool {
			deferred, ok := inner.(*ast.DeferStmt)
			if !ok {
				return true
			}
			if selector, ok := deferred.Call.Fun.(*ast.SelectorExpr); ok && selector.Sel.Name == "restore" {
				t.Error("runInteractive defers the terminal's restore, so raw mode lasts as long " +
					"as the session and every command runs inside it; restore the probe at once")
			}
			return true
		})
		return false
	})
	if !found {
		t.Fatal("runInteractive was not found in session.go, so this guard checked nothing")
	}
}

// TestRawMode_isEnteredOnlyAroundReadingALine keeps the borrow in the one helper that gives
// it back, so a new caller cannot widen the lifetime by accident.
func TestRawMode_isEnteredOnlyAroundReadingALine(t *testing.T) {
	allowed := map[string]bool{"readLineInRawMode": true, "runInteractive": true}
	fileSet := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(fileSet, filepath.Join(".", name), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		checked++
		ast.Inspect(parsed, func(node ast.Node) bool {
			function, ok := node.(*ast.FuncDecl)
			if !ok {
				return true
			}
			ast.Inspect(function.Body, func(inner ast.Node) bool {
				call, ok := inner.(*ast.CallExpr)
				if !ok {
					return true
				}
				if identifier, ok := call.Fun.(*ast.Ident); ok && identifier.Name == "enterRawMode" &&
					!allowed[function.Name.Name] {
					t.Errorf("%s:%s calls enterRawMode; raw mode belongs around one line read, "+
						"so a command run between two prompts finds the terminal it expects",
						name, function.Name.Name)
				}
				return true
			})
			return false
		})
	}
	// The scan has to have looked at something: a glob that matched nothing would pass in
	// silence, which is a failure this project has paid for before.
	if checked < 10 {
		t.Fatalf("only %d files were scanned, so this proves nothing", checked)
	}
}
