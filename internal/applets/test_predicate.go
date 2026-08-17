package applets

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"golang.org/x/term"
)

func (e *testEvaluator) applyBinary(left, operator, right string) (bool, error) {
	switch operator {
	case "=", "==":
		return left == right, nil
	case "!=":
		return left != right, nil
	case "<":
		return left < right, nil
	case ">":
		return left > right, nil
	case "-nt", "-ot", "-ef":
		return e.compareFiles(left, operator, right)
	}
	leftValue, err := testNumber(left)
	if err != nil {
		return false, err
	}
	rightValue, err := testNumber(right)
	if err != nil {
		return false, err
	}
	switch operator {
	case "-eq":
		return leftValue == rightValue, nil
	case "-ne":
		return leftValue != rightValue, nil
	case "-gt":
		return leftValue > rightValue, nil
	case "-ge":
		return leftValue >= rightValue, nil
	case "-lt":
		return leftValue < rightValue, nil
	default:
		return leftValue <= rightValue, nil
	}
}

// busybox's getn spells a non-numeric operand `%s: bad number` and leaves
// status 2 behind (coreutils/test.c:468).
func testNumber(operand string) (int64, error) {
	value, err := strconv.ParseInt(strings.TrimSpace(operand), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: bad number", operand)
	}
	return value, nil
}

func (e *testEvaluator) compareFiles(left, operator, right string) (bool, error) {
	leftInfo, leftErr := e.stat(left, false)
	rightInfo, rightErr := e.stat(right, false)
	if leftErr != nil || rightErr != nil {
		return false, nil
	}
	switch operator {
	case "-nt":
		return leftInfo.ModTime().After(rightInfo.ModTime()), nil
	case "-ot":
		return leftInfo.ModTime().Before(rightInfo.ModTime()), nil
	default:
		return os.SameFile(leftInfo, rightInfo), nil
	}
}

func (e *testEvaluator) unaryPrimary(operator, operand string) (bool, error) {
	switch operator {
	case "-z":
		return operand == "", nil
	case "-n":
		return operand != "", nil
	case "-t":
		return e.isTerminal(operand)
	case "-h", "-L":
		info, err := e.stat(operand, true)
		return err == nil && info.Mode()&os.ModeSymlink != 0, nil
	}
	info, err := e.stat(operand, false)
	if err != nil {
		return false, nil
	}
	return e.fileMode(operator, operand, info), nil
}

func (e *testEvaluator) fileMode(operator, operand string, info os.FileInfo) bool {
	mode := info.Mode()
	switch operator {
	case "-e":
		return true
	case "-f":
		return mode.IsRegular()
	case "-d":
		return mode.IsDir()
	case "-s":
		return info.Size() > 0
	case "-r":
		return mode.Perm()&0o444 != 0
	case "-w":
		return mode.Perm()&0o222 != 0
	case "-x":
		return isExecutableFile(operand, info)
	case "-p":
		return mode&os.ModeNamedPipe != 0
	case "-S":
		return mode&os.ModeSocket != 0
	case "-c":
		return mode&os.ModeCharDevice != 0
	case "-b":
		return mode&os.ModeDevice != 0 && mode&os.ModeCharDevice == 0
	case "-u":
		return mode&os.ModeSetuid != 0
	case "-g":
		return mode&os.ModeSetgid != 0
	case "-k":
		return mode&os.ModeSticky != 0
	default:
		// -O and -G ask whether the effective user owns the file. busybox-w32
		// answers them from a stat that reports a fixed owner for everything
		// (win32/mingw.c:749), so on Windows the question degrades to "does it
		// exist"; Nemosh gives the same answer on every platform rather than
		// one that means different things on each.
		return true
	}
}

func (e *testEvaluator) stat(operand string, followLink bool) (os.FileInfo, error) {
	native, err := resolveHostPath(e.view, operand)
	if err != nil {
		return nil, err
	}
	if followLink {
		return os.Lstat(native)
	}
	return os.Stat(native)
}

// -t asks about a descriptor rather than a path, so it can only answer for the
// three streams the applet was handed, and only when they are real files.
func (e *testEvaluator) isTerminal(operand string) (bool, error) {
	descriptor, err := testNumber(operand)
	if err != nil {
		return false, err
	}
	if descriptor < 0 || descriptor > 2 {
		return false, nil
	}
	file, ok := e.streams[descriptor].(*os.File)
	if !ok {
		return false, nil
	}
	return term.IsTerminal(int(file.Fd())), nil
}

// EvaluateConditionPrimary is the shell's way in to `test`'s primaries, so that
// `[[ ]]` can use them rather than grow a second `-f`.
//
// Two copies of a file test would drift, and this project has fixed that class of
// bug twice -- once in `command -v` and once between `kill` and `pkill`. The
// operators here are exactly the ones `[` implements; `[[ ]]` adds its own
// pattern and regular-expression comparisons on top, because those are the ones
// that differ.
//
// `right` is ignored for a unary operator, which is what makes one entry point
// serve both shapes.
func EvaluateConditionPrimary(view ProcessView, operator, left, right string) (bool, error) {
	evaluator := &testEvaluator{view: view}
	if IsUnaryConditionOperator(operator) {
		return evaluator.unaryPrimary(operator, left)
	}
	return evaluator.applyBinary(left, operator, right)
}

// IsUnaryConditionOperator reports whether the operator takes one operand. The
// list is `test`'s, so the two cannot disagree about what `-s` is.
func IsUnaryConditionOperator(operator string) bool {
	switch operator {
	case "-e", "-f", "-d", "-r", "-w", "-x", "-s", "-z", "-n", "-L", "-h", "-b", "-c", "-p", "-S", "-t", "-g", "-u", "-k", "-O", "-G":
		return true
	}
	return false
}
