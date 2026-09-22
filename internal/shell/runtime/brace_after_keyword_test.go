package runtime_test

import (
	"testing"
)

// **A brace group may follow a reserved word.**
//
// `if true; then { echo a; }; fi` is ordinary shell and answered
// `incomplete script: missing }`. So did every other position a command can sit in after a
// reserved word -- eight forms, all of which busybox and bash run.
//
// The `{` of a group is told from the `{` of a word by what precedes it: a separator, a
// bracket, or `function name`. A reserved word was not on that list, so `if { ...` left the
// `{` looking like data, the separator scan then cut inside the group, and the `}` arrived
// with nothing open.
//
// The rule is that **every** word before the brace is one a command may follow, which is
// what lets `if ! { false; }` through while keeping `echo if { a; }` refused -- and both
// references refuse that one too, measured rather than assumed.
//
// Found by a corpus sweep rather than by use: nine of its thirteen gaps were a compound
// command as a condition, and this is the half of that which is about the brace itself. The
// keyword compounds -- `if case ... esac; then` and friends -- need the span builder to open
// two frames from one line and are still recorded as gaps.

func TestBraceGroup_mayFollowAReservedWord(t *testing.T) {
	for _, testcase := range []struct {
		name   string
		script string
		want   string
	}{
		{name: "as an if condition", script: "if { true; }; then echo g; fi\n", want: "g\n"},
		{name: "as a then body", script: "if true; then { echo a; }; fi\n", want: "a\n"},
		{name: "as an else body", script: "if false; then :; else { echo e; }; fi\n", want: "e\n"},
		{name: "as an elif condition", script: "if false; then :; elif { true; }; then echo x; fi\n", want: "x\n"},
		{name: "as a while condition", script: "while { false; }; do :; done\necho ok\n", want: "ok\n"},
		{name: "as an until condition", script: "until { true; }; do :; done\necho ok\n", want: "ok\n"},
		{name: "as a do body", script: "for i in 1; do { echo loop; }; done\n", want: "loop\n"},
		{name: "negated", script: "! { false; }\necho $?\n", want: "0\n"},
		{name: "two of them in an and-or condition", script: "if { true; } && { true; }; then echo both; fi\n", want: "both\n"},
		{name: "after a separator, which always worked", script: "true && { echo and; }\n", want: "and\n"},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			stdout, status := runScriptCapturing(testcase.script)
			if stdout != testcase.want || status != 0 {
				t.Errorf("stdout = %q, status = %d; want %q and 0", stdout, status, testcase.want)
			}
		})
	}
}

// TestBraceGroup_afterAnOrdinaryWordIsStillData keeps the rule from widening into every
// `{`: a brace after a command name is an argument, and a group there is a syntax error in
// both references as well.
func TestBraceGroup_afterAnOrdinaryWordIsStillData(t *testing.T) {
	if _, status := runScriptCapturing("echo if { a; }\n"); status == 0 {
		t.Error("`echo if { a; }` was accepted; both references refuse it, because the `{` " +
			"follows a command name rather than a reserved word")
	}
	// And a brace that really is an argument still prints.
	if stdout, status := runScriptCapturing("echo then\n"); stdout != "then\n" || status != 0 {
		t.Errorf("a reserved word as an argument printed %q with status %d", stdout, status)
	}
	if stdout, status := runScriptCapturing("echo '{'\n"); stdout != "{\n" || status != 0 {
		t.Errorf("a quoted brace printed %q with status %d", stdout, status)
	}
}
