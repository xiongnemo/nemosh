package runtime

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// The options `read` takes.
//
// Until now it took none, and that was not a missing feature so much as a silent
// wrong answer: `read -r line` assigned the line to a variable literally named
// `-r` and left `line` empty, so `while read -r line` -- the most common way any
// shell script consumes input -- looped the right number of times and handed back
// nothing. Nothing failed, which is the worst shape a defect can have.
//
// Every behaviour below is measured against bash on the machine this was written
// on; the cases are recorded in builtin_read_test.go so the measurements are
// checkable rather than remembered.
type readOptions struct {
	// raw is -r: a backslash is data rather than an escape.
	raw bool
	// silent is -s: do not echo what is typed, for a password.
	silent bool
	// prompt is -p, written before reading and only to a terminal.
	prompt string
	// arrayName is -a: the fields go to an array instead of to names.
	arrayName string
	// delimiter is -d, `\n` unless asked otherwise. `-d ''` means NUL, which is
	// what pairs with `find -print0`.
	delimiter byte
	// limit is -n or -N, and -1 for no limit.
	limit int
	// exactly is -N rather than -n: the delimiter no longer stops the read, so
	// `-N 5` over "ab\ncd" reads all five bytes including the newline.
	exactly bool
	// descriptor is -u, 0 unless asked otherwise.
	descriptor int
	timeout    time.Duration
	hasTimeout bool
	names      []string
}

// optionsTakingValue are the letters that consume an argument, either attached
// (`-n3`) or as the next word (`-n 3`). bash allows both and `-rp 'x: '` is the
// common spelling of the pair, so clustering has to work.
const optionsTakingValue = "pandNut"

func parseReadOptions(args []string) (readOptions, error) {
	options := readOptions{delimiter: '\n', limit: -1}
	index := 0
	for ; index < len(args); index++ {
		argument := args[index]
		if argument == "--" {
			index++
			break
		}
		// A lone `-` is not an option, and neither is anything that does not
		// start with one: the rest are variable names.
		if len(argument) < 2 || argument[0] != '-' {
			break
		}
		cluster := argument[1:]
		for position := 0; position < len(cluster); position++ {
			letter := cluster[position]
			value := ""
			if strings.IndexByte(optionsTakingValue, letter) >= 0 {
				if position+1 < len(cluster) {
					value = cluster[position+1:]
					position = len(cluster)
				} else {
					index++
					if index >= len(args) {
						return readOptions{}, fmt.Errorf("-%c: option requires an argument", letter)
					}
					value = args[index]
				}
			}
			if err := options.apply(letter, value); err != nil {
				return readOptions{}, err
			}
		}
	}
	options.names = args[index:]
	if err := options.validate(); err != nil {
		return readOptions{}, err
	}
	return options, nil
}

func (o *readOptions) apply(letter byte, value string) error {
	switch letter {
	case 'r':
		o.raw = true
	case 's':
		o.silent = true
	case 'p':
		o.prompt = value
	case 'a':
		o.arrayName = value
	case 'd':
		// The empty argument is NUL rather than "no delimiter": bash reads
		// `-d ''` that way, and it is how a NUL-separated list is consumed.
		if value == "" {
			o.delimiter = 0
		} else {
			o.delimiter = value[0]
		}
	case 'n', 'N':
		count, err := strconv.Atoi(value)
		if err != nil || count < 0 {
			return fmt.Errorf("%s: invalid number", value)
		}
		o.limit, o.exactly = count, letter == 'N'
	case 'u':
		descriptor, err := strconv.Atoi(value)
		if err != nil || descriptor < 0 {
			return fmt.Errorf("%s: invalid file descriptor", value)
		}
		o.descriptor = descriptor
	case 't':
		// Fractional seconds are allowed, because `read -t 0.1` is how a script
		// polls without spinning.
		seconds, err := strconv.ParseFloat(value, 64)
		if err != nil || seconds < 0 {
			return fmt.Errorf("%s: invalid timeout specification", value)
		}
		o.timeout, o.hasTimeout = time.Duration(seconds*float64(time.Second)), true
	default:
		return fmt.Errorf("-%c: not an option this build has; it takes -r -s -p -a -d -n -N -u -t", letter)
	}
	return nil
}

func (o *readOptions) validate() error {
	if o.arrayName != "" && len(o.names) > 0 {
		// bash refuses the combination too. Splitting the same line into an
		// array and into names would have to pick one, and either choice makes
		// the other half of the command a lie.
		return fmt.Errorf("-a and variable names cannot both be given")
	}
	for _, name := range o.names {
		if !isValidVariableName(name) {
			return fmt.Errorf("%s: not a valid variable name", name)
		}
	}
	if o.arrayName != "" && !isValidVariableName(o.arrayName) {
		return fmt.Errorf("%s: not a valid array name", o.arrayName)
	}
	return nil
}
