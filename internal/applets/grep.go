package applets

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"regexp"
)

func newGrepApplet() Applet {
	return simpleApplet{name: "grep", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
		return grepStatus(runGrep(ctx, args, stdin, stdout, stderr))
	}}
}

// egrep and fgrep are grep -E and grep -F under names of their own, as busybox's grep_main
// has them: it reads the option off the name it was run as.
func newEgrepApplet() Applet { return grepAs("egrep", "-E") }
func newFgrepApplet() Applet { return grepAs("fgrep", "-F") }

func grepAs(name, option string) Applet {
	return simpleApplet{name: name, runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
		return grepStatus(runGrep(ctx, append([]string{option}, args...), stdin, stdout, stderr))
	}}
}

// grep_main raises xfunc_error_retval to 2 before it parses anything, so that 1
// stays reserved for "no match" (findutils/grep.c:718). Every other failure --
// usage, a bad pattern, an unreadable operand -- carries 2 out with it.
func grepStatus(err error) error {
	if err == nil || errors.Is(err, ErrExitFalse) {
		return err
	}
	// A bare status is already the answer: 2 after an operand that could not be read.
	if _, isStatus := StatusCode(err); isStatus {
		if _, carriesMessage := StatusMessage(err); !carriesMessage {
			return err
		}
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return ExitStatusMessage(2, err)
}

func runGrep(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	flags, paths, err := grepArgs(ctx, args, stdin)
	if err != nil {
		return err
	}
	expr, err := flags.compile()
	if err != nil {
		// A pattern regcomp refuses is status 2, as busybox's grep dies with it.
		return ExitStatusMessage(2, err)
	}
	// One printer for the whole run, because the group separator spans files: a
	// `--` belongs between the last group of one file and the first of the next.
	printer := newGrepPrinter(stdout, stderr, flags, expr)
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
	withNames := flags.showNames(len(paths), targets)
	matched, unread := false, false
	for _, target := range targets {
		found, err := grepOne(target, expr, flags, withNames, printer)
		// Of several operands, one that cannot be opened is named and the rest searched; see
		// operand_reporter.go. The status is 2 at the end, as busybox's is, whatever matched.
		var unreadable unreadableOperand
		if errors.As(err, &unreadable) && (unreadable.silent || len(targets) > 1 && reportOperand(ctx, unreadable.error)) {
			unread = true
			continue
		}
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
	if unread {
		return ExitStatus(2)
	}
	return grepMatchStatus(flags, matched, printer)
}

// unreadableOperand is an operand grep could not open, which it goes on past when there are
// others to search. silent is -s's: nothing is said of it, and the status is still 2, as
// busybox's is. -s answered 0, as though the operand had been searched.
type unreadableOperand struct {
	error
	silent bool
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
		return false, unreadableOperand{error: operandFailure(target.name, err), silent: flags.noMessages}
	}
	// Closed explicitly and joined rather than deferred: a close error is a real
	// failure -- a truncated read on a device -- and swallowing it would report a
	// clean search over a file that was not fully read. A test pins this.
	matched, err := grepScan(input, expr, flags, withNames, target.name, printer)
	return matched, errors.Join(err, input.Close())
}

func grepScan(input io.Reader, expr *regexp.Regexp, flags grepFlags, withNames bool, name string, printer *grepPrinter) (bool, error) {
	// A binary file is said to match rather than printed, or under -I read as holding no
	// match; see grep_binary.go.
	input, binary := probeBinary(input, flags.binary)
	if binary && flags.binary == grepBinarySkip {
		return false, nil
	}
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 0, 64*1024), maxTextLine)
	matched, count, lineNumber := false, 0, 0
	// before holds the lines that were read and not printed, because -B cannot
	// know a line is context until something below it matches. after counts the
	// trailing lines still owed to the last match.
	before := &grepRing{limit: flags.beforeContext}
	after := 0
	// -m0 reads no line and prints nothing, and the status is 1, as GNU's grep has it. busybox's
	// prints no line either, but counts the one it stopped at, so its -c says 1 and its status
	// is 0. -m0 printed every match.
	if flags.limited && flags.maxCount == 0 {
		return false, nil
	}
	for scanner.Scan() {
		lineNumber++
		// The scanner's own bytes, matched where they lie: a line is made a string only to be
		// printed or held for -B. It was made one to be matched, an allocation a line.
		line := scanner.Bytes()
		if !binary && flags.binary != grepBinaryText && bytes.IndexByte(line, 0) >= 0 {
			if binary = true; flags.binary == grepBinarySkip {
				break
			}
		}
		hit := expr.Match(line)
		if flags.invert {
			hit = !hit
		}
		if hit && binary && flags.binary != grepBinaryText && flags.reportsPerLine() {
			return true, printer.binaryMatches(name)
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
				if err := printer.emit(name, lineNumber, string(line), false, withNames); err != nil {
					return matched, err
				}
				continue
			}
			if before.limit > 0 {
				before.push(lineNumber, string(line))
			}
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
			if err := grepEmitMatch(printer, expr, flags, name, lineNumber, string(line), withNames); err != nil {
				return matched, err
			}
			after = flags.afterContext
		}
		if flags.limited && count >= flags.maxCount {
			// The trailing context of the last match is still owed, which is what
			// busybox does: `grep -A1 -m1 M` prints the match and the line after
			// it. Measured. Then -c and -l answer for what was read, as without -m:
			// returning here left `grep -c -m2` and `grep -l -m1` saying nothing.
			// See grepDrainAfter for where the context ends.
			if err := grepDrainAfter(scanner, printer, expr, flags.invert, name, &lineNumber, after, withNames); err != nil {
				return matched, err
			}
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return matched, err
	}
	return matched, grepReportFile(printer, flags, name, matched, count, withNames)
}

// grepDrainAfter writes the trailing context still owed when -m stopped the scan.
// It ends at a line that would have been selected, as busybox's and GNU's grep end it there:
// `grep -A2 -m1 a` over banana, apple and cherry is banana alone. It went on through apple.
func grepDrainAfter(scanner *bufio.Scanner, printer *grepPrinter, expr *regexp.Regexp, invert bool, name string, lineNumber *int, after int, withNames bool) error {
	for ; after > 0 && scanner.Scan(); after-- {
		if expr.MatchString(scanner.Text()) != invert {
			return nil
		}
		*lineNumber++
		if err := printer.emit(name, *lineNumber, scanner.Text(), false, withNames); err != nil {
			return err
		}
	}
	return nil
}
