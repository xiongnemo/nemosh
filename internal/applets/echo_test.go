package applets_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

func runEcho(t *testing.T, args ...string) (string, error) {
	t.Helper()
	applet, ok := applets.DefaultRegistry.Lookup("echo")
	if !ok {
		t.Fatal("expected echo applet to be registered")
	}
	var stdout bytes.Buffer
	err := applet.Run(context.Background(), args, &bytes.Buffer{}, &stdout, &bytes.Buffer{})
	return stdout.String(), err
}

func TestEcho_followsTheFancyEchoOptionRules(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "plain", args: []string{"a", "b"}, want: "a b\n"},
		{name: "no arguments", args: nil, want: "\n"},
		{name: "n drops the newline", args: []string{"-n", "abc"}, want: "abc"},
		{name: "n alone", args: []string{"-n"}, want: ""},
		{name: "e enables escapes", args: []string{"-e", `a\tb`}, want: "a\tb\n"},
		{name: "escapes are literal without e", args: []string{`a\tb`}, want: `a\tb` + "\n"},
		{name: "E disables escapes again", args: []string{"-e", "-E", `a\tb`}, want: `a\tb` + "\n"},
		{name: "clustered ne", args: []string{"-ne", `a\nb`}, want: "a\nb"},
		{name: "newline escape", args: []string{"-e", `a\nb`}, want: "a\nb\n"},
		{name: "backslash escape", args: []string{"-e", `a\\b`}, want: `a\b` + "\n"},
		{name: "c cancels the rest", args: []string{"-e", `ab\ccd`, "ef"}, want: "ab"},
		{name: "octal escape", args: []string{"-e", `A\0101B`}, want: "AAB\n"},
		{name: "unknown flag is echoed", args: []string{"-z", "rest"}, want: "-z rest\n"},
		{name: "partly unknown cluster is echoed", args: []string{"-nz", "rest"}, want: "-nz rest\n"},
		{name: "lone dash is an operand", args: []string{"-"}, want: "-\n"},
		{name: "operand after options stops parsing", args: []string{"-n", "x", "-e", `a\tb`}, want: `x -e a\tb`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			got, err := runEcho(t, test.args...)

			// Then
			if err != nil {
				t.Fatalf("echo %q: unexpected error %v", test.args, err)
			}
			if got != test.want {
				t.Fatalf("echo %q = %q, want %q", test.args, got, test.want)
			}
		})
	}
}
