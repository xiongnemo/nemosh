package runtime_test

import "testing"

// `function name { body; }` reported `unsupported syntax: function`, and it is the
// spelling a large share of shell scripts use.
//
// All three forms bash accepts now work: the POSIX one, the keyword without
// parentheses, and both together.
func TestFunctionKeyword_definesAFunction(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{name: "the keyword alone", script: "function g { echo fn; }\ng\n", want: "fn\n"},
		{name: "the keyword with parentheses", script: "function g() { echo fn; }\ng\n", want: "fn\n"},
		{name: "the POSIX form", script: "g() { echo fn; }\ng\n", want: "fn\n"},
		{name: "arguments reach it", script: "function g { echo \"$1-$2\"; }\ng a b\n", want: "a-b\n"},
		{name: "the argument count", script: "function g { echo $#; }\ng a b c\n", want: "3\n"},
		{name: "a return status", script: "function g { return 3; }\ng\necho $?\n", want: "3\n"},
		{name: "two statements in the body", script: "function g { echo a; echo b; }\ng\n", want: "a\nb\n"},
		{
			name:   "written across lines",
			script: "function g {\n  echo one\n  echo two\n}\ng\n", want: "one\ntwo\n",
		},
		{name: "a local", script: "function g { local v=1; echo $v; }\ng\necho \"[$v]\"\n", want: "1\n[]\n"},
		{
			name:   "calling another function",
			script: "function inner { echo deep; }\nfunction outer { inner; }\nouter\n", want: "deep\n",
		},
		{
			name:   "redefining",
			script: "function g { echo first; }\nfunction g { echo second; }\ng\n", want: "second\n",
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

// A `{` after a bare word is data everywhere except after `function name`, and that is
// the distinction the brace scanner had to learn. These are the forms that would break
// if it had learned it too broadly.
func TestFunctionKeyword_leavesBracesAlone(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{name: "a brace as an argument", script: "echo {\n", want: "{\n"},
		{name: "a quoted brace", script: "echo \"{\"\n", want: "{\n"},
		{name: "a brace group", script: "{ echo group; }\n", want: "group\n"},
		{name: "brace expansion", script: "echo {a,b}\n", want: "a b\n"},
		{
			// A name that merely begins with the keyword is not the keyword.
			name:   "a command whose name starts with function",
			script: "functional() { echo no; }\nfunctional\n", want: "no\n",
		},
		{name: "the word function as an argument", script: "echo function\n", want: "function\n"},
		{name: "a variable called function", script: "function=1\necho $function\n", want: "1\n"},
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
