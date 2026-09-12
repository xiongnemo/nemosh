package applets

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// `truncate`, `link` and `unlink`: the three commands that change a file's size or its name
// without touching what is inside it.
//
// All three take paths, so all three resolve through the process view -- `cd` in this shell
// does not move the process, so a relative name has to be resolved against where the
// *shell* is rather than where the executable happens to be.

// truncate sets a file's size exactly, which is how a log is emptied without the file being
// replaced -- anything holding it open keeps the same file.
//
// A size may carry a suffix (`K`, `M`, `G`, and the `KB`/`MB` decimal forms), and may be
// **relative**: `+1M` grows and `-1M` shrinks. busybox refuses the relative forms and GNU
// accepts them; they are accepted here because reading the size first is all they cost and
// `truncate -s +1M` is a thing people write. GNU's `<`, `>`, `/` and `%` are refused by
// name rather than approximated.
//
// `-c` means do not create: a file that is not there is left alone, and that is **not** an
// error, which is what lets `truncate -c -s 0 maybe.log` be safe to run either way.
func newTruncateApplet() Applet {
	return simpleApplet{name: "truncate", runContext: func(ctx context.Context, args []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
		options, operands, err := parseAppletOptions(args, "c", "s")
		if err != nil {
			return err
		}
		if !options.has('s') {
			return fmt.Errorf("you must specify a size with -s")
		}
		if len(operands) == 0 {
			return missingOperand()
		}
		view := ProcessViewFromContext(ctx)
		for _, operand := range operands {
			if err := truncateOne(view, operand, options.values['s'], options.has('c')); err != nil {
				return err
			}
		}
		return nil
	}}
}

func truncateOne(view ProcessView, operand, size string, keepMissing bool) error {
	native, err := resolveHostPath(view, operand)
	if err != nil {
		return err
	}
	info, statErr := os.Stat(native)
	if statErr != nil {
		if !os.IsNotExist(statErr) {
			return cannotStat(operand, statErr)
		}
		if keepMissing {
			// -c: absent is the answer, not a failure.
			return nil
		}
	}
	current := int64(0)
	if statErr == nil {
		current = info.Size()
	}
	target, err := truncateSize(size, current)
	if err != nil {
		return err
	}
	if err := os.Truncate(native, target); err != nil {
		if os.IsNotExist(err) && !keepMissing {
			// Truncate does not create, so a missing file is made first and then sized.
			return createThenTruncate(native, operand, target)
		}
		return operandFailure(operand, err)
	}
	return nil
}

func createThenTruncate(native, operand string, target int64) error {
	file, err := os.OpenFile(native, os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return cannotCreate(operand, err)
	}
	defer file.Close()
	if err := file.Truncate(target); err != nil {
		return operandFailure(operand, err)
	}
	return nil
}

// truncateSize reads the -s value against the file's current size.
func truncateSize(spec string, current int64) (int64, error) {
	if spec == "" {
		return 0, fmt.Errorf("invalid number ''")
	}
	relative := int64(0)
	switch spec[0] {
	case '+':
		relative, spec = 1, spec[1:]
	case '-':
		relative, spec = -1, spec[1:]
	case '<', '>', '/', '%':
		return 0, fmt.Errorf("%c sizes are not supported; use a plain, + or - size", spec[0])
	}
	amount, err := parseSizeWithSuffix(spec)
	if err != nil {
		return 0, err
	}
	if relative == 0 {
		return amount, nil
	}
	target := current + relative*amount
	if target < 0 {
		// Shrinking past the start empties the file rather than failing, which is what
		// GNU does and the only answer that is not an error message about arithmetic.
		return 0, nil
	}
	return target, nil
}

// parseSizeWithSuffix reads a byte count, with the suffixes both references accept.
//
// `K` is 1024 and `KB` is 1000, which is the split GNU uses and the one that stops
// `truncate -s 1K` quietly meaning something different from what the reader assumed.
func parseSizeWithSuffix(spec string) (int64, error) {
	multipliers := []struct {
		suffix string
		factor int64
	}{
		{"KB", 1000}, {"MB", 1000 * 1000}, {"GB", 1000 * 1000 * 1000},
		{"K", 1024}, {"M", 1024 * 1024}, {"G", 1024 * 1024 * 1024},
	}
	digits, factor := spec, int64(1)
	upper := strings.ToUpper(spec)
	for _, multiplier := range multipliers {
		if strings.HasSuffix(upper, multiplier.suffix) {
			digits, factor = spec[:len(spec)-len(multiplier.suffix)], multiplier.factor
			break
		}
	}
	value, err := strconv.ParseInt(digits, 10, 64)
	if err != nil || value < 0 {
		return 0, fmt.Errorf("invalid number '%s'", spec)
	}
	return value * factor, nil
}

// link makes a second name for one file, which is the raw form of what `ln` does.
//
// It refuses an existing name rather than replacing it: a hard link that quietly overwrote
// its target would destroy the thing it was asked to point at.
func newLinkApplet() Applet {
	return simpleApplet{name: "link", runContext: func(ctx context.Context, args []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
		_, operands, err := parseAppletOptions(args, "", "")
		if err != nil {
			return err
		}
		if len(operands) != 2 {
			return missingOperand()
		}
		view := ProcessViewFromContext(ctx)
		source, err := resolveHostPath(view, operands[0])
		if err != nil {
			return err
		}
		target, err := resolveHostPath(view, operands[1])
		if err != nil {
			return err
		}
		if err := os.Link(source, target); err != nil {
			return fmt.Errorf("cannot create hard link '%s' to '%s': %s",
				operands[1], operands[0], CauseText(err))
		}
		return nil
	}}
}

// unlink removes one name, and only a name: it is `rm` with no options, no recursion and no
// prompt, which is why a script that means exactly that reaches for it.
//
// A directory is refused, because removing one is what `rmdir` is for and `unlink` on a
// directory is the mistake this command exists to make impossible.
func newUnlinkApplet() Applet {
	return simpleApplet{name: "unlink", runContext: func(ctx context.Context, args []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
		_, operands, err := parseAppletOptions(args, "", "")
		if err != nil {
			return err
		}
		if len(operands) != 1 {
			return missingOperand()
		}
		native, err := resolveHostPath(ProcessViewFromContext(ctx), operands[0])
		if err != nil {
			return err
		}
		if info, err := os.Lstat(native); err == nil && info.IsDir() {
			return fmt.Errorf("cannot unlink '%s': Is a directory", operands[0])
		}
		if err := os.Remove(native); err != nil {
			return cannotRemove(operands[0], err)
		}
		return nil
	}}
}
