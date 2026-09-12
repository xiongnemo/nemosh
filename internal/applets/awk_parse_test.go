package applets

import (
	"strings"
	"testing"
)

// The parser, asserted through a fully parenthesised reprint.
//
// Precedence is invisible in a tree, so every case below is written as the *shape* the
// parser must produce. `1 " " -1` is the one that catches everybody: it is `1` joined to
// `(" " - 1)`, printing `1-1`, and not three concatenated parts printing `1 -1`. Both
// references agree, and without a reprinter that claim cannot be written down legibly.
//
// Every rung of the ladder was measured against gawk 5.4.1 and busybox-w32 1.38.0 before
// the parser was written; see awk_parse_expr.go for the four places they disagree.

func reprint(t *testing.T, source string) string {
	t.Helper()
	program, err := parseAwkProgram(source)
	if err != nil {
		t.Fatalf("parse %q: %v", source, err)
	}
	return awkReprintProgram(program)
}

// reprintExpr wraps a bare expression in the smallest program that can hold one.
func reprintExpr(t *testing.T, source string) string {
	t.Helper()
	whole := reprint(t, "BEGIN { x = "+source+" }")
	const prefix, suffix = "BEGIN {(x = ", ")}"
	return strings.TrimSuffix(strings.TrimPrefix(whole, prefix), suffix)
}

func TestAwkParser_precedence(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
		want   string
	}{
		// Arithmetic. `^` binds tighter than unary minus, so `-2^2` is -4.
		{name: "unary minus below power", source: "-2^2", want: "(-(2 ^ 2))"},
		// Right-associative: 512, gawk's answer. busybox says 64 and is wrong.
		{name: "power is right associative", source: "2^3^2", want: "(2 ^ (3 ^ 2))"},
		{name: "power takes a unary exponent", source: "2^-1", want: "(2 ^ (-1))"},
		{name: "star before plus", source: "1+2*3", want: "(1 + (2 * 3))"},
		{name: "plus is left associative", source: "1-2-3", want: "((1 - 2) - 3)"},

		// Concatenation: tighter than comparison, looser than additive.
		{name: "concatenation", source: `1 " " 2`, want: `concat(1, " ", 2)`},
		{name: "concat binds tighter than comparison", source: "1 2 < 3", want: "(concat(1, 2) < 3)"},
		// The surprising one: the `-` is binary, so `" "` is its left operand.
		{name: "binary minus beats concatenation", source: `1 " " -1`, want: `concat(1, (" " - 1))`},
		{name: "and the same with a leading minus", source: `-1 " " -1`, want: `concat((-1), (" " - 1))`},
		{name: "prefix increment does concatenate", source: `1 " " ++x`, want: `concat(1, " ", (++x))`},

		// Comparison, match, in, and the logical pair.
		{name: "not binds tighter than equals", source: "!1 == 0", want: "((!1) == 0)"},
		{name: "concat binds tighter than match", source: `"ab" ~ "a" "b"`, want: `("ab" ~ concat("a", "b"))`},
		{name: "and binds tighter than or", source: "0 && 1 || 1", want: "((0 && 1) || 1)"},
		{name: "comparison binds tighter than and", source: "1 == 1 && 1", want: "((1 == 1) && 1)"},
		// gawk's placement: `in` is tighter than `==`, so this is `(1 in a) == 1`.
		{name: "in binds tighter than equals", source: "1 in a == 1", want: "(((1) in a) == 1)"},

		// Ternary and assignment are right-associative.
		{name: "ternary nests to the right", source: "1 ? 2 : 3 ? 4 : 5", want: "(1 ? 2 : (3 ? 4 : 5))"},
		{name: "assignment nests to the right", source: "y = z = 3", want: "(y = (z = 3))"},
		{name: "compound assignment is lowest", source: "y += 2 * 3", want: "(y += (2 * 3))"},

		// Fields. `$` binds tighter than a postfix increment, so `$x++` is `($x)++`.
		{name: "dollar then postfix", source: "$x++", want: "($(x)++)"},
		{name: "prefix increment inside a field", source: "$++x", want: "$((++x))"},
		{name: "a field of an expression", source: "$(i+1)", want: "$(group((i + 1)))"},
		{name: "a field of a name", source: "$NF", want: "$(NF)"},
		{name: "a negative field", source: "$-1", want: "$((-1))"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := reprintExpr(t, test.source); got != test.want {
				t.Fatalf("%s\n  got  %s\n  want %s", test.source, got, test.want)
			}
		})
	}
}

// Program structure: the five kinds of item, and what a missing action means.
func TestAwkParser_programStructure(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
		want   string
	}{
		{name: "a bare action", source: "{ print }", want: "{print()}"},
		{name: "BEGIN and END", source: "BEGIN { x } END { y }", want: "BEGIN {x} | END {y}"},
		// A pattern with no action means `{ print }`, and the nil records that the
		// program wrote no braces.
		{name: "a pattern with no action", source: "/x/", want: "/x/ <default>"},
		{name: "a pattern with an action", source: "/x/ { print }", want: "/x/ {print()}"},
		{name: "an expression pattern", source: "NR > 1 { print }", want: "(NR > 1) {print()}"},
		{name: "a range", source: "/a/, /b/ { print }", want: "/a/, /b/ {print()}"},
		{name: "several items", source: "BEGIN{x}\n/y/{z}\nEND{w}", want: "BEGIN {x} | /y/ {z} | END {w}"},
		{
			name:   "a function",
			source: "function f(a, b) { return a + b }",
			want:   "function f(a, b) {return (a + b)}",
		},
		{name: "func is a synonym", source: "func g() { return }", want: "function g() {return }"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := reprint(t, test.source); got != test.want {
				t.Fatalf("%s\n  got  %s\n  want %s", test.source, got, test.want)
			}
		})
	}
}

// Statements, including the three shapes of `for` and print's redirect.
func TestAwkParser_statements(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
		want   string
	}{
		{name: "if", source: "{ if (x) y }", want: "{if x y}"},
		{name: "if else", source: "{ if (x) y; else z }", want: "{if x y else z}"},
		{name: "else on its own line", source: "{ if (x)\ny\nelse\nz }", want: "{if x y else z}"},
		{name: "while", source: "{ while (x) y }", want: "{while x y}"},
		{name: "do while", source: "{ do x; while (y) }", want: "{do x while y}"},
		{name: "a C for", source: "{ for (i=0; i<3; i++) x }", want: "{for ((i = 0); (i < 3); (i++)) x}"},
		{name: "a for with empty parts", source: "{ for (;;) x }", want: "{for (; ; ) x}"},
		{name: "for in", source: "{ for (k in a) x }", want: "{for (k in a) x}"},
		// The C form whose initialiser contains `in` is not a for-in, which is what the
		// four-token lookahead is for.
		{name: "a C for whose test uses in", source: "{ for (i=0; (i in a); i++) x }",
			want: "{for ((i = 0); group(((i) in a)); (i++)) x}"},

		{name: "print with no arguments", source: "{ print }", want: "{print()}"},
		{name: "print with arguments", source: "{ print 1, 2 }", want: "{print(1, 2)}"},
		// A `>` in the argument list is a redirect, not a comparison.
		{name: "print redirects", source: `{ print 1 > "f" }`, want: `{print(1) > "f"}`},
		{name: "print appends", source: `{ print 1 >> "f" }`, want: `{print(1) >> "f"}`},
		{name: "print pipes", source: `{ print 1 | "sort" }`, want: `{print(1) | "sort"}`},
		// A parenthesis puts the comparison back.
		{name: "a parenthesised comparison", source: "{ print (1 > 0) }", want: "{print(group((1 > 0)))}"},
		{name: "printf", source: `{ printf "%s", x }`, want: `{printf("%s", x)}`},

		{name: "delete an element", source: "{ delete a[1] }", want: "{delete a[1]}"},
		{name: "delete an array", source: "{ delete a }", want: "{delete a}"},
		{name: "next and exit", source: "{ next; exit 1 }", want: "{next; exit 1}"},
		{name: "exit with no status", source: "{ exit }", want: "{exit }"},
		{name: "a nested block", source: "{ { x; y } }", want: "{{x; y}}"},
		{name: "semicolons are optional", source: "{ x\ny }", want: "{x; y}"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := reprint(t, test.source); got != test.want {
				t.Fatalf("%s\n  got  %s\n  want %s", test.source, got, test.want)
			}
		})
	}
}

func TestAwkParser_getline(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
		want   string
	}{
		{name: "plain", source: "{ getline }", want: "{(getline)}"},
		{name: "into a variable", source: "{ getline x }", want: "{(getline x)}"},
		{name: "from a file", source: `{ getline < "f" }`, want: `{(getline < "f")}`},
		{name: "into a variable from a file", source: `{ getline x < "f" }`, want: `{(getline x < "f")}`},
		{name: "into a field", source: `{ getline $1 }`, want: "{(getline $(1))}"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := reprint(t, test.source); got != test.want {
				t.Fatalf("%s\n  got  %s\n  want %s", test.source, got, test.want)
			}
		})
	}
}

// What the parser refuses, and that it says where.
//
// The first two are the places POSIX and gawk are stricter than busybox. Accepting them
// would mean running a program every other awk rejects.
func TestAwkParser_refusals(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
		says   string
	}{
		{name: "a chained comparison", source: "BEGIN { x = 1 < 2 < 3 }", says: "cannot be chained"},
		{name: "assignment to a literal", source: "BEGIN { 1 = 2 }", says: "needs a variable"},
		{name: "an unclosed block", source: "BEGIN { x", says: "expected } to close a block"},
		{name: "an unclosed group", source: "BEGIN { x = (1 }", says: "expected )"},
		{name: "an unclosed subscript", source: "BEGIN { x = a[1 }", says: "expected ] to close a subscript"},
		{name: "a ternary with no colon", source: "BEGIN { x = 1 ? 2 }", says: "expected : to close a ?:"},
		{name: "a function defined twice", source: "function f() {}\nfunction f() {}", says: "defined twice"},
		{name: "do with no while", source: "BEGIN { do x }", says: "expected while"},
		{name: "a builtin without parentheses", source: "BEGIN { substr }", says: "needs its arguments in parentheses"},
		{name: "increment of a literal", source: "BEGIN { ++1 }", says: "needs a variable"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseAwkProgram(test.source)
			if err == nil {
				t.Fatalf("%q was accepted", test.source)
			}
			if !strings.Contains(err.Error(), test.says) {
				t.Fatalf("%q said %q, which does not contain %q", test.source, err, test.says)
			}
			if !strings.Contains(err.Error(), "line ") {
				t.Errorf("%q does not name a line: %v", test.source, err)
			}
		})
	}
}
