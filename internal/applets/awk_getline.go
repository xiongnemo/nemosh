package applets

import (
	"bufio"
	"os"
)

// `getline`, in all six of its forms.
//
// The forms differ in **where they read** and in **what they set**, and the second half is
// the part people get wrong. Measured, and both references agree:
//
//	getline              $0  NF  NR  FNR     the next record of the main input
//	getline var          var     NR  FNR     no re-split, so NF does not move
//	getline < file       $0  NF               a file, with no bearing on NR
//	getline var < file   var
//	cmd | getline        $0  NF  NR
//	cmd | getline var    var     NR
//
// So `BEGIN { getline v }` on an input of `l1` leaves NR at 1 and **NF at 0**: nothing was
// split, because nothing became the record.
//
// The return value has three meanings and a program is expected to tell them apart:
// **1 read something, 0 reached the end, -1 could not open**. A missing file is -1 rather
// than an error, which is what makes `while ((getline line < f) > 0)` the idiom it is --
// and why the condition is written `> 0` rather than as a plain truth test, since -1 is
// true.

// awkInput is one source a program is reading from.
type awkInput struct {
	reader *bufio.Reader
	file   *os.File
}

func (s *awkInput) close() {
	if s.file != nil {
		_ = s.file.Close()
	}
}

func (in *awkInterp) evalGetline(node awkGetlineExpr) (awkValue, error) {
	switch node.mode {
	case awkGetlineFile:
		return in.getlineFrom(node, false)
	case awkGetlineCommand:
		return in.getlineFrom(node, true)
	}
	return in.getlineMain(node)
}

// getlineMain reads the next record of the program's own input.
func (in *awkInterp) getlineMain(node awkGetlineExpr) (awkValue, error) {
	if in.records == nil {
		// A BEGIN block may call `getline` before the record loop has started, so the
		// main reader is made on demand rather than when the loop opens.
		in.records = bufio.NewReader(in.input)
	}
	record, ok, err := in.readRecord(in.records)
	if err != nil {
		return awkNum(-1), nil
	}
	if !ok {
		return awkNum(0), nil
	}
	// NR and FNR move for both forms, because a record really was consumed.
	in.vars["NR"] = awkNum(in.vars["NR"].num() + 1)
	in.vars["FNR"] = awkNum(in.vars["FNR"].num() + 1)
	if err := in.deliver(node.target, record); err != nil {
		return awkValue{}, err
	}
	return awkNum(1), nil
}

// getlineFrom reads from a file or from a command's output.
func (in *awkInterp) getlineFrom(node awkGetlineExpr, isCommand bool) (awkValue, error) {
	source, err := in.eval(node.source)
	if err != nil {
		return awkValue{}, err
	}
	name := source.str(in.convfmt())
	stream, err := in.resolveInput(name, isCommand)
	if err != nil {
		// Could not open: -1, and not a failure of the program. A command that is not
		// an applet is the exception -- that is a mistake in the program rather than a
		// missing file, and it says so.
		if isCommand {
			return awkValue{}, err
		}
		return awkNum(-1), nil
	}
	record, ok, err := in.readRecord(stream.reader)
	if err != nil {
		return awkNum(-1), nil
	}
	if !ok {
		return awkNum(0), nil
	}
	// A command's records count towards NR, a file's do not. That asymmetry is POSIX's
	// and both references have it.
	if isCommand {
		in.vars["NR"] = awkNum(in.vars["NR"].num() + 1)
	}
	if err := in.deliver(node.target, record); err != nil {
		return awkValue{}, err
	}
	return awkNum(1), nil
}

// deliver puts a record where the form asked for it.
//
// With no target the record becomes `$0` and is split; with one it is assigned as a
// **strnum**, like a field, so `getline n < f; if (n > 10)` compares numerically.
func (in *awkInterp) deliver(target awkExpr, record string) error {
	if target == nil {
		in.setRecord(record)
		return nil
	}
	return in.storeLvalue(target, awkStrnumOf(record))
}

// resolveInput answers the reader for a name, opening it on first use.
//
// Kept open between calls, which is the whole point: `while ((getline l < f) > 0)` reads
// successive lines because the second call finds the reader the first one left.
func (in *awkInterp) resolveInput(name string, isCommand bool) (*awkInput, error) {
	if existing, open := in.inputs[name]; open {
		return existing, nil
	}
	stream, err := in.openInput(name, isCommand)
	if err != nil {
		return nil, err
	}
	if in.inputs == nil {
		in.inputs = map[string]*awkInput{}
	}
	in.inputs[name] = stream
	return stream, nil
}

func (in *awkInterp) openInput(name string, isCommand bool) (*awkInput, error) {
	if isCommand {
		// The command runs once, here, and the program reads its output a record at a
		// time. See awk_command.go for why it is an applet or nothing.
		out, err := in.runCommandOutput(name)
		if err != nil {
			return nil, err
		}
		return &awkInput{reader: bufio.NewReader(out)}, nil
	}
	if name == "-" || name == "/dev/stdin" {
		return &awkInput{reader: bufio.NewReader(in.input)}, nil
	}
	file, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	// Read through the same UTF-16 decoding every text applet uses, so a file written by
	// a Windows tool is text rather than interleaved NULs.
	return &awkInput{reader: bufio.NewReader(decodeTextInput(file)), file: file}, nil
}
