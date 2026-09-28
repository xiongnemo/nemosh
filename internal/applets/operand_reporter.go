package applets

import (
	"context"
	"fmt"
	"io"
)

// An applet given several operands goes on past one it cannot read, as busybox's do. It names
// that operand on stderr, reads the rest, and exits 1 at the end. The first such operand used to
// end the command, so `cat missing.txt present.txt` printed nothing of present.txt, and neither
// did tac, rev, nl, fold, expand, strings or od.
//
// The diagnostic goes out under the applet's name, which the applet does not otherwise carry
// into its helpers, so simpleApplet puts the name and its stderr in the context.

type operandReporter struct {
	name   string
	stderr io.Writer
}

type operandReporterKey struct{}

func withOperandReporter(ctx context.Context, name string, stderr io.Writer) context.Context {
	return context.WithValue(ctx, operandReporterKey{}, operandReporter{name: name, stderr: stderr})
}

// reportOperand writes err as the applet's diagnostic line. It answers false when no applet is
// there to write it for, and the caller returns the error instead.
func reportOperand(ctx context.Context, err error) bool {
	reporter, ok := ctx.Value(operandReporterKey{}).(operandReporter)
	if !ok || reporter.stderr == nil {
		return false
	}
	fmt.Fprintf(reporter.stderr, "%s: %v\n", reporter.name, err)
	return true
}
