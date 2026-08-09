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
	return simpleApplet{name: "grep", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, _ io.Writer) error {
		return grepStatus(runGrep(ctx, args, stdin, stdout))
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

func runGrep(ctx context.Context, args []string, stdin io.Reader, stdout io.Writer) error {
	options, pattern, paths, err := grepArgs(args)
	if err != nil {
		return err
	}
	expr, err := regexp.Compile(options.patternPrefix() + pattern)
	if err != nil {
		return err
	}
	matched := false
	if len(paths) == 0 {
		matched, err = grepReader(stdout, expr, options, stdin)
		if err != nil {
			return err
		}
		if !matched {
			return ErrExitFalse
		}
		return nil
	}
	view := ProcessViewFromContext(ctx)
	for _, path := range paths {
		file, err := OpenProcessInput(ctx, view, path)
		if err != nil {
			return operandFailure(path, err)
		}
		fileMatched, grepErr := grepReader(stdout, expr, options, file)
		closeErr := file.Close()
		if err := errors.Join(grepErr, closeErr); err != nil {
			return err
		}
		matched = matched || fileMatched
	}
	if !matched {
		return ErrExitFalse
	}
	return nil
}

type grepOptions struct {
	ignoreCase bool
	invert     bool
	lineNumber bool
}

func (o grepOptions) patternPrefix() string {
	if o.ignoreCase {
		return "(?i)"
	}
	return ""
}

func grepArgs(args []string) (grepOptions, string, []string, error) {
	var options grepOptions
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
		// ls had the same defect and the same fix; grep never got it.
		if strings.HasPrefix(arg, "--") {
			name, value, present := strings.Cut(arg[2:], "=")
			if name != "color" {
				return grepOptions{}, "", nil, fmt.Errorf("unsupported grep option: %s", arg)
			}
			// Accepted and ignored, which is exactly what busybox does: its
			// option table maps --color to a pseudo-flag with a NULL sink
			// (findutils/grep.c:728) and nothing reads it. The option exists so
			// that `alias grep='grep --color=auto'`, which everyone copies from
			// a GNU system, does not break the shell it is pasted into.
			//
			// The value is still checked, unlike busybox, so a typo is refused
			// rather than silently swallowed by an option that does nothing.
			if _, err := parseColorWhen(value, present); err != nil {
				return grepOptions{}, "", nil, err
			}
			index++
			continue
		}
		for _, flag := range arg[1:] {
			switch flag {
			case 'i':
				options.ignoreCase = true
			case 'v':
				options.invert = true
			case 'n':
				options.lineNumber = true
			default:
				return grepOptions{}, "", nil, fmt.Errorf("unsupported grep option: -%c", flag)
			}
		}
		index++
	}
	if index >= len(args) {
		// The shell prefixes the applet name; grep must not add its own.
		return grepOptions{}, "", nil, errors.New("missing pattern")
	}
	return options, args[index], args[index+1:], nil
}

func grepReader(stdout io.Writer, expr *regexp.Regexp, options grepOptions, input io.Reader) (bool, error) {
	scanner := bufio.NewScanner(input)
	matched := false
	lineNumber := 1
	for scanner.Scan() {
		line := scanner.Text()
		lineMatches := expr.MatchString(line)
		if options.invert {
			lineMatches = !lineMatches
		}
		if lineMatches {
			matched = true
			if options.lineNumber {
				if _, err := fmt.Fprintf(stdout, "%d:", lineNumber); err != nil {
					return false, err
				}
			}
			if _, err := fmt.Fprintln(stdout, line); err != nil {
				return false, err
			}
		}
		lineNumber++
	}
	return matched, scanner.Err()
}
