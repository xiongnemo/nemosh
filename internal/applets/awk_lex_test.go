package applets

import (
	"strings"
	"testing"
)

// Scanning an awk program, and the two rules that make it harder than it looks.
//
// Both were measured against gawk 5.4.1 and busybox-w32 1.38.0 before the scanner was
// written, and both references agree on every case below except the one divergence named
// at the end.

// kinds renders a token stream compactly, so a case reads as the shape it expects.
func kinds(t *testing.T, source string) string {
	t.Helper()
	tokens, err := scanAwkTokens(source)
	if err != nil {
		t.Fatalf("scan %q: %v", source, err)
	}
	var out []string
	for _, token := range tokens {
		switch token.kind {
		case awkTokenEOF:
		case awkTokenNewline:
			out = append(out, "NL")
		case awkTokenNumber:
			out = append(out, "num("+token.text+")")
		case awkTokenString:
			out = append(out, "str("+token.text+")")
		case awkTokenRegex:
			out = append(out, "re("+token.text+")")
		case awkTokenFuncName:
			out = append(out, "call("+token.text+")")
		case awkTokenBuiltin:
			out = append(out, "builtin("+token.text+")")
		case awkTokenKeyword:
			out = append(out, "kw("+token.text+")")
		case awkTokenName:
			out = append(out, "name("+token.text+")")
		default:
			out = append(out, token.text)
		}
	}
	return strings.Join(out, " ")
}

// The hardest question in an awk lexer: is `/` division or the start of a regex?
//
// The classic ambiguity is the case that settles it. Both references answer 1 for
// `a=4; b=2; print a /b/ 2`, which is `4/2/2` -- so after a **name**, a slash divides.
func TestAwkLexer_slashIsDivisionOrRegex(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
		want   string
	}{
		// Division: the previous token could end an expression.
		{name: "after a name", source: "a/b", want: "name(a) / name(b)"},
		{name: "after a number", source: "1/2", want: "num(1) / num(2)"},
		{name: "after a field's number", source: "$1/2", want: "$ num(1) / num(2)"},
		{name: "after a close paren", source: "(4)/2", want: "( num(4) ) / num(2)"},
		{name: "after a close bracket", source: "a[1]/2", want: "name(a) [ num(1) ] / num(2)"},
		{name: "after a bare builtin", source: "length/2", want: "builtin(length) / num(2)"},
		{name: "the classic ambiguity", source: "a /b/ 2", want: "name(a) / name(b) / num(2)"},

		// A regex: the previous token cannot end an expression.
		{name: "at the start", source: "/x/", want: "re(x)"},
		{name: "after print", source: "print /x/", want: "kw(print) re(x)"},
		{name: "after assignment", source: "x = /y/", want: "name(x) = re(y)"},
		{name: "after a comma", source: "f(1, /y/)", want: "call(f) ( num(1) , re(y) )"},
		{name: "after and", source: "1 && /y/", want: "num(1) && re(y)"},
		{name: "after a match operator", source: "$0 ~ /y/", want: "$ num(0) ~ re(y)"},
		{name: "after an open paren", source: "(/y/)", want: "( re(y) )"},
		// `$` is a prefix, so what follows is an operand: `$/re/` is the field numbered
		// by a match, not a division.
		{name: "after a dollar", source: "$/y/", want: "$ re(y)"},

		// An escaped slash stays inside the pattern rather than ending it.
		{name: "an escaped slash", source: `/a\/b/`, want: "re(a/b)"},
		// Every other backslash is left as written, for the regexp engine to read.
		{name: "a regex keeps its backslashes", source: `/a\.b/`, want: `re(a\.b)`},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := kinds(t, test.source); got != test.want {
				t.Fatalf("%q\n  got  %s\n  want %s", test.source, got, test.want)
			}
		})
	}
}

// A newline ends a statement except after a few tokens, all of them measured.
func TestAwkLexer_newlineRules(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
		want   string
	}{
		// Continues: the newline is swallowed entirely.
		{name: "after an open brace", source: "{\nx", want: "{ name(x)"},
		{name: "after and", source: "1 &&\n2", want: "num(1) && num(2)"},
		{name: "after or", source: "1 ||\n2", want: "num(1) || num(2)"},
		{name: "after a comma", source: "1,\n2", want: "num(1) , num(2)"},
		{name: "after do", source: "do\nx", want: "kw(do) name(x)"},
		{name: "after else", source: "else\nx", want: "kw(else) name(x)"},
		{name: "after a question mark", source: "1 ?\n2 :\n3", want: "num(1) ? num(2) : num(3)"},
		// Runs of blank lines collapse rather than yielding a terminator each.
		{name: "blank lines collapse", source: "x\n\n\ny", want: "name(x) NL name(y)"},

		// Terminates. `(` is the one people expect to continue and which both
		// references refuse.
		{name: "between statements", source: "x\ny", want: "name(x) NL name(y)"},
		{name: "after an open paren", source: "(\n1", want: "( NL num(1)"},
		// After `=` gawk refuses and busybox accepts; POSIX and gawk are followed.
		{name: "after assignment", source: "x =\n1", want: "name(x) = NL num(1)"},

		// A backslash-newline joins lines anywhere.
		{name: "backslash continuation", source: "x \\\ny", want: "name(x) name(y)"},
		{name: "backslash inside a statement", source: "1 +\\\n2", want: "num(1) + num(2)"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := kinds(t, test.source); got != test.want {
				t.Fatalf("%q\n  got  %s\n  want %s", test.source, got, test.want)
			}
		})
	}
}

func TestAwkLexer_literalsAndNames(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
		want   string
	}{
		// Hex is accepted as a *source literal* even though the string "0x10" converts
		// to 0 -- which is gawk's exact split, and both references print 16 here.
		{name: "hex literal", source: "0x10", want: "num(0x10)"},
		{name: "exponent", source: "1e3", want: "num(1e3)"},
		{name: "leading dot", source: ".5", want: "num(.5)"},
		// `.` is not an operator in awk, and a lone one is refused -- see the refusal
		// table. This case asserted the opposite until both references were asked:
		// `print a.b` is a syntax error in each of them.
		{name: "a dot needs a digit", source: ".5e2", want: "num(.5e2)"},

		// A name followed directly by `(` is a call; with a space it is not, and the
		// difference is the whole reason the two are separate kinds.
		{name: "a call", source: "f(1)", want: "call(f) ( num(1) )"},
		{name: "a name and a group", source: "f (1)", want: "name(f) ( num(1) )"},

		{name: "keywords", source: "if else while", want: "kw(if) kw(else) kw(while)"},
		{name: "builtins", source: "length substr", want: "builtin(length) builtin(substr)"},
		{name: "underscores and digits", source: "_a1 b_2", want: "name(_a1) name(b_2)"},

		// Longest-match operators: `>=` is one token, not two.
		{name: "two-character operators", source: ">= != && || ++ -- !~", want: ">= != && || ++ -- !~"},

		// Comments run to the end of the line and leave the terminator behind.
		{name: "a comment", source: "x # ignored\ny", want: "name(x) NL name(y)"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := kinds(t, test.source); got != test.want {
				t.Fatalf("%q\n  got  %s\n  want %s", test.source, got, test.want)
			}
		})
	}
}

// String escapes are decoded by the scanner, so nothing downstream carries a backslash it
// must remember to interpret.
func TestAwkLexer_stringEscapes(t *testing.T) {
	for _, test := range []struct {
		source string
		want   string
	}{
		{source: `"a\tb"`, want: "a\tb"},
		{source: `"a\nb"`, want: "a\nb"},
		{source: `"a\\b"`, want: `a\b`},
		{source: `"a\"b"`, want: `a"b`},
		{source: `"a\/b"`, want: "a/b"},
		// Octal, one to three digits: measured as `a1b`.
		{source: `"a\061b"`, want: "a1b"},
		{source: `"\0"`, want: "\x00"},
		// An unknown escape is the character itself, backslash dropped. busybox does
		// this; gawk warns and then does the same.
		{source: `"a\qb"`, want: "aqb"},
	} {
		t.Run(test.source, func(t *testing.T) {
			tokens, err := scanAwkTokens(test.source)
			if err != nil {
				t.Fatalf("%v", err)
			}
			if tokens[0].kind != awkTokenString || tokens[0].text != test.want {
				t.Fatalf("got %q, want %q", tokens[0].text, test.want)
			}
		})
	}
}

// What the scanner refuses, and refuses by saying where.
func TestAwkLexer_refusals(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
		says   string
	}{
		{name: "unterminated string", source: `"abc`, says: "unterminated string"},
		{name: "newline in a string", source: "\"ab\ncd\"", says: "newline in string"},
		{name: "unterminated regex", source: "x = /abc", says: "unterminated regular expression"},
		{name: "a stray backslash", source: "x \\ y", says: "backslash not followed by a newline"},
		{name: "an unexpected character", source: "x @ y", says: "unexpected character"},
		// `.` is not an operator: both references refuse `print a.b` outright.
		{name: "a lone dot", source: "a.b", says: "unexpected character"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := scanAwkTokens(test.source)
			if err == nil {
				t.Fatalf("%q was accepted", test.source)
			}
			if !strings.Contains(err.Error(), test.says) {
				t.Fatalf("%q said %q, which does not contain %q", test.source, err, test.says)
			}
			// Every refusal names a line, because a program of a hundred lines with an
			// unterminated string is unhelpful without one.
			if !strings.Contains(err.Error(), "line ") {
				t.Errorf("%q does not name a line: %v", test.source, err)
			}
		})
	}
}
