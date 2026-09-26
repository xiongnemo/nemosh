package runtime_test

import "testing"

// `base#digits` reaches base 64 -- 0-9, a-z, A-Z, @ and _ -- and a shift count is taken
// modulo 64. Above base 36 was a syntax error, and a shift by 64 or more, or by a negative
// count, was 0. busybox-w32 and bash 5.3 agree on every answer here.
func TestRuntime_arithmeticBasesAndShifts(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{`echo $((64#a))-$((64#z)), $((64#A))-$((64#Z)), $((64#@)), $(( 64#_ ))`, "10-35, 36-61, 62, 63\n"},
		{`echo $(( 36#z )) $(( 36#Z )) $(( 37#A )) $(( 37#a )) $(( 16#ff )) $(( 2#101 ))`, "35 35 36 10 255 5\n"},
		{`echo $(( 5 << -1 )) $(( 16 >> -1 )) $(( 1 << 64 )) $(( 1 << 63 ))`, "-9223372036854775808 0 1 -9223372036854775808\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as busybox and bash answer", stdout, status, test.want)
			}
		})
	}
}
