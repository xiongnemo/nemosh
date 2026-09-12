package applets

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
)

// The command line, the operand walk, and the applet itself.
//
// These run through `awkApplet{}.Run` rather than through the interpreter directly, because
// the command line is the part being tested. Where a case can be handed to the references
// unchanged it is, but most name files in a temp directory, so the literal expectation is
// what holds.

// runAwkCommand runs the applet as the shell would.
func runAwkCommand(t *testing.T, args []string, stdin string) (string, string, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	err := awkApplet{}.Run(context.Background(), args, strings.NewReader(stdin), &stdout, &stderr)
	status := 0
	if err != nil {
		status = 1
		if code, carried := StatusCode(err); carried {
			status = code
		}
		if message, carried := StatusMessage(err); carried {
			stderr.WriteString(message)
		} else {
			stderr.WriteString(err.Error())
		}
	}
	return stdout.String(), stderr.String(), status
}

func TestAwkCommandLine(t *testing.T) {
	t.Parallel()
	for _, testcase := range []struct {
		name  string
		args  []string
		stdin string
		want  string
	}{
		{name: "a program and stdin", args: []string{"{print $2}"}, stdin: "a b\n", want: "b\n"},
		{name: "-F apart from its value", args: []string{"-F", ":", "{print $2}"}, stdin: "a:b:c\n", want: "b\n"},
		{name: "-F joined to its value", args: []string{"-F:", "{print $2}"}, stdin: "a:b:c\n", want: "b\n"},
		{
			// The escapes are processed, so this is one tab and not a backslash and a t.
			name: "-F with an escape", args: []string{"-F", `\t`, "{print $2}"}, stdin: "a\tb\n", want: "b\n",
		},
		{name: "-F as a regular expression", args: []string{"-F", "[,;]", "{print $2}"}, stdin: "a,b;c\n", want: "b\n"},
		{name: "-v apart", args: []string{"-v", "x=1", "BEGIN{print x}"}, want: "1\n"},
		{name: "-v joined", args: []string{"-vx=2", "BEGIN{print x}"}, want: "2\n"},
		{name: "-v repeated", args: []string{"-v", "a=1", "-v", "b=2", "BEGIN{print a, b}"}, want: "1 2\n"},
		{
			// A -v value is a strnum, so it compares as a number.
			name: "-v makes a strnum", args: []string{"-v", "n=10", "BEGIN{print (n==10), (n<9)}"}, want: "1 0\n",
		},
		{name: "-v with an escape", args: []string{"-v", `x=a\tb`, "BEGIN{print length(x)}"}, want: "3\n"},
		{
			// BEGIN still wins, because -F lands before it rather than after.
			name: "BEGIN overrides -F", args: []string{"-F", ":", "BEGIN{FS=\",\"}", "{print $2}"}, stdin: "a,b\n", want: "",
		},
		{name: "-- ends the options", args: []string{"--", "BEGIN{print 1}"}, want: "1\n"},
		{name: "ARGV and ARGC with no operands", args: []string{"BEGIN{print ARGC, ARGV[0]}"}, want: "1 awk\n"},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()
			got, stderr, status := runAwkCommand(t, testcase.args, testcase.stdin)
			if got != testcase.want || stderr != "" || status != 0 {
				t.Fatalf("%v\n got %q stderr %q status %d\nwant %q", testcase.args, got, stderr, status, testcase.want)
			}
		})
	}
}

func TestAwkFileOperands(t *testing.T) {
	t.Parallel()
	// Two files, so that NR and FNR can be told apart.
	setup := func(t *testing.T) (string, string) {
		t.Helper()
		dir := t.TempDir()
		one := filepath.Join(dir, "one")
		two := filepath.Join(dir, "two")
		writeFileText(t, one, "a b\nc d\n")
		writeFileText(t, two, "e f\n")
		return filepath.ToSlash(one), filepath.ToSlash(two)
	}

	t.Run("NR runs on and FNR restarts", func(t *testing.T) {
		t.Parallel()
		one, two := setup(t)
		got, _, _ := runAwkCommand(t, []string{"{print FILENAME, NR, FNR}", one, two}, "")
		want := one + " 1 1\n" + one + " 2 2\n" + two + " 3 1\n"
		if got != want {
			t.Fatalf("got %q want %q", got, want)
		}
	})

	t.Run("FILENAME is empty in BEGIN", func(t *testing.T) {
		t.Parallel()
		one, _ := setup(t)
		if got, _, _ := runAwkCommand(t, []string{`BEGIN{print "[" FILENAME "]"}`, one}, ""); got != "[]\n" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("an assignment operand takes effect where it stands", func(t *testing.T) {
		t.Parallel()
		one, two := setup(t)
		got, _, _ := runAwkCommand(t, []string{"{print v, $1}", one, "v=SET", two}, "")
		if got != " a\n c\nSET e\n" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("nextfile abandons the rest of a file", func(t *testing.T) {
		t.Parallel()
		one, two := setup(t)
		got, _, _ := runAwkCommand(t, []string{"FNR==1{next} {print}", one, two}, "")
		if got != "c d\n" {
			t.Fatalf("got %q", got)
		}
		got, _, _ = runAwkCommand(t, []string{"FNR==1{nextfile} {print}", one, two}, "")
		if got != "" {
			t.Fatalf("nextfile got %q", got)
		}
	})

	t.Run("a dash means standard input", func(t *testing.T) {
		t.Parallel()
		if got, _, _ := runAwkCommand(t, []string{`{print "got:" $0}`, "-"}, "piped\n"); got != "got:piped\n" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("assignments alone still read standard input", func(t *testing.T) {
		t.Parallel()
		if got, _, _ := runAwkCommand(t, []string{"{print v, $0}", "v=1"}, "x\n"); got != "1 x\n" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("ARGV is walked at run time", func(t *testing.T) {
		t.Parallel()
		one, _ := setup(t)
		// The program replaces what it was given, and the loop reads the replacement.
		program := `BEGIN{ARGV[1]="` + one + `"; ARGC=2} {print "read:" $1}`
		if got, _, _ := runAwkCommand(t, []string{program, "nonexistent"}, ""); got != "read:a\nread:c\n" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("an emptied ARGV element is skipped", func(t *testing.T) {
		t.Parallel()
		one, two := setup(t)
		program := `BEGIN{ARGV[1]=""} {print $1}`
		if got, _, _ := runAwkCommand(t, []string{program, one, two}, ""); got != "e\n" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("-f reads the program from a file", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		source := filepath.Join(dir, "prog.awk")
		writeFileText(t, filepath.ToSlash(source), "{ print $2 }\n")
		if got, _, _ := runAwkCommand(t, []string{"-f", source}, "a b\n"); got != "b\n" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("two -f files are one program", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		first := filepath.Join(dir, "a.awk")
		second := filepath.Join(dir, "b.awk")
		writeFileText(t, filepath.ToSlash(first), "function double(n) { return n*2 }\n")
		writeFileText(t, filepath.ToSlash(second), "{ print double($1) }\n")
		if got, _, _ := runAwkCommand(t, []string{"-f", first, "-f", second}, "21\n"); got != "42\n" {
			t.Fatalf("got %q", got)
		}
	})
}

func TestAwkCommandRefusals(t *testing.T) {
	t.Parallel()
	for _, testcase := range []struct {
		name   string
		args   []string
		says   string
		status int
	}{
		{name: "no program at all", args: nil, says: "operand", status: 2},
		{name: "an unknown option", args: []string{"-Z", "{print}"}, says: "invalid option", status: 2},
		{name: "-v without an equals", args: []string{"-v", "x", "BEGIN{}"}, says: "var=value", status: 2},
		{name: "-F with nothing after it", args: []string{"-F"}, says: "needs a value", status: 2},
		{name: "a program that will not parse", args: []string{"BEGIN{"}, says: "", status: 2},
		{name: "a missing input file", args: []string{"{print}", "no-such-file-here"}, says: "no-such-file-here", status: 2},
		{name: "a missing program file", args: []string{"-f", "no-such-program-here"}, says: "no-such-program-here", status: 2},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()
			_, stderr, status := runAwkCommand(t, testcase.args, "")
			if status != testcase.status {
				t.Fatalf("%v exited %d, want %d (stderr %q)", testcase.args, status, testcase.status, stderr)
			}
			if !strings.Contains(stderr, testcase.says) {
				t.Fatalf("%v said %q, which does not contain %q", testcase.args, stderr, testcase.says)
			}
		})
	}
}

// TestAwkExitStatus covers the number a program chooses for itself, which a script tests.
func TestAwkExitStatus(t *testing.T) {
	t.Parallel()
	for _, testcase := range []struct {
		name   string
		args   []string
		status int
	}{
		{name: "a program that finishes", args: []string{"BEGIN{print 1}"}, status: 0},
		{name: "exit with a number", args: []string{"BEGIN{exit 3}"}, status: 3},
		{name: "exit with none", args: []string{"BEGIN{exit}"}, status: 0},
		{name: "END still runs after exit", args: []string{"BEGIN{exit 4} END{print \"tidy\"}"}, status: 4},
		// A run-time failure is 2, which is what tells it apart from a chosen status.
		{name: "a run-time failure", args: []string{"BEGIN{print 1/0}"}, status: 2},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()
			if _, _, status := runAwkCommand(t, testcase.args, ""); status != testcase.status {
				t.Fatalf("%v exited %d, want %d", testcase.args, status, testcase.status)
			}
		})
	}
}
