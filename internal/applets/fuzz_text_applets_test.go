package applets

import (
	"bytes"
	"context"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

// printf, expr and tr take their whole behaviour from operands a script computes, and each
// only reads its operands and standard input and writes standard output, so any operand they
// can be given is safe to try. A panic in one is a Go trace where a shell error belongs. A
// count of five digits or more is left out: `%99999999d` and `[a*99999999]` ask for that much
// output, here as in the references.
var fuzzLongNumber = regexp.MustCompile(`[0-9]{5}`)

func fuzzApplet(t *testing.T, name, stdin string, args ...string) {
	t.Helper()
	applet, ok := DefaultRegistry.Lookup(name)
	if !ok {
		t.Fatalf("%s is not registered", name)
	}
	var stdout, stderr bytes.Buffer
	_ = applet.Run(context.Background(), args, strings.NewReader(stdin), &stdout, &stderr)
}

func fuzzable(texts ...string) bool {
	for _, text := range texts {
		if !utf8.ValidString(text) || len(text) > 256 || fuzzLongNumber.MatchString(text) {
			return false
		}
	}
	return true
}

func FuzzPrintf(f *testing.F) {
	for _, seed := range [][3]string{
		{"%s\n", "a", "b"}, {"%d %i", "42", "-7"}, {"%5.2f|%-8s|", "3.14159", "x"}, {"%x %X %o %#x", "255", "8"},
		{"%c%c", "abc", ""}, {"%b", `a\tb\n\c`, ""}, {"%q", "a b'c", ""}, {"%*d", "5", "3"}, {"%.*s", "2", "abcdef"},
		{"%e %g %G", "1e10", "0.0001"}, {"%%%s%%", "x", ""}, {"%", "", ""}, {"%5", "", ""}, {"%-+ #0d", "1", ""},
		{"%(%Y)T", "0", ""}, {"%d", "0x1F", ""}, {"%d", "'a", ""}, {"%u", "-1", ""}, {"\\x41\\101\\u263a", "", ""},
	} {
		f.Add(seed[0], seed[1], seed[2])
	}
	f.Fuzz(func(t *testing.T, format, first, second string) {
		if !fuzzable(format, first, second) {
			t.Skip()
		}
		fuzzApplet(t, "printf", "", format, first, second)
	})
}

func FuzzExpr(f *testing.F) {
	for _, seed := range [][3]string{
		{"1", "+", "2"}, {"10", "/", "0"}, {"abc", ":", "a\\(.\\)"}, {"a", "<", "b"}, {"9223372036854775807", "+", "1"},
		{"length", "abc", ""}, {"substr", "abcdef", "2"}, {"index", "abc", "c"}, {"(", "1", ")"}, {"1", "|", "0"},
		{"", "&", "1"}, {"-", "-", "-"}, {"x", ":", "*"}, {"x", ":", "\\("}, {"match", "aaa", "a*"},
	} {
		f.Add(seed[0], seed[1], seed[2])
	}
	f.Fuzz(func(t *testing.T, a, operator, b string) {
		if !fuzzable(a, operator, b) {
			t.Skip()
		}
		fuzzApplet(t, "expr", "", a, operator, b)
	})
}

func FuzzTr(f *testing.F) {
	for _, seed := range [][3]string{
		{"a-z", "A-Z", "hello"}, {"[:lower:]", "[:upper:]", "abc"}, {"-d", "aeiou", "education"}, {"-s", " ", "a  b"},
		{"-c", "a", "abc"}, {"\\n", " ", "a\nb"}, {"[=a=]", "x", "banana"}, {"[a*3]", "xyz", "aaa"}, {"z-a", "x", "abc"},
		{"\\777", "x", "a"}, {"[:alpha:", "x", "a"}, {"-dc", "[:digit:]", "a1b2"}, {"", "", "x"}, {"a", "", "a"},
	} {
		f.Add(seed[0], seed[1], seed[2])
	}
	f.Fuzz(func(t *testing.T, first, second, input string) {
		if !fuzzable(first, second, input) {
			t.Skip()
		}
		fuzzApplet(t, "tr", input, first, second)
	})
}
