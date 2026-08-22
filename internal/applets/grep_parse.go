package applets

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// grep's arguments, once more than one option can carry a value.
//
// -m was handled by a special case that searched the word for an `m`, which does
// not generalise: -A, -B, -C, -e and -f all take values too, and -e and -f mean
// the first operand is no longer the pattern. So the letters are walked once,
// and a letter that takes a value takes the rest of its word or the next
// argument -- the same rule getopt uses and the one `-m1` already relied on.

// grepValuedLetters take an argument. busybox's usage text names exactly these:
// `[-m N] [-A|B|C N] { PATTERN | -e PATTERN... | -f FILE... }`.
const grepValuedLetters = "mABCef"

func grepArgs(ctx context.Context, args []string) (grepFlags, []string, error) {
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
			if err := parseGrepLongOption(arg); err != nil {
				return grepFlags{}, nil, err
			}
			index++
			continue
		}
		consumed, err := readGrepLetters(ctx, arg, args, index, &flags)
		if err != nil {
			return grepFlags{}, nil, err
		}
		index += consumed
	}

	// -e and -f already supplied the patterns; without them the first operand is
	// the pattern, which is the POSIX form.
	if len(flags.patterns) == 0 && !flags.patternsGiven {
		if index >= len(args) {
			// The shell prefixes the applet name; grep must not add its own.
			return grepFlags{}, nil, errors.New("missing pattern")
		}
		flags.patterns = []string{args[index]}
		index++
	}
	return flags, args[index:], nil
}

// parseGrepLongOption accepts --color and refuses the rest.
//
// Accepted and ignored, which is exactly what busybox does: its option table
// maps --color to a pseudo-flag with a NULL sink (findutils/grep.c:728) and
// nothing reads it. The option exists so that `alias grep='grep --color=auto'`,
// which everyone copies from a GNU system, does not break the shell it is pasted
// into. The value is still checked, unlike busybox, so a typo is refused rather
// than silently swallowed by an option that does nothing.
func parseGrepLongOption(arg string) error {
	name, value, present := strings.Cut(arg[2:], "=")
	if name != "color" {
		return fmt.Errorf("unsupported grep option: %s", arg)
	}
	_, err := parseColorWhen(value, present)
	return err
}

// readGrepLetters reads one argument's worth of clustered letters, reporting how
// many arguments it used. A valued letter ends the cluster, because the rest of
// the word is its value.
func readGrepLetters(ctx context.Context, arg string, args []string, index int, flags *grepFlags) (int, error) {
	for position := 1; position < len(arg); position++ {
		letter := arg[position]
		if !strings.ContainsRune(grepValuedLetters, rune(letter)) {
			if err := parseGrepFlags(string(letter), flags); err != nil {
				return 0, err
			}
			continue
		}
		value, consumed, err := grepOptionValue(arg, args, index, position, letter)
		if err != nil {
			return 0, err
		}
		if err := applyGrepValue(ctx, letter, value, flags); err != nil {
			return 0, err
		}
		return consumed, nil
	}
	return 1, nil
}

// grepOptionValue is the rest of the word, or the next argument when the word
// ends at the letter. The count of arguments used includes this one.
func grepOptionValue(arg string, args []string, index, position int, letter byte) (string, int, error) {
	if position+1 < len(arg) {
		return arg[position+1:], 1, nil
	}
	if index+1 >= len(args) {
		return "", 0, fmt.Errorf("option requires an argument -- '%c'", letter)
	}
	return args[index+1], 2, nil
}

func applyGrepValue(ctx context.Context, letter byte, value string, flags *grepFlags) error {
	switch letter {
	case 'm':
		count, err := parseGrepNumber(value)
		if err != nil {
			return err
		}
		flags.maxCount = count
	case 'A':
		count, err := parseGrepNumber(value)
		if err != nil {
			return err
		}
		flags.afterContext = count
	case 'B':
		count, err := parseGrepNumber(value)
		if err != nil {
			return err
		}
		flags.beforeContext = count
	case 'C':
		count, err := parseGrepNumber(value)
		if err != nil {
			return err
		}
		flags.afterContext, flags.beforeContext = count, count
	case 'e':
		// Recorded even when empty, because an empty pattern matches every line
		// and is a legitimate thing to ask for.
		flags.patterns = append(flags.patterns, value)
		flags.patternsGiven = true
	case 'f':
		patterns, err := readGrepPatternFile(ctx, value)
		if err != nil {
			return err
		}
		flags.patterns = append(flags.patterns, patterns...)
		// Set even for an empty file, so that no pattern means no match rather
		// than falling through and taking an operand as the pattern.
		flags.patternsGiven = true
	}
	return nil
}

// readGrepPatternFile reads one pattern per line.
//
// An empty file yields no patterns, and grep then matches nothing and exits 1 --
// the measured reference answer, and the opposite of what treating "no pattern"
// as "empty pattern" would give.
func readGrepPatternFile(ctx context.Context, path string) ([]string, error) {
	view := ProcessViewFromContext(ctx)
	file, err := openProcessTextInput(ctx, view, path)
	if err != nil {
		return nil, operandFailure(path, err)
	}
	data, readErr := io.ReadAll(file)
	closeErr := file.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		return nil, operandFailure(path, err)
	}
	text := strings.TrimSuffix(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	if text == "" {
		return nil, nil
	}
	return strings.Split(text, "\n"), nil
}

// parseGrepNumber reads a count, with busybox's wording for a bad one:
// `grep -A x` answers `grep: invalid number 'x'`. Measured 2026-08-22.
func parseGrepNumber(value string) (int, error) {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("invalid number '%s'", value)
	}
	return parsed, nil
}
