package runtime

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// `mapfile` and `readarray`: a whole stream into an array in one go.
//
// One builtin under two names, which is what bash has -- `readarray` was added as the
// more memorable spelling and the two are the same thing. The registry lists both and the
// dispatch sends them to the same place, the way nano and micro share an editor.
//
// The alternative people write when this is missing is
//
//	while IFS= read -r line; do lines+=("$line"); done < file
//
// which is four decisions long -- the `IFS=` to keep leading blanks, the `-r` to keep
// backslashes, the quoting to keep the line whole -- and each of them is a thing to get
// wrong. That is the argument for the builtin: not that the loop is slow, but that the
// loop is a quiz.
//
// **The delimiter is kept by default and `-t` strips it**, which is the opposite of what
// people expect and is bash's rule. A line read without `-t` still ends in its newline.

// mapfileOptions is what the operands asked for.
type mapfileOptions struct {
	name string
	// strip is -t: remove the delimiter from each element.
	strip bool
	// count is -n: stop after this many lines. Zero means everything.
	count int
	// origin is -O: the index to begin assigning at, leaving earlier elements alone.
	origin int
	// skip is -s: discard this many lines before keeping any.
	skip int
	// delimiter is -d, a newline unless something else was named. An empty -d '' means
	// the NUL byte, which is how `find -print0` output is read.
	delimiter byte
	// descriptor is -u.
	descriptor int
}

func (r Runtime) mapfile(ctx context.Context, name string, args []string) int {
	options, err := parseMapfileOptions(args)
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "%s: %v\n", name, err)
		return 2
	}
	if r.isReadonly(options.name) {
		fmt.Fprintf(r.streams.Stderr, "%s: %s: readonly variable\n", name, options.name)
		return 1
	}
	input, err := r.fds.reader(options.descriptor)
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "%s: %v\n", name, err)
		return 1
	}
	lines, err := readMapfileLines(ctx, input, options)
	if err != nil {
		if ctx.Err() != nil {
			return contextStatus(ctx)
		}
		fmt.Fprintf(r.streams.Stderr, "%s: %v\n", name, err)
		return 1
	}
	// -O assigns *into* the array rather than replacing it, which is what makes it
	// useful for reading two files into one list. Without an origin the array is
	// replaced outright, so a second mapfile does not append to the first by accident.
	if options.origin > 0 {
		for offset, line := range lines {
			r.arrays.setElement(options.name, options.origin+offset, line)
		}
	} else {
		r.arrays.set(options.name, lines)
	}
	r.syncArrayScalar(options.name)
	r.markVarMutation(options.name)
	return 0
}

// readMapfileLines reads until the count is met or the stream ends.
func readMapfileLines(ctx context.Context, input io.Reader, options mapfileOptions) ([]string, error) {
	reader := bufio.NewReader(input)
	lines := make([]string, 0, 16)
	for read := 0; options.count == 0 || len(lines) < options.count; read++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		chunk, err := reader.ReadString(options.delimiter)
		if chunk == "" && err != nil {
			// End of stream with nothing pending. A read error that arrives *with*
			// data is not an error yet: the last line of a file with no terminator
			// comes back exactly this way, and dropping it would lose it.
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if read < options.skip {
			continue
		}
		if options.strip {
			chunk = strings.TrimSuffix(chunk, string(options.delimiter))
		}
		lines = append(lines, chunk)
		if err != nil {
			break
		}
	}
	return lines, nil
}

func parseMapfileOptions(args []string) (mapfileOptions, error) {
	options := mapfileOptions{delimiter: '\n', descriptor: 0}
	index := 0
	for ; index < len(args); index++ {
		arg := args[index]
		if arg == "--" {
			index++
			break
		}
		if len(arg) < 2 || arg[0] != '-' {
			break
		}
		letter := arg[1]
		// The valued options take the rest of the word or the next one, which is how
		// `-n3` and `-n 3` both work in bash.
		value := arg[2:]
		needsValue := strings.ContainsRune("ndOsu", rune(letter))
		if needsValue && value == "" {
			index++
			if index >= len(args) {
				return options, fmt.Errorf("-%c: option requires an argument", letter)
			}
			value = args[index]
		}
		switch letter {
		case 't':
			options.strip = true
		case 'd':
			// An empty delimiter is the NUL byte, which is what reads `find -print0`.
			if value == "" {
				options.delimiter = 0
				break
			}
			options.delimiter = value[0]
		case 'n', 'O', 's', 'u':
			number, err := strconv.Atoi(value)
			if err != nil || number < 0 {
				return options, fmt.Errorf("%s: invalid number", value)
			}
			switch letter {
			case 'n':
				options.count = number
			case 'O':
				options.origin = number
			case 's':
				options.skip = number
			case 'u':
				options.descriptor = number
			}
		case 'C', 'c':
			// bash runs a callback every N lines. Refused rather than ignored: a
			// script that passes -C expects its function to run, and silently not
			// running it is the quiet wrongness this project refuses elsewhere.
			return options, fmt.Errorf("-%c: callbacks are not implemented", letter)
		default:
			return options, fmt.Errorf("-%c: invalid option", letter)
		}
	}
	rest := args[index:]
	switch len(rest) {
	case 0:
		// bash defaults the name to MAPFILE, and so does this: `mapfile < f` then
		// `${MAPFILE[@]}` is a real idiom rather than a mistake.
		options.name = "MAPFILE"
	case 1:
		options.name = rest[0]
	default:
		return options, fmt.Errorf("too many arguments")
	}
	if options.name == "" {
		return options, fmt.Errorf("invalid array name")
	}
	return options, nil
}
