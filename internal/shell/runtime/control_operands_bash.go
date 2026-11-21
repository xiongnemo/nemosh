package runtime

import (
	"fmt"
	"strconv"
	"strings"
)

// Bash mode reads the operands of break, continue, exit, return and shift as bash 5.3's
// get_numeric_arg and get_exitstat read them, measured against Git for Windows' bash:
//
//   - a `--` in front of them is passed over: `exit -- 3` is 3;
//   - a number is bash's legal_number, blanks around a sign and digits in 64 bits: `break ' 1 '`
//     is 1, 99999999999999999999 no number;
//   - a second operand is `too many arguments`, and the rest of the line is abandoned, status 2,
//     whatever `set -e` says (no_args, jumping to the top level to discard the command);
//   - break's or continue's count that is no number ends the script, status 2, as
//     throw_to_top_level ends a shell that is not interactive; one of 0 or less is `loop count
//     out of range`, status 1, and every loop is left;
//   - `shift -1` is `shift count out of range`, status 1.
//
// busybox reads every one of them another way -- `--` is no number to it, a count of 0 ends the
// script, a second operand is ignored -- and nemosh and sh keep busybox's.

// bashControlOperands is the operands of a control builtin, a `--` in front passed over.
func bashControlOperands(args []string) []string {
	operands := args[1:]
	if len(operands) > 0 && operands[0] == "--" {
		operands = operands[1:]
	}
	return operands
}

// bashLegalNumber is bash's legal_number.
func bashLegalNumber(text string) (int64, bool) {
	trimmed := strings.TrimRight(strings.TrimLeft(text, " \t\n\v\f\r"), " \t")
	value, err := strconv.ParseInt(trimmed, 10, 64)
	return value, err == nil
}

// bashTooMany says a control builtin's second operand as bash says it, and abandons the line.
func (r Runtime) bashTooMany(name string) lineResult {
	fmt.Fprintf(r.streams.Stderr, "%s%s: too many arguments\n", r.diagnosticPrefix(), name)
	return lineResult{status: 2, control: flowSkipLine}
}

// bashLoopControl is break or continue in bash mode. Outside a loop it is bash's note, status 0,
// whatever its operands are, as bash looks at the loop first.
func (r Runtime) bashLoopControl(name string, args []string) lineResult {
	if r.loops.depth == 0 {
		fmt.Fprintf(r.streams.Stderr, "%s%s: only meaningful in a `for', `while', or `until' loop\n", r.diagnosticPrefix(), name)
		return lineResult{status: 0}
	}
	operands := bashControlOperands(append([]string{name}, args...))
	count := int64(1)
	if len(operands) > 0 {
		value, ok := bashLegalNumber(operands[0])
		if !ok {
			fmt.Fprintf(r.streams.Stderr, "%s%s: %s: numeric argument required\n", r.diagnosticPrefix(), name, operands[0])
			return lineResult{status: 2, control: flowAbort}
		}
		if len(operands) > 1 {
			return r.bashTooMany(name)
		}
		count = value
	}
	if count <= 0 {
		fmt.Fprintf(r.streams.Stderr, "%s%s: %s: loop count out of range\n", r.diagnosticPrefix(), name, operands[0])
		r.loops.request(r.loops.depth)
		return lineResult{status: 1, control: flowBreak}
	}
	r.loops.request(int(min(count, int64(r.loops.depth))))
	if name == "break" {
		return lineResult{control: flowBreak}
	}
	return lineResult{control: flowContinue}
}

// bashStatusOperand is exit's or return's operand in bash mode: the status, and the result to
// answer instead when the operands are bash's error. A status that is no number is said, and is
// 2, exit going on with the line and return returning, as bash 5.3 has them.
func (r Runtime) bashStatusOperand(args []string, savedStatus int) (int, *lineResult) {
	operands := bashControlOperands(args)
	if len(operands) == 0 {
		return savedStatus, nil
	}
	value, ok := bashLegalNumber(operands[0])
	if !ok {
		fmt.Fprintf(r.streams.Stderr, "%s%s: %s: numeric argument required\n", r.diagnosticPrefix(), args[0], operands[0])
		if args[0] == "exit" {
			return 2, &lineResult{status: 2}
		}
		return 2, nil
	}
	if len(operands) > 1 {
		result := r.bashTooMany(args[0])
		return 2, &result
	}
	return int(value & 0xff), nil
}

// bashShiftCount is shift's count in bash mode, or, when its operands are bash's error, the
// status to answer and true. A count past the parameters is one past them, which is enough to
// be out of range without holding a 64-bit number in an int.
func (r Runtime) bashShiftCount(args []string) (int, int, bool) {
	operands := bashControlOperands(append([]string{"shift"}, args...))
	if len(operands) == 0 {
		return 1, 0, false
	}
	value, ok := bashLegalNumber(operands[0])
	switch {
	case !ok:
		fmt.Fprintf(r.streams.Stderr, "%sshift: %s: numeric argument required\n", r.diagnosticPrefix(), operands[0])
		return 0, 2, true
	case len(operands) > 1:
		fmt.Fprintf(r.streams.Stderr, "%sshift: too many arguments\n", r.diagnosticPrefix())
		r.expansion.shellError, r.expansion.discard, r.expansion.skipLine = true, true, true
		r.expansion.shellErrorStatus = 2
		return 0, 2, true
	case value < 0:
		fmt.Fprintf(r.streams.Stderr, "%sshift: %s: shift count out of range\n", r.diagnosticPrefix(), operands[0])
		return 0, 1, true
	}
	return int(min(value, int64(len(r.params.values))+1)), 0, false
}
