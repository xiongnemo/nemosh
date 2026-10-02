package applets

import (
	"bytes"
	"context"
	"os"
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

// fuzzReadOnlyApplets read their operands and standard input and write only standard output:
// none of them writes a file, runs a command, follows input for ever, or sets the clock, the
// ones that can -- sort -o, iconv -o, uniq's output, uudecode, tail -f, date -s, env, xargs --
// left out. iconv was in, and its -o wrote seventy files named by the fuzzer's operands into
// this package, which is why the target also runs in a directory of its own.
var fuzzReadOnlyApplets = []string{
	"base32", "base64", "basename", "cksum", "crc32", "cut", "dirname", "expand", "fold", "hd",
	"hexdump", "od", "xxd", "head", "md5sum", "sha1sum", "sha256sum", "nl", "paste", "rev",
	"strings", "sum", "tac", "tsort", "unexpand", "uuencode", "wc", "getopt", "test", "echo", "cal",
}

// FuzzReadOnlyApplets runs one of them with two operands and standard input. An operand that
// names something that is there, or a device, is left out, so nothing is read but the input:
// `/dev/zero` and Windows' CON would be read for ever.
func FuzzReadOnlyApplets(f *testing.F) {
	for index, seed := range [][3]string{
		{"-c", "1-3", "hello\n"}, {"-d", "", "aGVsbG8=\n"}, {"-t", "x1", "abc"}, {"-w", "3", "abcdefg"},
		{"-n", "2", "a\nb\nc\n"}, {"-s", "-", "a\tb\n"}, {"-f", "2", "a b c\n"}, {"-A", "x", "\x00\x01"},
		{"-l", "", "one two\n"}, {"-o", "%s", "x"}, {"-z", "1", "a\x00b"}, {"--", "-x", "y"},
	} {
		f.Add(uint8(index), seed[0], seed[1], seed[2])
	}
	f.Chdir(f.TempDir())
	f.Fuzz(func(t *testing.T, pick uint8, first, second, input string) {
		if !fuzzable(first, second, input) {
			t.Skip()
		}
		for _, operand := range []string{first, second} {
			if _, err := os.Stat(operand); err == nil || strings.Contains(strings.ToLower(operand), "dev") {
				t.Skip()
			}
		}
		name := fuzzReadOnlyApplets[int(pick)%len(fuzzReadOnlyApplets)]
		fuzzApplet(t, name, input, first, second)
	})
}
