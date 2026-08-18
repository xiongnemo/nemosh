package runtime_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Three pieces of syntax that a script written this decade assumes, each measured
// against bash.
//
//	$'...'   ANSI-C quoting -- expanded to the six characters `$a\tb` before
//	<<<      a here-string  -- reported the whole script incomplete
//	&> &>>   both streams   -- was a background command plus a redirect of nothing

func TestAnsiQuoting_decodesTheEscapes(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{name: "a tab", script: "printf '%s' $'a\\tb'\n", want: "a\tb"},
		{name: "a newline", script: "printf '%s' $'a\\nb'\n", want: "a\nb"},
		{
			// The reason this feature earns its keep: a colour code without going
			// through printf's own format.
			name: "an escape character", script: "printf '%s' $'\\e[1m'\n", want: "\x1b[1m",
		},
		{name: "the other spelling of escape", script: "printf '%s' $'\\E.'\n", want: "\x1b."},
		{name: "a carriage return", script: "printf '%s' $'a\\rb'\n", want: "a\rb"},
		{name: "a bell", script: "printf '%s' $'\\a'\n", want: "\a"},
		{name: "a vertical tab", script: "printf '%s' $'\\v'\n", want: "\v"},
		{name: "hex", script: "printf '%s' $'\\x41'\n", want: "A"},
		{name: "octal", script: "printf '%s' $'\\101'\n", want: "A"},
		{name: "octal with a leading zero", script: "printf '%s' $'\\0101'\n", want: "A"},
		{name: "a unicode code point", script: "printf '%s' $'\\u00e9'\n", want: "é"},
		{name: "a long unicode code point", script: "printf '%s' $'\\U0001F600'\n", want: "😀"},
		{name: "a control character", script: "printf '%s' $'\\cA'\n", want: "\x01"},
		{name: "an escaped backslash", script: "printf '%s' $'a\\\\b'\n", want: "a\\b"},
		{name: "an escaped single quote", script: "printf '%s' $'it\\'s'\n", want: "it's"},
		{
			// bash keeps the backslash for an escape it does not have, rather than
			// erroring or dropping it.
			name: "an escape that does not exist", script: "printf '%s' $'a\\qb'\n", want: "a\\qb",
		},
		{name: "no escapes at all", script: "printf '%s' $'plain'\n", want: "plain"},
		{name: "empty", script: "printf '[%s]' $''\n", want: "[]"},
		{
			// It is a quoted string, so this is one word rather than two.
			name: "a blank does not split the word", script: "printf '[%s]' $'a b'\n", want: "[a b]",
		},
		{
			// A parameter is not expanded inside it: `$'...'` is a single-quoted
			// string that understands escapes, not a double-quoted one.
			name: "a parameter is not expanded", script: "x=v\nprintf '%s' $'$x'\n", want: "$x",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, stdout, stderr := runSetScript(t, test.script)

			// Then
			if status != 0 {
				t.Fatalf("status = %d, stderr = %q", status, stderr)
			}
			if stdout != test.want {
				t.Fatalf("%q printed %q, want %q", test.script, stdout, test.want)
			}
		})
	}
}

// The forms that must keep meaning what they meant. `$'` is only special unquoted,
// and an ordinary single-quoted string still takes no escapes at all.
func TestAnsiQuoting_leavesTheOtherQuotingAlone(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{
			// Measured: bash prints `$'a'` here, because inside double quotes the
			// dollar-quote is not special.
			name: "inside double quotes it is literal", script: "printf '%s' \"$'a'\"\n", want: "$'a'",
		},
		{name: "a plain single-quoted string", script: "printf '%s' 'a\\tb'\n", want: "a\\tb"},
		{name: "a dollar and a variable", script: "x=v\nprintf '%s' \"$x\"\n", want: "v"},
		{name: "a lone dollar", script: "printf '%s' '$'\n", want: "$"},
		{name: "arithmetic is untouched", script: "printf '%s' $((1+1))\n", want: "2"},
		{name: "command substitution is untouched", script: "printf '%s' $(echo x)\n", want: "x"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, stdout, stderr := runSetScript(t, test.script)

			// Then
			if status != 0 {
				t.Fatalf("status = %d, stderr = %q", status, stderr)
			}
			if stdout != test.want {
				t.Fatalf("%q printed %q, want %q", test.script, stdout, test.want)
			}
		})
	}
}

func TestHereString_feedsOneWordToStdin(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{name: "a literal", script: "cat <<< \"a b\"\n", want: "a b\n"},
		{name: "a parameter", script: "x=hi\ncat <<< \"$x there\"\n", want: "hi there\n"},
		{
			// The trailing newline is part of the form: without it `read` would see
			// end of input and report failure.
			name: "read gets a whole line", script: "read v <<< \"one two\"\necho \"[$v]\"\n",
			want: "[one two]\n",
		},
		{
			// Unquoted, and still one word: a here-string's operand is a value, not
			// a list of arguments.
			name: "an unquoted parameter is not split", script: "x=\"a b\"\ncat <<< $x\n", want: "a b\n",
		},
		{name: "a command substitution", script: "cat <<< $(echo nested)\n", want: "nested\n"},
		{name: "empty", script: "cat <<< \"\"\n", want: "\n"},
		{name: "with wc", script: "wc -l <<< \"one\"\n", want: "1\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, stdout, stderr := runSetScript(t, test.script)

			// Then
			if status != 0 {
				t.Fatalf("status = %d, stderr = %q", status, stderr)
			}
			if stdout != test.want {
				t.Fatalf("%q printed %q, want %q", test.script, stdout, test.want)
			}
		})
	}
}

// The heredoc forms have to keep working, because `<<<` was being read as `<<`
// followed by a stray `<` and the scanner that found the delimiter is the one that
// had to learn the difference.
func TestHereString_doesNotDisturbHeredocs(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{name: "a heredoc", script: "cat <<EOF\nline\nEOF\n", want: "line\n"},
		{name: "a tab-stripping heredoc", script: "cat <<-EOF\n\tline\n\tEOF\n", want: "line\n"},
		{
			name: "a quoted delimiter takes no expansion", script: "x=v\ncat <<'EOF'\n$x\nEOF\n",
			want: "$x\n",
		},
		{name: "an expanding heredoc", script: "x=v\ncat <<EOF\n$x\nEOF\n", want: "v\n"},
		{
			// The shift operator carries a `<<` that is not a heredoc, which the
			// scanner already knew and must go on knowing.
			name: "a left shift", script: "echo $((1<<4))\n", want: "16\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, stdout, stderr := runSetScript(t, test.script)

			// Then
			if status != 0 {
				t.Fatalf("status = %d, stderr = %q", status, stderr)
			}
			if stdout != test.want {
				t.Fatalf("%q printed %q, want %q", test.script, stdout, test.want)
			}
		})
	}
}

// `&>` sends both streams to one place. It has to be recognised before the bare `&`,
// or `cmd &> log` is a background `cmd` followed by a redirect of nothing.
func TestBothStreamsRedirect_takesStdoutAndStderr(t *testing.T) {
	directory := t.TempDir()
	path := filepath.ToSlash(filepath.Join(directory, "out.txt"))

	t.Run("truncating", func(t *testing.T) {
		// When -- one line on stdout and one on stderr.
		status, _, stderr := runSetScript(t,
			"printf 'to-stdout\\n'; printf 'to-stderr\\n' >&2\n")
		if status != 0 {
			t.Fatalf("the fixture itself failed: %q", stderr)
		}
		status, _, stderr = runSetScript(t,
			"{ printf 'to-stdout\\n'; printf 'to-stderr\\n' >&2; } &> "+path+"\n")

		// Then
		if status != 0 {
			t.Fatalf("status = %d, stderr = %q", status, stderr)
		}
		contents, err := os.ReadFile(filepath.FromSlash(path))
		if err != nil {
			t.Fatalf("read the redirect target: %v", err)
		}
		for _, want := range []string{"to-stdout", "to-stderr"} {
			if !strings.Contains(string(contents), want) {
				t.Fatalf("file holds %q, want it to hold %q", contents, want)
			}
		}
	})

	t.Run("appending", func(t *testing.T) {
		// When
		status, _, stderr := runSetScript(t, "printf 'first\\n' > "+path+"\nprintf 'second\\n' &>> "+path+"\n")

		// Then
		if status != 0 {
			t.Fatalf("status = %d, stderr = %q", status, stderr)
		}
		contents, err := os.ReadFile(filepath.FromSlash(path))
		if err != nil {
			t.Fatalf("read the redirect target: %v", err)
		}
		if want := "first\nsecond\n"; string(contents) != want {
			t.Fatalf("file holds %q, want %q -- &>> must append", contents, want)
		}
	})
}

// The bare `&` still backgrounds, and `&&` is still and-if. Both live one character
// away from the new operator.
func TestBothStreamsRedirect_leavesTheAmpersandOperatorsAlone(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{name: "background then wait", script: "echo bg &\nwait\n", want: "bg\n"},
		{name: "and-if", script: "true && echo yes\n", want: "yes\n"},
		{name: "or-if", script: "false || echo no\n", want: "no\n"},
		{name: "dup to stdout", script: "printf 'x\\n' 2>&1\n", want: "x\n"},
		{
			// The long-hand spelling of what `&>` does, which must still work.
			name: "redirect then dup", script: "{ printf 'a\\n'; printf 'b\\n' >&2; } 2>&1\n", want: "a\nb\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, stdout, stderr := runSetScript(t, test.script)

			// Then
			if status != 0 {
				t.Fatalf("status = %d, stderr = %q", status, stderr)
			}
			if stdout != test.want {
				t.Fatalf("%q printed %q, want %q", test.script, stdout, test.want)
			}
		})
	}
}
