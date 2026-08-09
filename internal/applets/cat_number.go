package applets

import (
	"bufio"
	"context"
	"fmt"
	"io"
)

// lineNumberer is `cat -n`: every line prefixed with its number.
//
// It carries the count across operands rather than restarting per file, because
// `cat -n a b` is one document with continuous numbering -- that is what makes
// the option worth having over numbering each file separately.
//
// The layout is busybox's, which is GNU's: the number right-aligned in six
// columns and then a tab (coreutils/cat.c). Six is wide enough for any file
// anyone reads this way and keeps the text in one column until it is not.
type lineNumberer struct {
	on   bool
	line int
}

// copy writes input to stdout, numbered or not.
//
// Unnumbered it is the plain stream copy, byte for byte, which matters: cat is
// the applet a binary goes through, and a scanner would mangle one. Numbering
// necessarily reads lines, so that path accepts the cost it cannot avoid.
func (n *lineNumberer) copy(ctx context.Context, stdout io.Writer, input io.Reader) (int64, error) {
	if !n.on {
		return copyWithContext(ctx, stdout, input)
	}
	written := int64(0)
	scanner := bufio.NewScanner(contextReader{ctx: ctx, reader: input})
	for scanner.Scan() {
		n.line++
		count, err := fmt.Fprintf(stdout, "%6d\t%s\n", n.line, scanner.Text())
		written += int64(count)
		if err != nil {
			return written, err
		}
	}
	return written, scanner.Err()
}
