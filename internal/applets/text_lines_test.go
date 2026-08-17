package applets_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// runFilter feeds stdin through an applet and returns what came out.
func runFilter(t *testing.T, name string, args []string, stdin string) (string, string, error) {
	t.Helper()
	applet, ok := applets.DefaultRegistry.Lookup(name)
	if !ok {
		t.Fatalf("%s is not registered", name)
	}
	var stdout, stderr bytes.Buffer
	err := applet.Run(context.Background(), args, strings.NewReader(stdin), &stdout, &stderr)
	return stdout.String(), stderr.String(), err
}

// The expectations here are what GNU coreutils printed for the same input on the
// machine this was written on. Not busybox: busybox's versions are the small
// ones, and the behaviour people rely on -- and compare published checksums
// against -- is GNU's.
func TestTac(t *testing.T) {
	tests := []struct {
		name  string
		stdin string
		want  string
	}{
		{name: "reverses lines", stdin: "one\ntwo\nthree\n", want: "three\ntwo\none\n"},
		{
			// The case worth pinning, and the one that is not obvious. GNU:
			//
			//	$ printf 'a\nb' | tac | od -c
			//	0000000   b   a  \n
			//
			// The line that had no newline still has none after moving to the
			// front. tac moves lines; it does not add separators.
			name: "keeps a missing final newline missing", stdin: "a\nb", want: "ba\n",
		},
		{name: "one line", stdin: "only\n", want: "only\n"},
		{name: "nothing at all", stdin: "", want: ""},
		{name: "keeps empty lines", stdin: "a\n\nb\n", want: "b\n\na\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			got, _, err := runFilter(t, "tac", nil, test.stdin)

			// Then
			if err != nil {
				t.Fatalf("tac: %v", err)
			}
			if got != test.want {
				t.Fatalf("tac(%q) = %q, want %q", test.stdin, got, test.want)
			}
		})
	}
}

func TestRev(t *testing.T) {
	tests := []struct {
		name  string
		stdin string
		want  string
	}{
		{name: "reverses each line", stdin: "abc\ndef\n", want: "cba\nfed\n"},
		{name: "nothing at all", stdin: "", want: ""},
		{name: "an empty line stays empty", stdin: "\n", want: "\n"},
		{
			// By rune, not by byte. util-linux's rev is byte-oriented in the C
			// locale and would cut these in half; reversing runes leaves valid
			// UTF-8, which is what anybody means by reversing a line.
			name: "reverses runes rather than bytes", stdin: "日本語\n", want: "語本日\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			got, _, err := runFilter(t, "rev", nil, test.stdin)

			// Then
			if err != nil {
				t.Fatalf("rev: %v", err)
			}
			if got != test.want {
				t.Fatalf("rev(%q) = %q, want %q", test.stdin, got, test.want)
			}
		})
	}
}

// nl's default surprises people, so it is the first case: GNU numbers only
// non-empty lines unless told otherwise.
//
//	$ printf 'a\n\nb\n' | nl
//	     1  a
//
//	     2  b
func TestNl(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		stdin string
		want  string
	}{
		{
			name:  "numbers only non-empty lines by default",
			stdin: "a\n\nb\n",
			want:  "     1\ta\n       \n     2\tb\n",
		},
		{
			name: "-ba numbers every line",
			args: []string{"-ba"}, stdin: "a\n\nb\n",
			want: "     1\ta\n     2\t\n     3\tb\n",
		},
		{
			name: "-bn numbers none",
			args: []string{"-bn"}, stdin: "a\nb\n",
			want: "       a\n       b\n",
		},
		{name: "nothing at all", stdin: "", want: ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			got, _, err := runFilter(t, "nl", test.args, test.stdin)

			// Then
			if err != nil {
				t.Fatalf("nl: %v", err)
			}
			if got != test.want {
				t.Fatalf("nl(%q) = %q, want %q", test.stdin, got, test.want)
			}
		})
	}
}

// A numbering style nl does not have is refused rather than treated as the
// default, because a script asking for one is asking for something specific.
func TestNl_refusesAnUnknownStyle(t *testing.T) {
	// When
	_, _, err := runFilter(t, "nl", []string{"-bz"}, "a\n")

	// Then
	if err == nil || !strings.Contains(err.Error(), "unsupported numbering style") {
		t.Fatalf("nl -bz = %v, want a refusal naming the style", err)
	}
}
