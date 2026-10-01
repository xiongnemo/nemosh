package runtime_test

import "testing"

// $SRANDOM is bash 5.1's: a fresh number of 32 bits at each expansion, which an assignment does
// not seed. It was unset. Five draws that all agree would be a one-in-2^128 coincidence.
func TestSRANDOM_isAFreshNumberEachTime(t *testing.T) {
	// When
	status, stdout, stderr := runSetScript(t, "SRANDOM=5\nfor i in 1 2 3 4 5; do printf '%s\\n' \"$SRANDOM\"; done | sort -u | wc -l\n"+
		"case $SRANDOM in ''|*[!0-9]*) echo bad;; *) echo number;; esac\n")

	// Then
	if status != 0 || stdout == "" || stdout[0] == '1' || stdout[len(stdout)-7:] != "number\n" {
		t.Fatalf("status %d, stdout %q, stderr %q, want several distinct numbers", status, stdout, stderr)
	}
}

// `[[ -R ref ]]` and `test -R ref` are bash's: true for a set nameref, false for one with nothing
// to lead to and for a plain variable. [[ said "unexpected ref", and test that -R was no operator.
func TestNameref_isWhatDashRAsks(t *testing.T) {
	// When
	status, stdout, stderr := runSetScript(t, "declare -n ref=x\n[[ -R ref ]] && echo one\n[ -R ref ] && echo two\n"+
		"declare -n empty\n[[ -R empty ]] || echo three\nx=1\n[[ -R x ]] || echo four\n")

	// Then
	if status != 0 || stdout != "one\ntwo\nthree\nfour\n" {
		t.Fatalf("status %d, stdout %q, stderr %q", status, stdout, stderr)
	}
}
