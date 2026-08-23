package applets

import (
	"strings"
	"testing"
)

// The eleven tables, against real source rather than from memory.
//
// The engine's own tests use a made-up language so a failure there is a scanning bug.
// These are the opposite: the engine is assumed to work and what is checked is whether
// each table says the right thing about text somebody would actually open.
//
// Every case names a *token* and what it should be, rather than drawing a whole line,
// because a table changes when a keyword is added and a whole-line expectation would
// then need rewriting for a change that broke nothing.

// groupOf answers what group covers the first byte of the first occurrence of needle,
// which is how a case says "in this line, that token is a string".
func groupOf(t *testing.T, syntax *highlightSyntax, line, needle string) highlightGroup {
	t.Helper()
	at := strings.Index(line, needle)
	if at < 0 {
		t.Fatalf("the fixture %q does not contain %q", line, needle)
	}
	spans, _ := highlightLine(syntax, line, nil)
	for _, span := range spans {
		if at >= span.start && at < span.end {
			return span.group
		}
	}
	return groupNone
}

func TestHighlightSyntax_eachLanguageOnRealSource(t *testing.T) {
	for _, test := range []struct {
		language string
		line     string
		token    string
		want     highlightGroup
	}{
		// Go.
		{language: "go", line: `func main() {`, token: "func", want: groupKeyword},
		{language: "go", line: `var x int = 42`, token: "int", want: groupType},
		{language: "go", line: `var x int = 42`, token: "42", want: groupNumber},
		{language: "go", line: `s := "hello"`, token: `"hello"`, want: groupString},
		{language: "go", line: `x := 1 // why`, token: "// why", want: groupComment},
		{language: "go", line: "s := `raw \\n`", token: "`raw", want: groupString},
		{language: "go", line: `r := 'x'`, token: `'x'`, want: groupString},
		// The word that contains a keyword is not one.
		{language: "go", line: `refunction()`, token: "refunction", want: groupNone},

		// C, and its preprocessor line, which was the shape most likely to be wrong.
		{language: "c", line: `#include <stdio.h>`, token: "#include", want: groupKeyword},
		{language: "c", line: `int main(void) {`, token: "int", want: groupType},
		{language: "c", line: `unsigned long x = 0xFF;`, token: "0xFF", want: groupNumber},
		{language: "c", line: `char c = '\n';`, token: `'\n'`, want: groupString},
		{language: "c", line: `/* note */ int x;`, token: "/* note */", want: groupComment},

		// C++, whose list is C's plus its own.
		{language: "c++", line: `class Tree {`, token: "class", want: groupKeyword},
		{language: "c++", line: `std::unique_ptr<int> p;`, token: "unique_ptr", want: groupType},
		{language: "c++", line: `auto f = [](int x) { return x; };`, token: "return", want: groupKeyword},
		{language: "c++", line: `if (p == nullptr) {`, token: "nullptr", want: groupKeyword},

		// Python, including the triple-quoted string that a shorter rule would misread.
		{language: "python", line: `def f(x):`, token: "def", want: groupKeyword},
		{language: "python", line: `"""a docstring"""`, token: `"""a`, want: groupString},
		{language: "python", line: `x = None`, token: "None", want: groupType},
		{language: "python", line: `@property`, token: "@property", want: groupKeyword},
		{language: "python", line: `# a comment`, token: "# a comment", want: groupComment},

		// Shell: the two quote kinds differ in whether a backslash escapes.
		{language: "shell", line: `if [ -f "$HOME/x" ]; then`, token: "if", want: groupKeyword},
		{language: "shell", line: `echo "$PATH"`, token: "echo", want: groupType},
		{language: "shell", line: `x=$HOME`, token: "$HOME", want: groupType},
		{language: "shell", line: `x=${HOME:-/tmp}`, token: "${HOME:-/tmp}", want: groupType},
		{language: "shell", line: `# a comment`, token: "# a comment", want: groupComment},

		// Haskell: the pragma is not a comment, and a primed identifier is not a string.
		{language: "haskell", line: `{-# LANGUAGE OverloadedStrings #-}`, token: "{-#", want: groupKeyword},
		{language: "haskell", line: `data Tree a = Leaf`, token: "data", want: groupKeyword},
		{language: "haskell", line: `data Tree a = Leaf`, token: "Tree", want: groupType},
		{language: "haskell", line: `insert x t@(Node l v r)`, token: "Node", want: groupType},
		{language: "haskell", line: `f x' = x'`, token: "x'", want: groupNone},
		{language: "haskell", line: `main = print 'c'`, token: `'c'`, want: groupString},
		{language: "haskell", line: `-- a comment`, token: "-- a comment", want: groupComment},

		// Prolog: the character-code notation, which is the trap in this language.
		{language: "prolog", line: `% a comment`, token: "% a comment", want: groupComment},
		{language: "prolog", line: `ancestor(X, Y) :- parent(X, Y).`, token: "X", want: groupType},
		{language: "prolog", line: `X = 0'a, write(X).`, token: `0'a`, want: groupNumber},
		{language: "prolog", line: `Y = 0'\n, write(Y).`, token: `0'\n`, want: groupNumber},
		{language: "prolog", line: `format("hi ~w", ['world']).`, token: `'world'`, want: groupString},
		{language: "prolog", line: `:- module(demo, []).`, token: ":-", want: groupSymbol},

		// JSON.
		{language: "json", line: `{"key": "value"}`, token: `"key"`, want: groupString},
		{language: "json", line: `{"n": -1.5e3}`, token: "-1.5e3", want: groupNumber},
		{language: "json", line: `{"ok": true}`, token: "true", want: groupType},

		// YAML: the key is what makes a file readable at a glance.
		{language: "yaml", line: `name: value`, token: "name", want: groupKeyword},
		{language: "yaml", line: `  nested: 1`, token: "nested", want: groupKeyword},
		{language: "yaml", line: `# a comment`, token: "# a comment", want: groupComment},
		{language: "yaml", line: `ref: *anchor`, token: "*anchor", want: groupType},
		{language: "yaml", line: `---`, token: "---", want: groupKeyword},

		// TOML: a table header, a key, and a date the number rule would have broken up.
		{language: "toml", line: `[package]`, token: "[package]", want: groupKeyword},
		{language: "toml", line: `[[bin]]`, token: "[[bin]]", want: groupKeyword},
		{language: "toml", line: `name = "x"`, token: "name", want: groupType},
		{language: "toml", line: `when = 2026-08-24T10:00:00Z`, token: "2026-08-24T10:00:00Z", want: groupNumber},
		{language: "toml", line: `path = 'C:\Users\x'`, token: `'C:\Users\x'`, want: groupString},

		// Markdown, where nearly everything is line-leading.
		{language: "markdown", line: `# Heading`, token: "# Heading", want: groupKeyword},
		{language: "markdown", line: `### Deeper`, token: "### Deeper", want: groupKeyword},
		{language: "markdown", line: `> quoted`, token: ">", want: groupComment},
		{language: "markdown", line: `- an item`, token: "- ", want: groupSymbol},
		{language: "markdown", line: "use `code` here", token: "`code`", want: groupString},
		{language: "markdown", line: `**bold** text`, token: "**bold**", want: groupType},
		{language: "markdown", line: `see [text](http://x)`, token: "[text](http://x)", want: groupSymbol},
	} {
		t.Run(test.language+" "+test.token, func(t *testing.T) {
			syntax := highlightByNameOrFail(t, test.language)
			if got := groupOf(t, syntax, test.line, test.token); got != test.want {
				t.Fatalf("in %s, %q of %q is group %d, want %d\n  %s\n  %s",
					test.language, test.token, test.line, got, test.want,
					test.line, render(test.line, mustSpans(syntax, test.line)))
			}
		})
	}
}

// A keyword inside a comment or a string is not a keyword, in every language. One loop
// rather than a case per language, because it is the same property eleven times and a
// table that got it wrong would be a table that put its comment rule in the wrong place.
func TestHighlightSyntax_noKeywordsInsideCommentsOrStrings(t *testing.T) {
	for _, test := range []struct{ language, line, token string }{
		{language: "go", line: `// func main`, token: "func"},
		{language: "go", line: `s := "func main"`, token: "func"},
		{language: "c", line: `/* int x */`, token: "int"},
		{language: "c", line: `char *s = "int x";`, token: "int"},
		{language: "c++", line: `// class X`, token: "class"},
		{language: "python", line: `# def f`, token: "def"},
		{language: "python", line: `s = "def f"`, token: "def"},
		{language: "shell", line: `# if then`, token: "if"},
		{language: "shell", line: `echo "if then"`, token: "if"},
		{language: "haskell", line: `-- data Tree`, token: "data"},
		{language: "haskell", line: `s = "data Tree"`, token: "data"},
		{language: "prolog", line: `% module foo`, token: "module"},
		{language: "yaml", line: `# key: value`, token: "key"},
		{language: "toml", line: `# [table]`, token: "[table]"},
	} {
		t.Run(test.language+" "+test.token, func(t *testing.T) {
			syntax := highlightByNameOrFail(t, test.language)
			got := groupOf(t, syntax, test.line, test.token)
			if got == groupKeyword || got == groupType {
				t.Fatalf("in %s, %q inside %q was highlighted as code (group %d)\n  %s\n  %s",
					test.language, test.token, test.line, got,
					test.line, render(test.line, mustSpans(syntax, test.line)))
			}
		})
	}
}

// Detection: which table a file name gets.
func TestHighlightSyntaxFor_matchesByNameAndExtension(t *testing.T) {
	for _, test := range []struct{ name, want string }{
		{name: "main.go", want: "go"},
		{name: "a.c", want: "c"},
		{name: "a.h", want: "c"},
		{name: "a.cpp", want: "c++"},
		{name: "a.hpp", want: "c++"},
		{name: "a.py", want: "python"},
		{name: "a.sh", want: "shell"},
		{name: "a.hs", want: "haskell"},
		{name: "a.pl", want: "prolog"},
		{name: "a.json", want: "json"},
		{name: "a.yaml", want: "yaml"},
		{name: "a.yml", want: "yaml"},
		{name: "a.toml", want: "toml"},
		{name: "README.md", want: "markdown"},
		// A path, not just a name: only the last element is looked at.
		{name: `C:\Users\nemo\project\main.go`, want: "go"},
		{name: "/home/x/main.go", want: "go"},
		// Case does not matter.
		{name: "MAIN.GO", want: "go"},
		{name: "Cargo.TOML", want: "toml"},
		// Whole names, which have no useful extension.
		{name: ".bashrc", want: "shell"},
		{name: ".zshrc", want: "shell"},
		// And nothing recognised is no highlighting rather than a guess.
		{name: "notes.txt", want: ""},
		{name: "a.rs", want: ""},
		{name: "Makefile", want: ""},
		{name: "", want: ""},
		{name: "noextension", want: ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			syntax := highlightSyntaxFor(test.name)
			if test.want == "" {
				if syntax != nil {
					t.Fatalf("%q matched %q, want no highlighting", test.name, syntax.name)
				}
				return
			}
			if syntax == nil {
				t.Fatalf("%q matched nothing, want %q", test.name, test.want)
			}
			if syntax.name != test.want {
				t.Fatalf("%q matched %q, want %q", test.name, syntax.name, test.want)
			}
		})
	}
}

// Every table is reachable by at least one name, so a language cannot be written and
// then never selected -- which is the failure mode of a detection list maintained
// separately from the tables.
func TestHighlightSyntax_everyLanguageIsReachable(t *testing.T) {
	for _, syntax := range highlightSyntaxList() {
		reached := false
		for _, extension := range syntax.extensions {
			if found := highlightSyntaxFor("file" + extension); found != nil && found.name == syntax.name {
				reached = true
				break
			}
		}
		if !reached {
			for _, whole := range syntax.filenames {
				if found := highlightSyntaxFor(whole); found != nil && found.name == syntax.name {
					reached = true
					break
				}
			}
		}
		if !reached {
			t.Errorf("no file name selects the %q table, so it can never be used", syntax.name)
		}
	}
	if got := len(highlightSyntaxList()); got != 11 {
		t.Errorf("there are %d tables, want the 11 the scope asked for", got)
	}
}

func highlightByNameOrFail(t *testing.T, name string) *highlightSyntax {
	t.Helper()
	highlightSyntaxList()
	syntax := highlightByName[name]
	if syntax == nil {
		t.Fatalf("no syntax table named %q", name)
	}
	return syntax
}

func mustSpans(syntax *highlightSyntax, line string) []highlightSpan {
	spans, _ := highlightLine(syntax, line, nil)
	return spans
}
