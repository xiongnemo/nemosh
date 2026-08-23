package applets

import (
	"regexp"
	"sort"
	"strings"
	"sync"
)

// The rule tables, and how a file is matched to one.
//
// Built lazily behind a sync.Once. Eleven languages of compiled regular expressions at
// package init would be work before main for a shell that mostly never opens the
// editor -- and the init allocation ceiling is at 2750 against 2545 already used, with
// the whole reason this engine exists rather than micro's being that yaml.v2 spent 262
// of them. Compiling on first use costs the editor's first keystroke and nothing else.

// highlightHelpers, so a table reads as rules rather than as regexp syntax.
//
// Every expression is anchored with `^` because the scanner asks "does this match
// starting exactly here" -- see matchAt, which enforces it.

// lineComment matches an introducer and the rest of the line.
//
// A **pattern**, not a region, and that is the one structural thing to know about these
// tables. As a region ending at `^$` it never closed -- the scan stops at the end of the
// line before an empty match can happen -- so the comment carried into every following
// line. Measured before the tables were written, which is why none of them has it.
func lineComment(introducer string, group highlightGroup) highlightPattern {
	return highlightPattern{match: regexp.MustCompile(`^` + regexp.QuoteMeta(introducer) + `.*`), group: group}
}

// blockComment spans lines. nested is for Haskell, whose `{- {- -} -}` needs two
// closers, and against C, whose `/* /* */` needs one.
func blockComment(open, close string, nested bool) highlightRegion {
	return highlightRegion{
		start:  regexp.MustCompile(`^` + regexp.QuoteMeta(open)),
		end:    regexp.MustCompile(`^` + regexp.QuoteMeta(close)),
		nested: nested,
		group:  groupComment,
	}
}

// quoted is a string delimited by the same character at both ends, with backslash
// escapes if escaped -- without which `"a\""` ends at the middle quote.
func quoted(delimiter string, escaped bool) highlightRegion {
	region := highlightRegion{
		start: regexp.MustCompile(`^` + regexp.QuoteMeta(delimiter)),
		end:   regexp.MustCompile(`^` + regexp.QuoteMeta(delimiter)),
		group: groupString,
	}
	if escaped {
		region.skip = regexp.MustCompile(`^\\.`)
	}
	return region
}

// words is a keyword list. wordOnly is set because the scanner slices the line, so a
// leading word boundary in the expression would always succeed and `int` would match
// inside `print`.
func words(group highlightGroup, list ...string) highlightPattern {
	quoted := make([]string, len(list))
	for index, word := range list {
		quoted[index] = regexp.QuoteMeta(word)
	}
	return highlightPattern{
		match:    regexp.MustCompile(`^(?:` + strings.Join(quoted, "|") + `)\b`),
		group:    group,
		wordOnly: true,
	}
}

// symbols is an operator or punctuation list, which needs no word boundary because
// none of it is a word character.
func symbols(list ...string) highlightPattern {
	quoted := make([]string, len(list))
	for index, symbol := range list {
		quoted[index] = regexp.QuoteMeta(symbol)
	}
	return highlightPattern{
		match: regexp.MustCompile(`^(?:` + strings.Join(quoted, "|") + `)`),
		group: groupSymbol,
	}
}

// expr is a rule whose shape a list cannot express: a number format, an identifier
// convention.
func expr(pattern string, group highlightGroup, wordOnly bool) highlightPattern {
	return highlightPattern{match: regexp.MustCompile(pattern), group: group, wordOnly: wordOnly}
}

// numbers is the shape most of these languages share: decimal, hex, and a float with
// an optional exponent. Listed longest-first, because `0x1f` must not be read as `0`
// followed by an identifier.
func numbers() highlightPattern {
	return expr(`^(?:0[xX][0-9a-fA-F]+|0[bB][01]+|[0-9]+\.[0-9]*(?:[eE][-+]?[0-9]+)?|[0-9]+(?:[eE][-+]?[0-9]+)?)`,
		groupNumber, true)
}

var (
	highlightOnce     sync.Once
	highlightSyntaxes []*highlightSyntax
	highlightByName   map[string]*highlightSyntax
)

func highlightSyntaxList() []*highlightSyntax {
	highlightOnce.Do(func() {
		highlightSyntaxes = []*highlightSyntax{
			goSyntax(), cSyntax(), cppSyntax(), pythonSyntax(), shellSyntax(),
			haskellSyntax(), prologSyntax(),
			jsonSyntax(), yamlSyntax(), tomlSyntax(), markdownSyntax(),
		}
		highlightByName = make(map[string]*highlightSyntax, len(highlightSyntaxes))
		for _, syntax := range highlightSyntaxes {
			highlightByName[syntax.name] = syntax
		}
	})
	return highlightSyntaxes
}

// highlightSyntaxFor picks a language from a file name, or nil.
//
// Whole names are tried before extensions, because `Makefile` has no extension and a
// file called `Makefile.old` should still be a makefile. Extensions are tried
// **longest first**, so a two-part extension cannot lose to its own tail -- the case
// that matters here is `.tar.gz` against `.gz`, and it costs nothing to be right about
// it now rather than after adding a language where it bites.
func highlightSyntaxFor(name string) *highlightSyntax {
	if name == "" {
		return nil
	}
	lower := strings.ToLower(name)
	if index := strings.LastIndexAny(lower, `/\`); index >= 0 {
		lower = lower[index+1:]
	}
	for _, syntax := range highlightSyntaxList() {
		for _, whole := range syntax.filenames {
			if lower == whole || strings.HasPrefix(lower, whole+".") {
				return syntax
			}
		}
	}
	best, bestLength := (*highlightSyntax)(nil), 0
	for _, syntax := range highlightSyntaxList() {
		for _, extension := range syntax.extensions {
			if strings.HasSuffix(lower, extension) && len(extension) > bestLength {
				best, bestLength = syntax, len(extension)
			}
		}
	}
	return best
}

// highlightLanguageNames is the list `-H` prints, generated from the tables so it
// cannot claim a language that has no rules -- the same property the key list has.
func highlightLanguageNames() []string {
	syntaxes := highlightSyntaxList()
	names := make([]string, 0, len(syntaxes))
	for _, syntax := range syntaxes {
		names = append(names, syntax.name)
	}
	sort.Strings(names)
	return names
}
