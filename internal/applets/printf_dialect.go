package applets

import (
	"context"
	"fmt"
	"io"
)

// printfDialect is whose printf this is: busybox's, or bash's when the shell running it is in
// bash mode. The two part on a number that is not read whole -- busybox writes zero, bash what
// it read before it stopped, and the largest the conversion holds for one too large -- and on
// how a refusal is said: bash's words, after the shell's prefix, as a builtin of bash's says
// them. The shell's view answers which (BashBuiltinPrefix); anything else is busybox's.
type printfDialect struct {
	bash   bool
	prefix string
}

func printfDialectOf(ctx context.Context) printfDialect {
	view, ok := ctx.Value(processViewKey{}).(interface{ BashBuiltinPrefix() (string, bool) })
	if !ok {
		return printfDialect{}
	}
	prefix, bash := view.BashBuiltinPrefix()
	return printfDialect{bash: bash, prefix: prefix}
}

// errPrintfRange is an operand too large for its conversion, which bash writes as the largest
// the conversion holds; busybox's is an invalid number.
type errPrintfRange struct{ operand string }

func (e errPrintfRange) Error() string { return e.operand + ": Numerical result out of range" }

// errPrintfUnfinished is a format that ends inside a conversion, `%5`.
type errPrintfUnfinished struct{ spec string }

func (e errPrintfUnfinished) Error() string { return e.spec + ": invalid format" }

// errPrintfConversion is a conversion neither printf has, `%k`.
type errPrintfConversion struct{ verb byte }

func (e errPrintfConversion) Error() string {
	return fmt.Sprintf("invalid conversion specification %%%c", e.verb)
}

// say writes err, an operand's or the format's, as this printf says it.
func (d printfDialect) say(diagnostics io.Writer, err error) {
	if !d.bash {
		fmt.Fprintf(diagnostics, "printf: %v\n", err)
		return
	}
	message := err.Error()
	switch failure := err.(type) {
	case errPrintfNumber:
		message = failure.operand + ": " + bashInvalidNumber(failure.operand)
	case errPrintfUnfinished:
		message = "`" + failure.spec + "': missing format character"
	case errPrintfConversion:
		message = fmt.Sprintf("`%c': invalid format character", failure.verb)
	}
	fmt.Fprintf(diagnostics, "%sprintf: %s\n", d.prefix, message)
}

// bashInvalidNumber is bash's sh_invalidnum: an operand written as octal or hex is said to be
// an invalid one of those.
func bashInvalidNumber(operand string) string {
	switch {
	case len(operand) > 1 && operand[0] == '0' && isDecimalDigit(operand[1]):
		return "invalid octal number"
	case len(operand) > 1 && operand[0] == '0' && operand[1] == 'x':
		return "invalid hex number"
	}
	return "invalid number"
}
