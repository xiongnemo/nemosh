package applets

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// Running a program, checked **against the references themselves** rather than against
// expectations typed from memory.
//
// gawk 5.4.1 and busybox-w32 1.38.0 are both on this machine, so a case can state a
// program and an input and let the two of them supply the answer. Where they disagree the
// case says which is followed and why; everywhere else, agreement is the assertion.
//
// A case still carries a literal `want`, because a differential test that only compares
// against a reference cannot run where the reference is absent -- and CI's ubuntu and
// macos runners have no busybox-w32. So `want` is what the run must produce, and the
// reference comparison is an extra check that runs when the binary is there.

// runAwk runs a program over an input and answers stdout, stderr and the status.
func runAwk(t *testing.T, program, input string) (string, string, int) {
	t.Helper()
	parsed, err := parseAwkProgram(program)
	if err != nil {
		t.Fatalf("parse %q: %v", program, err)
	}
	var stdout, stderr bytes.Buffer
	status, runErr := runAwkProgram(parsed, strings.NewReader(input), &stdout, &stderr)
	if runErr != nil {
		stderr.WriteString(runErr.Error())
	}
	return stdout.String(), stderr.String(), status
}

// referenceAwk runs the same program through a reference, or reports that it is absent.
//
// The program is handed over in a **file** rather than as an argument, and that is not a
// style choice. gawk on Windows collapses a doubled backslash in an argv-delivered
// program: `gsub(/l/, "[\\&]", s)` then behaves as though it had been written with one
// backslash and answers the matched text, where the same bytes in a file answer a literal
// `&`. Measured across zero to four backslashes, gawk's two delivery paths disagree at two
// and at four while busybox's agree everywhere -- so an argv harness would have reported
// this project's escape handling as a divergence from gawk when it is not one. A file is
// also how the rules were measured in the first place, for the reason AGENTS.md gives
// about awkward bytes and text channels.
func referenceAwk(t *testing.T, binary string, prefix []string, program, input string) (string, bool) {
	t.Helper()
	path, err := exec.LookPath(binary)
	if err != nil {
		return "", false
	}
	file := filepath.Join(t.TempDir(), "program.awk")
	if err := os.WriteFile(file, []byte(program), 0o644); err != nil {
		t.Fatalf("write the reference program: %v", err)
	}
	command := exec.Command(path, append(append([]string{}, prefix...), "-f", file)...)
	command.Stdin = strings.NewReader(input)
	var out bytes.Buffer
	command.Stdout = &out
	command.Stderr = &bytes.Buffer{}
	_ = command.Run()
	return out.String(), true
}

// busyboxIsTheReference reports whether the busybox on PATH is busybox-w32, the build this
// project measures against.
//
// A distro's busybox is a **different program with different answers**: ubuntu's prints 4
// for `BEGIN { print -2^2 }` where busybox-w32 1.38.0 prints -4, so a suite comparing
// against whichever busybox it happened to find went red on CI over a disagreement between
// two busyboxes rather than anything about nemosh. That is the "a test must not sample the
// machine it runs on" failure AGENTS.md records, and it had already cost nine red commits
// once. busybox-w32 names itself in its banner: its version carries an `-FRP-` suffix that
// the upstream builds do not.
var busyboxIsTheReference = sync.OnceValue(func() bool {
	path, err := exec.LookPath("busybox")
	if err != nil {
		return false
	}
	// busybox with no arguments prints its banner and exits non-zero, so the status is
	// not the question -- the first line is.
	out, _ := exec.Command(path).CombinedOutput()
	first, _, _ := strings.Cut(string(out), "\n")
	return strings.Contains(first, "-FRP-")
})

// checkAgainstReferences compares a result with gawk and busybox when they are installed.
//
// Not fatal when they are missing: this suite runs on ubuntu and macos in CI, where
// busybox-w32 is not present. The literal expectation in each case is what holds there.
func checkAgainstReferences(t *testing.T, program, input, got string, diverges ...string) {
	t.Helper()
	skip := map[string]bool{}
	for _, name := range diverges {
		skip[name] = true
	}
	for _, reference := range []struct {
		name   string
		binary string
		prefix []string
		usable func() bool
	}{
		{name: "gawk", binary: "gawk"},
		{name: "busybox awk", binary: "busybox", prefix: []string{"awk"}, usable: busyboxIsTheReference},
	} {
		if reference.usable != nil && !reference.usable() {
			// Present, but not the build this project measures against.
			continue
		}
		if skip[reference.name] {
			// A declared divergence. Naming it here rather than dropping the comparison
			// altogether keeps the *other* reference checking the case.
			continue
		}
		want, present := referenceAwk(t, reference.binary, reference.prefix, program, input)
		if !present {
			continue
		}
		if got != want {
			t.Errorf("%s disagrees for %q on %q\n  nemosh: %q\n  %s: %q",
				reference.name, program, input, got, reference.name, want)
		}
	}
}

func TestAwkRun_patternsAndPrint(t *testing.T) {
	for _, test := range []struct {
		name    string
		program string
		input   string
		want    string
		// diverges names a reference this case knowingly disagrees with.
		diverges []string
	}{
		{name: "the whole record", program: "{ print }", input: "a b\nc d\n", want: "a b\nc d\n"},
		{name: "a field", program: "{ print $2 }", input: "a b c\n", want: "b\n"},
		{name: "the last field", program: "{ print $NF }", input: "a b c\n", want: "c\n"},
		{name: "several fields", program: "{ print $1, $3 }", input: "a b c\n", want: "a c\n"},
		// Concatenation, which has no operator: the two fields join with nothing.
		{name: "concatenated fields", program: "{ print $1 $3 }", input: "a b c\n", want: "ac\n"},
		{name: "a count", program: "{ print NF }", input: "a b c\n", want: "3\n"},
		{name: "the record number", program: "{ print NR, $0 }", input: "x\ny\n", want: "1 x\n2 y\n"},

		// Patterns.
		{name: "a regex pattern", program: "/b/ { print }", input: "abc\nxyz\n", want: "abc\n"},
		{name: "a pattern with no action", program: "/b/", input: "abc\nxyz\n", want: "abc\n"},
		{name: "an expression pattern", program: "NR > 1 { print }", input: "a\nb\nc\n", want: "b\nc\n"},
		{name: "a negated match", program: "$0 !~ /b/ { print }", input: "abc\nxyz\n", want: "xyz\n"},
		{name: "a field comparison", program: "$1 > 2 { print }", input: "1\n3\n", want: "3\n"},
		// The strnum rule, end to end: fields compare as numbers.
		{name: "fields compare numerically", program: "{ print ($1 < $2) }", input: "10 9\n", want: "0\n"},

		// Ranges, including the single-line form.
		{name: "a range", program: "/a/,/c/ { print }", input: "x\na\nb\nc\nd\n", want: "a\nb\nc\n"},
		{name: "a one-line range", program: "/a/,/a/ { print }", input: "x\na\nb\na\n", want: "a\na\n"},

		// BEGIN and END.
		{name: "BEGIN only", program: "BEGIN { print \"hi\" }", input: "", want: "hi\n"},
		{name: "END sees the last record", program: "END { print NR, $0 }", input: "a\nb\n", want: "2 b\n"},
		{name: "BEGIN and END", program: "BEGIN{print \"s\"} {print} END{print \"e\"}", input: "x\n", want: "s\nx\ne\n"},

		// Separators.
		{name: "OFS", program: "BEGIN { OFS=\"-\" } { print $1, $2 }", input: "a b\n", want: "a-b\n"},
		{name: "ORS", program: "BEGIN { ORS=\"|\" } { print }", input: "a\nb\n", want: "a|b|"},
		{name: "FS", program: "BEGIN { FS=\",\" } { print $2 }", input: "a,b,c\n", want: "b\n"},

		// Assignment to fields and NF.
		{name: "assigning a field rebuilds the record", program: "{ $2=\"X\"; print }", input: "a b c\n", want: "a X c\n"},
		{name: "assigning past NF extends", program: "{ $5=\"E\"; print NF; print }", input: "a b c\n", want: "5\na b c  E\n"},
		{name: "assigning NF truncates", program: "{ NF=2; print; print NF }", input: "a b c\n", want: "a b\n2\n"},
		{name: "assigning the record resplits", program: "{ $0=\"x y\"; print NF }", input: "a b c\n", want: "2\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, stderr, status := runAwk(t, test.program, test.input)
			if status != 0 {
				t.Fatalf("status %d, stderr %q", status, stderr)
			}
			if got != test.want {
				t.Fatalf("%s on %q\n  got  %q\n  want %q", test.program, test.input, got, test.want)
			}
			checkAgainstReferences(t, test.program, test.input, got, test.diverges...)
		})
	}
}

func TestAwkRun_expressionsAndControl(t *testing.T) {
	for _, test := range []struct {
		name    string
		program string
		input   string
		want    string
		// diverges names a reference this case knowingly disagrees with.
		diverges []string
	}{
		{name: "arithmetic", program: "BEGIN { print 1+2*3 }", input: "", want: "7\n"},
		{
			// busybox parses `^` left-associatively and answers 64, which contradicts
			// POSIX. gawk answers 512 and is followed; see awk_parse_expr.go. Declaring
			// the divergence here rather than dropping the comparison keeps gawk
			// checking this case.
			name: "power is right associative", program: "BEGIN { print 2^3^2 }", input: "",
			want: "512\n", diverges: []string{"busybox awk"},
		},
		{name: "unary minus below power", program: "BEGIN { print -2^2 }", input: "", want: "-4\n"},
		{name: "modulo keeps the sign", program: "BEGIN { print -5 % 3 }", input: "", want: "-2\n"},
		{name: "OFMT on a fraction", program: "BEGIN { print 1/3 }", input: "", want: "0.333333\n"},
		{name: "an integral float prints whole", program: "BEGIN { print 3.0 }", input: "", want: "3\n"},

		{name: "string comparison", program: `BEGIN { print ("10" < "9") }`, input: "", want: "1\n"},
		{name: "numeric comparison", program: "BEGIN { print (10 < 9) }", input: "", want: "0\n"},
		{name: "concatenation is a string", program: `BEGIN { print (1 2 < 3) }`, input: "", want: "1\n"},
		// The one that catches everybody: the minus is binary.
		{name: "binary minus beats concatenation", program: `BEGIN { print (1 " " -1) }`, input: "", want: "1-1\n"},

		{name: "and short-circuits to one", program: "BEGIN { print (2 && 3) }", input: "", want: "1\n"},
		{name: "a ternary", program: "BEGIN { print (1 ? \"y\" : \"n\") }", input: "", want: "y\n"},
		{name: "increment answers the old value", program: "BEGIN { x=1; print x++, x }", input: "", want: "1 2\n"},
		{name: "prefix answers the new one", program: "BEGIN { x=1; print ++x, x }", input: "", want: "2 2\n"},

		{name: "a while loop", program: "BEGIN { i=0; while (i<3) { out = out i; i++ } print out }", input: "", want: "012\n"},
		{name: "a for loop", program: "BEGIN { for (i=0;i<3;i++) out = out i; print out }", input: "", want: "012\n"},
		{name: "break", program: "BEGIN { for (i=0;i<9;i++) { if (i==2) break; out = out i } print out }", input: "", want: "01\n"},
		{name: "continue", program: "BEGIN { for (i=0;i<4;i++) { if (i==1) continue; out = out i } print out }", input: "", want: "023\n"},
		{name: "do while runs once", program: "BEGIN { do print \"x\"; while (0) }", input: "", want: "x\n"},

		{name: "next skips the rest", program: "{ next; print \"no\" } { print \"also no\" }", input: "a\n", want: ""},
		// exit still runs END, which is what makes the tidy-up idiom work.
		{name: "exit runs END", program: "BEGIN { exit } END { print \"end\" }", input: "", want: "end\n"},

		{name: "arrays", program: "BEGIN { a[\"k\"]=1; print a[\"k\"] }", input: "", want: "1\n"},
		{name: "in does not create", program: "BEGIN { if (\"k\" in a) print \"y\"; else print \"n\" }", input: "", want: "n\n"},
		{name: "reading creates", program: "BEGIN { x=a[\"k\"]; if (\"k\" in a) print \"y\" }", input: "", want: "y\n"},
		{name: "for in walks", program: "BEGIN { a[1]=1; a[2]=2; for (k in a) out = out k; print out }", input: "", want: "12\n"},
		{name: "delete one", program: "BEGIN { a[1]=1; a[2]=2; delete a[1]; for (k in a) out = out k; print out }", input: "", want: "2\n"},
		{name: "a two-part subscript", program: "BEGIN { a[1,2]=\"x\"; print a[1,2] }", input: "", want: "x\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, stderr, status := runAwk(t, test.program, test.input)
			if status != 0 {
				t.Fatalf("status %d, stderr %q", status, stderr)
			}
			if got != test.want {
				t.Fatalf("%s\n  got  %q\n  want %q", test.program, got, test.want)
			}
			checkAgainstReferences(t, test.program, test.input, got, test.diverges...)
		})
	}
}

// What the interpreter refuses at run time, and what it has not got to yet.
//
// The unimplemented ones say so by name rather than doing something approximate, which is
// the rule AGENTS.md sets: a capability that is absent must fail loudly.
func TestAwkRun_refusals(t *testing.T) {
	for _, test := range []struct {
		name    string
		program string
		says    string
	}{
		{name: "division by zero", program: "BEGIN { print 1/0 }", says: "division by zero"},
		{name: "modulo by zero", program: "BEGIN { print 1%0 }", says: "division by zero"},
		{name: "a negative field", program: "BEGIN { print $-1 }", says: "field"},
		{name: "a builtin called wrongly", program: `BEGIN { print index("x") }`, says: "called with 1 arguments"},
		{name: "a parenthesised list as a value", program: "BEGIN { x = (1, 2) }", says: "before `in`"},
		// Not implemented yet, and loud about it.
		{name: "a redirect", program: `BEGIN { print 1 > "f" }`, says: "not supported yet"},
		{name: "a printf redirect", program: `BEGIN { printf "%s", 1 > "f" }`, says: "not supported yet"},
		{name: "system", program: `BEGIN { system("true") }`, says: "not supported yet"},
		{name: "a user function", program: "function f() { return 1 } BEGIN { print f() }", says: "not supported yet"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, stderr, status := runAwk(t, test.program, "")
			if status == 0 {
				t.Fatalf("%q succeeded", test.program)
			}
			if !strings.Contains(stderr, test.says) {
				t.Fatalf("%q said %q, which does not contain %q", test.program, stderr, test.says)
			}
		})
	}
}
