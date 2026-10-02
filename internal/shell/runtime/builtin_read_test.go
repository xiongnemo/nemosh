package runtime_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// `read` used to take no options at all, which was not a missing feature but a
// silent wrong answer: `read -r line` assigned the line to a variable named `-r`
// and left `line` empty, so `while read -r line` looped the right number of times
// and handed back nothing. And `read a b c` put the whole line in `a`, which is
// POSIX field splitting simply not happening.
//
// Every expectation below was measured against bash on the machine this was
// written on. Where bash and POSIX could differ the bash answer is taken, because
// bash is what the scripts people paste were written against.
func runReadScript(t *testing.T, stdin, source string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{
		Stdin: strings.NewReader(stdin), Stdout: &stdout, Stderr: &stderr,
	})
	return rt.RunScript(context.Background(), source), stdout.String(), stderr.String()
}

func TestRead_splitsFieldsAcrossNames(t *testing.T) {
	tests := []struct {
		name   string
		stdin  string
		script string
		want   string
	}{
		{
			// The whole point: the last name takes the remainder, separators and
			// all. Measured: a=one b=two three.
			name: "the last name takes the rest", stdin: "one two three\n",
			script: "read a b\necho \"[$a][$b]\"\n", want: "[one][two three]\n",
		},
		{
			name: "more names than fields leaves the rest empty", stdin: "one two\n",
			script: "read a b c\necho \"[$a][$b][$c]\"\n", want: "[one][two][]\n",
		},
		{
			// Leading and trailing IFS whitespace goes; an inner run stays,
			// because it is inside the remainder rather than a delimiter at the
			// edge. Measured: b holds `b  c` with both spaces.
			name: "surrounding blanks are trimmed and inner runs kept", stdin: "  a  b  c  \n",
			script: "read a b\necho \"[$a][$b]\"\n", want: "[a][b  c]\n",
		},
		{
			name: "a single name takes the trimmed line", stdin: "  a  b  c  \n",
			script: "read x\necho \"[$x]\"\n", want: "[a  b  c]\n",
		},
		{
			// A non-whitespace IFS keeps empty fields, which is the difference
			// that makes `IFS=:` usable on /etc/passwd-shaped data.
			name: "a custom separator keeps empty fields", stdin: "a::b\n",
			script: "IFS=: read a b c\necho \"[$a][$b][$c]\"\n", want: "[a][][b]\n",
		},
		{
			name: "the remainder keeps its separators", stdin: "a:b:c:d\n",
			script: "IFS=: read a b\necho \"[$a][$b]\"\n", want: "[a][b:c:d]\n",
		},
		{
			name: "leading and trailing separators make empty fields", stdin: ":a:\n",
			script: "IFS=: read a b c\necho \"[$a][$b][$c]\"\n", want: "[][a][]\n",
		},
		{
			// IFS empty is not "default IFS"; it turns splitting off, and with it
			// the trimming.
			name: "an empty IFS splits nothing and trims nothing", stdin: " a b \n",
			script: "IFS= read x\necho \"[$x]\"\n", want: "[ a b ]\n",
		},
		{
			// No names at all: bash documents REPLY as the line "otherwise
			// unmodified", so not even the blanks go.
			name: "no names fills REPLY unmodified", stdin: "  a  b  \n",
			script: "read\necho \"[$REPLY]\"\n", want: "[  a  b  ]\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, stdout, stderr := runReadScript(t, test.stdin, test.script)

			// Then
			if status != 0 {
				t.Fatalf("status = %d, stderr = %q", status, stderr)
			}
			if stdout != test.want {
				t.Fatalf("stdout = %q, want %q", stdout, test.want)
			}
		})
	}
}

// IFS whitespace beside a separator that is not whitespace is part of the one delimiter, the
// last name gets a lone field left without the delimiter after it, `-a` makes no empty field of
// a delimiter at the end nor any of an empty line, and a backslash before a newline continues
// the line under `-d` too. busybox-w32 and bash answer every row the same, `-a` aside, which
// busybox has not got.
func TestRead_splitsAsBothReferencesDo(t *testing.T) {
	const pair, triple = "echo \"[$a][$b]\"\n", "echo \"[$a][$b][$c]\"\n"
	const array = "printf '[%s]' \"${a[@]}\"; echo \" ${#a[@]}\"\n"
	for _, test := range []struct{ stdin, script, want string }{
		{"a:b:\n", "IFS=: read a b\n" + pair, "[a][b]\n"},
		{"a:b:c:\n", "IFS=: read a b\n" + pair, "[a][b:c:]\n"},
		{"a : b :  \n", "IFS=': ' read a b\n" + pair, "[a][b]\n"},
		{"a :: b\n", "IFS=': ' read a b c\n" + triple, "[a][][b]\n"},
		{"xx\n", "IFS='x ' read a b\n" + pair, "[][]\n"},
		{"xxx\n", "IFS='x ' read a b\n" + pair, "[][xx]\n"},
		{"xaxx  \n", "IFS='x ' read a b\n" + pair, "[][axx]\n"},
		{"a x b\n", "IFS='x ' read -a a\n" + array, "[a][b] 2\n"},
		{"a xx b\n", "IFS='x ' read -a a\n" + array, "[a][][b] 3\n"},
		{"a b x x\n", "IFS='x ' read -a a\n" + array, "[a][b][] 3\n"},
		{"a:b:\n", "IFS=: read -a a\n" + array, "[a][b] 2\n"},
		{"\n", "read -a a\n" + array, "[] 0\n"},
		{"\n", "IFS= read -a a\n" + array, "[] 0\n"},
		{"a b\\\nc d\n", "read -d ,; printf '[%s]\\n' \"$REPLY\"\n", "[a bc d\n]\n"},
		{"a b\\,c d\n", "read -d ,; printf '[%s]\\n' \"$REPLY\"\n", "[a b,c d\n]\n"},
		// Names after -a are checked and ignored, as bash 5.3 ignores them.
		{"a b c\\ d\n", "read -r -a a x y\n" + array + "echo \"[$x$y]\"\n", "[a][b][c\\][d] 4\n[]\n"},
	} {
		if status, stdout, stderr := runReadScript(t, test.stdin, test.script); stdout != test.want || status != 0 {
			t.Errorf("%q over %q: got %q/%d %q, want %q/0", test.script, test.stdin, stdout, status, stderr, test.want)
		}
	}
}

func TestRead_handlesBackslashesAndTheRawOption(t *testing.T) {
	tests := []struct {
		name   string
		stdin  string
		script string
		want   string
	}{
		{
			name: "a backslash is removed and the next character kept", stdin: "a\\bc\n",
			script: "read x\necho \"[$x]\"\n", want: "[abc]\n",
		},
		{
			name: "-r keeps the backslash", stdin: "a\\bc\n",
			script: "read -r x\necho \"[$x]\"\n", want: "[a\\bc]\n",
		},
		{
			// The idiom this whole change exists for.
			name: "while read -r line reads the lines", stdin: "one\ntwo\n",
			script: "while read -r line; do echo \"<$line>\"; done\n", want: "<one>\n<two>\n",
		},
		{
			name: "a trailing backslash continues the line", stdin: "a\\\nb\n",
			script: "read x\necho \"[$x]\"\n", want: "[ab]\n",
		},
		{
			// With -r there is no continuation, so the backslash ends up as the
			// last character of the first line.
			name: "-r ends the line at the newline", stdin: "a\\\nb\n",
			script: "read -r x\necho \"[$x]\"\n", want: "[a\\]\n",
		},
		{
			// An escaped separator is data, so it must not split. This is why the
			// collector records which bytes were escaped instead of unescaping
			// first and splitting after.
			name: "an escaped space does not split", stdin: "a\\ b c\n",
			script: "read p q\necho \"[$p][$q]\"\n", want: "[a b][c]\n",
		},
		{
			name: "-r lets the space split", stdin: "a\\ b c\n",
			script: "read -r p q\necho \"[$p][$q]\"\n", want: "[a\\][b c]\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, stdout, stderr := runReadScript(t, test.stdin, test.script)

			// Then
			if status != 0 {
				t.Fatalf("status = %d, stderr = %q", status, stderr)
			}
			if stdout != test.want {
				t.Fatalf("stdout = %q, want %q", stdout, test.want)
			}
		})
	}
}

// End of input is a failure *and* an assignment, which is the pair that makes
// `while read x` terminate after processing a file with no final newline instead
// of dropping its last line.
func TestRead_reportsEndOfInputWhileStillAssigning(t *testing.T) {
	tests := []struct {
		name       string
		stdin      string
		wantStatus int
		wantOut    string
	}{
		{name: "a line with no newline", stdin: "a", wantStatus: 1, wantOut: "[a]\n"},
		{name: "nothing at all", stdin: "", wantStatus: 1, wantOut: "[]\n"},
		{name: "an empty line", stdin: "\n", wantStatus: 0, wantOut: "[]\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, stdout, stderr := runReadScript(t, test.stdin,
				"read x\nsaved=$?\necho \"[$x]\"\nexit $saved\n")

			// Then
			if status != test.wantStatus {
				t.Fatalf("status = %d, want %d, stderr = %q", status, test.wantStatus, stderr)
			}
			if stdout != test.wantOut {
				t.Fatalf("stdout = %q, want %q", stdout, test.wantOut)
			}
		})
	}
}

// A file whose last line has no newline must not lose that line, which is the
// practical consequence of the pair above.
func TestRead_keepsTheLastLineOfAFileWithoutATrailingNewline(t *testing.T) {
	// When
	status, stdout, stderr := runReadScript(t, "one\ntwo\nthree",
		"while read -r line || [ -n \"$line\" ]; do echo \"<$line>\"; done\n")

	// Then
	if status != 0 {
		t.Fatalf("status = %d, stderr = %q", status, stderr)
	}
	if want := "<one>\n<two>\n<three>\n"; stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestRead_honoursItsOptions(t *testing.T) {
	tests := []struct {
		name   string
		stdin  string
		script string
		want   string
	}{
		{
			name: "-a fills an array", stdin: "one two three\n",
			script: "read -a arr\necho \"${#arr[@]}:${arr[0]}:${arr[2]}\"\n", want: "3:one:three\n",
		},
		{
			name: "-a splits on a custom IFS", stdin: "a:b:c\n",
			script: "IFS=: read -a arr\necho \"${#arr[@]}:${arr[1]}\"\n", want: "3:b\n",
		},
		{
			name: "-d takes another delimiter", stdin: "a:b",
			script: "read -d : x\necho \"[$x]\"\n", want: "[a]\n",
		},
		{
			name: "-n stops after a count", stdin: "abcdef\n",
			script: "read -n 3 x\necho \"[$x]\"\n", want: "[abc]\n",
		},
		{
			// -n still stops at the delimiter; -N does not, which is the whole
			// difference between them.
			name: "-n stops at the delimiter too", stdin: "ab\ncd\n",
			script: "read -n 5 x\necho \"[$x]\"\n", want: "[ab]\n",
		},
		{
			// Five bytes including the newline, so the variable holds it and the
			// echo prints two lines.
			name: "-N ignores the delimiter", stdin: "ab\ncd\n",
			script: "read -N 5 x\necho \"[$x]\"\n", want: "[ab\ncd]\n",
		},
		{
			name: "clustered options", stdin: "a\\b\n",
			script: "read -rn 4 x\necho \"[$x]\"\n", want: "[a\\b]\n",
		},
		{
			name: "-p takes its prompt and reads", stdin: "v\n",
			script: "read -p 'name: ' x\necho \"[$x]\"\n", want: "[v]\n",
		},
		{
			name: "-- ends the options", stdin: "v\n",
			script: "read -- x\necho \"[$x]\"\n", want: "[v]\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, stdout, stderr := runReadScript(t, test.stdin, test.script)

			// Then
			if status != 0 {
				t.Fatalf("status = %d, stderr = %q", status, stderr)
			}
			if stdout != test.want {
				t.Fatalf("stdout = %q, want %q", stdout, test.want)
			}
		})
	}
}

// A prompt is for a person at a terminal: with the input a pipe or a file both references
// write none, and read the answer all the same -- `echo hi | { read -p 'P'; echo $REPLY; }`
// is hi and nothing on stderr. At a terminal the prompt goes to stderr, so that
// `x=$(read -p 'q: ' v; echo $v)` captures the answer and not the question.
func TestRead_promptsOnlyAtATerminal(t *testing.T) {
	// When
	_, stdout, stderr := runReadScript(t, "value\n", "read -p 'question: ' x\necho \"$x\"\n")

	// Then
	if stdout != "value\n" || stderr != "" {
		t.Fatalf("stdout = %q, stderr = %q, want the answer and no prompt", stdout, stderr)
	}
}

// A bad option has to say so. The whole reason this rewrite exists is that `-r`
// was accepted as a variable name, so an unknown option quietly becoming one is
// exactly the regression to guard against. An option it cannot take is a usage
// error, 2, and a name it cannot assign is 1, as both references have them; the
// name was 2 as well.
func TestRead_refusesWhatItCannotDo(t *testing.T) {
	tests := []struct {
		name     string
		script   string
		fragment string
		status   int
	}{
		{name: "an unknown option", script: "read -Z x\n", fragment: "read: illegal option -Z", status: 2},
		{name: "an option with no argument", script: "read -n\n", fragment: "requires an argument", status: 2},
		{name: "a non-numeric count", script: "read -n abc x\n", fragment: "invalid number", status: 2},
		{name: "a bad timeout", script: "read -t abc x\n", fragment: "invalid timeout", status: 2},
		{name: "a bad descriptor", script: "read -u abc x\n", fragment: "invalid file descriptor", status: 2},
		{name: "not a variable name", script: "read 9bad\n", fragment: "not a valid variable name", status: 1},
		{name: "-a with a bad name after it", script: "read -a arr 9bad\n", fragment: "not a valid variable name", status: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, _, stderr := runReadScript(t, "input\n", test.script)

			// Then
			if status != test.status {
				t.Fatalf("status = %d, want %d, stderr = %q", status, test.status, stderr)
			}
			if !strings.Contains(stderr, test.fragment) {
				t.Fatalf("stderr = %q, want it to contain %q", stderr, test.fragment)
			}
		})
	}
}

// -t is 128 + SIGALRM, which is the number a script tests for.
func TestRead_reports142WhenItTimesOut(t *testing.T) {
	// Given a reader that never produces anything and never closes.
	var stdout, stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{
		Stdin: blockingReader{}, Stdout: &stdout, Stderr: &stderr,
	})

	// When
	status := rt.RunScript(context.Background(), "read -t 0.05 x\necho \"status=$?\"\n")

	// Then
	if !strings.Contains(stdout.String(), "status=142") {
		t.Fatalf("stdout = %q, want status=142; overall status %d, stderr %q",
			stdout.String(), status, stderr.String())
	}
}

// blockingReader never returns, which is what a terminal with nobody typing at it
// looks like.
type blockingReader struct{}

func (blockingReader) Read([]byte) (int, error) {
	select {}
}

// A readonly name is refused rather than written through, and the diagnostic names
// it. `read` is a write like any other.
func TestRead_refusesAReadonlyName(t *testing.T) {
	// When
	status, _, stderr := runReadScript(t, "value\n", "readonly v=1\nread v\n")

	// Then
	if status == 0 {
		t.Fatalf("status = %d, want a failure", status)
	}
	if !strings.Contains(stderr, "readonly") {
		t.Fatalf("stderr = %q, want it to name the readonly variable", stderr)
	}
}
