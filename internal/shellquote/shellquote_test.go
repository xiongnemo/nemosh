package shellquote

import (
	"strings"
	"testing"
)

// Every expectation was measured on bash 5.3.15 in a UTF-8 locale: `printf %q`, `${v@Q}`,
// `declare -p v`, and the key of `declare -A m; m[$v]=x; declare -p m`. The emoji is the
// exception: MSYS's bash has a 16-bit wchar_t and cannot see one as printable, and bash on
// Linux, where the Oils expectations were recorded, prints it as itself.
func TestQuoting_asBashWritesIt(t *testing.T) {
	tests := []struct {
		value, backslash, single, double, key string
	}{
		{"plain", "plain", "'plain'", `"plain"`, "plain"},
		{"a b", `a\ b`, "'a b'", `"a b"`, `"a b"`},
		{"it's", `it\'s`, `'it'\''s'`, `"it's"`, `"it's"`},
		{`a"b`, `a\"b`, `'a"b'`, `"a\"b"`, `"a\"b"`},
		{`a\b`, `a\\b`, `'a\b'`, `"a\\b"`, `"a\\b"`},
		{"$x", `\$x`, "'$x'", `"\$x"`, `"\$x"`},
		{"`x`", "\\`x\\`", "'`x`'", "\"\\`x\\`\"", "\"\\`x\\`\""},
		{"!x", `\!x`, "'!x'", `"!x"`, `"!x"`},
		{"é", "é", "'é'", `"é"`, "é"},
		{"é b", `é\ b`, "'é b'", `"é b"`, `"é b"`},
		{"\u00a0", "\u00a0", "'\u00a0'", "\"\u00a0\"", "\u00a0"},
		{"😀", "😀", "'😀'", `"😀"`, "😀"},
		{"a=~", `a=\~`, "'a=~'", `"a=~"`, `"a=~"`},
		{"a:~b", `a:\~b`, "'a:~b'", `"a:~b"`, `"a:~b"`},
		{"~x", `\~x`, "'~x'", `"~x"`, `"~x"`},
		{"#x", `\#x`, "'#x'", `"#x"`, `"#x"`},
		{"x#", "x#", "'x#'", `"x#"`, "x#"},
		{"a,b", `a\,b`, "'a,b'", `"a,b"`, "a,b"},
		{"*", `\*`, "'*'", `"*"`, `"*"`},
		{"@", "@", "'@'", `"@"`, `"@"`},
		{"a]b", `a\]b`, "'a]b'", `"a]b"`, `"a]b"`},
		{"{a}", `\{a\}`, "'{a}'", `"{a}"`, `"{a}"`},
		{"-", "-", "'-'", `"-"`, "-"},
		{"=", "=", "'='", `"="`, "="},
		{"%", "%", "'%'", `"%"`, "%"},
		{"^x", `\^x`, "'^x'", `"^x"`, `"^x"`},
		{"a|b", `a\|b`, "'a|b'", `"a|b"`, `"a|b"`},
		{";", `\;`, "';'", `";"`, `";"`},
		{"&", `\&`, "'&'", `"&"`, `"&"`},
		{"<>", `\<\>`, "'<>'", `"<>"`, `"<>"`},
		{"()", `\(\)`, "'()'", `"()"`, `"()"`},
		{"?", `\?`, "'?'", `"?"`, `"?"`},
		{"[", `\[`, "'['", `"["`, `"["`},
		{"x y z", `x\ y\ z`, "'x y z'", `"x y z"`, `"x y z"`},
		{`\`, `\\`, `'\'`, `"\\"`, `"\\"`},
		{"'", `\'`, `\'`, `"'"`, `"'"`},
		{`"`, `\"`, `'"'`, `"\""`, `"\""`},
		{" ", `\ `, "' '", `" "`, `" "`},
		{"a b\"c$d`e\\f!g", "a\\ b\\\"c\\$d\\`e\\\\f\\!g", "'a b\"c$d`e\\f!g'", "\"a b\\\"c\\$d\\`e\\\\f!g\"", "\"a b\\\"c\\$d\\`e\\\\f!g\""},
	}
	for _, test := range tests {
		t.Run(test.value, func(t *testing.T) {
			if got := Backslash(test.value); got != test.backslash {
				t.Errorf("Backslash = %s, want %s", got, test.backslash)
			}
			if got := Single(test.value); got != test.single {
				t.Errorf("Single = %s, want %s", got, test.single)
			}
			if got := Double(test.value); got != test.double {
				t.Errorf("Double = %s, want %s", got, test.double)
			}
			if got := Key(test.value); got != test.key {
				t.Errorf("Key = %s, want %s", got, test.key)
			}
		})
	}
}

// A string that needs $'...' gets it whole, in every form, and each form answers the same.
func TestQuoting_dollarQuotesWhatNoOtherQuotingHolds(t *testing.T) {
	for value, want := range map[string]string{
		"a\nb":         `$'a\nb'`,
		"\t":           `$'\t'`,
		"\r":           `$'\r'`,
		"\a\b\f\v":     `$'\a\b\f\v'`,
		"\x1b":         `$'\E'`,
		"\x01":         `$'\001'`,
		"\x7f":         `$'\177'`,
		"it's\n":       `$'it\'s\n'`,
		"a\x01'b":      `$'a\001\'b'`,
		"a\\b\n":       `$'a\\b\n'`,
		"\xff":         `$'\377'`,
		"\xce":         `$'\316'`,
		"\xce\xce\xbc": `$'\316μ'`,
		"μ\xce":        `$'μ\316'`,
		"\u200b":       `$'\342\200\213'`,
		"\u0085":       `$'\302\205'`,
		"one\ntwo μ":   `$'one\ntwo μ'`,
	} {
		for name, quote := range map[string]func(string) string{"Backslash": Backslash, "Single": Single, "Double": Double, "Key": Key} {
			if got := quote(value); got != want {
				t.Errorf("%s(%q) = %s, want %s", name, value, got, want)
			}
		}
	}
}

// A bare `declare` and bash's `set` quote only a value that needs it, measured.
func TestReusable_quotesOnlyWhatNeedsIt(t *testing.T) {
	for value, want := range map[string]string{
		"plain": "plain", "": "", "a b": "'a b'", "it's": `'it'\''s'`, "a\tb": `$'a\tb'`,
		"a,b": "a,b", "x=1": "x=1", "~x": "'~x'", "#x": "'#x'", "x#": "x#", "$x": "'$x'",
	} {
		if got := Reusable(value); got != want {
			t.Errorf("Reusable(%q) = %s, want %s", value, got, want)
		}
	}
}

// The empty string, which nothing but a pair of quotes can write.
func TestQuoting_theEmptyString(t *testing.T) {
	if got := Backslash(""); got != "''" {
		t.Errorf("Backslash = %s", got)
	}
	if got := Single(""); got != "''" {
		t.Errorf("Single = %s", got)
	}
	if got := Double(""); got != `""` {
		t.Errorf("Double = %s", got)
	}
}

// Over every printable ASCII byte between two letters, `printf %q` escapes exactly the ones
// bash does and lets the rest through. A table of examples cannot say that nothing else
// slips by; walking the range can.
func TestBackslash_escapesExactlyWhatBashDoes(t *testing.T) {
	const escaped = " !\"$&'()*,;<>?[\\]^`{|}"
	for value := 0x20; value < 0x7f; value++ {
		char := string(rune(value))
		want := "a" + char + "b"
		if strings.Contains(escaped, char) {
			want = `a\` + char + "b"
		}
		if got := Backslash("a" + char + "b"); got != want {
			t.Errorf("Backslash(%q) = %s, want %s", "a"+char+"b", got, want)
		}
	}
}
