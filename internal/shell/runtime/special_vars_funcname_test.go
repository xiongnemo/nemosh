package runtime_test

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

// `$FUNCNAME` names the function running now -- through a nested call, back in the caller,
// and inside a subshell of the call -- and is unset outside one. The whole transcript is
// busybox-w32's, measured; it used to be empty everywhere.
func TestFuncname_namesTheFunctionRunningNow(t *testing.T) {
	script := "f() { echo \"f=$FUNCNAME\"; g; echo \"back=$FUNCNAME\"; }\n" +
		"g() { echo \"g=$FUNCNAME\"; (echo \"sub=$FUNCNAME\"); }\n" +
		"f\necho \"top=${FUNCNAME:-unset}\"\n"
	want := "f=f\ng=g\nsub=g\nback=f\ntop=unset\n"
	if stdout, status := runScriptCapturing(script); stdout != want || status != 0 {
		t.Fatalf("got %q/%d, want %q", stdout, status, want)
	}
}

// Both references have these, and a timestamp without starting `date` is what they are for.
func TestEpochVariables_tellTheTime(t *testing.T) {
	before := time.Now().Unix()
	stdout, _ := runScriptCapturing("echo $EPOCHSECONDS\necho $EPOCHREALTIME\n")
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 2 {
		t.Fatalf("stdout = %q, want two lines", stdout)
	}
	seconds, err := strconv.ParseInt(lines[0], 10, 64)
	if err != nil || seconds < before || seconds > time.Now().Unix() {
		t.Fatalf("EPOCHSECONDS = %q, want the time now", lines[0])
	}
	whole, fraction, found := strings.Cut(lines[1], ".")
	if !found || len(fraction) != 6 || whole < lines[0] {
		t.Fatalf("EPOCHREALTIME = %q, want seconds and six decimal places", lines[1])
	}
}
