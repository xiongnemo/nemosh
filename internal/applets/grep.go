package applets

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
)

func newGrepApplet() Applet {
	return simpleApplet{name: "grep", run: func(args []string, stdin io.Reader, stdout, _ io.Writer) error {
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
		for _, path := range paths {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			fileMatched, grepErr := grepReader(stdout, expr, options, file)
			closeErr := file.Close()
			if grepErr != nil {
				return grepErr
			}
			if closeErr != nil {
				return closeErr
			}
			matched = matched || fileMatched
		}
		if !matched {
			return ErrExitFalse
		}
		return nil
	}}
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
	for index < len(args) && len(args[index]) > 1 && args[index][0] == '-' {
		for _, flag := range args[index][1:] {
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
		return grepOptions{}, "", nil, fmt.Errorf("grep: missing pattern")
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
