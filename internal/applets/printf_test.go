package applets_test

import (
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
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
		{
			// Bare octal, and at most three digits, which is what POSIX gives printf's
			// format and what busybox-w32 does: `\0101` is octal 010 and then a
			// literal 1, not octal 101. This asserted the `A` that XSI echo produces,
			// so it had been pinning a divergence from the reference. Measured:
			// `busybox printf '\0101' | xxd -p` is 0831.
			name: "bare octal in the format", args: []string{"\0101\n"}, want: "\b1\n",
		},
		{
			// And hex, which neither applet had at all.
			name: "hex in the format", args: []string{"\x41\x42\n"}, want: "AB\n",
		},
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

// A non-numeric operand is POSIX's case exactly, and busybox's answer: a diagnostic, zero
// in its place with the conversion's width, the rest still processed, and status 1. This
// used to print `%!d(string=abc)` and exit 0, and after that to stop at the first one.
func TestPrintf_reportsANonNumericOperandAndGoesOn(t *testing.T) {
	// When
	stdout, stderr, err := runApplet(t, "printf", "[%3d][%s]\n", "abc", "next")

	// Then
	if stdout != "[  0][next]\n" {
		t.Fatalf("stdout = %q, want the zero and the rest", stdout)
	}
	if !strings.Contains(stderr, "abc: invalid number") {
		t.Fatalf("stderr = %q, want the diagnostic", stderr)
	}
	if status, ok := applets.StatusCode(err); !ok || status != 1 {
		t.Fatalf("status = %d (recognised %v), want 1", status, ok)
	}
}

// `%(format)T` renders seconds since the epoch through strftime, with the conversion's width;
// bash's answers, measured. Mid-year, so the year and month hold in any zone.
func TestPrintf_rendersATimeConversion(t *testing.T) {
	stdout, err := runPrintf(t, "[%(%Y-%m)T][%8(%Y)T]\n", "17280000", "17280000")
	if err != nil || stdout != "[1970-07][    1970]\n" {
		t.Fatalf("stdout = %q, err = %v", stdout, err)
	}
}

// A leading `--` ends the options, as in both references; it was taken for the format.
func TestPrintf_skipsALeadingDoubleDash(t *testing.T) {
	if stdout, err := runPrintf(t, "--", "-v %s\n", "x"); err != nil || stdout != "-v x\n" {
		t.Fatalf("stdout = %q, err = %v, want %q", stdout, err, "-v x\n")
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
