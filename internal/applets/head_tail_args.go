package applets

import (
	"fmt"
	"strconv"
)

// head and tail take their options with the value attached or detached, and they
// head their output when there is more than one file to tell apart.
//
// Both were missing, and the first was missing in the most confusing way
// available: `head -2` worked, `head -n 2` worked, and `head -n2` was refused.
// A user cannot predict that. The cause was that this package has two option
// parsers -- parseAppletOptions is a real getopt and takes an attached value
// like `-m755`, while streamOptionsAndOperands matches whole strings against a
// whitelist -- and head and tail were built on the second, which cannot express
// "this letter carries a value".
//
// A shared getopt is not the fix either, because neither the bare `-N` form nor
// the signed counts `+2` and `-2` are getopt shapes. So this is head and tail's
// own reader, which is what busybox has too (coreutils/head.c reads its own).

// headerMode is whether a file's name is printed above its lines.
type headerMode byte

const (
	// headersWhenMany is the default: a header per file when more than one file
	// was named, and none for a single file or for stdin. Without it,
	// `head *.log` is lines with no way to tell which file they came from --
	// which is what this did before 2026-08-22.
	headersWhenMany headerMode = iota
	headersNever               // -q
	headersAlways              // -v
)

// headTailArgs reads the options of head or tail.
//
// allowBytes is whether -c is offered; both have it now, and the asymmetry that
// existed while only head did was documented as deliberate for exactly as long
// as it took to implement the other half.
func headTailArgs(applet string, args []string, defaultCount int, allowBytes bool) (countSpec, headerMode, []string, error) {
	spec := countSpec{count: defaultCount}
	headers := headersWhenMany
	index := 0
	for ; index < len(args); index++ {
		arg := args[index]
		if arg == "--" {
			index++
			break
		}
		// A lone `-` is stdin, which is an operand and not an option.
		if len(arg) < 2 || arg[0] != '-' {
			break
		}
		// `-3` is the obsolete form POSIX still lists, and it is what everybody
		// types. busybox takes it; refusing it made `head -3` an error in a shell
		// whose whole point is that the muscle memory works.
		//
		// It is tried before the letters so that `-2` is a count rather than an
		// unknown option, and a trailing letter makes it a bad count rather than
		// a cluster: busybox says `invalid number '2n'` for `head -2n`.
		if digits, ok, err := bareCountOption(arg); err != nil {
			return countSpec{}, headers, nil, err
		} else if ok {
			spec = countSpec{count: digits}
			continue
		}
		consumed, err := readHeadTailFlags(arg, args, index, allowBytes, &spec, &headers)
		if err != nil {
			return countSpec{}, headers, nil, err
		}
		if consumed < 0 {
			break
		}
		index += consumed
	}

	// Whatever is left has to be operands, and an option among them is still
	// refused by name rather than opened as a file -- `tail -f x` reports -f and
	// not a missing file.
	paths, err := streamOperands(applet, args[index:], headTailSupported(allowBytes)...)
	if err != nil {
		return countSpec{}, headers, nil, err
	}
	return spec, headers, paths, nil
}

func headTailSupported(allowBytes bool) []string {
	supported := []string{"-n", "-q", "-v"}
	if allowBytes {
		supported = append(supported, "-c")
	}
	return supported
}

// readHeadTailFlags reads one argument's worth of clustered letters, reporting
// how many extra arguments it took. A negative return means the argument was not
// an option at all.
func readHeadTailFlags(arg string, args []string, index int, allowBytes bool, spec *countSpec, headers *headerMode) (int, error) {
	for position := 1; position < len(arg); position++ {
		switch letter := arg[position]; letter {
		case 'q':
			*headers = headersNever
		case 'v':
			*headers = headersAlways
		case 'n', 'c':
			if letter == 'c' && !allowBytes {
				// Left for streamOperands, so an option this build lacks is
				// refused by name in one place rather than two.
				return -1, nil
			}
			// A valued letter takes the rest of the word, so `-qn1` works and
			// `-n1` works, and ends the cluster either way.
			value, consumed, err := headTailCountValue(arg, args, index, position, letter)
			if err != nil {
				return 0, err
			}
			parsed, err := parseCountSpec(value, letter == 'c')
			if err != nil {
				return 0, err
			}
			// -n and -c write the same count, so the last one given wins, which
			// is what busybox does.
			*spec = parsed
			return consumed, nil
		default:
			// Not an option this build has. Left for streamOperands to refuse by
			// name, so there is one message for the case rather than two.
			return -1, nil
		}
	}
	return 0, nil
}

// headTailCountValue is the rest of the word, or the next argument when the word
// ends at the letter.
func headTailCountValue(arg string, args []string, index, position int, letter byte) (string, int, error) {
	if position+1 < len(arg) {
		return arg[position+1:], 0, nil
	}
	if index+1 >= len(args) {
		return "", 0, fmt.Errorf("-%c: requires a count", letter)
	}
	return args[index+1], 1, nil
}

// bareCountOption reads the `-3` form: a dash followed by digits and nothing
// else.
//
// A trailing letter is an error rather than a cluster, because busybox reads the
// whole thing as the number and says so -- `head -2n` answers
// `invalid number '2n'`. Reporting it here keeps that message, where falling
// through to the letters would have produced "unsupported option -n" for an
// option that exists.
func bareCountOption(arg string) (int, bool, error) {
	if len(arg) < 2 || arg[0] != '-' {
		return 0, false, nil
	}
	rest := arg[1:]
	if rest[0] < '0' || rest[0] > '9' {
		return 0, false, nil
	}
	count, err := strconv.Atoi(rest)
	if err != nil || count < 0 {
		// Single quotes, matching busybox: `head -2n` answers
		// `head: invalid number '2n'`.
		return 0, false, fmt.Errorf("invalid number '%s'", rest)
	}
	return count, true, nil
}

// headTailHeader is the `==> name <==` line, with the blank line that separates
// one file's block from the next.
//
// The name is the operand as spelled rather than a resolved path, so a caller
// that passed `./a.txt` sees `./a.txt` back.
func headTailHeader(name string, first bool) string {
	if first {
		return fmt.Sprintf("==> %s <==\n", name)
	}
	return fmt.Sprintf("\n==> %s <==\n", name)
}

// wantsHeader answers for one run: how many files were named decides the
// default, and -q and -v override it.
func wantsHeader(headers headerMode, fileCount int) bool {
	switch headers {
	case headersNever:
		return false
	case headersAlways:
		return true
	}
	return fileCount > 1
}
