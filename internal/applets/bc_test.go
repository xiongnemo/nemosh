package applets

import (
	"os/exec"
	"strings"
	"testing"
)

// bc.
//
// Every case carries a literal answer and is also handed to busybox-w32 where it is
// installed. The scale rules are the specification and a calculator that agrees only with
// itself has proved nothing -- which is the same reasoning the awk suite runs on.

func runBc(t *testing.T, program string) (string, string, int) {
	t.Helper()
	return runApplet(t, "bc", nil, program+"\n")
}

func checkBcAgainstBusybox(t *testing.T, program, got string, diverges bool) {
	t.Helper()
	if diverges || !busyboxIsTheReference() {
		return
	}
	path, err := exec.LookPath("busybox")
	if err != nil {
		return
	}
	command := exec.Command(path, "bc")
	command.Stdin = strings.NewReader(program + "\n")
	out, _ := command.Output()
	want := strings.ReplaceAll(string(out), "\r\n", "\n")
	if got != want {
		t.Fatalf("bc %q disagrees with busybox\n got %q\nwant %q", program, got, want)
	}
}

func TestBc(t *testing.T) {
	t.Parallel()
	for _, testcase := range []struct {
		name     string
		program  string
		want     string
		diverges bool
	}{
		{name: "arithmetic", program: "2+3", want: "5\n"},
		{name: "division truncates at scale 0", program: "10/3", want: "3\n"},
		{name: "a scale", program: "scale=5;10/3", want: "3.33333\n"},
		{name: "scale is a variable and a function", program: "scale=3;scale;scale(1.234)", want: "3\n3\n"},
		{name: "square root", program: "scale=4;sqrt(2)", want: "1.4142\n"},
		{name: "power", program: "2^10", want: "1024\n"},
		{name: "a negative power", program: "scale=4;2^-2", want: ".2500\n"},
		{name: "power is right-associative", program: "2^3^2", want: "512\n"},
		{name: "multiplication keeps the smaller scale", program: "scale=0;2.5*2.5", want: "6.2\n"},
		{name: "remainder at a scale", program: "scale=2;10%3", want: ".01\n"},
		{name: "thirty digits", program: "scale=30;1/3", want: ".333333333333333333333333333333\n"},
		{name: "an input base", program: "ibase=16;FF", want: "255\n"},
		{name: "a digit too large is clamped", program: "ibase=8;19", want: "15\n"},
		{name: "an output base", program: "obase=2;5", want: "101\n"},
		{name: "length counts every digit", program: "length(123.45)", want: "5\n"},
		{name: "variables", program: "x=5;x+1", want: "6\n"},
		{name: "an assignment prints nothing", program: "x=5", want: ""},
		{
			// The parenthesis makes it an expression that happens to assign, so it prints.
			name: "a parenthesised assignment prints", program: "(x=5)", want: "5\n",
		},
		{name: "assignment is right-associative", program: "a=b=5;a+b", want: "10\n"},
		{name: "compound assignment", program: "x=10;x/=3;x", want: "3\n"},
		{name: "increment", program: "x=1;x++;x;++x;x", want: "1\n2\n3\n3\n"},
		{name: "last", program: "5\n.+1", want: "5\n6\n"},
		{name: "while", program: "i=0;while(i<3){i;i+=1}", want: "0\n1\n2\n"},
		{name: "for", program: "for(i=0;i<3;i++){i}", want: "0\n1\n2\n"},
		{name: "if and else", program: "if(1)2 else 3", want: "2\n"},
		{name: "the else branch", program: "if(0)2 else 3", want: "3\n"},
		{name: "break", program: "for(i=0;i<9;i++){if(i==2)break;i}", want: "0\n1\n"},
		{name: "continue", program: "for(i=0;i<4;i++){if(i==1)continue;i}", want: "0\n2\n3\n"},
		{name: "a function", program: "define f(n){return(n*2)};f(21)", want: "42\n"},
		{name: "recursion", program: "define g(n){if(n<=1)return(1);return(n*g(n-1))};g(5)", want: "120\n"},
		{name: "print has no separators", program: `print "hi", 1+1, "!"`, want: "hi2!"},
		{name: "a bare string prints", program: `"text"`, want: "text"},
		{name: "arrays", program: "a[0]=7;a[1]=8;a[0]+a[1]", want: "15\n"},
		{name: "comparison answers one or zero", program: "1<2;2<1", want: "1\n0\n"},
		{name: "logical and", program: "1&&0;1&&2", want: "0\n1\n"},
		{name: "negation", program: "!0;!5", want: "1\n0\n"},
		{name: "a comment", program: "1 /* two */ +2", want: "3\n"},
		{name: "a hash comment", program: "1+2 # three", want: "3\n"},
		{name: "a long number wraps", program: "2^300", want: bcWrapped},
		{
			// POSIX and GNU say -4 because `^` binds tighter than unary minus. busybox
			// answers 4, which is the divergence recorded in the support matrix.
			name: "power binds tighter than unary minus", program: "-2^2", want: "-4\n", diverges: true,
		},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()
			got, stderr, status := runBc(t, testcase.program)
			if got != testcase.want || stderr != "" || status != 0 {
				t.Fatalf("bc %q\n got %q stderr %q status %d\nwant %q",
					testcase.program, got, stderr, status, testcase.want)
			}
			checkBcAgainstBusybox(t, testcase.program, got, testcase.diverges)
		})
	}
}

// bcWrapped is 2^300, which is 91 digits and so breaks across two lines.
const bcWrapped = "20370359763344860862684456884093781610514683936659362506361404493543\\\n" +
	"81299763336706183397376\n"

func TestBcFunctionsAndScope(t *testing.T) {
	t.Parallel()
	for _, testcase := range []struct {
		name    string
		program string
		want    string
	}{
		{
			// An auto is local and restored, which is the whole of what makes recursion
			// work: the outer call's `i` must survive the inner one.
			name: "autos are local", program: "define f(n){auto i;i=99;return(i)};i=1;f(0);i", want: "99\n1\n",
		},
		{name: "a parameter is local", program: "define f(n){n=99;return(n)};n=1;f(0);n", want: "99\n1\n"},
		{name: "an auto starts at zero", program: "define f(){auto i;return(i)};i=5;f()", want: "0\n"},
		{name: "a function with no return answers zero", program: "define f(){1};f()", want: "1\n0\n"},
		{name: "globals are visible", program: "g=7;define f(){return(g)};f()", want: "7\n"},
		{
			// By value: POSIX says so, and a caller whose array changed underneath it
			// would be a worse surprise than the copy is a cost.
			name: "an array argument is copied", program: "a[0]=1;define f(x[]){x[0]=9;return(x[0])};f(a[]);a[0]", want: "9\n1\n",
		},
		{name: "nested calls", program: "define d(n){return(n*2)};define q(n){return(d(d(n)))};q(3)", want: "12\n"},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()
			got, stderr, status := runBc(t, testcase.program)
			if got != testcase.want || stderr != "" || status != 0 {
				t.Fatalf("bc %q\n got %q stderr %q status %d\nwant %q",
					testcase.program, got, stderr, status, testcase.want)
			}
		})
	}
}

func TestBcRefusals(t *testing.T) {
	t.Parallel()
	for _, testcase := range []struct {
		program string
		says    string
	}{
		{program: "1/0", says: "divide by zero"},
		{program: "scale=0;1%0", says: "divide by zero"},
		{program: "sqrt(-1)", says: "square root of a negative"},
		{program: "ibase=1", says: "between 2 and 16"},
		{program: "obase=99", says: "between 2 and 16"},
		{program: "scale=-1", says: "must not be negative"},
		{program: "f(1)", says: "is not defined"},
		{program: "define f(a){return(a)};f(1,2)", says: "takes 1 arguments"},
		{program: "1<2<3", says: "cannot be chained"},
		{program: "read()", says: "read() is not supported"},
		{program: "1+", says: "unexpected"},
		{program: `"unterminated`, says: "unterminated string"},
		{program: "define f(){return(f())};f()", says: "recursed more than"},
	} {
		t.Run(testcase.program, func(t *testing.T) {
			t.Parallel()
			_, stderr, status := runBc(t, testcase.program)
			if status == 0 {
				t.Fatalf("bc %q was accepted", testcase.program)
			}
			if !strings.Contains(stderr, testcase.says) {
				t.Fatalf("bc %q said %q, which does not contain %q", testcase.program, stderr, testcase.says)
			}
		})
	}
	// -l would be a series expansion, and a wrong one is wrong in the digits that matter.
	if _, stderr, status := runApplet(t, "bc", []string{"-l"}, ""); status == 0 || !strings.Contains(stderr, "-l is not supported") {
		t.Fatalf("bc -l: stderr %q status %d", stderr, status)
	}
	// The options that ask for what it already does are accepted.
	for _, option := range []string{"-q", "-s", "-w"} {
		if got, _, status := runApplet(t, "bc", []string{option}, "1+1\n"); status != 0 || got != "2\n" {
			t.Fatalf("bc %s gave %q status %d", option, got, status)
		}
	}
}

// TestBcHalt covers the statement that ends the program from inside a loop or a function.
func TestBcHalt(t *testing.T) {
	t.Parallel()
	got, _, status := runBc(t, "1;halt;2")
	if got != "1\n" || status != 0 {
		t.Fatalf("halt gave %q status %d", got, status)
	}
	inLoop, _, _ := runBc(t, "for(i=0;i<9;i++){i;if(i==1)halt}")
	if inLoop != "0\n1\n" {
		t.Fatalf("halt in a loop gave %q", inLoop)
	}
}
