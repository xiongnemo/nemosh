package applets

import (
	"errors"
	"strconv"
	"strings"
	"syscall"
)

// bashTest is test as bash 5.3's test.c reads its arguments, which bash mode's test and [ take
// (newTestApplet): by how many there are, POSIX 2.14's rules, with -a and -o between two words
// their AND and OR of the two as single arguments, so `[ -z -a ] ]` is true; past four, bash's
// grammar, which looks for a binary operator before a unary one and evaluates both sides of -a
// and -o; and unary -a is -e. A refusal is bash's words, status 2. Measured against Git for
// Windows' bash, a line of a script per case. busybox's, testEvaluator, refuses `[ -a / ]`.
type bashTest struct {
	e    *testEvaluator
	args []string
	pos  int
	// after is the word after the last argument, `]` for [, which bash names when a group is not
	// closed: "`)' expected, found ]".
	after string
}

// errBashTest is a refusal, in bash's words.
type errBashTest string

func (e errBashTest) Error() string { return string(e) }

func (t *bashTest) run() (bool, error) {
	var value bool
	var err error
	switch len(t.args) {
	case 0:
	case 1:
		value = t.args[0] != ""
	case 2:
		value, err = t.two()
	case 3:
		value, err = t.three()
	case 4:
		switch {
		case t.args[0] == "!":
			t.pos = 1
			value, err = t.three()
			value = !value
		case t.args[0] == "(" && t.args[3] == ")":
			t.pos = 1
			value, err = t.two()
		default:
			return t.whole()
		}
	default:
		return t.whole()
	}
	return value, err
}

// whole is bash's grammar over every argument, which has to use them all.
func (t *bashTest) whole() (bool, error) {
	value, err := t.expression()
	if err == nil && t.pos < len(t.args) {
		err = errBashTest("too many arguments")
	}
	return value, err
}

// two is two arguments from pos: `!` and one, or a unary operator and its operand.
func (t *bashTest) two() (bool, error) {
	first, second := t.args[t.pos], t.args[t.pos+1]
	t.pos = len(t.args)
	switch {
	case first == "!":
		return second == "", nil
	case bashTestUnary(first):
		return t.unary(first, second)
	}
	return false, errBashTest(first + ": unary operator expected")
}

// three is three arguments from pos.
func (t *bashTest) three() (bool, error) {
	first, middle, last := t.args[t.pos], t.args[t.pos+1], t.args[t.pos+2]
	switch {
	case bashTestBinary(middle):
		t.pos = len(t.args)
		return t.binary(first, middle, last)
	case middle == "-a" || middle == "-o":
		t.pos = len(t.args)
		if middle == "-a" {
			return first != "" && last != "", nil
		}
		return first != "" || last != "", nil
	case first == "!":
		t.pos++
		value, err := t.two()
		return !value, err
	case first == "(" && strings.HasPrefix(last, ")"):
		t.pos = len(t.args)
		return middle != "", nil
	}
	return false, errBashTest(middle + ": binary operator expected")
}

func (t *bashTest) expression() (bool, error) {
	if t.pos >= len(t.args) {
		return false, errBashTest("argument expected")
	}
	return t.or()
}

// or and and parse both sides before they combine them, as bash's do.
func (t *bashTest) or() (bool, error) {
	value, err := t.and()
	if err == nil && t.pos < len(t.args) && t.args[t.pos] == "-o" {
		t.pos++
		right, rightErr := t.or()
		return value || right, rightErr
	}
	return value, err
}

func (t *bashTest) and() (bool, error) {
	value, err := t.term()
	if err == nil && t.pos < len(t.args) && t.args[t.pos] == "-a" {
		t.pos++
		right, rightErr := t.and()
		return value && right, rightErr
	}
	return value, err
}

func (t *bashTest) term() (bool, error) {
	if t.pos >= len(t.args) {
		return false, errBashTest("argument expected")
	}
	word := t.args[t.pos]
	switch {
	case word == "!":
		negate := false
		for t.pos < len(t.args) && t.args[t.pos] == "!" {
			t.pos++
			negate = !negate
		}
		value, err := t.term()
		return value != negate, err
	case word == "(":
		t.pos++
		value, err := t.expression()
		if err != nil {
			return false, err
		}
		switch {
		case t.pos >= len(t.args) && t.after == "":
			return false, errBashTest("`)' expected")
		case t.pos >= len(t.args):
			return false, errBashTest("`)' expected, found " + t.after)
		case t.args[t.pos] != ")":
			return false, errBashTest("`)' expected, found " + t.args[t.pos])
		}
		t.pos++
		return value, nil
	case t.pos+3 <= len(t.args) && bashTestBinary(t.args[t.pos+1]):
		t.pos += 3
		return t.binary(word, t.args[t.pos-2], t.args[t.pos-1])
	case t.pos+2 <= len(t.args) && bashTestUnary(word):
		t.pos += 2
		return t.unary(word, t.args[t.pos-1])
	}
	t.pos++
	return word != "", nil
}

// unary is a unary test: -a is -e, -t's descriptor a number as bash reads one, and the rest
// test's own.
func (t *bashTest) unary(operator, operand string) (bool, error) {
	switch operator {
	case "-a":
		operator = "-e"
	case "-t":
		descriptor, ok := bashTestNumber(operand)
		if !ok {
			return false, errBashTest(operand + ": integer expected")
		}
		return t.e.isTerminal(strconv.FormatInt(descriptor, 10))
	case "-N":
		return t.e.modifiedSinceRead(operand), nil
	}
	return t.e.unaryPrimary(operator, operand)
}

// binary is a binary test; a number compared is one as bash's legal_number reads it.
func (t *bashTest) binary(left, operator, right string) (bool, error) {
	if !strings.HasPrefix(operator, "-") || operator == "-nt" || operator == "-ot" || operator == "-ef" {
		return t.e.applyBinary(left, operator, right)
	}
	a, ok := bashTestNumber(left)
	if !ok {
		return false, errBashTest(left + ": integer expected")
	}
	b, ok := bashTestNumber(right)
	if !ok {
		return false, errBashTest(right + ": integer expected")
	}
	switch operator {
	case "-eq":
		return a == b, nil
	case "-ne":
		return a != b, nil
	case "-lt":
		return a < b, nil
	case "-le":
		return a <= b, nil
	case "-gt":
		return a > b, nil
	}
	return a >= b, nil
}

// bashTestNumber is bash's legal_number: blanks around a sign and decimal digits, in 64 bits.
func bashTestNumber(text string) (int64, bool) {
	value, err := strconv.ParseInt(strings.TrimRight(strings.TrimLeft(text, " \t\n\v\f\r"), " \t"), 10, 64)
	return value, err == nil
}

// bashTestUnary is bash's test_unop: a dash and one of its letters.
func bashTestUnary(word string) bool {
	return len(word) == 2 && word[0] == '-' && strings.IndexByte("abcdefghknoprstuvwxzGLNORS", word[1]) >= 0
}

// bashTestBinary is bash's test_binop.
func bashTestBinary(word string) bool {
	switch word {
	case "=", "==", "!=", "<", ">", "-nt", "-ot", "-ef", "-eq", "-ne", "-lt", "-le", "-gt", "-ge":
		return true
	}
	return false
}

// errBashMissingBracket is [ without its ], in bash's words.
var errBashMissingBracket = errors.New("missing `]'")

// modifiedSinceRead is bash's -N: the file is there and was written after it was last read.
func (e *testEvaluator) modifiedSinceRead(operand string) bool {
	info, err := e.stat(operand, false)
	if err != nil {
		return false
	}
	data, ok := info.Sys().(*syscall.Win32FileAttributeData)
	return ok && data.LastWriteTime.Nanoseconds() > data.LastAccessTime.Nanoseconds()
}
