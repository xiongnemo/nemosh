package applets

import (
	"bufio"
	"context"
	"io"
	"strings"
)

// Running a program: BEGIN, the record loop, then END.
//
// Three things about the order are worth stating, because each is a rule a program can
// see:
//
//   - **END runs even after `exit`.** That is what makes `{ if (bad) exit 1 } END { ... }`
//     a working idiom -- the END block is the one place a program can tidy up. An `exit`
//     *inside* END is the only one that leaves immediately.
//   - **BEGIN alone means the input is never read.** A program with no main rules and no
//     END does not wait on stdin, which is what makes `awk 'BEGIN{print 1}'` return at
//     once rather than hanging on a terminal.
//   - **A pattern with no action means `{ print }`**, which the parser recorded as a nil
//     action rather than synthesising, so the decision is made here where it is visible.

// runAwkProgram is the whole run: it answers the exit status.
func runAwkProgram(ctx context.Context, program *awkProgram, invocation *awkInvocation,
	input io.Reader, output, errors io.Writer) (int, error) {
	interp := newAwkInterp(ctx, program, input, output, errors)
	defer interp.buffered.Flush()

	if err := interp.applyInvocation(invocation); err != nil {
		return 2, err
	}
	if err := interp.runBegin(); err != nil {
		return 2, err
	}
	// The input is only read when something could act on it. Both references skip it
	// for a BEGIN-only program, and reading anyway would make `awk 'BEGIN{print}'` hang
	// on a terminal.
	if !interp.exiting && interp.readsInput() {
		if err := interp.runInputs(); err != nil {
			return 2, err
		}
	}
	if err := interp.runEnd(); err != nil {
		return 2, err
	}
	// Everything still open is finished here: a file redirect is flushed and closed, and
	// a pipe finally runs its applet. A program that never calls `close` still gets its
	// output, which is what both references do.
	if err := interp.closeAllStreams(); err != nil {
		return 2, err
	}
	if interp.deferred != nil {
		return 2, interp.deferred
	}
	return interp.exitStatus, nil
}

// readsInput reports whether any rule could act on a record.
func (in *awkInterp) readsInput() bool {
	for _, item := range in.program.items {
		if item.kind != awkItemBegin {
			return true
		}
	}
	return false
}

func (in *awkInterp) runBegin() error {
	for index := range in.program.items {
		if in.program.items[index].kind != awkItemBegin {
			continue
		}
		flow, err := in.execBlock(in.program.items[index].action)
		if err != nil {
			return err
		}
		if flow == awkFlowExit {
			return nil
		}
	}
	return nil
}

func (in *awkInterp) runEnd() error {
	// An `exit` in BEGIN or a rule still runs END, but it must not re-arm: `exiting`
	// is cleared so that an `exit` inside END is the one that leaves.
	in.exiting = false
	for index := range in.program.items {
		if in.program.items[index].kind != awkItemEnd {
			continue
		}
		flow, err := in.execBlock(in.program.items[index].action)
		if err != nil {
			return err
		}
		if flow == awkFlowExit {
			return nil
		}
	}
	return nil
}

// applyInvocation settles what the command line asked for, before BEGIN runs.
//
// The order matters: `-F` and `-v` land first so a BEGIN block can still override either,
// which is what makes `awk -F: 'BEGIN{FS=","}'` use a comma.
func (in *awkInterp) applyInvocation(invocation *awkInvocation) error {
	if invocation == nil {
		invocation = &awkInvocation{}
	}
	if invocation.hasSeparator {
		in.vars["FS"] = awkStr(invocation.fieldSeparator)
	}
	for _, assignment := range invocation.assignments {
		name, value, ok := awkOperandAssignment(assignment)
		if !ok {
			return in.errorf("-v takes var=value, not %q", assignment)
		}
		// A `-v` value is a strnum, so `-v n=10` compares numerically.
		in.setVar(name, awkStrnumOf(awkExpandAssignmentValue(value)))
	}
	in.setArguments(invocation.operands)
	return nil
}

// readRecord reads one record, honouring RS.
//
// RS is a single character or empty here; POSIX allows no more, and a regular expression
// RS is a gawk extension this refuses by not implementing. An **empty RS** is paragraph
// mode: records are separated by blank lines and a newline always separates fields.
func (in *awkInterp) readRecord(reader *bufio.Reader) (string, bool, error) {
	separator := in.vars["RS"].str(in.convfmt())
	if separator == "" {
		return in.readParagraph(reader)
	}
	delimiter := byte('\n')
	if separator != "" {
		delimiter = separator[0]
	}
	text, err := reader.ReadString(delimiter)
	if text == "" && err != nil {
		if err == io.EOF {
			return "", false, nil
		}
		return "", false, err
	}
	text = strings.TrimSuffix(text, string(delimiter))
	// A CRLF file read with the default RS leaves the carriage return on the record,
	// which would then be the last field's tail. Windows is the platform here, so it
	// goes -- see docs/support-matrix.md.
	if delimiter == '\n' {
		text = strings.TrimSuffix(text, "\r")
	}
	return text, true, nil
}

// readParagraph implements the empty-RS form: records separated by one or more blank
// lines, with leading blank lines skipped.
func (in *awkInterp) readParagraph(reader *bufio.Reader) (string, bool, error) {
	var lines []string
	started := false
	for {
		line, err := reader.ReadString('\n')
		trimmed := strings.TrimRight(line, "\r\n")
		atEnd := err != nil
		if trimmed == "" && line != "" || (trimmed == "" && atEnd && len(lines) > 0) {
			if started {
				return strings.Join(lines, "\n"), true, nil
			}
			// Leading blank lines are skipped rather than producing empty records.
			if atEnd {
				return "", false, nil
			}
			continue
		}
		if trimmed != "" {
			lines = append(lines, trimmed)
			started = true
		}
		if atEnd {
			if started {
				return strings.Join(lines, "\n"), true, nil
			}
			return "", false, nil
		}
	}
}

// runRules applies every main rule to the current record.
func (in *awkInterp) runRules() (awkFlow, error) {
	for index := range in.program.items {
		item := &in.program.items[index]
		if item.kind == awkItemBegin || item.kind == awkItemEnd {
			continue
		}
		matched, err := in.itemMatches(item)
		if err != nil {
			return awkFlowNone, err
		}
		if !matched {
			continue
		}
		flow, err := in.runAction(item)
		if err != nil {
			return awkFlowNone, err
		}
		switch flow {
		case awkFlowNext:
			return awkFlowNone, nil
		case awkFlowNextFile, awkFlowExit:
			// `next` abandons the record and `nextfile` the whole input, which only
			// became two different things once there were several files to leave.
			return flow, nil
		}
	}
	return awkFlowNone, nil
}

// runAction runs a rule's body, or prints the record when it had none.
func (in *awkInterp) runAction(item *awkItem) (awkFlow, error) {
	if item.action == nil {
		return awkFlowNone, in.write(in.getRecord() + in.vars["ORS"].str(in.convfmt()))
	}
	return in.execBlock(item.action)
}

// itemMatches decides whether a rule applies to the current record.
//
// A **range** is the one that carries state: it turns on at the record matching the first
// pattern and off at the one matching the second, and a record can do both at once --
// `/a/,/a/` selects single lines, which both references confirm.
func (in *awkInterp) itemMatches(item *awkItem) (bool, error) {
	switch item.kind {
	case awkItemAlways:
		return true, nil
	case awkItemPattern:
		value, err := in.eval(item.pattern)
		if err != nil {
			return false, err
		}
		return value.boolean(), nil
	case awkItemRange:
		return in.rangeMatches(item)
	}
	return false, nil
}

func (in *awkInterp) rangeMatches(item *awkItem) (bool, error) {
	if !item.active {
		start, err := in.eval(item.pattern)
		if err != nil {
			return false, err
		}
		if !start.boolean() {
			return false, nil
		}
		item.active = true
	}
	// The closing pattern is tested on the *same* record that opened the range, so
	// `/a/,/a/` matches one line at a time.
	stop, err := in.eval(item.until)
	if err != nil {
		return false, err
	}
	if stop.boolean() {
		item.active = false
	}
	return true, nil
}
