package runtime_test

import "testing"

// An integer constant is read as C and both references read it: hex after 0x, octal after a
// leading 0, decimal otherwise, wrapping at 64 bits. `-9223372036854775808` and the other
// overflowing constants were syntax errors, and Go's own `0b101`, `0o17` and `1_000` were
// numbers, which neither reference has.
func TestArithmetic_readsIntegerConstantsAsC(t *testing.T) {
	for index, test := range []struct{ script, want string }{
		{`echo $(( -9223372036854775808 )) $(( 9223372036854775808 ))`, "-9223372036854775808 -9223372036854775808\n"},
		{`echo $(( 18446744073709551615 )) $(( 18446744073709551616 )) $(( 99999999999999999999 ))`, "-1 0 7766279631452241919\n"},
		{`echo $(( 0xffffffffffffffff )) $(( 0x10000000000000000 )) $(( 01777777777777777777777 ))`, "-1 0 -1\n"},
		{`echo $(( 0x )) $(( 0X1F )) $(( 00 )) $(( 007 )) $(( 017 )) $(( 1234567890123456789 ))`, "0 31 0 7 15 1234567890123456789\n"},
		{`x=0x10; y=-5; echo $(( x + y ))`, "11\n"},
	} {
		if stdout, status := runScriptCapturing(test.script + "\n"); stdout != test.want || status != 0 {
			t.Errorf("%d: %s: got %q/%d, want %q/0, as both references answer", index, test.script, stdout, status, test.want)
		}
	}
	for _, script := range []string{`echo $(( 0b101 ))`, `echo $(( 1_000 ))`, `echo $(( 0o17 ))`, `echo $(( 08 ))`} {
		if stdout, status := runScriptCapturing(script + "\n"); status == 0 {
			t.Errorf("%s: got %q/0, want an arithmetic error, as both references answer", script, stdout)
		}
	}
}
