package applets

import (
	"context"
	"fmt"
	"io"
	"os"
)

// `cp a b c dir/` and `mv a b c dir/` -- the POSIX `source_file... target_directory` form.
//
// Both applets took exactly two operands and refused a third with `extra operand 'c'`, which
// is a refusal POSIX does not license and which scripts hit constantly. busybox-w32 was
// measured for the shape of the answer: it copies what it can, names each source it could
// not, and exits 1 if any failed.
//
// The last operand must already be a directory. That is the test both references apply, and
// it is the only one that can be applied -- `cp a b c` with c a plain file cannot mean
// anything, and guessing would overwrite c three times.

// copyManyOperands applies one operation to every source in turn, reporting each failure and
// carrying on. It answers whether all of them succeeded.
//
// A per-operand loop rather than a fail-fast one, because stopping at the first failure
// leaves a half-done copy with no record of which half: busybox names each source and
// continues, and a script reading stderr can then tell what it still has to do.
func copyManyOperands(
	ctx context.Context,
	applet string,
	sources []string,
	directory string,
	stderr io.Writer,
	operation func(source, dest pathOperand) error,
) error {
	view := ProcessViewFromContext(ctx)
	destHost, err := resolveHostPath(view, directory)
	if err != nil {
		return err
	}
	info, statErr := os.Stat(destHost)
	if statErr != nil || !info.IsDir() {
		// Named as the failure it is rather than as "extra operand": the operands are
		// fine, and what is wrong is that the last one is not a directory they can go
		// into. busybox says the same thing.
		return fmt.Errorf("target '%s' is not a directory", directory)
	}
	failed := false
	for _, source := range sources {
		sourceHost, resolveErr := resolveHostPath(view, source)
		if resolveErr != nil {
			failed = true
			if _, writeErr := fmt.Fprintf(stderr, "%s: %v\n", applet, resolveErr); writeErr != nil {
				return writeErr
			}
			continue
		}
		from := pathOperand{host: sourceHost, operand: source}
		into := copyDestination(from, pathOperand{host: destHost, operand: directory})
		if err := operation(from, into); err != nil {
			failed = true
			if _, writeErr := fmt.Fprintf(stderr, "%s: %v\n", applet, err); writeErr != nil {
				return writeErr
			}
		}
	}
	if failed {
		return ErrExitFalse
	}
	return nil
}
