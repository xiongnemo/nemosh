package applets

import (
	"bufio"
	"io"
	"os"
)

// Walking the operands: which files are read, in what order, and what that does to the
// built-in variables.
//
// **ARGV is walked at run time, not copied at the start.** A program may rewrite it in
// BEGIN, and both references honour that -- `BEGIN { ARGV[1] = "have"; ARGC = 2 }` reads
// `have` whatever was named on the command line. So each step re-reads ARGC and the element
// rather than iterating a slice captured earlier, and an element set to the empty string is
// skipped, which is how a program drops a file from its own argument list.
//
// **NR runs across every file and FNR restarts in each**, which is the whole reason both
// exist. FILENAME follows the file being read and is empty in BEGIN, since no file is open
// yet -- measured, and both references agree.

// runInputs is the record loop over everything the command line named.
func (in *awkInterp) runInputs() error {
	read := false
	for index := 1; index < in.argc(); index++ {
		argument := in.argvAt(index)
		if argument == "" {
			continue
		}
		if name, value, isAssignment := awkOperandAssignment(argument); isAssignment {
			// An assignment takes effect **here**, between one file and the next, which
			// is what makes `awk '{print v}' a v=1 b` print nothing for a's records.
			// A strnum, like a field, so `v=10` compares numerically.
			in.setVar(name, awkStrnumOf(awkExpandAssignmentValue(value)))
			continue
		}
		read = true
		if err := in.runOneFile(argument); err != nil {
			return err
		}
		if in.exiting {
			return nil
		}
	}
	if !read {
		// No file operands at all -- only assignments, or nothing -- means standard
		// input. That is what makes `awk '{print v, $0}' v=1` work on a pipe.
		return in.runRecordsFrom(bufio.NewReader(in.input))
	}
	return nil
}

func (in *awkInterp) argc() int {
	count := int(in.getVar("ARGC").num())
	if count < 0 {
		return 0
	}
	return count
}

func (in *awkInterp) argvAt(index int) string {
	array, known := in.lookupArray("ARGV")
	if !known {
		return ""
	}
	value, _ := array.get(formatAwkNumber(float64(index), in.convfmt()))
	return value.str(in.convfmt())
}

// setArguments fills ARGV and ARGC before BEGIN runs.
//
// ARGV[0] is the name of the program itself, which is `awk` here; the operands follow, and
// ARGC counts all of them together.
func (in *awkInterp) setArguments(operands []string) {
	array := in.getArray("ARGV")
	array.clear()
	array.set("0", awkStr("awk"))
	for index, operand := range operands {
		array.set(formatAwkNumber(float64(index+1), "%.6g"), awkStrnumOf(operand))
	}
	in.vars["ARGC"] = awkNum(float64(len(operands) + 1))
}

// runOneFile reads one named input to its end, or until `nextfile` or `exit`.
func (in *awkInterp) runOneFile(name string) error {
	reader, file, err := in.openRecordSource(name)
	if err != nil {
		return err
	}
	if file != nil {
		defer file.Close()
	}
	in.vars["FILENAME"] = awkStr(name)
	// FNR counts within the file and starts again at each one; NR does not.
	in.vars["FNR"] = awkNum(0)
	return in.runRecordsFrom(reader)
}

// openRecordSource opens a named input, with `-` meaning standard input.
func (in *awkInterp) openRecordSource(name string) (*bufio.Reader, *os.File, error) {
	if name == "-" || name == "/dev/stdin" {
		return bufio.NewReader(in.input), nil, nil
	}
	file, err := os.Open(name)
	if err != nil {
		return nil, nil, cannotOpen(name, err)
	}
	// Through the same UTF-16 decoding every text applet uses.
	return bufio.NewReader(decodeTextInput(file)), file, nil
}

// runRecordsFrom is the record loop over one input.
//
// The reader is left on the interpreter as well as used here, because a plain `getline`
// reads from the file the loop is reading: two readers over one stream would each buffer a
// different part of it and the records would interleave.
func (in *awkInterp) runRecordsFrom(reader *bufio.Reader) error {
	in.records = reader
	for {
		record, ok, err := in.readRecord(reader)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		in.setRecord(record)
		in.vars["NR"] = awkNum(in.vars["NR"].num() + 1)
		in.vars["FNR"] = awkNum(in.vars["FNR"].num() + 1)

		flow, err := in.runRules()
		if err != nil {
			return err
		}
		if in.streaming {
			// One flush per record, so `tail -f log | awk '...'` shows a line when the
			// line happens. busybox's awk does this and gawk does not; busybox is the
			// reference here and is the more useful of the two. It is skipped when the
			// output is a regular file, where nobody is watching and a write per record
			// would be paid for nothing.
			if err := in.buffered.Flush(); err != nil {
				return err
			}
		}
		switch flow {
		case awkFlowExit:
			return nil
		case awkFlowNextFile:
			// `nextfile` abandons this input and moves to the next operand, which is
			// what distinguishes it from `next`.
			return nil
		}
	}
}

// readAllText reads a whole reader through the text decoding, for `-f`.
func readAllText(reader io.Reader) (string, error) {
	data, err := io.ReadAll(decodeTextInput(reader))
	if err != nil {
		return "", err
	}
	return string(data), nil
}
