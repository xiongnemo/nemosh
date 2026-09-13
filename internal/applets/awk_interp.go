package applets

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"math/rand"
)

// The interpreter's state.
//
// Variables and arrays are separate maps because awk keeps them separate: a name is one
// or the other, never both, and `length(a)` on an array means something different from
// `length(s)` on a scalar.
//
// **NF and `$0` are computed from each other, lazily.** Assigning a field rebuilds `$0`
// with OFS; assigning `$0` re-splits; assigning NF truncates or extends. Doing any of that
// eagerly would re-split a record on every `$1` a program reads, so both directions are
// marked dirty and settled when someone asks. Measured against both references, which
// agree on all eight cases the tests cover.

type awkInterp struct {
	program *awkProgram
	vars    map[string]awkValue
	arrays  map[string]*awkArray
	// frames is the call stack's locals, innermost last, and is empty outside a call.
	// See awk_scope.go for what a parameter shadows and when.
	frames []*awkFrame
	// returned is what the most recent `return` answered. It lives here rather than in
	// the flow signal because a signal is a plain value, and the returning statement is
	// several frames below the call that wants the answer.
	returned awkValue

	// record is `$0` and fields are `$1`..`$NF`. recordStale means the fields have
	// been changed and `$0` must be rebuilt; fieldsStale means the reverse.
	record      string
	fields      []string
	recordStale bool
	fieldsStale bool

	// ctx belongs to the applet this is running inside, and reaches the applets that
	// `system`, `print | cmd` and `cmd | getline` dispatch to. See awk_command.go for
	// why those are applets and never OS processes.
	ctx context.Context
	// input is the program's own standard input, which both the record loop and a plain
	// `getline` read from -- through the one reader in `records`, so that the two cannot
	// each buffer a different part of it.
	input   io.Reader
	records *bufio.Reader

	// outputs and inputs are the redirections a program has open, keyed by the text of
	// the name. Opening once and keeping is what makes `{ print > "out" }` truncate the
	// file once rather than once per record.
	outputs map[string]*awkOutput
	inputs  map[string]*awkInput
	// deferred carries a failure from somewhere that had to answer a number instead --
	// `close()` on a pipe whose applet failed is the only one.
	deferred error

	output io.Writer
	errors io.Writer
	// buffered wraps output so a program printing a million lines does not make a
	// million writes. Flushed when the program ends and whenever a command needs the
	// order to be right.
	buffered *bufio.Writer

	// exiting and exitStatus carry `exit`, which leaves the record loop but still runs
	// END -- the one control flow in awk that is not a loop or a return.
	exiting    bool
	exitStatus int

	// streaming says to flush after every record rather than when the buffer fills.
	// See awk_run.go's runRecordsFrom for why it is decided from the output's kind.
	streaming bool

	// random is what `rand` draws from and randSeed the seed it was last given. The seed
	// starts at 1 rather than at the clock, because both references make a program that
	// never calls `srand` produce the same sequence every run.
	random   *rand.Rand
	randSeed float64
}

// awkSpecialDefaults are the built-in variables and what they start as.
//
// CONVFMT and OFMT are both `%.6g`, which is what makes `print 1/3` answer `0.333333`.
// SUBSEP is the character POSIX names, and it is not printable on purpose: it joins the
// parts of `a[i,j]` and must not collide with a real subscript.
var awkSpecialDefaults = map[string]string{
	"FS": " ", "OFS": " ", "ORS": "\n", "RS": "\n",
	"CONVFMT": "%.6g", "OFMT": "%.6g", "SUBSEP": "\x1c",
	"FILENAME": "",
}

func newAwkInterp(ctx context.Context, program *awkProgram, input io.Reader, output, errors io.Writer) *awkInterp {
	interp := &awkInterp{
		ctx:     ctx,
		program: program,
		vars:    map[string]awkValue{},
		arrays:  map[string]*awkArray{},
		input:   input,
		outputs: map[string]*awkOutput{},
		inputs:  map[string]*awkInput{},
		errors:  errors,
	}
	interp.buffered = bufio.NewWriter(output)
	interp.output = interp.buffered
	interp.streaming = !writerIsRegularFile(output)
	for name, value := range awkSpecialDefaults {
		interp.vars[name] = awkStr(value)
	}
	for _, name := range []string{"NR", "NF", "FNR", "RSTART", "RLENGTH"} {
		interp.vars[name] = awkNum(0)
	}
	// RLENGTH is -1 before any match, which is what a program tests to find out that
	// `match` failed.
	interp.vars["RLENGTH"] = awkNum(-1)
	interp.randSeed = 1
	interp.random = rand.New(rand.NewSource(1))
	return interp
}

// convfmt is the format a number takes when it becomes a string.
func (in *awkInterp) convfmt() string { return in.vars["CONVFMT"].str("%.6g") }

// ofmt is the format `print` uses, which is a different variable from CONVFMT even though
// both start as `%.6g`.
func (in *awkInterp) ofmt() string { return in.vars["OFMT"].str("%.6g") }

// text renders a value for output through OFMT rather than CONVFMT.
//
// The two differ only for `print`, and only for a number that is not integral: awk uses
// OFMT there and CONVFMT everywhere else a number becomes a string.
func (in *awkInterp) text(value awkValue) string {
	if value.kind == awkNumber {
		return formatAwkNumber(value.number, in.ofmt())
	}
	return value.str(in.convfmt())
}

// errorf is how the interpreter refuses at run time.
//
// A run-time failure in awk stops the program: both references print the message and exit
// 2, rather than skipping the record and carrying on.
func (in *awkInterp) errorf(format string, args ...any) error {
	return fmt.Errorf(format, args...)
}
