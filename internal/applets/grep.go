package applets

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
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
	flags, pattern, paths, err := grepArgs(args)
	if err != nil {
		return err
	}
	expr, err := flags.compile(pattern)
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		matched, err := grepOne(grepTarget{opener: readerTarget(stdin)}, expr, flags, false, stdout)
		if err != nil {
			return err
		}
		if !matched {
			return ErrExitFalse
		}
		return nil
	}
	targets, err := grepTargets(ctx, flags, paths, stderr)
	if err != nil {
		return err
	}
	withNames := flags.showNames(len(targets))
	matched := false
	for _, target := range targets {
		found, err := grepOne(target, expr, flags, withNames, stdout)
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
func grepOne(target grepTarget, expr *regexp.Regexp, flags grepFlags, withNames bool, stdout io.Writer) (bool, error) {
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
	matched, err := grepScan(input, expr, flags, withNames, target.name, stdout)
	return matched, errors.Join(err, input.Close())
}

func grepScan(input io.Reader, expr *regexp.Regexp, flags grepFlags, withNames bool, name string, stdout io.Writer) (bool, error) {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 0, 64*1024), maxTextLine)
	matched, count, lineNumber := false, 0, 0
	prefix := ""
	if withNames {
		prefix = name + ":"
	}
	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()
		hit := expr.MatchString(line)
		if flags.invert {
			hit = !hit
		}
		if !hit {
			continue
		}
		matched = true
		count++
		switch {
		case flags.quiet:
			// Nothing printed, and the caller stops.
			return true, nil
		case flags.countOnly, flags.filesOnly:
			// Both are reported after the file rather than per line.
		case flags.onlyMatching:
			// Each match on its own line, which is what makes -o useful for
			// pulling values out.
			//
			// With -v there is nothing to print: the line was selected for *not*
			// matching, so it has no matched text. Measured -- GNU prints nothing
			// and exits 0, where falling through to the whole line would have
			// printed it.
			if flags.invert {
				break
			}
			for _, found := range expr.FindAllString(line, -1) {
				if err := writeGrepLine(stdout, prefix, flags, lineNumber, found); err != nil {
					return matched, err
				}
			}
		default:
			if err := writeGrepLine(stdout, prefix, flags, lineNumber, line); err != nil {
				return matched, err
			}
		}
		if flags.maxCount > 0 && count >= flags.maxCount {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return matched, err
	}
	switch {
	case flags.filesOnly:
		if matched {
			_, err := fmt.Fprintln(stdout, name)
			return matched, err
		}
	case flags.countOnly:
		_, err := fmt.Fprintf(stdout, "%s%d\n", prefix, count)
		return matched, err
	}
	return matched, nil
}

func writeGrepLine(stdout io.Writer, prefix string, flags grepFlags, lineNumber int, text string) error {
	if flags.lineNumber {
		_, err := fmt.Fprintf(stdout, "%s%d:%s\n", prefix, lineNumber, text)
		return err
	}
	_, err := fmt.Fprintf(stdout, "%s%s\n", prefix, text)
	return err
}

func grepArgs(args []string) (grepFlags, string, []string, error) {
	flags := grepFlags{}
	index := 0
	for index < len(args) {
		arg := args[index]
		if arg == "--" {
			index++
			break
		}
		if len(arg) <= 1 || arg[0] != '-' {
			break
		}
		// A long option is one word, matched whole rather than letter by letter.
		// Without this `--color=auto` was read as the flags `-`, `-c`, `-o`, ...
		// and refused as the bare `-` it began with, so the diagnostic said
		// `unsupported grep option: --` and named nothing the user had typed.
		if strings.HasPrefix(arg, "--") {
			name, value, present := strings.Cut(arg[2:], "=")
			if name != "color" {
				return grepFlags{}, "", nil, fmt.Errorf("unsupported grep option: %s", arg)
			}
			// Accepted and ignored, which is exactly what busybox does: its option
			// table maps --color to a pseudo-flag with a NULL sink
			// (findutils/grep.c:728) and nothing reads it. The option exists so
			// that `alias grep='grep --color=auto'`, which everyone copies from a
			// GNU system, does not break the shell it is pasted into.
			//
			// The value is still checked, unlike busybox, so a typo is refused
			// rather than silently swallowed by an option that does nothing.
			if _, err := parseColorWhen(value, present); err != nil {
				return grepFlags{}, "", nil, err
			}
			index++
			continue
		}
		// -m takes a value, either attached or as the next word, so it is settled
		// before the remaining letters are walked.
		letters, consumed, err := grepValueOption(args, index, &flags)
		if err != nil {
			return grepFlags{}, "", nil, err
		}
		if consumed > 0 {
			index += consumed
			if err := parseGrepFlags(letters, &flags); err != nil {
				return grepFlags{}, "", nil, err
			}
			continue
		}
		if err := parseGrepFlags(arg[1:], &flags); err != nil {
			return grepFlags{}, "", nil, err
		}
		index++
	}
	if index >= len(args) {
		// The shell prefixes the applet name; grep must not add its own.
		return grepFlags{}, "", nil, errors.New("missing pattern")
	}
	return flags, args[index], args[index+1:], nil
}

// grepValueOption handles -m, reporting how many arguments it consumed and any
// letters that preceded it in the same word.
func grepValueOption(args []string, index int, flags *grepFlags) (string, int, error) {
	arg := args[index]
	position := strings.IndexByte(arg, 'm')
	if position < 1 {
		return "", 0, nil
	}
	before := arg[1:position]
	rest := arg[position+1:]
	if rest != "" {
		count, err := parseMaxCount(rest)
		if err != nil {
			return "", 0, err
		}
		flags.maxCount = count
		return before, 1, nil
	}
	if index+1 >= len(args) {
		return "", 0, fmt.Errorf("option requires an argument -- 'm'")
	}
	count, err := parseMaxCount(args[index+1])
	if err != nil {
		return "", 0, err
	}
	flags.maxCount = count
	return before, 2, nil
}
