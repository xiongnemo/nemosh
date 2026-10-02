package applets

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
)

// newJoinApplet is busybox's join (coreutils/join.c): an equality join of two files sorted on
// their join fields, read a set at a time from each -- the lines in a row that share a key. Equal
// keys pair every line of one set with every line of the other; a key only one file has is that
// file's unpaired set, which -a prints as well and -v prints alone.
//
// It was a loop over every pair of lines, which paired lines of unsorted files that busybox never
// compares, and it took -t and did nothing with it: `join -t: f g` split on blanks and joined
// nothing. -a -v -e and -o were refused. Each answer here is busybox-w32's, measured, but for an
// -o list of nothing but 0s: busybox's parsejformat allocates one int too few for it and reads
// past the list, so `-o 0` printed the key six times. It prints it once here, as the list says.
//
// -j sets both join fields, which is GNU's shorthand. busybox has no -j; refusing a standard
// option would be the larger divergence.
func newJoinApplet() Applet {
	return simpleApplet{name: "join", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, _ io.Writer) error {
		options, operands, err := parseAppletOptions(ctx, args, "", "aveot12j")
		if err != nil {
			return err
		}
		switch {
		case len(operands) < 2:
			return missingOperand()
		case len(operands) > 2:
			return fmt.Errorf("extra operand '%s'", operands[2])
		}
		spec, err := parseJoinSpec(options)
		if err != nil {
			return err
		}
		if operands[0] == "-" && operands[1] == "-" {
			return errors.New("cannot combine stdin with itself")
		}
		view := ProcessViewFromContext(ctx)
		var files [2]*joinFile
		for index, operand := range operands {
			input, err := OpenProcessOperand(ctx, view, operand, stdin)
			if err != nil {
				return operandFailure(operand, err)
			}
			defer input.Close()
			files[index] = newJoinFile(input, spec.fields[index], spec.separator)
		}
		out := bufio.NewWriter(stdout)
		return errors.Join(spec.merge(out, files), out.Flush())
	}}
}

// joinSpec is what join's options asked for.
type joinSpec struct {
	// separator splits and joins fields: -t's, or 0 for runs of blanks in and one blank out.
	separator byte
	// fields is each file's join field, counted from 0.
	fields [2]int
	// unpaired is which files' unpaired sets are printed, and paired whether the matched ones
	// are, which -v says not.
	unpaired [2]bool
	paired   bool
	// empty is -e's text, printed for a field that is empty or not there.
	empty string
	// listed says -o was given, and format is its list, which may be empty.
	listed bool
	format []joinOutputField
}

// joinOutputField is one of -o's entries: field of file 1 or 2, or with file 0 the join field.
type joinOutputField struct{ file, field int }

// parseJoinSpec reads join's options, refusing a bad one in busybox's words.
func parseJoinSpec(options appletOptions) (joinSpec, error) {
	spec := joinSpec{paired: !options.has('v'), empty: options.value('e')}
	if options.has('a') && options.has('v') {
		return spec, errors.New("-a and -v are exclusive")
	}
	for _, which := range slices.Concat(options.all('a'), options.all('v')) {
		if which != "1" && which != "2" {
			return spec, errors.New("-a and -v take either 1 or 2")
		}
		spec.unpaired[which[0]-'1'] = true
	}
	for _, letter := range []byte{'j', '1', '2'} {
		if !options.has(letter) {
			continue
		}
		field, err := positiveNumber(options.value(letter))
		if err != nil {
			return spec, err
		}
		if field == 0 {
			return spec, errors.New("field 0 does not exist")
		}
		if letter == 'j' {
			spec.fields = [2]int{field - 1, field - 1}
		} else {
			spec.fields[letter-'1'] = field - 1
		}
	}
	if options.has('t') {
		if len(options.value('t')) != 1 {
			return spec, errors.New("separators are single characters")
		}
		spec.separator = options.value('t')[0]
	}
	if options.has('o') {
		format, err := parseJoinFormat(options.value('o'))
		if err != nil {
			return spec, err
		}
		spec.listed, spec.format = true, format
	}
	return spec, nil
}

// parseJoinFormat reads -o's list as busybox's parsejformat does: entries between blanks and
// commas, each 0 for the join field or FILE.FIELD with FILE 1 or 2.
func parseJoinFormat(list string) ([]joinOutputField, error) {
	var format []joinOutputField
	for _, entry := range strings.FieldsFunc(list, func(char rune) bool { return strings.ContainsRune(" \t,", char) }) {
		switch {
		case entry == "0":
			format = append(format, joinOutputField{})
		case len(entry) >= 3 && (entry[0] == '1' || entry[0] == '2') && entry[1] == '.':
			if len(entry) > 21 {
				return nil, errors.New("field specifier too large")
			}
			field, err := positiveNumber(entry[2:])
			if err != nil {
				return nil, err
			}
			if field == 0 {
				return nil, errors.New("field number cannot be 0")
			}
			format = append(format, joinOutputField{file: int(entry[0] - '0'), field: field - 1})
		default:
			return nil, errors.New("field specifier must be 0, 1.x or 2.x")
		}
	}
	return format, nil
}
