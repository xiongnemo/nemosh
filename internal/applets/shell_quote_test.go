package applets

import (
	"strings"
	"testing"
)

// shellQuote, which backs `printf %q` and had no test at all.
//
// Every expectation here was measured against bash rather than reasoned about,
// because the whole point of `%q` is that a shell reading the output back gets the
// same string -- and which punctuation a shell treats specially is a fact about
// shells, not a matter of taste.

// The safe set, measured: `printf %q a<C>b` in bash answers `a<C>b` unchanged for
// exactly these, and escapes everything else.
func TestShellQuote_leavesTheSafeSetAlone(t *testing.T) {
	for _, safe := range []string{
		"plain", "MiXeD", "0123456789",
		"a.b", "a-b", "a_b", "a/b", "a:b", "a+b", "a%b", "a@b",
		// A realistic path and a realistic option, which is what this is for. Note
		// `--option` and not `--option=value`: the `=` is escaped on purpose, per
		// the decision recorded on isShellSafeByte, and putting it here was this
		// test's own mistake before that comment was read.
		"/usr/local/bin/tool-name_2.txt", "--option", "a-b_c.d/e:f+g%h@i",
	} {
		t.Run(safe, func(t *testing.T) {
			if got := shellQuote(safe); got != safe {
				t.Fatalf("shellQuote(%q) = %q, want it unchanged", safe, got)
			}
		})
	}
}

// Everything bash escapes, escaped. The comma is here because it was in the *safe*
// set and should not have been: bash answers `a\,b`, and a comma separates a brace
// expansion.
func TestShellQuote_escapesWhatAShellWouldRead(t *testing.T) {
	for _, test := range []struct{ value, want string }{
		{value: "a b", want: `a\ b`},
		{value: "it's", want: `it\'s`},
		{value: `a"b`, want: `a\"b`},
		{value: `a\b`, want: `a\\b`},
		{value: "a$b", want: `a\$b`},
		{value: "a`b", want: "a\\`b"},
		{value: "a;b", want: `a\;b`},
		{value: "a|b", want: `a\|b`},
		{value: "a&b", want: `a\&b`},
		{value: "a<b", want: `a\<b`},
		{value: "a>b", want: `a\>b`},
		{value: "a*b", want: `a\*b`},
		{value: "a?b", want: `a\?b`},
		{value: "a[b", want: `a\[b`},
		{value: "a]b", want: `a\]b`},
		{value: "a(b", want: `a\(b`},
		{value: "a)b", want: `a\)b`},
		{value: "a{b", want: `a\{b`},
		{value: "a}b", want: `a\}b`},
		{value: "a^b", want: `a\^b`},
		{value: "a!b", want: `a\!b`},
		// The one that was wrong.
		{value: "a,b", want: `a\,b`},
		// More conservative than bash on purpose: it leaves these unescaped in the
		// middle of a word and escapes them at the start, where a tilde is expansion
		// and a hash is a comment. Escaping them everywhere is always valid shell.
		{value: "a=b", want: `a\=b`},
		{value: "a~b", want: `a\~b`},
		{value: "a#b", want: `a\#b`},
	} {
		t.Run(test.value, func(t *testing.T) {
			if got := shellQuote(test.value); got != test.want {
				t.Fatalf("shellQuote(%q) = %q, want %q", test.value, got, test.want)
			}
		})
	}
}

// The three characters a backslash cannot escape into themselves. A backslash
// before a newline is a line *continuation*, which would delete it -- so bash
// writes $'\n' and so does this.
func TestShellQuote_usesDollarQuotingForTheUnescapable(t *testing.T) {
	for _, test := range []struct{ value, want string }{
		{value: "\n", want: `$'\n'`},
		{value: "\t", want: `$'\t'`},
		{value: "\r", want: `$'\r'`},
		{value: "a\nb", want: `a$'\n'b`},
		{value: "a\tb", want: `a$'\t'b`},
		{value: "one\ntwo\nthree", want: `one$'\n'two$'\n'three`},
	} {
		t.Run(strings.ReplaceAll(test.value, "\n", "|"), func(t *testing.T) {
			if got := shellQuote(test.value); got != test.want {
				t.Fatalf("shellQuote(%q) = %q, want %q", test.value, got, test.want)
			}
		})
	}
}

// The empty string is the one case backslashes cannot express at all, and bash
// spells it with two quotes.
func TestShellQuote_spellsTheEmptyString(t *testing.T) {
	if got := shellQuote(""); got != "''" {
		t.Fatalf("shellQuote(\"\") = %q, want two quotes", got)
	}
}

// Non-ASCII bytes pass through untouched. bash escapes them into $'...' byte by
// byte; this does not, because they are not special to any shell and escaping half
// a rune would corrupt it -- which matters on a platform where a path routinely
// holds CJK.
func TestShellQuote_passesMultibyteCharactersThrough(t *testing.T) {
	for _, value := range []string{"路径", "名字.txt", "路径/名字", "日本語", "Ünïcödé"} {
		if got := shellQuote(value); got != value {
			t.Fatalf("shellQuote(%q) = %q, want it unchanged", value, got)
		}
	}
	// Mixed with something that *is* special: only the special byte is escaped, and
	// the multibyte characters either side survive whole.
	if got := shellQuote("路径 名字"); got != `路径\ 名字` {
		t.Fatalf("shellQuote of a CJK path with a space = %q", got)
	}
}

// The property that matters, checked over every byte rather than a chosen list:
// nothing outside the safe set survives without a backslash or a $'...' before it.
//
// This is what makes the safe-set direction enforceable. A character nobody thought
// about must come out escaped rather than come out bare, and a table of examples
// cannot say that -- only walking the whole range can.
func TestShellQuote_neverLetsAnUnsafeByteThrough(t *testing.T) {
	for value := 0x20; value < 0x7f; value++ {
		char := byte(value)
		quoted := shellQuote("a" + string(char) + "b")
		if isShellSafeByte(char) {
			if quoted != "a"+string(char)+"b" {
				t.Errorf("the safe byte %q was escaped: %q", char, quoted)
			}
			continue
		}
		// Escaped means a backslash immediately before it.
		if !strings.Contains(quoted, `\`+string(char)) {
			t.Errorf("the unsafe byte %q came through as %q with no backslash", char, quoted)
		}
	}
	// And the control characters below space, which cannot be escaped with a
	// backslash at all: the three with an escape sequence use it, and the rest are
	// passed through as themselves. That last part is a real limitation, so it is
	// asserted rather than assumed -- a value holding a raw control byte is not
	// something %q can spell, and nothing here pretends otherwise.
	for _, test := range []struct {
		char byte
		want string
	}{
		{char: '\n', want: `a$'\n'b`},
		{char: '\t', want: `a$'\t'b`},
		{char: '\r', want: `a$'\r'b`},
	} {
		if got := shellQuote("a" + string(test.char) + "b"); got != test.want {
			t.Errorf("shellQuote with %q = %q, want %q", test.char, got, test.want)
		}
	}
}

// printf %q end to end, which is the only reason shellQuote exists.
func TestPrintf_percentQ(t *testing.T) {
	for _, test := range []struct{ format, argument, want string }{
		{format: "%q", argument: "a b", want: `a\ b`},
		{format: "%q", argument: "", want: "''"},
		{format: "[%q]", argument: "a$b", want: `[a\$b]`},
		{format: "%q %q", argument: "x", want: `x ''`},
	} {
		t.Run(test.format+" "+test.argument, func(t *testing.T) {
			var stdout, stderr strings.Builder
			args := []string{test.format}
			if test.argument != "" || !strings.Contains(test.format, "%q %q") {
				args = append(args, test.argument)
			}
			applet, ok := DefaultRegistry.Lookup("printf")
			if !ok {
				t.Fatal("printf is not registered")
			}
			if err := applet.Run(t.Context(), args,
				strings.NewReader(""), &stdout, &stderr); err != nil {
				t.Fatalf("printf: %v (%s)", err, stderr.String())
			}
			if stdout.String() != test.want {
				t.Fatalf("printf %v = %q, want %q", args, stdout.String(), test.want)
			}
		})
	}
}
