package applets

import (
	"os/exec"
	"strings"
	"testing"
)

// dc, and the decimal arithmetic underneath it.
//
// Each case carries a literal answer and is also handed to busybox-w32 where it is
// installed, for the same reason awk's cases are: the scale rules are the specification, and
// a calculator that agrees with itself has proved nothing.

func runDc(t *testing.T, script string) (string, string, int) {
	t.Helper()
	return runApplet(t, "dc", nil, script)
}

func checkDcAgainstBusybox(t *testing.T, script, got string) {
	t.Helper()
	if !busyboxIsTheReference() {
		return
	}
	path, err := exec.LookPath("busybox")
	if err != nil {
		return
	}
	command := exec.Command(path, "dc")
	command.Stdin = strings.NewReader(script)
	out, _ := command.Output()
	want := strings.ReplaceAll(string(out), "\r\n", "\n")
	if got != want {
		t.Fatalf("dc %q disagrees with busybox\n got %q\nwant %q", script, got, want)
	}
}

func TestDc(t *testing.T) {
	t.Parallel()
	for _, testcase := range []struct {
		name   string
		script string
		want   string
	}{
		{name: "addition", script: "2 3 + p", want: "5\n"},
		{name: "subtraction is below minus top", script: "2 3 - p", want: "-1\n"},
		{name: "division truncates at scale 0", script: "2 3 / p", want: "0\n"},
		{name: "division at a scale", script: "5 k 2 3 / p", want: ".66666\n"},
		{name: "power", script: "2 3 ^ p", want: "8\n"},
		{name: "remainder", script: "7 3 % p", want: "1\n"},
		{name: "square root at scale 0", script: "2 v p", want: "1\n"},
		{name: "square root at a scale", script: "10 k 2 v p", want: "1.4142135623\n"},
		{name: "the stack prints top first", script: "1 2 3 f", want: "3\n2\n1\n"},
		{name: "duplicate", script: "1 2 d f", want: "2\n2\n1\n"},
		{name: "swap", script: "1 2 r f", want: "1\n2\n"},
		{name: "registers", script: "5 sa la p", want: "5\n"},
		{name: "an unset register is zero", script: "la 1 + sa la p", want: "1\n"},
		{name: "a register is a stack", script: "1 Sa 2 Sa La p La p", want: "2\n1\n"},
		{name: "depth", script: "1 2 z p", want: "2\n"},
		{name: "digit count", script: "99 Z p", want: "2\n"},
		{name: "scale of a number", script: "3.14 X p", want: "2\n"},
		{name: "a string", script: "[hi] p", want: "hi\n"},
		{name: "executing a string", script: "[1 2 +] x p", want: "3\n"},
		{name: "output base", script: "16 o 255 p", want: "FF\n"},
		{name: "binary output", script: "2 o 5 p", want: "101\n"},
		{name: "input base", script: "16 i FF p", want: "255\n"},
		{name: "the scale register", script: "5 k K p", want: "5\n"},
		{name: "clear", script: "c 1 2 f", want: "2\n1\n"},
		{name: "an underscore is a negative literal", script: "_5 p", want: "-5\n"},
		{name: "quit stops", script: "q 5 p", want: ""},
		// The comparison is of the top against the one below it, so this runs and the
		// next one does not -- which reads backwards from the order they were written.
		{name: "a conditional runs its register", script: "[99 p] sa 2 1 <a", want: "99\n"},
		{name: "a conditional that does not hold", script: "[99 p] sa 1 2 <a", want: ""},
		{name: "a greater-than conditional", script: "[99 p] sa 1 2 >a", want: "99\n"},
		{name: "an equality conditional", script: "[99 p] sa 2 2 =a", want: "99\n"},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()
			got, stderr, status := runDc(t, testcase.script)
			if got != testcase.want || stderr != "" || status != 0 {
				t.Fatalf("dc %q\n got %q stderr %q status %d\nwant %q",
					testcase.script, got, stderr, status, testcase.want)
			}
			checkDcAgainstBusybox(t, testcase.script, got)
		})
	}
}

func TestDcRefusals(t *testing.T) {
	t.Parallel()
	for _, testcase := range []struct {
		script string
		says   string
	}{
		{script: "1 0 / p", says: "divide by zero"},
		{script: "1 + p", says: "too few elements"},
		{script: "@ p", says: "bad character"},
		{script: "_1 v p", says: "square root of a negative"},
		{script: "2 0.5 ^ p", says: "not an integer"},
		{script: "1 ! p", says: "shell escape"},
		{script: "? p", says: "reading a line"},
		{script: "99 o 5 p", says: "between 2 and 16"},
		{script: "_1 k 5 p", says: "must not be negative"},
		{script: "[unterminated", says: "unterminated string"},
	} {
		t.Run(testcase.script, func(t *testing.T) {
			t.Parallel()
			_, stderr, status := runDc(t, testcase.script)
			if status == 0 {
				t.Fatalf("dc %q was accepted", testcase.script)
			}
			if !strings.Contains(stderr, testcase.says) {
				t.Fatalf("dc %q said %q, which does not contain %q", testcase.script, stderr, testcase.says)
			}
		})
	}
	// An error stops the script, so what came after it does not run.
	got, _, _ := runDc(t, "1 0 / 5 p")
	if got != "" {
		t.Fatalf("dc kept going after an error and printed %q", got)
	}
	// But output already produced is kept. The reference loses it, because it dies with a
	// full buffer; throwing away a number that was computed and printed is worse.
	kept, _, _ := runDc(t, "1 2 + p @ 7 p")
	if kept != "3\n" {
		t.Fatalf("dc discarded output printed before the error: %q", kept)
	}
}

// TestDecimalScaleRules covers the arithmetic on its own, where the scale of a result is the
// part that is specified rather than obvious.
func TestDecimalScaleRules(t *testing.T) {
	t.Parallel()
	// The parser reads digits, not signs: dc writes a negative literal with `_` and bc
	// builds one with unary minus, so neither ever hands it a `-`.
	number := func(t *testing.T, text string) bigDecimal {
		t.Helper()
		negative := strings.HasPrefix(text, "-")
		value, err := parseDecimalDigits(strings.TrimPrefix(text, "-"), 10)
		if err != nil {
			t.Fatal(err)
		}
		if negative {
			return negateDecimal(value)
		}
		return value
	}
	for _, testcase := range []struct {
		name     string
		a, b     string
		operator byte
		scale    int
		want     string
	}{
		// Addition keeps the larger scale, so this is 3.0 and not 3.
		{name: "addition keeps the larger scale", a: "1.5", b: "1.5", operator: '+', scale: 0, want: "3.0"},
		{name: "subtraction likewise", a: "1.50", b: "1.5", operator: '-', scale: 0, want: "0"},
		// min(scale(a)+scale(b), max(scale, scale(a), scale(b))) -- one place, truncated.
		{name: "multiplication at scale 0", a: "2.5", b: "2.5", operator: '*', scale: 0, want: "6.2"},
		{name: "multiplication at scale 5", a: "2.5", b: "2.5", operator: '*', scale: 5, want: "6.25"},
		{name: "division takes the scale exactly", a: "10", b: "3", operator: '/', scale: 2, want: "3.33"},
		{name: "division truncates toward zero", a: "-7", b: "2", operator: '/', scale: 0, want: "-3"},
		// a - (a/b)*b, with the division at the current scale -- not an integer remainder.
		{name: "remainder at scale 0", a: "10", b: "3", operator: '%', scale: 0, want: "1"},
		{name: "remainder at scale 2", a: "10", b: "3", operator: '%', scale: 2, want: ".01"},
		{name: "power", a: "2", b: "10", operator: '^', scale: 0, want: "1024"},
		{name: "power of a fraction", a: "2.5", b: "2", operator: '^', scale: 0, want: "6.2"},
		{name: "a negative power", a: "2", b: "-2", operator: '^', scale: 4, want: ".2500"},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()
			got, err := applyDecimal(testcase.operator, number(t, testcase.a), number(t, testcase.b), testcase.scale)
			if err != nil {
				t.Fatal(err)
			}
			text, err := formatDecimal(got, 10)
			if err != nil {
				t.Fatal(err)
			}
			if text != testcase.want {
				t.Fatalf("%s %c %s at scale %d = %s, want %s",
					testcase.a, testcase.operator, testcase.b, testcase.scale, text, testcase.want)
			}
		})
	}
}

func TestDecimalText(t *testing.T) {
	t.Parallel()
	// A digit too large for the base is clamped, which is POSIX's rule: ibase=8 reads 19
	// as 17, which is 15.
	if value, err := parseDecimalDigits("19", 8); err != nil || value.integer().Int64() != 15 {
		t.Fatalf("parseDecimalDigits(\"19\", 8) = %v, %v", value.integer(), err)
	}
	if value, err := parseDecimalDigits("FF", 16); err != nil || value.integer().Int64() != 255 {
		t.Fatalf("parseDecimalDigits(\"FF\", 16) = %v, %v", value.integer(), err)
	}
	// The leading zero is dropped on output.
	half, _ := parseDecimalDigits("0.5", 10)
	if text, _ := formatDecimal(half, 10); text != ".5" {
		t.Fatalf("0.5 formatted as %q, want .5", text)
	}
	if _, err := formatDecimal(half, 17); err == nil {
		t.Fatal("an output base of 17 was accepted")
	}
	// Long output is broken with a backslash after 68 digits, as the reference breaks it.
	big, _ := parseDecimalDigits(strings.Repeat("9", 100), 10)
	text, _ := formatDecimal(big, 10)
	lines := strings.Split(text, "\n")
	if len(lines) != 2 || len(lines[0]) != 69 || !strings.HasSuffix(lines[0], "\\") {
		t.Fatalf("a hundred digits wrapped as %q", text)
	}
}
