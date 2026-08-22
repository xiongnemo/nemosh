package applets

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
)

func newGrepApplet() Applet {
	return simpleApplet{name: "grep", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
		return grepStatus(runGrep(ctx, args, stdin, stdout, stderr))
	}}
}

// grep_main raises xfunc_error_retval to 2 before it parses anything, so that 1
// stays reserved for "no match" (findutils/grep.c:718). Every other failure --
// usage, a bad pattern, an unreadable operand -- carries 2 out with it.
func grepStatus(err error) error {
	if err == nil || errors.Is(err, ErrExitFalse) {
		return err
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return ExitStatusMessage(2, err)
}

func runGrep(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	flags, paths, err := grepArgs(ctx, args)
	if err != nil {
		return err
	}
	expr, err := flags.compile()
	if err != nil {
		return err
	}
	// One printer for the whole run, because the group separator spans files: a
	// `--` belongs between the last group of one file and the first of the next.
	printer := &grepPrinter{stdout: stdout, flags: flags}
	if len(paths) == 0 {
		matched, err := grepOne(grepTarget{opener: readerTarget(stdin)}, expr, flags, false, printer)
		if err != nil {
			return err
		}
		return grepMatchStatus(flags, matched, printer)
	}
	targets, err := grepTargets(ctx, flags, paths, stdin, stderr)
	if err != nil {
		return err
	}
	withNames := flags.showNames(len(targets))
	matched := false
	for _, target := range targets {
		found, err := grepOne(target, expr, flags, withNames, printer)
		if err != nil {
			return err
		}
		matched = matched || found
		if found && flags.quiet {
			// -q stops at the first match: the status is the whole answer, and
			// reading the rest of a tree to reach the same status is work nobody
			// asked for.
			return nil
		}
	}
	return grepMatchStatus(flags, matched, printer)
}

// grepMatchStatus turns what happened into an exit status.
//
// -L inverts the question: it exits 0 when it *listed* something, which is when
// some file did not match. Measured -- `grep -L M z.txt` prints z.txt and exits
// 0 where the match status alone would say 1.
func grepMatchStatus(flags grepFlags, matched bool, printer *grepPrinter) error {
	if flags.withoutMatch {
		if printer.wrote {
			return nil
		}
		return ErrExitFalse
	}
	if !matched {
		return ErrExitFalse
	}
	return nil
}

func readerTarget(stdin io.Reader) func() (io.ReadCloser, error) {
	// Decoded here too: `grep hello < notepad.txt` and `type x | nemosh grep hello` are the
	// same question as naming the file, and a BOM arrives down a pipe exactly as it does off
	// a disk.
	return func() (io.ReadCloser, error) { return io.NopCloser(decodeTextInput(stdin)), nil }
}

// grepOne searches one target and reports whether anything matched.
func grepOne(target grepTarget, expr *regexp.Regexp, flags grepFlags, withNames bool, printer *grepPrinter) (bool, error) {
	input, err := target.opener()
	if err != nil {
		if flags.noMessages {
			return false, nil
		}
		return false, operandFailure(target.name, err)
	}
	// Closed explicitly and joined rather than deferred: a close error is a real
	// failure -- a truncated read on a device -- and swallowing it would report a
	// clean search over a file that was not fully read. A test pins this.
	matched, err := grepScan(input, expr, flags, withNames, target.name, printer)
	return matched, errors.Join(err, input.Close())
}

func grepScan(input io.Reader, expr *regexp.Regexp, flags grepFlags, withNames bool, name string, printer *grepPrinter) (bool, error) {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 0, 64*1024), maxTextLine)
	matched, count, lineNumber := false, 0, 0
	// before holds the lines that were read and not printed, because -B cannot
	// know a line is context until something below it matches. after counts the
	// trailing lines still owed to the last match.
	before := &grepRing{limit: flags.beforeContext}
	after := 0
	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()
		hit := expr.MatchString(line)
		if flags.invert {
			hit = !hit
		}
		if !hit {
			if !flags.reportsPerLine() {
				continue
			}
			if after > 0 {
				// Trailing context. Printed now, so it never enters the ring and
				// cannot be printed a second time as another match's leading
				// context.
				after--
				if err := printer.emit(name, lineNumber, line, false, withNames); err != nil {
					return matched, err
				}
				continue
			}
			before.push(lineNumber, line)
			continue
		}
		matched = true
		count++
		switch {
		case flags.quiet:
			// Nothing printed, and the caller stops.
			return true, nil
		case flags.countOnly, flags.filesOnly, flags.withoutMatch:
			// All three are reported after the file rather than per line, and
			// context does not apply to them.
		default:
			if err := printer.flushBefore(before, name, withNames); err != nil {
				return matched, err
			}
			if err := grepEmitMatch(printer, expr, flags, name, lineNumber, line, withNames); err != nil {
				return matched, err
			}
			after = flags.afterContext
		}
		if flags.maxCount > 0 && count >= flags.maxCount {
			// The trailing context of the last match is still owed, which is what
			// busybox does: `grep -A1 -m1 M` prints the match and the line after
			// it. Measured.
			if err := grepDrainAfter(scanner, printer, name, &lineNumber, after, withNames); err != nil {
				return matched, err
			}
			return matched, scanner.Err()
		}
	}
	if err := scanner.Err(); err != nil {
		return matched, err
	}
	return matched, grepReportFile(printer, flags, name, matched, count, withNames)
}

// grepEmitMatch writes a matching line, whole or in pieces under -o.
func grepEmitMatch(printer *grepPrinter, expr *regexp.Regexp, flags grepFlags, name string, lineNumber int, line string, withNames bool) error {
	if !flags.onlyMatching {
		return printer.emit(name, lineNumber, line, true, withNames)
	}
	// Each match on its own line, which is what makes -o useful for pulling
	// values out.
	//
	// With -v there is nothing to print: the line was selected for *not*
	// matching, so it has no matched text. Measured -- GNU prints nothing and
	// exits 0, where falling through to the whole line would have printed it.
	if flags.invert {
		return nil
	}
	for _, found := range expr.FindAllString(line, -1) {
		if err := printer.emit(name, lineNumber, found, true, withNames); err != nil {
			return err
		}
	}
	return nil
}

// grepDrainAfter writes the trailing context still owed when -m stopped the scan.
func grepDrainAfter(scanner *bufio.Scanner, printer *grepPrinter, name string, lineNumber *int, after int, withNames bool) error {
	for ; after > 0 && scanner.Scan(); after-- {
		*lineNumber++
		if err := printer.emit(name, *lineNumber, scanner.Text(), false, withNames); err != nil {
			return err
		}
	}
	return nil
}

// grepReportFile writes the per-file answer that -l, -L and -c give instead of
// lines.
func grepReportFile(printer *grepPrinter, flags grepFlags, name string, matched bool, count int, withNames bool) error {
	switch {
	case flags.filesOnly:
		if matched {
			_, err := fmt.Fprintln(printer.stdout, name)
			printer.wrote = true
			return err
		}
	case flags.withoutMatch:
		// -L is -l inverted: the files that did not match.
		if !matched {
			_, err := fmt.Fprintln(printer.stdout, name)
			printer.wrote = true
			return err
		}
	case flags.countOnly:
		prefix := ""
		if withNames {
			prefix = name + ":"
		}
		_, err := fmt.Fprintf(printer.stdout, "%s%d\n", prefix, count)
		printer.wrote = true
		return err
	}
	return nil
}
