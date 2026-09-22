package main

import (
	"strings"
	"testing"
)

// **What the line on screen says about itself.**
//
// Reported by somebody typing the one-liner that had just been fixed in the parser:
//
//	a() { case a in a) echo bingo;; *) echo hmm;; esac; }
//
// It drew `a()` red, `case` red, and every single thing after the first pattern plain. Red
// is this palette's "no such command", so the line made two false claims -- about a
// function being defined, and about a reserved word -- and then said nothing at all about
// the two `echo`s it contains.
//
// The cause was one thing: the highlighter split on blanks, and the shell's lexer does not.
// `bingo;;` is one word to a blank-splitter, so the `;;` in it was invisible and the
// command position never came back. Everything downstream of that was a consequence.
//
// What the roles mean, and it is worth saying because two of them are new:
//
//   - a **command** is green when this shell can run it, red when it cannot. That is a
//     claim about the world, so it is only made about words that really are command names.
//   - a **reserved word** is its own colour. It is not a command and saying "no such
//     command" about `case` is worse than saying nothing.
//   - a **definition** -- the `name` of `name()` -- is its own colour too. It is not
//     runnable *yet*, and it is not unknown either.
//   - an **operator** is drawn plainly. It is punctuation; the eye finds it anyway. `{`
//     and `}` go with it even though POSIX calls them reserved words: colouring them while
//     `(` and `)` sit beside them uncoloured is a distinction a reader gains nothing from.
//   - a **case pattern** is data, so `*)` is plain. It reads as a command position and is
//     not one, which is the last place this drew a colour that said something false.

// spanKinds renders the spans as a list of "text:role" for a readable failure.
func spanKinds(spans []span, colours palette) []string {
	role := func(codes []string) string {
		joined := strings.Join(codes, ",")
		switch {
		case joined == "":
			return "plain"
		case joined == strings.Join(colours.knownCommand, ","):
			return "command"
		case joined == strings.Join(colours.unknownCommand, ","):
			return "no-such-command"
		case joined == strings.Join(colours.reservedWord, ","):
			return "keyword"
		case joined == strings.Join(colours.definition, ","):
			return "definition"
		case joined == strings.Join(colours.knownOption, ","):
			return "option"
		case joined == strings.Join(colours.unknownOption, ","):
			return "unknown-option"
		}
		return "other(" + joined + ")"
	}
	var out []string
	for _, item := range spans {
		if strings.TrimSpace(item.text) == "" {
			continue
		}
		out = append(out, item.text+":"+role(item.codes))
	}
	return out
}

// testOracle knows `echo`, `true` and `cat`, and nothing else -- so anything else drawn as
// a command comes out as "no-such-command" and is visible in the expectation.
func testOracle(name string) commandStanding {
	switch name {
	case "echo", "true", "cat":
		return standingRunnable
	}
	return standingUnknown
}

func TestHighlight_readsTheLineTheWayTheShellDoes(t *testing.T) {
	colours := defaultPalette()
	for _, testcase := range []struct {
		name string
		line string
		want []string
	}{
		{
			name: "the reported one-liner",
			line: "a() { case a in a) echo bingo;; *) echo hmm;; esac; }",
			want: []string{
				"a:definition", "(:plain", "):plain", "{:plain",
				"case:keyword", "a:plain", "in:keyword", "a:plain", "):plain",
				"echo:command", "bingo:plain", ";;:plain",
				"*:plain", "):plain",
				"echo:command", "hmm:plain", ";;:plain",
				"esac:keyword", ";:plain", "}:plain",
			},
		},
		{
			name: "a command after a case pattern",
			line: "case a in a) echo one;; esac",
			want: []string{
				"case:keyword", "a:plain", "in:keyword", "a:plain", "):plain",
				"echo:command", "one:plain", ";;:plain", "esac:keyword",
			},
		},
		{
			name: "a command after then",
			line: "if true; then echo yes; fi",
			want: []string{
				"if:keyword", "true:command", ";:plain", "then:keyword",
				"echo:command", "yes:plain", ";:plain", "fi:keyword",
			},
		},
		{
			name: "a command after a pipe with no blank",
			line: "echo one|cat",
			want: []string{"echo:command", "one:plain", "|:plain", "cat:command"},
		},
		{
			name: "a command after a semicolon with no blank",
			line: "true;cat",
			want: []string{"true:command", ";:plain", "cat:command"},
		},
		{
			name: "an unknown command is still called out",
			line: "nosuchthing arg",
			want: []string{"nosuchthing:no-such-command", "arg:plain"},
		},
		{
			name: "a command after do",
			line: "for i in 1; do echo $i; done",
			want: []string{
				"for:keyword", "i:plain", "in:keyword", "1:plain", ";:plain", "do:keyword",
				"echo:command", "$i:plain", ";:plain", "done:keyword",
			},
		},
		{
			name: "a quoted separator separates nothing",
			line: `echo ";" cat`,
			want: []string{"echo:command", `";":plain`, "cat:plain"},
		},
		{
			name: "a brace expansion is a word, not punctuation",
			line: "echo {a,b}",
			want: []string{"echo:command", "{a,b}:plain"},
		},
		{
			name: "a parameter in braces is a word",
			line: "echo ${x}",
			want: []string{"echo:command", "${x}:plain"},
		},
		{
			name: "a subshell opens a command position",
			line: "(cat)",
			want: []string{"(:plain", "cat:command", "):plain"},
		},
		{
			name: "a function defined with a blank before the parentheses",
			line: "a () { true; }",
			want: []string{
				"a:definition", "(:plain", "):plain", "{:plain",
				"true:command", ";:plain", "}:plain",
			},
		},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			got := spanKinds(highlight(testcase.line, -1, colours, testOracle), colours)
			if strings.Join(got, " ") != strings.Join(testcase.want, " ") {
				t.Errorf("line %q drew\n  %s\nwant\n  %s", testcase.line,
					strings.Join(got, " "), strings.Join(testcase.want, " "))
			}
		})
	}
}

// TestHighlight_keepsEveryCharacter is the invariant that matters more than any colour: the
// spans concatenate back to the line. A tokeniser that drops or duplicates a character
// would corrupt what is on screen, and it would do it while the line is being typed.
func TestHighlight_keepsEveryCharacter(t *testing.T) {
	for _, line := range []string{
		"a() { case a in a) echo bingo;; *) echo hmm;; esac; }",
		"echo one|cat",
		`echo ";" 'x' "y z" \\ {a,b} ${x} $((1+2))`,
		"echo <<-EOF >>out 2>&1 <>rw >|clobber",
		"   leading and   inner   blanks   ",
		"echo 'unterminated",
		`echo "half`,
		"echo tail\\",
		"",
		"你好 世界 | cat",
	} {
		var rebuilt strings.Builder
		for _, item := range highlight(line, -1, defaultPalette(), testOracle) {
			rebuilt.WriteString(item.text)
		}
		if rebuilt.String() != line {
			t.Errorf("spans rebuilt %q from %q", rebuilt.String(), line)
		}
	}
}
