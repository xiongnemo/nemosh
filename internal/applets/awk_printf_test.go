package applets

import (
	"strings"
	"testing"
)

// `printf`, `sprintf` and the numeric built-ins.
//
// awk's printf is a different utility from the shell's, and the cases that say so are the
// ones worth having: the format is **not reused** when arguments remain, and a format's
// escapes were already decoded by the lexer so nothing re-expands them here.

func TestAwkPrintf(t *testing.T) {
	t.Parallel()
	for _, testcase := range []struct {
		name     string
		program  string
		input    string
		want     string
		diverges []string
	}{
		{name: "an integer truncates toward zero", program: `BEGIN{printf "%d|%d|%d\n", "3.9", -3.9, 3.9}`, want: "3|-3|3\n"},
		{name: "width, precision and justification", program: `BEGIN{printf "%i|%5.2f|%-5s|\n", 7, 3.14159, "ab"}`, want: "7| 3.14|ab   |\n"},
		{name: "a string precision", program: `BEGIN{printf "%.2s|\n", "abcdef"}`, want: "ab|\n"},
		{name: "the other radixes", program: `BEGIN{printf "%x|%X|%o|%u\n", 255, 255, 8, 7}`, want: "ff|FF|10|7\n"},
		{name: "a literal percent", program: `BEGIN{printf "%%|\n"}`, want: "%|\n"},
		{name: "no trailing newline of its own", program: `BEGIN{printf "abc"}`, want: "abc"},
		{name: "a number becomes a string through CONVFMT", program: `BEGIN{printf "%s\n", 1/3}`, want: "0.333333\n"},
		{name: "sprintf answers a string", program: `BEGIN{x=sprintf("%03d",7);print x, length(x)}`, want: "007 3\n"},
		{name: "a character from a code", program: `BEGIN{printf "%c%c|\n", 65, "BC"}`, want: "AB|\n"},
		{
			// A strnum counts as a number here, so a field of `65` is `A` and not `6`.
			name: "a character from a field", program: `{printf "%c|%c|\n", $1, $2}`, input: "65 A\n", want: "A|A|\n",
		},
		{name: "a character takes a width", program: `BEGIN{printf "%5c|%-5c|\n", "A", "B"}`, want: "    A|B    |\n"},
		{
			// The format is used once. The shell's printf would print `a` and then `b`.
			name: "extra arguments are dropped", program: `BEGIN{printf "%s\n", "a", "b"}`, want: "a\n",
		},
		{
			// Already decoded when the string literal was lexed, so `printf s` writes the
			// two characters rather than a newline.
			name: "the format is not re-escaped", program: `BEGIN{s="a\\nb";printf s;print ""}`, want: `a\nb` + "\n",
		},
		{name: "a parenthesised argument list", program: `BEGIN{printf("%s-%s\n","a","b")}`, want: "a-b\n"},
		{name: "print takes one too", program: `BEGIN{print (1,2)}`, want: "1 2\n"},
		{name: "a parenthesised comparison is still one", program: `BEGIN{print (1 > 0)}`, want: "1\n"},
		{
			// gawk supports `*`; busybox refuses the conversion outright.
			name: "a width from an argument", program: `BEGIN{printf "%*d|%-*d|\n", 5, 42, 5, 42}`,
			want: "   42|42   |\n", diverges: []string{"busybox awk"},
		},
		{
			// busybox on Windows writes a three-digit exponent, which is the MSVC runtime
			// rather than a rule; C99 and gawk both say two.
			name: "an exponent has two digits", program: `BEGIN{printf "%e|%g\n", 1234.5, 0.0001}`,
			want: "1.234500e+03|0.0001\n", diverges: []string{"busybox awk"},
		},
		{
			// busybox wraps these to a 32-bit int and prints -2147483648 for both.
			name: "large integers are 64-bit", program: `BEGIN{printf "%d|%d\n", 2147483648, 1e18}`,
			want: "2147483648|1000000000000000000\n", diverges: []string{"busybox awk"},
		},
		{
			// POSIX and busybox: the missing operands are the uninitialised value, which
			// is both "" and 0. gawk makes it fatal instead.
			name: "missing arguments are empty", program: `BEGIN{printf "[%s][%d]\n"}`,
			want: "[][0]\n", diverges: []string{"gawk"},
		},
		{
			// Not a number, so `0x1A` is the string prefix `0` -- awk has no hex literal.
			name: "hex is not a number", program: `BEGIN{printf "%d\n", "0x1A"}`,
			want: "0\n", diverges: []string{"busybox awk"},
		},
		{
			// Rounded half to even, which is Go and gawk; busybox's C runtime rounds away
			// from zero and prints 3.
			name: "a half rounds to even", program: `BEGIN{printf "%.0f\n", 2.5}`,
			want: "2\n", diverges: []string{"busybox awk"},
		},
		{
			// The references in a C locale write the single byte 0xE9, which is not valid
			// UTF-8 on its own. This writes the rune.
			name: "a character code becomes a rune", program: `BEGIN{printf "%c\n", 233}`,
			want: "é\n", diverges: []string{"gawk", "busybox awk"},
		},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()
			got, stderr, status := runAwk(t, testcase.program, testcase.input)
			if got != testcase.want || stderr != "" || status != 0 {
				t.Fatalf("%s\n got %q stderr %q status %d\nwant %q", testcase.program, got, stderr, status, testcase.want)
			}
			checkAgainstReferences(t, testcase.program, testcase.input, got, testcase.diverges...)
		})
	}
}

func TestAwkNumericBuiltins(t *testing.T) {
	t.Parallel()
	for _, testcase := range []struct {
		name    string
		program string
		want    string
	}{
		{name: "int truncates toward zero", program: `BEGIN{print int(-3.9), int(3.9)}`, want: "-3 3\n"},
		{name: "int of a numeric prefix", program: `BEGIN{print int("4x")}`, want: "4\n"},
		{name: "the ordinary functions", program: `BEGIN{print sqrt(9), exp(0), log(1)}`, want: "3 1 0\n"},
		{name: "trigonometry", program: `BEGIN{printf "%.4f %.4f %.4f\n", sin(0), cos(0), atan2(0,-1)}`, want: "0.0000 1.0000 3.1416\n"},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()
			got, _, _ := runAwk(t, testcase.program, "")
			if got != testcase.want {
				t.Fatalf("%s\n got %q\nwant %q", testcase.program, got, testcase.want)
			}
			checkAgainstReferences(t, testcase.program, "", got)
		})
	}
}

// TestAwkRandIsSeeded checks what can honestly be checked about `rand`.
//
// Which floats a seed produces belongs to the generator and not to the language, so the
// references cannot supply the answer. What they do settle, and what is asserted here, is
// that the sequence starts from seed 1 without an `srand`, that a seed repeats, and that
// `srand` answers the *previous* seed.
func TestAwkRandIsSeeded(t *testing.T) {
	t.Parallel()
	for _, testcase := range []struct {
		name    string
		program string
		want    string
	}{
		{name: "a seed repeats", program: `BEGIN{srand(1);a=rand();srand(1);print (a==rand())}`, want: "1\n"},
		{name: "the range is a half-open unit", program: `BEGIN{ok=1;for(i=0;i<200;i++){r=rand();if(r<0||r>=1)ok=0}print ok}`, want: "1\n"},
		{name: "srand answers the previous seed", program: `BEGIN{print srand(42);print srand(7)}`, want: "1\n42\n"},
		{name: "different seeds differ", program: `BEGIN{srand(1);a=rand();srand(2);print (a!=rand())}`, want: "1\n"},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()
			got, _, _ := runAwk(t, testcase.program, "")
			if got != testcase.want {
				t.Fatalf("%s\n got %q\nwant %q", testcase.program, got, testcase.want)
			}
			checkAgainstReferences(t, testcase.program, "", got)
		})
	}
}

// TestAwkRandIsReproducibleAcrossRuns is the half of `rand` that a single process cannot
// show: a program which never calls `srand` must give the same answer to a *later* run.
func TestAwkRandIsReproducibleAcrossRuns(t *testing.T) {
	t.Parallel()
	const program = `BEGIN{for(i=0;i<5;i++)printf "%.6f ", rand()}`
	first, _, _ := runAwk(t, program, "")
	second, _, _ := runAwk(t, program, "")
	if first != second || strings.TrimSpace(first) == "" {
		t.Fatalf("two runs of %s gave %q and %q", program, first, second)
	}
}
