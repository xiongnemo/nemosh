package applets_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// The applet returns its diagnostic rather than printing it; the shell prefixes
// the applet name and writes it (internal/shell/runtime/runtime.go).
func runTestApplet(t *testing.T, args ...string) (int, string) {
	t.Helper()
	applet, ok := applets.DefaultRegistry.Lookup("test")
	if !ok {
		t.Fatal("expected test applet to be registered")
	}
	err := applet.Run(context.Background(), args, &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{})
	message := ""
	if text, ok := applets.StatusMessage(err); ok {
		message = text
	}
	return appletStatus(err), message
}

func appletStatus(err error) int {
	if err == nil {
		return 0
	}
	if status, ok := applets.StatusCode(err); ok {
		return status
	}
	return 1
}

func TestTest_evaluatesStringAndLogicPrimaries(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want int
	}{
		{name: "no arguments", args: nil, want: 1},
		{name: "non-empty string", args: []string{"x"}, want: 0},
		{name: "empty string", args: []string{""}, want: 1},
		{name: "-n on a value", args: []string{"-n", "x"}, want: 0},
		{name: "-n on empty", args: []string{"-n", ""}, want: 1},
		{name: "-z on empty", args: []string{"-z", ""}, want: 0},
		{name: "-z on a value", args: []string{"-z", "x"}, want: 1},
		{name: "string equal", args: []string{"a", "=", "a"}, want: 0},
		{name: "string unequal", args: []string{"a", "=", "b"}, want: 1},
		{name: "string not equal", args: []string{"a", "!=", "b"}, want: 0},
		{name: "double equal is accepted", args: []string{"a", "==", "a"}, want: 0},
		{name: "string less than", args: []string{"a", "<", "b"}, want: 0},
		{name: "string greater than", args: []string{"b", ">", "a"}, want: 0},
		{name: "negation of true", args: []string{"!", "x"}, want: 1},
		{name: "negation of false", args: []string{"!", ""}, want: 0},
		{name: "negation of a primary", args: []string{"!", "a", "=", "b"}, want: 0},
		{name: "and both true", args: []string{"x", "-a", "y"}, want: 0},
		{name: "and one false", args: []string{"x", "-a", ""}, want: 1},
		{name: "or one true", args: []string{"", "-o", "y"}, want: 0},
		{name: "or both false", args: []string{"", "-o", ""}, want: 1},
		{name: "and binds tighter than or", args: []string{"", "-a", "x", "-o", "y"}, want: 0},
		{name: "parentheses regroup", args: []string{"(", "", "-o", "x", ")", "-a", "y"}, want: 0},
		{name: "parenthesised single operand", args: []string{"(", "x", ")"}, want: 0},
		{name: "binary wins over unary at position two", args: []string{"-f", "=", "-f"}, want: 0},
		{name: "integer equal", args: []string{"3", "-eq", "3"}, want: 0},
		{name: "integer unequal", args: []string{"3", "-eq", "4"}, want: 1},
		{name: "integer not equal", args: []string{"3", "-ne", "4"}, want: 0},
		{name: "integer greater", args: []string{"4", "-gt", "3"}, want: 0},
		{name: "integer greater or equal", args: []string{"3", "-ge", "3"}, want: 0},
		{name: "integer less", args: []string{"3", "-lt", "4"}, want: 0},
		{name: "integer less or equal", args: []string{"3", "-le", "3"}, want: 0},
		{name: "negative integers compare", args: []string{"-2", "-lt", "-1"}, want: 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, stderr := runTestApplet(t, test.args...)

			// Then
			if status != test.want {
				t.Fatalf("test %q = %d, want %d (message %q)", test.args, status, test.want, stderr)
			}
		})
	}
}

// A unary operator with nothing after it is the one-argument form -- a
// non-empty string -- rather than a missing operand, and `! -f` is the
// two-argument form negating it. POSIX 2.14 spells both out by argument count.
func TestTest_treatsATrailingOperatorAsAString(t *testing.T) {
	// When
	alone, _ := runTestApplet(t, "-f")
	negated, _ := runTestApplet(t, "!", "-f")

	// Then
	if alone != 0 || negated != 1 {
		t.Fatalf("test -f = %d and test ! -f = %d, want 0 and 1", alone, negated)
	}
}

func TestTest_evaluatesFilePrimaries(t *testing.T) {
	// Given
	dir := t.TempDir()
	regular := filepath.Join(dir, "regular.txt")
	if err := os.WriteFile(regular, []byte("content"), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	empty := filepath.Join(dir, "empty.txt")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatalf("seed empty file: %v", err)
	}
	missing := filepath.Join(dir, "missing.txt")

	tests := []struct {
		name string
		args []string
		want int
	}{
		{name: "-e on a file", args: []string{"-e", regular}, want: 0},
		{name: "-e on a directory", args: []string{"-e", dir}, want: 0},
		{name: "-e on nothing", args: []string{"-e", missing}, want: 1},
		{name: "-f on a file", args: []string{"-f", regular}, want: 0},
		{name: "-f on a directory", args: []string{"-f", dir}, want: 1},
		{name: "-d on a directory", args: []string{"-d", dir}, want: 0},
		{name: "-d on a file", args: []string{"-d", regular}, want: 1},
		{name: "-s on a non-empty file", args: []string{"-s", regular}, want: 0},
		{name: "-s on an empty file", args: []string{"-s", empty}, want: 1},
		{name: "-r on a readable file", args: []string{"-r", regular}, want: 0},
		{name: "-r on nothing", args: []string{"-r", missing}, want: 1},
		{name: "-h on a regular file", args: []string{"-h", regular}, want: 1},
		{name: "-ef with itself", args: []string{regular, "-ef", regular}, want: 0},
		{name: "-ef with another", args: []string{regular, "-ef", empty}, want: 1},
		{name: "negated -f", args: []string{"!", "-f", missing}, want: 0},
		{name: "-f and -d together", args: []string{"-f", regular, "-a", "-d", dir}, want: 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, stderr := runTestApplet(t, test.args...)

			// Then
			if status != test.want {
				t.Fatalf("test %q = %d, want %d (stderr %q)", test.args, status, test.want, stderr)
			}
		})
	}
}

func TestTest_comparesModificationTimes(t *testing.T) {
	// Given
	dir := t.TempDir()
	older := filepath.Join(dir, "older")
	newer := filepath.Join(dir, "newer")
	for _, path := range []string{older, newer} {
		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatalf("seed %s: %v", path, err)
		}
	}
	oldInfo, err := os.Stat(older)
	if err != nil {
		t.Fatalf("stat older: %v", err)
	}
	if err := os.Chtimes(newer, oldInfo.ModTime().Add(time.Hour), oldInfo.ModTime().Add(time.Hour)); err != nil {
		t.Fatalf("age newer: %v", err)
	}

	// When
	newerStatus, _ := runTestApplet(t, newer, "-nt", older)
	olderStatus, _ := runTestApplet(t, older, "-ot", newer)

	// Then
	if newerStatus != 0 || olderStatus != 0 {
		t.Fatalf("-nt = %d, -ot = %d, want both 0", newerStatus, olderStatus)
	}
}

func TestTest_reportsSyntaxProblemsWithStatusTwo(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		fragment string
	}{
		{name: "unknown operand", args: []string{"3", "-lt", "5", "6"}, fragment: "unknown operand"},
		{name: "missing argument", args: []string{"x", "-a"}, fragment: "argument expected"},
		{name: "bad number", args: []string{"x", "-eq", "3"}, fragment: "bad number"},
		{name: "unclosed parenthesis", args: []string{"(", "x"}, fragment: "closing paren expected"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, stderr := runTestApplet(t, test.args...)

			// Then
			if status != 2 {
				t.Fatalf("test %q = %d, want 2 (stderr %q)", test.args, status, stderr)
			}
			if !strings.Contains(stderr, test.fragment) {
				t.Fatalf("test %q stderr = %q, want it to mention %q", test.args, stderr, test.fragment)
			}
		})
	}
}

func TestBracket_requiresItsClosingBracket(t *testing.T) {
	// Given
	applet, ok := applets.DefaultRegistry.Lookup("[")
	if !ok {
		t.Fatal("expected [ applet to be registered")
	}

	// When
	var closedErr, openErr bytes.Buffer
	closed := applet.Run(context.Background(), []string{"x", "]"}, &bytes.Buffer{}, &bytes.Buffer{}, &closedErr)
	open := applet.Run(context.Background(), []string{"x"}, &bytes.Buffer{}, &bytes.Buffer{}, &openErr)

	// Then
	if appletStatus(closed) != 0 {
		t.Fatalf("[ x ] = %d, want 0", appletStatus(closed))
	}
	if appletStatus(open) != 2 {
		t.Fatalf("[ x = %d, want 2", appletStatus(open))
	}
}
