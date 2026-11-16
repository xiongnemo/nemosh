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

// No package-level map literal in this tree's production code. A map is built when its package
// is initialised, before main, on every start, whether or not the run ever reads it: an
// allocation for the map and more for its buckets, for a table of long options one applet in a
// hundred and fifty reads. A switch, or a sorted array the linker lays down, answers the same
// question with no work before main.
func TestPackageInit_noPackageLevelMapLiterals(t *testing.T) {
	root := filepath.Join("..", "..")
	files := token.NewFileSet()
	checked := 0
	for _, tree := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(filepath.Join(root, tree), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if entry.Name() == "testutil" || entry.Name() == "testdata" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			parsed, err := parser.ParseFile(files, path, nil, parser.SkipObjectResolution)
			if err != nil {
				return err
			}
			checked++
			for _, declaration := range parsed.Decls {
				general, ok := declaration.(*ast.GenDecl)
				if !ok || general.Tok != token.VAR {
					continue
				}
				for _, spec := range general.Specs {
					for _, value := range spec.(*ast.ValueSpec).Values {
						if literal, ok := value.(*ast.CompositeLit); ok {
							if _, isMap := literal.Type.(*ast.MapType); isMap {
								relative, _ := filepath.Rel(root, path)
								t.Errorf("%s: a package-level map literal, built before main on every start", files.Position(literal.Pos()).String()[len(path)-len(relative):])
							}
						}
					}
				}
			}
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
