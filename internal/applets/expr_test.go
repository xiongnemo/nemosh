package applets_test

import (
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

func runExpr(t *testing.T, args ...string) (string, int) {
	t.Helper()
	got, _, err := runFilter(t, "expr", args, "")
	status := 0
	if code, ok := applets.StatusCode(err); ok {
		status = code
	} else if err != nil {
		status = -1
	}
	return strings.TrimRight(got, "\n"), status
}

// Precedence is the whole substance of expr: an implementation that evaluates
// left to right gets `2 + 3 * 4` wrong and nothing else about it looks broken.
// Every expectation here was measured from GNU expr.
func TestExpr(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		want   string
		status int
	}{
		{name: "addition", args: []string{"3", "+", "4"}, want: "7"},
		{name: "integer division truncates", args: []string{"10", "/", "3"}, want: "3"},
		{name: "modulo", args: []string{"7", "%", "3"}, want: "1"},
		{
			name: "multiplication binds tighter than addition",
			args: []string{"2", "+", "3", "*", "4"}, want: "14",
		},
		{
			name: "parentheses override it",
			args: []string{"(", "2", "+", "3", ")", "*", "4"}, want: "20",
		},
		{name: "numeric comparison", args: []string{"5", ">", "3"}, want: "1"},
		{
			// POSIX compares numerically when both sides are numbers and as
			// strings otherwise, which is why these two differ.
			name: "string comparison when a side is not a number",
			args: []string{"a10", ">", "a9"}, want: "0", status: 1,
		},
		{name: "string equality", args: []string{"abc", "=", "abc"}, want: "1"},
		{
			// `|` yields the first operand when it is neither null nor zero, and
			// the second otherwise. Not a boolean.
			name: "or yields an operand, not a boolean",
			args: []string{"", "|", "abc"}, want: "abc",
		},
		{name: "and", args: []string{"1", "&", "2"}, want: "1"},
		{name: "length", args: []string{"length", "abcde"}, want: "5"},
		{
			// Anchored at the start, and counting what matched -- which is why
			// this is 1 and not 3.
			name: "a match counts the characters it consumed",
			args: []string{"abc", ":", "a*"}, want: "1",
		},
		{
			// With a group it yields the group instead, which is the form that
			// pulls a version out of a string.
			name: "a group yields its text",
			args: []string{"v1.2.3", ":", `v\(.*\)`}, want: "1.2.3",
		},
		{name: "substr is one-based", args: []string{"substr", "abcdef", "2", "3"}, want: "bcd"},
		{
			// Out of range is the empty string rather than an error, which is
			// what a script slicing a short value needs.
			name: "substr past the end is empty",
			args: []string{"substr", "ab", "5", "3"}, want: "", status: 1,
		},
		{name: "index is one-based, zero for absent", args: []string{"index", "abc", "c"}, want: "3"},
		{
			// The status surprises people and is worth pinning: a result of zero
			// is a *failure* status, so expr cannot be used under `set -e` to
			// compute a sum that might legitimately be zero.
			name: "a zero result exits 1", args: []string{"1", "-", "1"}, want: "0", status: 1,
		},
		{
			// Measured: GNU prints 0 rather than an empty line here, so a null
			// result of a logical operator is spelled as the number.
			name: "a null result of or is spelled zero",
			args: []string{"", "|", ""}, want: "0", status: 1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			got, status := runExpr(t, test.args...)

			// Then
			if got != test.want || status != test.status {
				t.Fatalf("expr %v = (%q, %d), want (%q, %d)", test.args, got, status, test.want, test.status)
			}
		})
	}
}

// A bad expression is status 2, which is how a script tells "the answer was zero"
// from "that was not an expression".
func TestExpr_refusals(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "nothing at all", args: nil},
		{name: "a missing operand", args: []string{"1", "+"}},
		{name: "an unclosed group", args: []string{"(", "1", "+", "2"}},
		{name: "arithmetic on a word", args: []string{"abc", "+", "1"}},
		{name: "division by zero", args: []string{"1", "/", "0"}},
		{name: "trailing rubbish", args: []string{"1", "2"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			_, status := runExpr(t, test.args...)

			// Then
			if status != 2 {
				t.Fatalf("expr %v exited %d, want 2", test.args, status)
			}
		})
	}
}
