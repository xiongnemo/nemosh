package runtime

import (
	"errors"
	"slices"
	"strconv"
	"strings"
)

// arithmeticFault is an arithmetic error as nemosh says it, plain, and as bash does: the
// expression with its leading blanks gone, bash's message, and the expression from the token it
// was found at to its end, `1/0 : division by 0 (error token is "0 ")`. token indexes the
// expression's tokens; the evaluation that raised the fault places it in bash mode (locateFault),
// the innermost first, so an error in a variable's value names that value, as bash's does.
// Measured 2026-11-22 against Git for Windows' bash 5.3; see bash's expr.c, evalerror.
type arithmeticFault struct {
	plain, message string
	token          int
	expression     string
	at             int
	bash           bool
}

func (f *arithmeticFault) Error() string {
	if !f.bash {
		return f.plain
	}
	return strings.TrimLeft(f.expression, " \t") + ": " + f.message + ` (error token is "` + f.expression[f.at:] + `")`
}

func (p *arithmeticParser) fault(plain, message string, token int) error {
	return &arithmeticFault{plain: plain, message: message, token: token}
}

// lastRead is the token bash's error names when it was reading on: the one it had read ahead to,
// or at the end, the last there was.
func (p *arithmeticParser) lastRead() int {
	return min(p.index, len(p.tokens)-1)
}

// leftover is a token after a whole expression, `1 2` or `3 = 4`.
func (p *arithmeticParser) leftover() error {
	token, message := p.tokens[p.index], "arithmetic syntax error in expression"
	if slices.Contains(arithmeticAssignmentOperators, token) {
		message = "attempted assignment to non-variable"
	}
	return p.fault("arithmetic syntax error: unexpected "+strconv.Quote(token), message, p.index)
}

// strayFault is a character no operator is, where one goes: an invalid operator to bash, and the
// `[` of a subscript that does not close, `a[1`, a bad array subscript named by its array.
func (p *arithmeticParser) strayFault(token string) error {
	plain := "arithmetic syntax error: unexpected " + strconv.Quote(token)
	if token == "[" && p.index > 0 && isArithmeticName(p.tokens[p.index-1]) {
		return p.fault(plain, "bad array subscript", p.index-1)
	}
	return p.fault(plain, "arithmetic syntax error: invalid arithmetic operator", p.index)
}

// operandFault is what bash says of a token where an operand goes that is none: a number it
// cannot read, by why, and anything else an operand expected. A number is its own expression
// there, `08: value too great for base (error token is "08")` out of `08 + 1`, as bash reads it.
func (p *arithmeticParser) operandFault(token string) error {
	plain := "arithmetic syntax error: unexpected " + strconv.Quote(token)
	if token[0] < '0' || token[0] > '9' {
		return p.fault(plain, "arithmetic syntax error: operand expected", p.index-1)
	}
	message := "value too great for base"
	if base, digits, found := strings.Cut(token, "#"); found {
		if radix, err := strconv.ParseInt(base, 10, 32); err != nil || radix < 2 || radix > 64 {
			message = "invalid arithmetic base"
		} else if digits == "" {
			message = "invalid integer constant"
		}
	}
	return &arithmeticFault{plain: plain, message: message, expression: token}
}

// locateFault places a fault the evaluation of expression raised, in bash mode, where one deeper
// has not placed it already.
func (r Runtime) locateFault(err error, expression string, tokens []string) error {
	var fault *arithmeticFault
	if !r.BashMode() || !errors.As(err, &fault) || fault.bash {
		return err
	}
	if fault.bash = true; fault.expression != "" {
		return err
	}
	at := 0
	for index, token := range tokens {
		for at < len(expression) && strings.IndexByte(" \t\n", expression[at]) >= 0 {
			at++
		}
		if index == fault.token {
			break
		}
		at += len(token)
	}
	fault.expression, fault.at = expression, min(at, len(expression))
	return err
}

// isArithmeticFault is whether err is a fault placed in bash's words, which a caller passes on
// without its own prefix, as bash says them.
func isArithmeticFault(err error) bool {
	var fault *arithmeticFault
	return errors.As(err, &fault) && fault.bash
}

// attributedFault is what an integer name's assignment says of an error in its value: the name
// and the error, and in bash's words the error alone, `2 3: arithmetic syntax error in
// expression (error token is "3")` for `declare -i n; n='2 3'`.
func attributedFault(name string, err error) string {
	if isArithmeticFault(err) {
		return err.Error()
	}
	return name + ": " + err.Error()
}
