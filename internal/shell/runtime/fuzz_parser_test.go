package runtime

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// Parsing is the surface fuzzing is for. It takes arbitrary text from a user,
// runs before anything is executed, and the defects it has produced were all of
// the shape a fuzzer finds: a brace read as reserved outside command position, a
// `<<` inside `$((1<<4))` taken for a heredoc, a `))` counted twice. None was
// found by a test someone thought to write.
//
// ParseScript is pure -- it builds a program and runs none of it -- so a fuzz
// corpus costs nothing but time.
func FuzzParseScript(f *testing.F) {
	for _, seed := range []string{
		"echo hi",
		"f(){ echo a; }; f",
		"{ echo $((1+2)); }",
		"echo a}b",
		"if true; then echo x; fi",
		"for i in 1 2; do echo $i; done",
		"case x in x) echo y ;; esac",
		"cat <<EOF\nbody\nEOF",
		"a | b && c || d",
		"echo `date`",
		"echo ${x:-default}",
		"echo $((1<<4))",
		"while :; do break; done",
		"echo 'unterminated",
		"((((((((((",
		"$(($(($(($((1))))))))",
		"\\\n\\\n\\\n",
		"echo \"$(echo \"$(echo x)\")\"",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, source string) {
		// Invalid UTF-8 is a separate question from parsing, and Go's fuzzer
		// generates a great deal of it.
		if !utf8.ValidString(source) {
			t.Skip()
		}
		// A parser that recurses on nesting can be made to exhaust the stack by
		// input alone, which is a real defect but needs its own bound to test;
		// the depth limit is what protects it, and this keeps the corpus inside
		// what that limit was chosen for.
		if len(source) > 4096 {
			t.Skip()
		}

		_, err := ParseScript(source)

		// A parser has two honest outcomes: a program, or an error saying why
		// not. An error with nothing in it is neither.
		//
		// An empty program is *not* a defect, which this asserted at first and
		// the fuzzer disproved in a tenth of a second with `#000000000`: a
		// script that is only a comment is valid, produces no commands, and
		// exits zero, in busybox too. The corpus entry is kept as a seed.
		if err != nil {
			if err.Error() == "" {
				t.Fatalf("ParseScript(%q) failed with an empty message", source)
			}
			return
		}

		// A complete script stays complete when a newline is added after it --
		// unless it ends in a carriage return, where appending one turns that CR
		// from data into half of a line terminator. That asymmetry is not a
		// defect and is not ours alone: measured, busybox runs `0|<CR>` and
		// rejects `0|<CRLF>`, exactly as this does. dash treats both as data and
		// bash rejects both, so the references do not agree either.
		if strings.HasSuffix(source, "\r") {
			return
		}
		if _, err := ParseScript(source + "\n"); err != nil {
			t.Fatalf("ParseScript(%q) succeeded but ParseScript(%q) failed: %v", source, source+"\n", err)
		}
	})
}

// Parsing twice must give the same answer. A parser that depends on state left
// behind by the previous call is a defect that only shows under load.
func FuzzParseScript_isDeterministic(f *testing.F) {
	for _, seed := range []string{"echo hi", "{ echo a; }", "f(){ :; }", "echo $((1+1))"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, source string) {
		if !utf8.ValidString(source) || len(source) > 2048 {
			t.Skip()
		}
		first, firstErr := ParseScript(source)
		second, secondErr := ParseScript(source)
		if (firstErr == nil) != (secondErr == nil) {
			t.Fatalf("ParseScript(%q) failed once and succeeded once", source)
		}
		if firstErr != nil && firstErr.Error() != secondErr.Error() {
			t.Fatalf("ParseScript(%q) gave two different errors: %v then %v", source, firstErr, secondErr)
		}
		if firstErr == nil && len(first.program) != len(second.program) {
			t.Fatalf("ParseScript(%q) gave programs of length %d then %d", source, len(first.program), len(second.program))
		}
	})
}

// Pattern matching is the other pure surface with a history of defects, and it
// is reached by user text from three directions: case arms, parameter trimming,
// and globbing.
func FuzzMatchShellPattern(f *testing.F) {
	for _, seed := range [][2]string{
		{"*", "anything"},
		{"a*b", "axxb"},
		{"[abc]", "b"},
		{"[!abc]", "z"},
		{"[a-z]*", "hello"},
		{"\\*", "*"},
		{"[", "["},
		{"[]]", "]"},
		{"a?c", "abc"},
		{"", ""},
		{"[z-a]", "m"},
		{"你*", "你好"},
	} {
		f.Add(seed[0], seed[1])
	}
	f.Fuzz(func(t *testing.T, pattern, value string) {
		if !utf8.ValidString(pattern) || !utf8.ValidString(value) {
			t.Skip()
		}
		if len(pattern) > 512 || len(value) > 512 {
			t.Skip()
		}

		matched := matchShellPattern(pattern, value)

		// A pattern with no metacharacter is a literal, and that is checkable
		// independently of how the matcher works.
		if !strings.ContainsAny(pattern, `*?[\`) {
			if matched != (pattern == value) {
				t.Fatalf("matchShellPattern(%q, %q) = %v, want %v for a literal", pattern, value, matched, pattern == value)
			}
		}
	})
}
