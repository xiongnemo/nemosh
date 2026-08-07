package applets_test

import (
	"strings"
	"testing"
)

func runPrintf(t *testing.T, args ...string) (string, error) {
	t.Helper()
	stdout, _, err := runApplet(t, "printf", args...)
	return stdout, err
}

func TestPrintf_rendersTheConversions(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "string", args: []string{"%s\n", "hello"}, want: "hello\n"},
		{name: "decimal", args: []string{"%d\n", "42"}, want: "42\n"},
		{name: "decimal with a sign", args: []string{"%d\n", "-7"}, want: "-7\n"},
		{name: "width", args: []string{"[%5d]\n", "42"}, want: "[   42]\n"},
		{name: "left aligned width", args: []string{"[%-5d]\n", "42"}, want: "[42   ]\n"},
		{name: "zero padding", args: []string{"[%05d]\n", "42"}, want: "[00042]\n"},
		{name: "float precision", args: []string{"%5.2f\n", "3.14159"}, want: " 3.14\n"},
		{name: "hexadecimal", args: []string{"%x %X\n", "255", "255"}, want: "ff FF\n"},
		{name: "octal", args: []string{"%o\n", "8"}, want: "10\n"},
		{name: "character", args: []string{"%c%c\n", "ab", "cd"}, want: "ac\n"},
		{name: "literal percent", args: []string{"100%%\n"}, want: "100%\n"},
		{name: "escapes in the format", args: []string{`a\tb\nc\n`}, want: "a\tb\nc\n"},
		{name: "octal escape in the format", args: []string{`\0101\n`}, want: "A\n"},
		{name: "b processes the operand's escapes", args: []string{"%b\n", `x\ty`}, want: "x\ty\n"},
		{name: "s does not", args: []string{"%s\n", `x\ty`}, want: `x\ty` + "\n"},
		{name: "format is reused for extra operands", args: []string{"%s\n", "a", "b", "c"}, want: "a\nb\nc\n"},
		{name: "missing operands are empty and zero", args: []string{"[%s][%d]\n"}, want: "[][0]\n"},
		{name: "no newline unless asked", args: []string{"%s", "x"}, want: "x"},
		{name: "hex operand for an integer conversion", args: []string{"%d\n", "0x10"}, want: "16\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			stdout, err := runPrintf(t, test.args...)

			// Then
			if err != nil {
				t.Fatalf("printf %q: %v", test.args, err)
			}
			if stdout != test.want {
				t.Fatalf("printf %q = %q, want %q", test.args, stdout, test.want)
			}
		})
	}
}

func TestPrintf_refusesANonNumericOperandForAnIntegerConversion(t *testing.T) {
	// This used to print `%!d(string=abc)` and exit 0.
	// When
	_, err := runPrintf(t, "%d\n", "abc")

	// Then
	if err == nil || !strings.Contains(err.Error(), "numeric") {
		t.Fatalf("err = %v, want a numeric-value diagnostic", err)
	}
}

func TestPrintf_stopsAtTheCancelEscape(t *testing.T) {
	// When
	stdout, err := runPrintf(t, `ab\ccd`)

	// Then
	if err != nil {
		t.Fatalf("printf: %v", err)
	}
	if stdout != "ab" {
		t.Fatalf("stdout = %q, want %q", stdout, "ab")
	}
}

func TestPrintf_reportsAMissingFormat(t *testing.T) {
	// When
	_, err := runPrintf(t)

	// Then
	if err == nil || !strings.Contains(err.Error(), "missing operand") {
		t.Fatalf("err = %v, want a missing-operand diagnostic", err)
	}
}
